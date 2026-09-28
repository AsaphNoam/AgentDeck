package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentdeck/agentdeck/internal/config"
	"github.com/agentdeck/agentdeck/internal/remote"
	"github.com/agentdeck/agentdeck/internal/runtime"
	"github.com/agentdeck/agentdeck/internal/state"
)

// Push timing and bounds (TS-13.R11, INV §16). Variables so tests can shorten
// them.
var (
	pushWindow      = 10 * time.Second
	pushDebounce    = time.Second
	pushRecheck     = 30 * time.Second
	pushRetryDelays = []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}
)

const pushQueueSize = 64

// pushNote is one notification before encryption: only FS-20.R19's fields.
type pushNote struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Tag   string `json:"tag"`
	URL   string `json:"url"`
}

// pushSendFunc is the delivery seam; tests replace it so no suite contacts a
// real push service (TS-06.R27).
type pushSendFunc func(ctx context.Context, keys remote.VAPIDKeys, sub remote.PushSubscription, payload []byte, topic string) (int, error)

func (s *Server) vapidKeys() (remote.VAPIDKeys, error) {
	return remote.LoadOrCreateVAPID(filepath.Join(s.configStore.Home(), "remote", "vapid.json"))
}

// handleGetSelf implements tailnet-only GET /api/remote/self: what the phone
// needs to show its own settings and subscribe to push.
func (s *Server) handleGetSelf(w http.ResponseWriter, r *http.Request) {
	device := remoteFrom(r.Context()).device
	keys, err := s.vapidKeys()
	if err != nil {
		s.log.Error("remote: vapid keys", "err", err)
		writeRemoteError(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}
	notifications := "off"
	switch {
	case device.PushEndpoint != "" && device.PushState == "expired":
		notifications = "expired"
	case device.PushEndpoint != "" && device.PushEnabled:
		notifications = "on"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": device.ID, "name": device.Name, "notifications": notifications, "vapid_public_key": keys.Public,
	})
}

type pushSubscriptionBody struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// handlePutSelfPush implements tailnet-only PUT /api/remote/self/push.
func (s *Server) handlePutSelfPush(w http.ResponseWriter, r *http.Request) {
	var body pushSubscriptionBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&body); err != nil {
		writeAPIError(w, apiError(runtime.CodeInvalidField, "malformed JSON"))
		return
	}
	if !remote.ValidPushEndpoint(body.Endpoint) || len(body.Endpoint) > 2048 || body.Keys.P256dh == "" || body.Keys.Auth == "" ||
		len(body.Keys.P256dh) > 256 || len(body.Keys.Auth) > 256 {
		writeAPIError(w, apiError(runtime.CodeInvalidField, "this push subscription is not from a supported browser push service"))
		return
	}
	if err := s.stateStore.SetRemoteDevicePush(remoteFrom(r.Context()).device.ID, body.Endpoint, body.Keys.P256dh, body.Keys.Auth); err != nil {
		s.writeRemoteDeviceError(w, err)
		return
	}
	s.publishRemote()
	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteSelfPush implements tailnet-only DELETE /api/remote/self/push.
func (s *Server) handleDeleteSelfPush(w http.ResponseWriter, r *http.Request) {
	if err := s.stateStore.ClearRemoteDevicePush(remoteFrom(r.Context()).device.ID); err != nil {
		s.writeRemoteDeviceError(w, err)
		return
	}
	s.publishRemote()
	w.WriteHeader(http.StatusNoContent)
}

// attentionKey identifies one Needs-you entry across evaluations.
func attentionKey(it attentionItem) string { return it.Kind + "\x00" + it.ID + "\x00" + it.Reason }

// pushTag groups a run's attention by run and everything else by project, so
// the phone replaces rather than stacks (FS-20.R20).
func pushTag(it attentionItem) string {
	if it.Kind == "run" {
		return "run:" + it.ID
	}
	return "project:" + it.Project
}

// pushMuted applies the desktop per-type mutes (FS-02.R24) to an item.
func pushMuted(it attentionItem, muted map[string]bool) bool {
	switch {
	case it.Kind == "agent" && it.Reason == reasonPermission:
		return muted["permission_required"]
	case it.Kind == "agent" && it.Reason == reasonQuestion:
		return muted["waiting_input"]
	case it.Kind == "run":
		return muted["pipeline_needs_attention"]
	}
	return false
}

// noteFor builds the minimal notification for one item: who, where, and what
// kind of attention — never commands, diffs, or message text (FS-20.R19).
func noteFor(it attentionItem) pushNote {
	who := it.Title
	if !strings.Contains(who, "@") {
		who = fmt.Sprintf("%s in %s", it.Title, it.Project)
	}
	reason := it.Reason
	if it.Kind == "run" {
		reason = "needs attention"
	}
	path := "/agent/"
	switch it.Kind {
	case "task":
		path = "/task/"
	case "run":
		path = "/run/"
	}
	return pushNote{Title: "AgentDeck", Body: who + " " + reason, Tag: pushTag(it), URL: path + it.ID}
}

type pushWindowState struct {
	until   time.Time
	pending []attentionItem
}

// runPushSender turns new Needs-you entries into notifications. It uses the
// same attention helper as Home, so the two cannot disagree (TS-13.R10). The
// first evaluation only records what already needs the person.
func (s *Server) runPushSender(ctx context.Context) {
	queue := make(chan pushNote, pushQueueSize)
	go s.pushWorker(ctx, queue)
	events, unsub := s.eventBus.Subscribe()
	defer unsub()
	recheck := time.NewTicker(pushRecheck)
	defer recheck.Stop()
	flush := time.NewTicker(pushWindow / 10)
	defer flush.Stop()

	var seen map[string]bool
	windows := map[string]*pushWindowState{}
	var debounce <-chan time.Time
	enqueue := func(note pushNote) {
		select {
		case queue <- note:
		default:
			s.log.Warn("remote push: queue full, notification dropped", "tag", note.Tag)
		}
	}
	evaluate := func() {
		cfg, err := s.configStore.ReadConfig()
		if err != nil && !errors.Is(err, config.ErrNotFound) {
			return
		}
		lists, err := s.attention(time.Now())
		if err != nil {
			s.log.Warn("remote push: attention", "err", err)
			return
		}
		current := map[string]bool{}
		for _, it := range lists.NeedsYou {
			current[attentionKey(it)] = true
		}
		first := seen == nil
		previous := seen
		seen = current
		if first || !cfg.RemoteEnabled {
			return
		}
		now := time.Now()
		for _, it := range lists.NeedsYou {
			if previous[attentionKey(it)] || pushMuted(it, cfg.Notifications.Muted) {
				continue
			}
			tag := pushTag(it)
			if w := windows[tag]; w != nil && now.Before(w.until) {
				w.pending = append(w.pending, it)
				continue
			}
			windows[tag] = &pushWindowState{until: now.Add(pushWindow)}
			enqueue(noteFor(it))
		}
	}
	flushWindows := func() {
		now := time.Now()
		for tag, w := range windows {
			if now.Before(w.until) {
				continue
			}
			delete(windows, tag)
			switch len(w.pending) {
			case 0:
			case 1:
				enqueue(noteFor(w.pending[0]))
			default:
				first := w.pending[0]
				where := first.Project
				if first.Kind == "run" {
					where = first.Title
				}
				enqueue(pushNote{Title: "AgentDeck", Body: fmt.Sprintf("%d items in %s need you", len(w.pending), where), Tag: tag, URL: "/"})
			}
		}
	}

	evaluate()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-events:
			if !ok {
				return
			}
			if debounce == nil {
				debounce = time.After(pushDebounce)
			}
		case <-debounce:
			debounce = nil
			evaluate()
		case <-recheck.C:
			evaluate()
		case <-flush.C:
			flushWindows()
		}
	}
}

// pushWorker is the one sender. Mutes and per-device switches are read at send
// time; a 404/410 expires the subscription; other failures retry with bounded
// backoff, then drop and log without the payload (TS-13.R11).
func (s *Server) pushWorker(ctx context.Context, queue <-chan pushNote) {
	for {
		select {
		case <-ctx.Done():
			return
		case note := <-queue:
			s.deliverPush(ctx, note)
		}
	}
}

func (s *Server) deliverPush(ctx context.Context, note pushNote) {
	payload, err := json.Marshal(note)
	if err != nil || len(payload) > remote.PushPayloadLimit {
		return
	}
	devices, err := s.stateStore.ListRemoteDevices()
	if err != nil {
		s.log.Warn("remote push: list devices", "err", err)
		return
	}
	var keys remote.VAPIDKeys
	for _, d := range devices {
		if !d.PushEnabled || d.PushEndpoint == "" || d.PushState != "active" {
			continue
		}
		if keys.Private == "" {
			if keys, err = s.vapidKeys(); err != nil {
				s.log.Warn("remote push: vapid keys", "err", err)
				return
			}
		}
		sub := remote.PushSubscription{Endpoint: d.PushEndpoint, P256dh: d.PushP256dh, Auth: d.PushAuth}
		for attempt := 0; ; attempt++ {
			status, err := s.pushSend(ctx, keys, sub, payload, remote.PushTopic(note.Tag))
			if err == nil && status >= 200 && status < 300 {
				break
			}
			if status == http.StatusNotFound || status == http.StatusGone {
				if err := s.stateStore.ExpireRemoteDevicePush(d.ID, d.PushEndpoint); err != nil && !errors.Is(err, state.ErrNotFound) {
					s.log.Warn("remote push: expire subscription", "err", err)
				}
				s.publishRemote()
				break
			}
			if attempt >= len(pushRetryDelays) {
				// Transport errors embed the endpoint URL, a secret (TS-13.R12):
				// log only whether one occurred.
				s.log.Warn("remote push: delivery failed", "device_id", d.ID, "status", status, "transport_error", err != nil)
				break
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(pushRetryDelays[attempt]):
			}
		}
	}
}
