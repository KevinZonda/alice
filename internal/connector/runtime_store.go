package connector

import "sync"

// runtimeStore groups mutable connector runtime state in one place so App can
// focus on orchestration instead of map-level bookkeeping.
type runtimeStore struct {
	mu          sync.Mutex
	latest      map[string]uint64
	pending     map[string]Job
	mediaWindow map[string][]mediaWindowEntry
	sessionMu   map[string]*sync.Mutex

	runtimeStatePath           string
	runtimeStateVersion        uint64
	runtimeStateFlushedVersion uint64
}

func newRuntimeStore() *runtimeStore {
	return &runtimeStore{
		latest:      make(map[string]uint64),
		pending:     make(map[string]Job),
		mediaWindow: make(map[string][]mediaWindowEntry),
		sessionMu:   make(map[string]*sync.Mutex),
	}
}
