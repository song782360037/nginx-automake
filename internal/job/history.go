package job

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type HistoryEntry struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	Version   string    `json:"version"`
	Modules   []string  `json:"modules"`
	Status    string    `json:"status"`
	Artifact  string    `json:"artifact"`
	Error     string    `json:"error"`
}

type HistoryStore struct {
	path string
	mu   sync.Mutex
	list []HistoryEntry
}

func NewHistoryStore(path string) (*HistoryStore, error) {
	store := &HistoryStore{path: path}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (h *HistoryStore) load() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.path == "" {
		return nil
	}
	data, err := os.ReadFile(h.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, &h.list)
}

func (h *HistoryStore) List() []HistoryEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]HistoryEntry, len(h.list))
	copy(result, h.list)
	return result
}

func (h *HistoryStore) Append(entry HistoryEntry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.list = append([]HistoryEntry{entry}, h.list...)
	if len(h.list) > 200 {
		h.list = h.list[:200]
	}
	return h.persistLocked()
}

func (h *HistoryStore) persistLocked() error {
	if h.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(h.list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(h.path, data, 0o644)
}
