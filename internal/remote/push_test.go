package remote

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestValidPushEndpoint(t *testing.T) {
	for endpoint, want := range map[string]bool{
		"https://fcm.googleapis.com/fcm/send/abc":            true,
		"https://web.push.apple.com/QGx":                     true,
		"https://updates.push.services.mozilla.com/wpush/v2": true,
		"https://db5p.notify.windows.com/w/?token=x":         true,
		"http://fcm.googleapis.com/fcm/send/abc":             false,
		"https://evil.example/push.apple.com":                false,
		"https://push.apple.com.evil.example/x":              false,
		"https://fcm.googleapis.com:8443/x":                  false,
		"https://user@fcm.googleapis.com/x":                  false,
		"https://127.0.0.1/x":                                false,
	} {
		if got := ValidPushEndpoint(endpoint); got != want {
			t.Errorf("%s = %v, want %v", endpoint, got, want)
		}
	}
}

func TestVAPIDKeysPersistOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "remote", "vapid.json")
	a, err := LoadOrCreateVAPID(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadOrCreateVAPID(path)
	if err != nil || a != b {
		t.Fatalf("reload = %+v, %v", b, err)
	}
	st, _ := os.Stat(path)
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", st.Mode())
	}
	if len(PushTopic("run:r1")) != 32 || strings.ContainsAny(PushTopic("x"), "+/=") {
		t.Fatal("topic must be 32 URL-safe characters")
	}
}

func TestVAPIDKeysConcurrentFirstUseAndModeRepair(t *testing.T) {
	path := filepath.Join(t.TempDir(), "remote", "vapid.json")
	const callers = 32
	results := make([]VAPIDKeys, callers)
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = LoadOrCreateVAPID(path)
		}(i)
	}
	wg.Wait()
	for i := range results {
		if errs[i] != nil || results[i] != results[0] {
			t.Fatalf("caller %d = %+v, %v; first = %+v", i, results[i], errs[i], results[0])
		}
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateVAPID(path); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("repaired mode = %v", st.Mode())
	}
}

// SendPush posts an aes128gcm body with VAPID auth; the push service is a
// local fake, never a real one (TS-06.R27).
func TestSendPushRequestShape(t *testing.T) {
	var got *http.Request
	var body []byte
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		body = make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	old := pushClient
	pushClient = srv.Client()
	defer func() { pushClient = old }()

	keys, _ := LoadOrCreateVAPID(filepath.Join(t.TempDir(), "vapid.json"))
	priv, _ := ecdh.P256().GenerateKey(rand.Reader)
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	sub := PushSubscription{
		Endpoint: srv.URL + "/push",
		P256dh:   base64.RawURLEncoding.EncodeToString(priv.PublicKey().Bytes()),
		Auth:     base64.RawURLEncoding.EncodeToString(auth),
	}
	status, err := SendPush(context.Background(), keys, sub, []byte(`{"title":"t"}`), PushTopic("run:r1"))
	if err != nil || status != http.StatusCreated {
		t.Fatalf("send = %d, %v", status, err)
	}
	if got.Header.Get("Content-Encoding") != "aes128gcm" || !strings.HasPrefix(got.Header.Get("Authorization"), "vapid t=") ||
		got.Header.Get("TTL") == "" || got.Header.Get("Urgency") != "high" || got.Header.Get("Topic") != PushTopic("run:r1") {
		t.Fatalf("headers = %v", got.Header)
	}
	if strings.Contains(string(body), `"title"`) {
		t.Fatal("payload was not encrypted")
	}
	if _, err := SendPush(context.Background(), keys, sub, make([]byte, PushPayloadLimit+1), ""); err == nil {
		t.Fatal("oversized payload must be refused")
	}
}
