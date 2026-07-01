package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// pidfileName is the dashboard pidfile under the store home.
const pidfileName = "dashboard.pid"

// pidInfo is the JSON payload of the pidfile, so both `stop` and `open` can read
// the running PID and port.
type pidInfo struct {
	PID  int `json:"pid"`
	Port int `json:"port"`
}

// pidfilePath returns {home}/dashboard.pid.
func pidfilePath(home string) string {
	return filepath.Join(home, pidfileName)
}

// writePidfile writes the pid/port atomically (temp+rename) to {home}/dashboard.pid.
func writePidfile(home string, info pidInfo) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := pidfilePath(home)
	tmp, err := os.CreateTemp(home, ".pid-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	// Sync before rename so a crash right after `dashboard start --detach` can't
	// leave a truncated pidfile (which would make stop/open report "not running"
	// while the daemon is actually up).
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// readPidfile reads {home}/dashboard.pid. A missing file returns (zero, false, nil).
func readPidfile(home string) (pidInfo, bool, error) {
	data, err := os.ReadFile(pidfilePath(home))
	if err != nil {
		if os.IsNotExist(err) {
			return pidInfo{}, false, nil
		}
		return pidInfo{}, false, err
	}
	var info pidInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return pidInfo{}, false, fmt.Errorf("corrupt pidfile: %w", err)
	}
	return info, true, nil
}

// removePidfile deletes the pidfile, tolerating absence.
func removePidfile(home string) error {
	if err := os.Remove(pidfilePath(home)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// processAlive reports whether pid is a live process, via signal 0.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// On Unix, Signal(0) succeeds for a live process the caller can signal and
	// returns ESRCH for a dead one (EPERM still implies the process exists).
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	return err == syscall.EPERM
}
