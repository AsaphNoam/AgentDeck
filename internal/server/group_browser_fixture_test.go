package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/AsaphNoam/Chuck/internal/config"
	"github.com/AsaphNoam/Chuck/internal/remote"
)

// Opt-in isolated browser fixture for the group-journey.mjs acceptance run.
func TestGroupBrowserFixture(t *testing.T) {
	ready := os.Getenv("CHUCK_GROUP_BROWSER_READY")
	if ready == "" {
		t.Skip("manual browser fixture")
	}
	s := testServer(t, true)
	defer s.registry.Shutdown(context.Background())
	s.registry.Chat().SetCommand(buildFakeACP(t))
	cfg, err := s.configStore.ReadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.OnboardingComplete = true
	if err := s.configStore.WriteConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if err := s.configStore.WriteProject("other", config.Project{Title: "Other project", Cwd: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	localListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	desktop := httptest.NewUnstartedServer(s.routes())
	desktop.Listener.Close()
	desktop.Listener = localListener
	desktop.Start()
	defer desktop.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	domain := listener.Addr().String()
	phone := httptest.NewUnstartedServer(s.remoteRoutes(domain, func(context.Context, string) (remote.Peer, error) {
		return remote.Peer{StableID: "browser-phone", Login: "fixture@example.com"}, nil
	}))
	phone.Listener.Close()
	phone.Listener = listener
	phone.StartTLS()
	defer phone.Close()
	token := pairTestDevice(t, s, "browser-device", "browser-phone")
	data, _ := json.Marshal(map[string]string{"desktop": desktop.URL, "phone": phone.URL, "cookie": remoteDeviceCookie, "token": token})
	if err := os.WriteFile(ready, data, 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 900; i++ {
		if _, err := os.Stat(ready + ".done"); err == nil {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatal("browser fixture timed out")
}
