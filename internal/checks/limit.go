package checks

import (
	"sync"
	"time"
)

// Limiter caps new (paid) checks per client per hour and across all
// clients per day, so a public demo URL cannot drain the API credits.
// Reused results are free and not counted.
type Limiter struct {
	PerClientHour int
	PerDay        int
	Now           func() time.Time

	mu      sync.Mutex
	clients map[string][]time.Time
	day     string
	dayN    int
}

// Allow records and permits one check for client, or says why not.
func (l *Limiter) Allow(client string) (bool, string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if l.Now != nil {
		now = l.Now()
	}
	if l.clients == nil {
		l.clients = map[string][]time.Time{}
	}
	d := now.UTC().Format("2006-01-02")
	if d != l.day {
		l.day, l.dayN = d, 0
	}
	if l.PerDay > 0 && l.dayN >= l.PerDay {
		return false, "daily demo limit reached; try tomorrow"
	}
	recent := l.clients[client][:0]
	for _, t := range l.clients[client] {
		if now.Sub(t) < time.Hour {
			recent = append(recent, t)
		}
	}
	if l.PerClientHour > 0 && len(recent) >= l.PerClientHour {
		l.clients[client] = recent
		return false, "too many checks from this device; wait a few minutes"
	}
	l.clients[client] = append(recent, now)
	l.dayN++
	return true, ""
}
