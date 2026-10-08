// Package state persists a snapshot of the last pull-request scout run so a
// later run can report only what changed since.
package state

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Entry is the durable shape of one pull request between runs.
type Entry struct {
	Status         string    `json:"status"`
	Owner          string    `json:"owner"`
	Title          string    `json:"title,omitempty"`
	URL            string    `json:"url,omitempty"`
	ResponseAuthor string    `json:"response_author,omitempty"`
	ResponseAt     time.Time `json:"response_at"`
}

// Snapshot is the whole saved state: when it was taken and one entry per pull
// request, keyed "owner/name#number".
type Snapshot struct {
	SavedAt time.Time        `json:"saved_at"`
	PRs     map[string]Entry `json:"prs"`
}

// Path returns the snapshot file location, under the user cache directory.
func Path() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gh-scout", "state.json"), nil
}

// Load reads the last snapshot. ok is false when none has been saved yet.
func Load() (Snapshot, bool, error) {
	p, err := Path()
	if err != nil {
		return Snapshot{}, false, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, err
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return Snapshot{}, false, err
	}
	if s.PRs == nil {
		s.PRs = map[string]Entry{}
	}
	return s, true, nil
}

// Save writes the snapshot atomically, creating its directory as needed.
func Save(s Snapshot) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
