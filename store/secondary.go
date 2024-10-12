package store

import (
	"context"
	"errors"
	"sync"

	"github.com/Layr-Labs/eigenda-proxy/metrics"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum/go-ethereum/log"
)

type ISecondary interface {
	Fallbacks() []PrecomputedKeyStore
	Caches() []PrecomputedKeyStore
	Enabled() bool
	Ingress() chan<- PutNotif
	CachingEnabled() bool
	FallbackEnabled() bool
	HandleRedundantWrites(ctx context.Context, commitment []byte, value []byte) error
	MultiSourceRead(context.Context, []byte, bool, func([]byte, []byte) error) ([]byte, error)
	StreamProcess(context.Context)
}

type PutNotif struct {
	Commitment []byte
	Value      []byte
}

// SecondaryRouter ... routing abstraction for secondary storage backends
type SecondaryRouter struct {
	stream chan PutNotif
	log    log.Logger
	m      metrics.Metricer

	caches    []PrecomputedKeyStore
	fallbacks []PrecomputedKeyStore

	verifyLock sync.RWMutex
}

// NewSecondaryRouter ... creates a new secondary storage router
func NewSecondaryRouter(log log.Logger, m metrics.Metricer, caches []PrecomputedKeyStore, fallbacks []PrecomputedKeyStore) (ISecondary, error) {
	return &SecondaryRouter{
		stream:    make(chan PutNotif),
		log:       log,
		m:         m,
		caches:    caches,
		fallbacks: fallbacks,
	}, nil
}

func (r *SecondaryRouter) Ingress() chan<- PutNotif {
	return r.stream
}

func (r *SecondaryRouter) Enabled() bool {
	return r.CachingEnabled() || r.FallbackEnabled()
}

func (r *SecondaryRouter) CachingEnabled() bool {
	return len(r.caches) > 0
}

func (r *SecondaryRouter) FallbackEnabled() bool {
	return len(r.fallbacks) > 0
}

// handleRedundantWrites ... writes to both sets of backends (i.e, fallback, cache)
// and returns an error if NONE of them succeed
// NOTE: multi-target set writes are done at once to avoid re-invocation of the same write function at the same
// caller step for different target sets vs. reading which is done conditionally to segment between a cached read type
// vs a fallback read type
func (r *SecondaryRouter) HandleRedundantWrites(ctx context.Context, commitment []byte, value []byte) error {
	println("HandleRedundantWrites")
	sources := r.caches
	sources = append(sources, r.fallbacks...)

	key := crypto.Keccak256(commitment)
	successes := 0

	for _, src := range sources {
		cb := r.m.RecordSecondaryRequest(src.BackendType().String(), "put")

		err := src.Put(ctx, key, value)
		if err != nil {
			r.log.Warn("Failed to write to redundant target", "backend", src.BackendType(), "err", err)
			cb("failure")
		} else {
			successes++
			cb("success")
		}
	}

	if successes == 0 {
		return errors.New("failed to write blob to any redundant targets")
	}

	return nil
}

func (r *SecondaryRouter) StreamProcess(ctx context.Context) {
	for {
		select {
		case notif := <-r.stream:
			err := r.HandleRedundantWrites(context.Background(), notif.Commitment, notif.Value)
			if err != nil {
				r.log.Error("Failed to write to redundant targets", "err", err)
			}

		case <-ctx.Done():
			return
		}
	}

}

// MultiSourceRead ... reads from a set of backends and returns the first successfully read blob
// NOTE: - this can also be parallelized when reading from multiple sources and discarding connections that fail
//   - for complete optimization we can profile secondary storage backends to determine the fastest / most reliable and always rout to it first
func (r *SecondaryRouter) MultiSourceRead(ctx context.Context, commitment []byte, fallback bool, verify func([]byte, []byte) error) ([]byte, error) {
	var sources []PrecomputedKeyStore
	if fallback {
		sources = r.fallbacks
	} else {
		sources = r.caches
	}

	key := crypto.Keccak256(commitment)
	for _, src := range sources {
		cb := r.m.RecordSecondaryRequest(src.BackendType().String(), "get")
		data, err := src.Get(ctx, key)
		if err != nil {
			cb("failure")
			r.log.Warn("Failed to read from redundant target", "backend", src.BackendType(), "err", err)
			continue
		}

		if data == nil {
			cb("miss")
			r.log.Debug("No data found in redundant target", "backend", src.BackendType())
			continue
		}

		// verify cert:data using provided verification function
		r.verifyLock.Lock()
		err = verify(commitment, data)
		if err != nil {
			cb("failure")
			log.Warn("Failed to verify blob", "err", err, "backend", src.BackendType())
			r.verifyLock.Unlock()
			continue
		}
		r.verifyLock.Unlock()
		cb("success")
		return data, nil
	}
	return nil, errors.New("no data found in any redundant backend")
}

func (r *SecondaryRouter) Fallbacks() []PrecomputedKeyStore {
	return r.fallbacks
}

func (r *SecondaryRouter) Caches() []PrecomputedKeyStore {
	return r.caches
}
