package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type SnapshotStore interface {
	Load() (*ArenaSnapshot, error)
	Save(snapshot *ArenaSnapshot) error
}

type ArenaSnapshot struct {
	Rooms []*ArenaRoom `json:"rooms"`
}

type MemorySnapshotStore struct {
	snapshot *ArenaSnapshot
}

func NewMemorySnapshotStore() *MemorySnapshotStore {
	return &MemorySnapshotStore{}
}

func (m *MemorySnapshotStore) Load() (*ArenaSnapshot, error) {
	if m.snapshot == nil {
		return &ArenaSnapshot{}, nil
	}
	data, err := json.Marshal(m.snapshot)
	if err != nil {
		return nil, err
	}
	var cp ArenaSnapshot
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, err
	}
	normalizeSnapshot(&cp)
	return &cp, nil
}

func (m *MemorySnapshotStore) Save(snapshot *ArenaSnapshot) error {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	var cp ArenaSnapshot
	if err := json.Unmarshal(data, &cp); err != nil {
		return err
	}
	normalizeSnapshot(&cp)
	m.snapshot = &cp
	return nil
}

type FileSnapshotStore struct {
	path string
}

func NewFileSnapshotStore(path string) *FileSnapshotStore {
	return &FileSnapshotStore{path: path}
}

func (f *FileSnapshotStore) Load() (*ArenaSnapshot, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &ArenaSnapshot{}, nil
		}
		return nil, err
	}
	var snapshot ArenaSnapshot
	if len(data) == 0 {
		return &ArenaSnapshot{}, nil
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}
	normalizeSnapshot(&snapshot)
	return &snapshot, nil
}

func (f *FileSnapshotStore) Save(snapshot *ArenaSnapshot) error {
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}

func normalizeSnapshot(snapshot *ArenaSnapshot) {
	if snapshot == nil {
		return
	}
	for _, room := range snapshot.Rooms {
		reconcilePicoclawRuntime(room)
	}
}
