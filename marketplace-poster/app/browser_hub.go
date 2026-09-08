package main

import (
	"fmt"
	"sync"

	"marketplaceposter/automation"
)

type BrowserHub struct {
	mu       sync.Mutex
	store    *Store
	sessions map[string]*automation.BrowserSession
}

func NewBrowserHub(store *Store) *BrowserHub {
	return &BrowserHub{store: store, sessions: map[string]*automation.BrowserSession{}}
}

func (h *BrowserHub) Session(accountID string) (*automation.BrowserSession, error) {
	if accountID == "" {
		accountID = h.store.Settings().ActiveAccountID
	}
	found := false
	for _, a := range h.store.Accounts() {
		if a.ID == accountID {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("account not found")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if s := h.sessions[accountID]; s != nil {
		return s, nil
	}
	s := automation.NewBrowserSession(h.store.AccountProfileDir(accountID))
	h.sessions[accountID] = s
	return s, nil
}

func (h *BrowserHub) Active() (*automation.BrowserSession, string, error) {
	id := h.store.Settings().ActiveAccountID
	s, err := h.Session(id)
	return s, id, err
}

func (h *BrowserHub) Status(accountID string) (string, string) {
	s, err := h.Session(accountID)
	if err != nil {
		return "error", err.Error()
	}
	return s.Status()
}

func (h *BrowserHub) Close(accountID string) {
	h.mu.Lock()
	s := h.sessions[accountID]
	delete(h.sessions, accountID)
	h.mu.Unlock()
	if s != nil {
		s.Close()
	}
}

func (h *BrowserHub) CloseAll() {
	h.mu.Lock()
	all := make([]*automation.BrowserSession, 0, len(h.sessions))
	for _, s := range h.sessions {
		all = append(all, s)
	}
	h.sessions = map[string]*automation.BrowserSession{}
	h.mu.Unlock()
	for _, s := range all {
		s.Close()
	}
}
