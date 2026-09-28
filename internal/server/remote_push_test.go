package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentdeck/agentdeck/internal/remote"
	"github.com/agentdeck/agentdeck/internal/state"
)

func TestRemoteSelfAndPushSubscription(t *testing.T) {
	s := testServer(t, true)
	h := s.remoteRoutes(testDomain, testWhoIs(map[string]string{"100.64.0.2:5000": "n"}))
	token := pairTestDevice(t, s, "d1", "n")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, phoneRequest(http.MethodGet, "/api/remote/self", "", token))
	var self map[string]string
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &self) != nil || self["vapid_public_key"] == "" || self["notifications"] != "off" {
		t.Fatalf("self = %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "private") {
		t.Fatal("self leaked the VAPID private key")
	}

	for body, want := range map[string]int{
		`{"endpoint":"https://evil.example/x","keys":{"p256dh":"k","auth":"a"}}`:                400,
		`{"endpoint":"https://fcm.googleapis.com/fcm/send/x","keys":{"p256dh":"","auth":"a"}}`:  400,
		`{"endpoint":"https://fcm.googleapis.com/fcm/send/x","keys":{"p256dh":"k","auth":"a"}}`: 204,
	} {
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, phoneRequest(http.MethodPut, "/api/remote/self/push", body, token))
		if rec.Code != want {
			t.Fatalf("PUT push %s = %d %s", body, rec.Code, rec.Body)
		}
	}
	if list := doGET(t, s.routes(), "/api/remote/devices").Body.String(); !strings.Contains(list, `"notifications":"on"`) || strings.Contains(list, "fcm.googleapis.com") {
		t.Fatalf("device list = %s", list)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, phoneRequest(http.MethodDelete, "/api/remote/self/push", "", token))
	if rec.Code != 204 || !strings.Contains(doGET(t, s.routes(), "/api/remote/devices").Body.String(), `"notifications":"off"`) {
		t.Fatalf("DELETE push = %d", rec.Code)
	}
}

type sentPush struct {
	note  pushNote
	topic string
}

// FS-20.A5 on the payload: each new attention event notifies once, honoring
// mutes; completions never notify; text carries no command or message content;
// bursts coalesce per tag; a 410 expires the subscription.
func TestPushSenderNotifiesAttentionOnce(t *testing.T) {
	oldWindow, oldDebounce, oldRetry := pushWindow, pushDebounce, pushRetryDelays
	pushWindow, pushDebounce, pushRetryDelays = 300*time.Millisecond, 20*time.Millisecond, []time.Duration{time.Millisecond}
	t.Cleanup(func() { pushWindow, pushDebounce, pushRetryDelays = oldWindow, oldDebounce, oldRetry })

	s := testServer(t, true)
	putRemote(t, s.routes(), `{"enabled":false}`) // writes config; remote_enabled set below
	cfg, _ := s.configStore.ReadConfig()
	cfg.RemoteEnabled = true
	cfg.Notifications.Muted = map[string]bool{"waiting_input": true}
	if err := s.configStore.WriteConfig(cfg); err != nil {
		t.Fatal(err)
	}
	pairTestDevice(t, s, "d1", "n")
	if err := s.stateStore.SetRemoteDevicePush("d1", "https://fcm.googleapis.com/fcm/send/x", "k", "a"); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var sent []sentPush
	status := http.StatusCreated
	s.pushSend = func(_ context.Context, _ remote.VAPIDKeys, _ remote.PushSubscription, payload []byte, topic string) (int, error) {
		var note pushNote
		_ = json.Unmarshal(payload, &note)
		mu.Lock()
		defer mu.Unlock()
		sent = append(sent, sentPush{note, topic})
		return status, nil
	}
	snapshot := func() []sentPush {
		mu.Lock()
		defer mu.Unlock()
		return append([]sentPush(nil), sent...)
	}
	agent := func(id, st, detail string) {
		update := state.AgentStateUpdate{AgentState: state.AgentState{AgentID: id, Role: "implementer", Project: "my-app", State: st, Detail: detail, UpdatedAt: time.Now().UnixMilli()}}
		s.eventBus.SetSnapshot(update)
		s.eventBus.PublishStateUpdate(update)
	}

	agent("pre", "error", "") // already needs the person at startup: never notified
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.runPushSender(ctx)
	time.Sleep(100 * time.Millisecond)

	s.permissionMu.Lock()
	s.permissionTools[permissionToolKey("a1", "g", "c")] = "Bash"
	s.permissionMu.Unlock()
	agent("a1", "waiting_input", "Permission: Bash rm -rf /secret")
	agent("a2", "waiting_input", "Should I continue?") // muted question
	agent("a3", "done", "")                            // completion
	waitFor(t, func() bool { return len(snapshot()) == 1 })
	first := snapshot()[0].note
	if first.Body != "implementer@my-app needs permission" || first.URL != "/agent/a1" || first.Tag != "project:my-app" {
		t.Fatalf("note = %+v", first)
	}
	for _, leak := range []string{"rm -rf", "secret", "Should I continue"} {
		if strings.Contains(first.Body+first.Title, leak) {
			t.Fatalf("payload leaked %q", leak)
		}
	}

	// Two more in the same project inside the window coalesce into one summary.
	agent("b1", "error", "")
	agent("b2", "error", "")
	waitFor(t, func() bool { return len(snapshot()) == 2 })
	if got := snapshot()[1].note; got.Body != "2 items in my-app need you" || got.Tag != "project:my-app" || snapshot()[1].topic != remote.PushTopic("project:my-app") {
		t.Fatalf("summary = %+v", got)
	}
	time.Sleep(pushWindow + 100*time.Millisecond)
	if n := len(snapshot()); n != 2 {
		t.Fatalf("sent %d, want no repeats", n)
	}

	// A gone subscription is marked expired and not retried.
	mu.Lock()
	status = http.StatusGone
	mu.Unlock()
	agent("c1", "error", "")
	waitFor(t, func() bool {
		devices, _ := s.stateStore.ListRemoteDevices()
		return len(devices) == 1 && devices[0].PushState == "expired"
	})
	before := len(snapshot())
	agent("c2", "error", "")
	time.Sleep(pushWindow + 100*time.Millisecond)
	if len(snapshot()) != before {
		t.Fatal("expired subscription was still sent to")
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
