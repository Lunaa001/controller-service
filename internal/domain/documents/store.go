package documents

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/google/uuid"
)

type Record struct {
	ID       string `json:"document_id"`
	OwnerID  string `json:"owner_id"`
	Status   string `json:"status"`
	Summary  string `json:"summary,omitempty"`
	Filename string `json:"filename,omitempty"`
	Hash     string `json:"hash,omitempty"`
}

type Store struct {
	mu     sync.RWMutex
	byID   map[string]Record
	byHash map[string]string
}

func NewStore() *Store {
	return &Store{
		byID:   make(map[string]Record),
		byHash: make(map[string]string),
	}
}

func (s *Store) Upsert(ownerID, filename string, content []byte) Record {
	hash := hashContent(content)

	s.mu.Lock()
	defer s.mu.Unlock()

	if id, ok := s.byHash[ownerID+":"+hash]; ok {
		return s.byID[id]
	}

	rec := Record{
		ID:       newID(),
		OwnerID:  ownerID,
		Status:   "pending",
		Filename: filename,
		Hash:     hash,
	}
	s.byID[rec.ID] = rec
	s.byHash[ownerID+":"+hash] = rec.ID
	return rec
}

func (s *Store) Status(id string) (Record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, ok := s.byID[id]
	return rec, ok
}

func (s *Store) Summary(id string) (Record, bool) {
	return s.Status(id)
}

func (s *Store) RequireOwner(id, ownerID string) (Record, error) {
	rec, ok := s.Status(id)
	if !ok {
		return Record{}, errors.New("not found")
	}
	if rec.OwnerID != ownerID {
		return Record{}, errors.New("forbidden")
	}
	return rec, nil
}

func (s *Store) SetSummary(id, summary string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.byID[id]
	if !ok {
		return errors.New("not found")
	}
	rec.Summary = summary
	rec.Status = "ready"
	s.byID[id] = rec
	return nil
}

func hashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func newID() string {
	return uuid.NewString()
}
