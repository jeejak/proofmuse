package checks

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeejak/proofmuse/internal/evidence"
	"github.com/jeejak/proofmuse/internal/pipeline"
)

func wait(t *testing.T, s *Service, id string) *Check {
	t.Helper()
	for range 200 {
		if c, _ := s.Get(id); c.Status == Done || c.Status == Failed {
			return c
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("check never finished")
	return nil
}

func TestIdempotentSubmit(t *testing.T) {
	var runs atomic.Int32
	release := make(chan struct{})
	run := func(ctx context.Context, photos [][]byte, o evidence.Order) (*pipeline.Result, error) {
		runs.Add(1)
		<-release
		return &pipeline.Result{Verdict: evidence.Result{Verdict: evidence.VerdictKuat}}, nil
	}
	dir := t.TempDir()
	s, err := New(run, "cfg1", dir, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	photos := [][]byte{[]byte("a"), []byte("b")}
	a := s.Submit("SYN-R01", evidence.Order{}, photos)
	b := s.Submit("SYN-R01", evidence.Order{}, photos) // while in flight
	if a.ID != b.ID {
		t.Fatalf("same input, different ids %s %s", a.ID, b.ID)
	}
	close(release)
	wait(t, s, a.ID)
	c := s.Submit("SYN-R01", evidence.Order{}, photos) // after done
	if !c.Reused || c.Status != Done || runs.Load() != 1 {
		t.Fatalf("not reused: %+v runs %d", c, runs.Load())
	}
	d := s.Submit("SYN-R01", evidence.Order{}, [][]byte{[]byte("b"), []byte("a")})
	e := s.Submit("SYN-R02", evidence.Order{}, photos)
	// Wait for both, so no worker writes into the temp dir after the test.
	wait(t, s, d.ID)
	wait(t, s, e.ID)
	if d.ID == a.ID {
		t.Fatal("photo order ignored by the key")
	}
	if e.ID == a.ID {
		t.Fatal("order ignored by the key")
	}
	// A restart reads finished results back.
	s2, err := New(run, "cfg1", dir, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c, ok := s2.Get(a.ID); !ok || c.Status != Done {
		t.Fatalf("not persisted: %+v", c)
	}
}

func TestFailedCheckIsRetried(t *testing.T) {
	var runs atomic.Int32
	run := func(ctx context.Context, photos [][]byte, o evidence.Order) (*pipeline.Result, error) {
		if runs.Add(1) == 1 {
			return nil, errors.New("upstream said: request id 123 body secret")
		}
		return &pipeline.Result{}, nil
	}
	s, _ := New(run, "cfg", "", 1, nil)
	a := s.Submit("o", evidence.Order{}, [][]byte{[]byte("x")})
	if c := wait(t, s, a.ID); c.Status != Failed || c.Error != "check failed; try again" {
		t.Fatalf("%+v", c)
	}
	b := s.Submit("o", evidence.Order{}, [][]byte{[]byte("x")})
	if c := wait(t, s, b.ID); c.Status != Done || runs.Load() != 2 {
		t.Fatalf("%+v runs %d", c, runs.Load())
	}
}

func TestLimiter(t *testing.T) {
	now := time.Date(2026, 10, 27, 10, 0, 0, 0, time.UTC)
	l := &Limiter{PerClientHour: 2, PerDay: 3, Now: func() time.Time { return now }}
	for i, want := range []bool{true, true, false} {
		if ok, _ := l.Allow("a"); ok != want {
			t.Fatalf("call %d: %v", i, ok)
		}
	}
	if ok, _ := l.Allow("b"); !ok {
		t.Fatal("other client blocked")
	}
	if ok, why := l.Allow("c"); ok || why == "" {
		t.Fatal("daily cap not enforced")
	}
	now = now.Add(24 * time.Hour)
	if ok, _ := l.Allow("a"); !ok {
		t.Fatal("not reset after an hour and a day")
	}
}
