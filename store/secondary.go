package store

import (
	"context"
	"errors"
	"sync"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum/go-ethereum/log"
)

type ISecondary interface {
	Fallbacks() []PrecomputedKeyStore
	Caches() []PrecomputedKeyStore
	Enabled() bool
	CachingEnabled() bool
	FallbackEnabled() bool
	HandleRedundantWrites(ctx context.Context, commitment []byte, value []byte) error
	MultiSourceRead(context.Context, []byte, bool, func([]byte, []byte) error) ([]byte, error)
}

// SecondaryRouter ... routing abstraction for secondary storage backends
type SecondaryRouter struct {
	log log.Logger

	caches    []PrecomputedKeyStore
	cacheLock sync.RWMutex

	fallbacks    []PrecomputedKeyStore
	fallbackLock sync.RWMutex
}

// NewSecondaryRouter ... creates a new secondary storage router
func NewSecondaryRouter(log log.Logger, caches []PrecomputedKeyStore, fallbacks []PrecomputedKeyStore) (ISecondary, error) {
	return &SecondaryRouter{
		log:       log,
		caches:    caches,
		cacheLock: sync.RWMutex{},

		fallbacks:    fallbacks,
		fallbackLock: sync.RWMutex{},
	}, nil
}

func (r *SecondaryRouter) Enabled() bool {
	return r.CachingEnabled() || r.FallbackEnabled()
}

func (r *SecondaryRouter) CachingEnabled() bool {
	r.cacheLock.RLock()
	defer r.cacheLock.RUnlock()

	return len(r.caches) > 0
}

func (r *SecondaryRouter) FallbackEnabled() bool {
	r.fallbackLock.RLock()
	defer r.fallbackLock.RUnlock()

	return len(r.fallbacks) > 0
}

// handleRedundantWrites ... writes to both sets of backends (i.e, fallback, cache)
// and returns an error if NONE of them succeed
// NOTE: multi-target set writes are done at once to avoid re-invocation of the same write function at the same
// caller step for different target sets vs. reading which is done conditionally to segment between a cached read type
// vs a fallback read type
func (r *SecondaryRouter) HandleRedundantWrites(ctx context.Context, commitment []byte, value []byte) error {
	r.cacheLock.RLock()
	r.fallbackLock.RLock()

	defer func() {
		r.cacheLock.RUnlock()
		r.fallbackLock.RUnlock()
	}()

	sources := r.caches
	sources = append(sources, r.fallbacks...)

	key := crypto.Keccak256(commitment)
	successes := 0

	for _, src := range sources {
		err := src.Put(ctx, key, value)
		if err != nil {
			r.log.Warn("Failed to write to redundant target", "backend", src.BackendType(), "err", err)
		} else {
			successes++
		}
	}

	if successes == 0 {
		return errors.New("failed to write blob to any redundant targets")
	}

	return nil
}

// MultiSourceRead ... reads from a set of backends and returns the first successfully read blob
func (r *SecondaryRouter) MultiSourceRead(ctx context.Context, commitment []byte, fallback bool, verify func([]byte, []byte) error) ([]byte, error) {
	var sources []PrecomputedKeyStore
	if fallback {
		r.fallbackLock.RLock()
		defer r.fallbackLock.RUnlock()

		sources = r.fallbacks
	} else {
		r.cacheLock.RLock()
		defer r.cacheLock.RUnlock()

		sources = r.caches
	}

	key := crypto.Keccak256(commitment)
	for _, src := range sources {
		data, err := src.Get(ctx, key)
		if err != nil {
			r.log.Warn("Failed to read from redundant target", "backend", src.BackendType(), "err", err)
			continue
		}

		if data == nil {
			r.log.Debug("No data found in redundant target", "backend", src.BackendType())
			continue
		}

		// verify cert:data using provided verification function
		err = verify(commitment, data)
		if err != nil {
			log.Warn("Failed to verify blob", "err", err, "backend", src.BackendType())
			continue
		}

		return data, nil
	}
	return nil, errors.New("no data found in any redundant backend")
}

func (r *SecondaryRouter) Fallbacks() []PrecomputedKeyStore {
	r.fallbackLock.RLock()
	defer r.fallbackLock.RUnlock()

	return r.fallbacks
}

func (r *SecondaryRouter) Caches() []PrecomputedKeyStore {
	r.cacheLock.RLock()
	defer r.cacheLock.RUnlock()

	return r.caches
}
