package remote

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Web Push from the Mac: RFC 8030 delivery, RFC 8291 aes128gcm encryption, and
// RFC 8292 VAPID, verified against webpush-go v1.4.0 (TS-13.R11).

// vapidSubject is the VAPID "sub" claim. Apple requires an https or mailto
// URL; the project URL avoids disclosing the person's tailnet name to push
// services.
const vapidSubject = "https://github.com/agentdeck/agentdeck"

// PushPayloadLimit bounds a notification payload (TS-13.R11).
const PushPayloadLimit = 4096

// VAPIDKeys is the Mac's application-server key pair.
type VAPIDKeys struct {
	Public  string `json:"public"`
	Private string `json:"private"`
}

var vapidMu sync.Mutex

// LoadOrCreateVAPID reads the key pair, generating it owner-only on first need.
func LoadOrCreateVAPID(path string) (VAPIDKeys, error) {
	vapidMu.Lock()
	defer vapidMu.Unlock()
	var keys VAPIDKeys
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &keys); err == nil && keys.Public != "" && keys.Private != "" {
			if err := os.Chmod(path, 0o600); err != nil {
				return VAPIDKeys{}, err
			}
			return keys, nil
		}
		return VAPIDKeys{}, fmt.Errorf("remote: %s is unreadable", filepath.Base(path))
	}
	if !errors.Is(err, os.ErrNotExist) {
		return VAPIDKeys{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return VAPIDKeys{}, err
	}
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return VAPIDKeys{}, err
	}
	keys = VAPIDKeys{Public: public, Private: private}
	data, _ = json.Marshal(keys)
	tmp, err := os.CreateTemp(filepath.Dir(path), ".vapid-*.tmp")
	if err != nil {
		return VAPIDKeys{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return VAPIDKeys{}, err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return VAPIDKeys{}, err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return VAPIDKeys{}, err
	}
	if err := tmp.Close(); err != nil {
		return VAPIDKeys{}, err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return VAPIDKeys{}, err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return VAPIDKeys{}, err
	}
	err = dir.Sync()
	dir.Close()
	if err != nil {
		return VAPIDKeys{}, err
	}
	return keys, nil
}

// pushHosts are the browser push services a subscription may name, so a phone
// cannot make the Mac post to arbitrary URLs (TS-13.R11).
var pushHostSuffixes = []string{
	"fcm.googleapis.com",
	".push.apple.com",
	".push.services.mozilla.com",
	".notify.windows.com",
}

// ValidPushEndpoint reports whether endpoint is https on a known push service.
func ValidPushEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, suffix := range pushHostSuffixes {
		if host == strings.TrimPrefix(suffix, ".") || strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

// PushSubscription is a stored device subscription.
type PushSubscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

// PushTopic derives the RFC 8030 Topic (≤32 URL-safe characters) from a tag
// so a push service replaces an undelivered message instead of stacking.
func PushTopic(tag string) string {
	sum := sha256.Sum256([]byte(tag))
	return base64.RawURLEncoding.EncodeToString(sum[:])[:32]
}

var pushClient = &http.Client{Timeout: 15 * time.Second}

// SendPush encrypts and posts one payload, returning the push service status.
func SendPush(ctx context.Context, keys VAPIDKeys, sub PushSubscription, payload []byte, topic string) (int, error) {
	if len(payload) > PushPayloadLimit {
		return 0, errors.New("remote: push payload too large")
	}
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		HTTPClient:      pushClient,
		Subscriber:      vapidSubject,
		Topic:           topic,
		TTL:             3600,
		Urgency:         webpush.UrgencyHigh,
		VAPIDPublicKey:  keys.Public,
		VAPIDPrivateKey: keys.Private,
	})
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return resp.StatusCode, nil
}
