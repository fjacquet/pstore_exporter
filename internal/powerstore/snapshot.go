package powerstore

import (
	"sync"
	"time"
)

// ArraySnapshot is the collected state for a single array at one collection cycle.
type ArraySnapshot struct {
	Array       string
	Up          bool
	BulkCapable bool
	ScrapeError string
	LastScrape  time.Time
	Samples     []Sample
}

// Snapshot is an immutable view of all arrays' samples, indexed by metric name.
type Snapshot struct {
	PerArray map[string]*ArraySnapshot
	byName   map[string][]Sample
	names    []string
}

// BuildSnapshot assembles an immutable Snapshot from per-array results.
func BuildSnapshot(arrays []*ArraySnapshot) *Snapshot {
	snap := &Snapshot{
		PerArray: make(map[string]*ArraySnapshot, len(arrays)),
		byName:   make(map[string][]Sample),
	}
	for _, as := range arrays {
		if as == nil {
			continue
		}
		snap.PerArray[as.Array] = as
		for _, s := range as.Samples {
			snap.byName[s.Name] = append(snap.byName[s.Name], s)
		}
	}
	snap.names = make([]string, 0, len(snap.byName))
	for name := range snap.byName {
		snap.names = append(snap.names, name)
	}
	return snap
}

// SamplesByName returns all samples (across arrays) for a metric name.
func (s *Snapshot) SamplesByName(name string) []Sample { return s.byName[name] }

// MetricNames returns the distinct metric names present in the snapshot.
func (s *Snapshot) MetricNames() []string { return s.names }

// SnapshotStore holds the latest published Snapshot under an RWMutex.
type SnapshotStore struct {
	mu      sync.RWMutex
	current *Snapshot
}

// NewSnapshotStore returns a store seeded with an empty snapshot.
func NewSnapshotStore() *SnapshotStore { return &SnapshotStore{current: BuildSnapshot(nil)} }

// Load returns the current snapshot (safe for concurrent readers).
func (st *SnapshotStore) Load() *Snapshot {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.current
}

// Store publishes a new snapshot.
func (st *SnapshotStore) Store(s *Snapshot) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.current = s
}
