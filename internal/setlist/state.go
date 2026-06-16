package setlist

import (
	"fmt"
	"sync"
)

// WrapMode controls behavior when advancing past the last profile.
type WrapMode int

const (
	WrapAround WrapMode = iota
	StayOnLast
)

// State manages the current position within a setlist.
type State struct {
	mu       sync.RWMutex
	setlist  *Setlist
	index    int
	wrapMode WrapMode
}

// NewState creates a new state machine starting at index 0.
func NewState(sl *Setlist, mode WrapMode) *State {
	return &State{setlist: sl, wrapMode: mode}
}

// Current returns the active profile.
func (s *State) Current() *Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &s.setlist.Profiles[s.index]
}

// Index returns the current profile index.
func (s *State) Index() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.index
}

// Len returns the number of profiles in the setlist.
func (s *State) Len() int {
	return len(s.setlist.Profiles)
}

// Setlist returns the underlying setlist.
func (s *State) Setlist() *Setlist {
	return s.setlist
}

// Next advances to the next profile.
func (s *State) Next() (*Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.setlist.Profiles)
	next := s.index + 1
	if next >= n {
		switch s.wrapMode {
		case WrapAround:
			next = 0
		case StayOnLast:
			next = n - 1
		}
	}
	s.index = next
	return &s.setlist.Profiles[s.index], nil
}

// Prev goes back to the previous profile.
func (s *State) Prev() (*Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.setlist.Profiles)
	prev := s.index - 1
	if prev < 0 {
		switch s.wrapMode {
		case WrapAround:
			prev = n - 1
		case StayOnLast:
			prev = 0
		}
	}
	s.index = prev
	return &s.setlist.Profiles[s.index], nil
}

// GoTo jumps to a specific profile index.
func (s *State) GoTo(index int) (*Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.setlist.Profiles) {
		return nil, fmt.Errorf("index %d out of range [0, %d)", index, len(s.setlist.Profiles))
	}
	s.index = index
	return &s.setlist.Profiles[s.index], nil
}
