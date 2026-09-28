package state

import (
	"errors"
	"testing"
	"time"
)

func TestRemoteDevicesLifecycle(t *testing.T) {
	st, _ := newTestStore(t)
	paired := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	for _, d := range []RemoteDevice{
		{ID: "d1", Name: "Pixel", TokenHash: "h1", NodeStableID: "n1", NodeLogin: "me@x", PairedAt: paired},
		{ID: "d2", Name: "iPhone", TokenHash: "h2", NodeStableID: "n2", PairedAt: paired.Add(time.Minute)},
	} {
		if err := st.InsertRemoteDevice(d); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.InsertRemoteDevice(RemoteDevice{ID: "d3", Name: "dup", TokenHash: "h1", NodeStableID: "n3"}); err == nil {
		t.Fatal("duplicate token hash must be rejected")
	}

	d, err := st.RemoteDeviceByTokenHash("h1")
	if err != nil || d.ID != "d1" || d.NodeStableID != "n1" || d.NodeLogin != "me@x" || !d.LastSeenAt.Equal(paired) || d.PushState != "active" || d.PushEnabled {
		t.Fatalf("by hash = %+v, %v", d, err)
	}
	if _, err := st.RemoteDeviceByTokenHash("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown hash err = %v", err)
	}

	if err := st.RenameRemoteDevice("d2", "Work iPhone"); err != nil {
		t.Fatal(err)
	}
	seen := paired.Add(time.Hour)
	if err := st.TouchRemoteDevice("d2", seen); err != nil {
		t.Fatal(err)
	}
	list, err := st.ListRemoteDevices()
	if err != nil || len(list) != 2 || list[0].ID != "d1" || list[1].Name != "Work iPhone" || !list[1].LastSeenAt.Equal(seen) {
		t.Fatalf("list = %+v, %v", list, err)
	}

	if err := st.DeleteRemoteDevice("d1"); err != nil {
		t.Fatal(err)
	}
	for _, err := range []error{st.DeleteRemoteDevice("d1"), st.RenameRemoteDevice("d1", "x"), st.TouchRemoteDevice("d1", seen)} {
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("missing device err = %v", err)
		}
	}
	if _, err := st.RemoteDeviceByTokenHash("h1"); !errors.Is(err, ErrNotFound) {
		t.Fatal("deleted device must not authenticate")
	}
}

func TestRemoteDevicePushSubscription(t *testing.T) {
	st, _ := newTestStore(t)
	if err := st.InsertRemoteDevice(RemoteDevice{ID: "d1", Name: "p", TokenHash: "h", NodeStableID: "n"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetRemoteDevicePush("d1", "https://fcm.googleapis.com/a", "k", "s"); err != nil {
		t.Fatal(err)
	}
	d, _ := st.RemoteDeviceByTokenHash("h")
	if !d.PushEnabled || d.PushEndpoint != "https://fcm.googleapis.com/a" || d.PushState != "active" {
		t.Fatalf("after set = %+v", d)
	}
	// A stale endpoint's rejection does not expire a replacement subscription.
	if err := st.ExpireRemoteDevicePush("d1", "https://fcm.googleapis.com/old"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stale expire err = %v", err)
	}
	if err := st.ExpireRemoteDevicePush("d1", "https://fcm.googleapis.com/a"); err != nil {
		t.Fatal(err)
	}
	d, _ = st.RemoteDeviceByTokenHash("h")
	if d.PushState != "expired" {
		t.Fatalf("state = %q", d.PushState)
	}
	if err := st.ClearRemoteDevicePush("d1"); err != nil {
		t.Fatal(err)
	}
	d, _ = st.RemoteDeviceByTokenHash("h")
	if d.PushEnabled || d.PushEndpoint != "" || d.PushState != "active" {
		t.Fatalf("after clear = %+v", d)
	}
}
