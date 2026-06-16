package setlist

import (
	"fmt"
	"testing"
)

func makeTestSetlist(n int) *Setlist {
	sl := &Setlist{ID: "test", Name: "Test"}
	for i := range n {
		sl.Profiles = append(sl.Profiles, Profile{
			ID:       fmt.Sprintf("p%d", i),
			Name:     fmt.Sprintf("Profile %d", i),
			SFPath:   "/test.sf2",
			Programs: []Program{{Channel: 0, Bank: 0, Program: i}},
		})
	}
	return sl
}

func TestStateNext(t *testing.T) {
	sl := makeTestSetlist(3)
	s := NewState(sl, WrapAround)

	if s.Index() != 0 {
		t.Fatalf("initial index = %d", s.Index())
	}

	p, err := s.Next()
	if err != nil || p.ID != "p1" {
		t.Fatalf("Next() = (%v, %v)", p, err)
	}

	s.Next()          // -> p2
	p, err = s.Next() // -> wraps to p0
	if err != nil || p.ID != "p0" {
		t.Fatalf("Next() after wrap = (%v, %v)", p, err)
	}
}

func TestStateStayOnLast(t *testing.T) {
	sl := makeTestSetlist(2)
	s := NewState(sl, StayOnLast)

	s.Next()           // -> p1
	p, err := s.Next() // should stay on p1
	if err != nil || p.ID != "p1" {
		t.Fatalf("Next() at end = (%v, %v)", p, err)
	}
	if s.Index() != 1 {
		t.Fatalf("index = %d, want 1", s.Index())
	}
}

func TestStatePrev(t *testing.T) {
	sl := makeTestSetlist(3)
	s := NewState(sl, WrapAround)

	p, err := s.Prev() // wraps to p2
	if err != nil || p.ID != "p2" {
		t.Fatalf("Prev() from 0 = (%v, %v)", p, err)
	}
}

func TestStateGoTo(t *testing.T) {
	sl := makeTestSetlist(5)
	s := NewState(sl, WrapAround)

	p, err := s.GoTo(3)
	if err != nil || p.ID != "p3" {
		t.Fatalf("GoTo(3) = (%v, %v)", p, err)
	}

	_, err = s.GoTo(10)
	if err == nil {
		t.Error("GoTo(10) should return error")
	}

	_, err = s.GoTo(-1)
	if err == nil {
		t.Error("GoTo(-1) should return error")
	}
}

func TestStateCurrent(t *testing.T) {
	sl := makeTestSetlist(2)
	s := NewState(sl, WrapAround)
	p := s.Current()
	if p.ID != "p0" {
		t.Errorf("Current() = %v", p)
	}
}
