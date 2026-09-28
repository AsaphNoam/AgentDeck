package remote

import (
	"os"
	"os/exec"
	"strconv"
	"sync"
)

// KeepAwake holds one idle-sleep assertion while the setting is on and work is
// active, and none otherwise (FS-20.R22, TS-13.R13). Reconciliation is
// serialized, so at most one assertion process exists.
type KeepAwake struct {
	start func() (stop func(), err error)

	mu     sync.Mutex
	stop   func()
	closed bool
}

// NewKeepAwake uses start to take the assertion; nil selects caffeinate.
func NewKeepAwake(start func() (func(), error)) *KeepAwake {
	if start == nil {
		start = startCaffeinate
	}
	return &KeepAwake{start: start}
}

// Set holds the assertion exactly when want is true. It reports an error only
// when taking the assertion failed.
func (k *KeepAwake) Set(want bool) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.closed {
		want = false
	}
	switch {
	case want && k.stop == nil:
		stop, err := k.start()
		if err != nil {
			return err
		}
		k.stop = stop
	case !want && k.stop != nil:
		k.stop()
		k.stop = nil
	}
	return nil
}

// Held reports whether the assertion is currently held.
func (k *KeepAwake) Held() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.stop != nil
}

// Close releases the assertion for good (server shutdown).
func (k *KeepAwake) Close() {
	k.mu.Lock()
	k.closed = true
	k.mu.Unlock()
	_ = k.Set(false)
}

// startCaffeinate prevents idle system sleep only (-i), never display or
// system-wide assertions, and ties the child to this server's life with -w so
// it ends even if the server dies without cleanup (TS-13.R13).
func startCaffeinate() (func(), error) {
	cmd := exec.Command("/usr/bin/caffeinate", "-i", "-w", strconv.Itoa(os.Getpid()))
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	return func() {
		_ = cmd.Process.Kill()
		<-done
	}, nil
}
