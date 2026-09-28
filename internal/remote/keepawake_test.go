package remote

import (
	"errors"
	"testing"
)

func TestKeepAwakeHoldsOneAssertionExactlyWhileWanted(t *testing.T) {
	starts, stops := 0, 0
	k := NewKeepAwake(func() (func(), error) {
		starts++
		return func() { stops++ }, nil
	})
	for _, want := range []bool{false, true, true, false, false, true} {
		if err := k.Set(want); err != nil {
			t.Fatal(err)
		}
		if k.Held() != want {
			t.Fatalf("held=%v want=%v", k.Held(), want)
		}
	}
	if starts != 2 || stops != 1 {
		t.Fatalf("starts=%d stops=%d", starts, stops)
	}
	k.Close()
	if k.Held() || stops != 2 {
		t.Fatalf("close left the assertion held (stops=%d)", stops)
	}
	if err := k.Set(true); err != nil || k.Held() {
		t.Fatal("a closed keep-awake must never assert again")
	}

	failing := NewKeepAwake(func() (func(), error) { return nil, errors.New("no caffeinate") })
	if err := failing.Set(true); err == nil || failing.Held() {
		t.Fatal("failed start must report and hold nothing")
	}
}
