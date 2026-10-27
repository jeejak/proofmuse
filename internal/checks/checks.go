// Package checks runs evidence checks as background jobs.
//
// A check is identified by an idempotency key: the order, the SHA-256 of
// every photo in order, the prompt version and the pipeline config. The
// same photos for the same order return the same check, finished or in
// flight, so a double tap or a retried upload never pays twice. Results
// (never photos) are kept in memory and, when Dir is set, as one JSON
// file per key so a restart keeps them.
package checks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jeejak/proofmuse/internal/evidence"
	"github.com/jeejak/proofmuse/internal/pipeline"
)

// Status of a check.
const (
	Queued  = "queued"
	Running = "running"
	Done    = "done"
	Failed  = "failed"
)

// Check is what the API returns.
type Check struct {
	ID        string           `json:"id"`
	OrderID   string           `json:"order_id"`
	Status    string           `json:"status"`
	Error     string           `json:"error,omitempty"`
	Result    *pipeline.Result `json:"result,omitempty"`
	Created   time.Time        `json:"created"`
	Finished  *time.Time       `json:"finished,omitempty"`
	Photos    int              `json:"photos"`
	Reused    bool             `json:"reused,omitempty"` // answered from an earlier identical check
	PhotoHash []string         `json:"photo_sha256"`
}

// Runner runs the pipeline; tests replace it.
type Runner func(ctx context.Context, photos [][]byte, order evidence.Order) (*pipeline.Result, error)

// Service queues and runs checks.
type Service struct {
	Run     Runner
	Dir     string // optional persistence for results
	Timeout time.Duration
	Logger  *slog.Logger
	// ConfigKey names the pipeline config; it is part of the idempotency key.
	ConfigKey string

	mu     sync.Mutex
	checks map[string]*Check
	sem    chan struct{}
}

// New returns a service running at most workers checks at once.
func New(run Runner, configKey, dir string, workers int, logger *slog.Logger) (*Service, error) {
	s := &Service{Run: run, Dir: dir, Timeout: 3 * time.Minute, Logger: logger, ConfigKey: configKey,
		checks: map[string]*Check{}, sem: make(chan struct{}, max(workers, 1))}
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		if err := s.load(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Key is the idempotency key for a set of photos against an order.
func Key(orderID, configKey string, photos [][]byte) (string, []string) {
	h := sha256.New()
	fmt.Fprintf(h, "proofmuse/v1\x00%s\x00%s\x00%s\x00", orderID, configKey, evidence.PromptVersion)
	hashes := make([]string, len(photos))
	for i, p := range photos {
		sum := sha256.Sum256(p)
		hashes[i] = hex.EncodeToString(sum[:])
		fmt.Fprintf(h, "%s\x00", hashes[i])
	}
	return hex.EncodeToString(h.Sum(nil))[:32], hashes
}

// Submit starts a check or returns the existing one for the same key.
// A failed check is retried on resubmission; a finished one is reused.
func (s *Service) Submit(orderID string, order evidence.Order, photos [][]byte) *Check {
	id, hashes := Key(orderID, s.ConfigKey, photos)
	s.mu.Lock()
	if c, ok := s.checks[id]; ok && c.Status != Failed {
		cp := *c
		cp.Reused = c.Status == Done
		s.mu.Unlock()
		return &cp
	}
	c := &Check{ID: id, OrderID: orderID, Status: Queued, Created: time.Now().UTC(), Photos: len(photos), PhotoHash: hashes}
	s.checks[id] = c
	cp := *c // copied under the lock, before the worker can write
	s.mu.Unlock()
	go s.work(c, order, photos)
	return &cp
}

// Get returns a copy of a check.
func (s *Service) Get(id string) (*Check, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.checks[id]
	if !ok {
		return nil, false
	}
	cp := *c
	return &cp, true
}

func (s *Service) set(c *Check, f func(*Check)) {
	s.mu.Lock()
	f(c)
	s.mu.Unlock()
}

func (s *Service) work(c *Check, order evidence.Order, photos [][]byte) {
	s.sem <- struct{}{}
	defer func() { <-s.sem }()
	s.set(c, func(c *Check) { c.Status = Running })
	ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
	defer cancel()
	res, err := s.Run(ctx, photos, order)
	now := time.Now().UTC()
	s.set(c, func(c *Check) {
		c.Finished, c.Result = &now, res
		if err != nil {
			c.Status, c.Error = Failed, publicError(err)
		} else {
			c.Status = Done
		}
	})
	if s.Logger != nil {
		cost := 0.0
		if res != nil {
			cost = res.Usage.CostUSD
		}
		s.Logger.Info("check", "id", c.ID, "order", c.OrderID, "photos", c.Photos, "status", c.Status, "cost_usd", cost, "err", err)
	}
	if err == nil && s.Dir != "" {
		s.mu.Lock()
		b, _ := json.Marshal(c)
		s.mu.Unlock()
		tmp := filepath.Join(s.Dir, c.ID+".json.tmp")
		if werr := os.WriteFile(tmp, b, 0o644); werr == nil {
			_ = os.Rename(tmp, filepath.Join(s.Dir, c.ID+".json"))
		}
	}
}

// publicError keeps upstream details (request ids, bodies) out of the
// browser; the log has them.
func publicError(err error) string {
	switch {
	case errors.Is(err, pipeline.ErrInvalid):
		return "model answer did not match the schema twice"
	case errors.Is(err, context.DeadlineExceeded):
		return "check timed out"
	default:
		return "check failed; try again"
	}
}

func (s *Service) load() error {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.Dir, e.Name()))
		if err != nil {
			return err
		}
		var c Check
		if err := json.Unmarshal(b, &c); err != nil || c.Status != Done {
			continue
		}
		s.checks[c.ID] = &c
	}
	return nil
}
