package state

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// RemoteDevice is one paired phone (TS-02.R37). TokenHash is the SHA-256 of the
// device credential; the credential itself is never stored (TS-13.R7).
type RemoteDevice struct {
	ID           string
	Name         string
	TokenHash    string
	NodeStableID string
	NodeLogin    string
	PairedAt     time.Time
	LastSeenAt   time.Time
	PushEndpoint string
	PushP256dh   string
	PushAuth     string
	PushEnabled  bool
	PushState    string
}

const remoteDeviceColumns = `id, name, token_hash, node_stable_id, node_login, paired_at, last_seen_at,
	COALESCE(push_endpoint, ''), COALESCE(push_p256dh, ''), COALESCE(push_auth, ''), push_enabled, push_state`

func scanRemoteDevice(row interface{ Scan(...any) error }) (RemoteDevice, error) {
	var d RemoteDevice
	var paired, seen string
	var pushEnabled int
	if err := row.Scan(&d.ID, &d.Name, &d.TokenHash, &d.NodeStableID, &d.NodeLogin, &paired, &seen,
		&d.PushEndpoint, &d.PushP256dh, &d.PushAuth, &pushEnabled, &d.PushState); err != nil {
		return RemoteDevice{}, err
	}
	var err error
	if d.PairedAt, err = parseTime(paired); err != nil {
		return RemoteDevice{}, err
	}
	if d.LastSeenAt, err = parseTime(seen); err != nil {
		return RemoteDevice{}, err
	}
	d.PushEnabled = pushEnabled != 0
	return d, nil
}

// InsertRemoteDevice records a newly allowed phone.
func (s *Store) InsertRemoteDevice(d RemoteDevice) error {
	now := timeNow()
	if d.PairedAt.IsZero() {
		d.PairedAt = now
	}
	if d.LastSeenAt.IsZero() {
		d.LastSeenAt = d.PairedAt
	}
	_, err := s.db.Exec(
		`INSERT INTO remote_devices(id, name, token_hash, node_stable_id, node_login, paired_at, last_seen_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.Name, d.TokenHash, d.NodeStableID, d.NodeLogin, formatTime(d.PairedAt), formatTime(d.LastSeenAt))
	if err != nil {
		return fmt.Errorf("state: insert remote device: %w", err)
	}
	return nil
}

// ListRemoteDevices returns every paired phone, oldest pairing first.
func (s *Store) ListRemoteDevices() ([]RemoteDevice, error) {
	rows, err := s.db.Query(`SELECT ` + remoteDeviceColumns + ` FROM remote_devices ORDER BY paired_at, id`)
	if err != nil {
		return nil, fmt.Errorf("state: list remote devices: %w", err)
	}
	defer rows.Close()
	out := []RemoteDevice{}
	for rows.Next() {
		d, err := scanRemoteDevice(rows)
		if err != nil {
			return nil, fmt.Errorf("state: scan remote device: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("state: iterate remote devices: %w", err)
	}
	return out, nil
}

// RemoteDeviceByTokenHash finds the device a credential belongs to, or
// ErrNotFound.
func (s *Store) RemoteDeviceByTokenHash(hash string) (RemoteDevice, error) {
	d, err := scanRemoteDevice(s.db.QueryRow(`SELECT `+remoteDeviceColumns+` FROM remote_devices WHERE token_hash = ?`, hash))
	if errors.Is(err, sql.ErrNoRows) {
		return RemoteDevice{}, ErrNotFound
	}
	if err != nil {
		return RemoteDevice{}, fmt.Errorf("state: read remote device: %w", err)
	}
	return d, nil
}

// RenameRemoteDevice changes a phone's display name.
func (s *Store) RenameRemoteDevice(id, name string) error {
	return s.updateRemoteDevice(`UPDATE remote_devices SET name = ? WHERE id = ?`, name, id)
}

// TouchRemoteDevice records when a phone was last seen.
func (s *Store) TouchRemoteDevice(id string, at time.Time) error {
	return s.updateRemoteDevice(`UPDATE remote_devices SET last_seen_at = ? WHERE id = ?`, formatTime(at), id)
}

// DeleteRemoteDevice hard-deletes a revoked or unpaired phone (TS-02.R37).
func (s *Store) DeleteRemoteDevice(id string) error {
	return s.updateRemoteDevice(`DELETE FROM remote_devices WHERE id = ?`, id)
}

func (s *Store) updateRemoteDevice(query string, args ...any) error {
	res, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("state: update remote device: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("state: update remote device: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
