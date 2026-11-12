package main

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"slices"
	"sync"
	"time"
)

type Item struct {
	ID         string    `json:"id" example:"itm_3f2a9c0d1e4b5a6f7081920a3b4c5d6e"`
	Name       string    `json:"name" example:"Kabel UTP Cat6"`
	Quantity   int64     `json:"quantity" example:"12"`
	CreatedBy  string    `json:"created_by" example:"usr_01a1..."`
	CreateTime time.Time `json:"create_time"`
	UpdateTime time.Time `json:"update_time"`
} // @name Item

// Store keeps the items of every tenant in memory; they are lost on restart.
// The tenant key is always the organization of the verified token.
type Store struct {
	mu      sync.Mutex
	tenants map[string]map[string]Item
	now     func() time.Time
}

func NewStore() *Store {
	return &Store{tenants: map[string]map[string]Item{}, now: time.Now}
}

// CreateTenant is the provisioning hook of nicetoken.ProvisionOnFirstUse, so it
// must be idempotent.
func (s *Store) CreateTenant(_ context.Context, org string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tenants[org]; !ok {
		s.tenants[org] = map[string]Item{}
	}
	return nil
}

func (s *Store) HasTenant(org string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.tenants[org]
	return ok
}

func (s *Store) List(org string) []Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Item, 0, len(s.tenants[org]))
	for _, it := range s.tenants[org] {
		out = append(out, it)
	}
	slices.SortFunc(out, func(a, b Item) int {
		return cmp.Or(a.CreateTime.Compare(b.CreateTime), cmp.Compare(a.ID, b.ID))
	})
	return out
}

func (s *Store) Get(org, id string) (Item, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.tenants[org][id]
	return it, ok
}

func (s *Store) Create(org, name string, quantity int64, createdBy string) Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tenants[org] == nil {
		s.tenants[org] = map[string]Item{}
	}
	now := s.now().UTC()
	it := Item{ID: newItemID(), Name: name, Quantity: quantity, CreatedBy: createdBy, CreateTime: now, UpdateTime: now}
	s.tenants[org][it.ID] = it
	return it
}

func (s *Store) Update(org, id, name string, quantity int64) (Item, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.tenants[org][id]
	if !ok {
		return Item{}, false
	}
	it.Name, it.Quantity, it.UpdateTime = name, quantity, s.now().UTC()
	s.tenants[org][id] = it
	return it, true
}

func (s *Store) Delete(org, id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tenants[org][id]; !ok {
		return false
	}
	delete(s.tenants[org], id)
	return true
}

func newItemID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "itm_" + hex.EncodeToString(b)
}
