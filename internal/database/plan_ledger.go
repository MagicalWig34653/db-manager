// SPDX-License-Identifier: AGPL-3.0-only
package database

import (
	"sync"
	"time"
)

const (
	planLedgerTTL = time.Hour
	planLedgerMax = 4096
)

// planLedger makes confirmed plans single-use. A plan digest is deterministic,
// so a fresh preview (plan call) re-arms the digest while every execute call
// consumes it before the first write. Replaying the same confirmed request
// (for example a network retry after a lost response) is refused instead of
// duplicating data. The ledger lives in memory: after a restart a replay is
// possible again, which the contracts document.
type planLedger struct {
	mu       sync.Mutex
	consumed map[string]time.Time
}

// arm is called when a preview was produced for the digest.
func (l *planLedger) arm(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.consumed, key)
}

// consume returns false when the digest was already used by an execute call.
func (l *planLedger) consume(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if t, ok := l.consumed[key]; ok && now.Sub(t) < planLedgerTTL {
		return false
	}
	if l.consumed == nil {
		l.consumed = map[string]time.Time{}
	}
	if len(l.consumed) >= planLedgerMax {
		var oldestKey string
		var oldest time.Time
		for k, t := range l.consumed {
			if now.Sub(t) >= planLedgerTTL {
				delete(l.consumed, k)
			} else if oldestKey == "" || t.Before(oldest) {
				oldestKey, oldest = k, t
			}
		}
		if len(l.consumed) >= planLedgerMax && oldestKey != "" {
			delete(l.consumed, oldestKey)
		}
	}
	l.consumed[key] = now
	return true
}
