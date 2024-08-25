package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/Layr-Labs/eigenda-proxy/commitments"
	"github.com/Layr-Labs/eigenda-proxy/utils"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
)

// Router ... storage backend routing layer
type Router struct {
	log     log.Logger
	eigenda KeyGeneratedStore
	s3      PrecomputedKeyStore

	caches    []PrecomputedKeyStore
	fallbacks []PrecomputedKeyStore
}

func NewRouter(eigenda KeyGeneratedStore, s3 PrecomputedKeyStore, l log.Logger,
	caches []PrecomputedKeyStore, fallbacks []PrecomputedKeyStore) (*Router, error) {
	return &Router{
		log:       l,
		eigenda:   eigenda,
		s3:        s3,
		caches:    caches,
		fallbacks: fallbacks,
	}, nil
}

// Get ... fetches a value from a storage backend based on the (commitment mode, type)
func (r *Router) Get(ctx context.Context, key []byte, cm commitments.CommitmentMode) ([]byte, error) {
	switch cm {
	case commitments.OptimismGeneric:

		if r.s3 == nil {
			return nil, errors.New("expected S3 backend for OP keccak256 commitment type, but none configured")
		}

		r.log.Debug("Retrieving data from S3 backend")
		value, err := r.s3.Get(ctx, key)
		if err != nil {
			return nil, err
		}

		err = r.s3.Verify(key, value)
		if err != nil {
			return nil, err
		}
		return value, nil

	case commitments.SimpleCommitmentMode, commitments.OptimismAltDA:
		if r.cacheEnabled() {
			r.log.Debug("Retrieving data from cached backends")
			data, err := r.multiSourceRead(ctx, key, false)
			if err == nil {
				// always verify cached data against EigenDA
				err = r.eigenda.Verify(key, data)
				if err != nil {
					log.Warn("Failed to verify data from cache", "err", err)
				} else {
					return data, nil
				}
			}
		}
		data, err := r.eigenda.Get(ctx, key)
		if err != nil && r.fallbackEnabled() { // rollover to fallbacks if data is non-retrievable from eigenda
			data, err = r.multiSourceRead(ctx, key, true)
			if err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}

		err = r.eigenda.Verify(key, data)
		if err != nil {
			return nil, err
		}

		return data, err

	default:
		return nil, errors.New("could not determine which storage backend to route to based on unknown commitment mode")
	}
}

// Put ... inserts a value into a storage backend based on the commitment mode
func (r *Router) Put(ctx context.Context, cm commitments.CommitmentMode, key, value []byte) ([]byte, error) {
	var commit []byte
	var err error

	switch cm {
	case commitments.OptimismGeneric:
		commit, err = r.PutWithKey(ctx, key, value)
	case commitments.OptimismAltDA, commitments.SimpleCommitmentMode:
		commit, err = r.PutWithoutKey(ctx, value)
	default:
		return nil, fmt.Errorf("unknown commitment mode")
	}

	if err != nil {
		return nil, err
	}

	err = r.handleRedundantWrites(ctx, commit, value)
	if err != nil {
		log.Error("Failed to write to backends", "err", err)
	}

	return commit, nil
}

// handleRedundantWrites ... writes to both sets of backends and returns an error if none of them succeed
func (r *Router) handleRedundantWrites(ctx context.Context, commitment []byte, value []byte) error {
	sources := r.caches
	sources = append(sources, r.fallbacks...)

	if len(sources) == 0 {
		return nil
	}

	key := crypto.Keccak256(commitment)
	successes := 0

	for _, src := range sources {
		err := src.Put(ctx, key, value)
		if err != nil {
			r.log.Warn("Failed to write to fallback", "backend", src.Backend(), "err", err)
		} else {
			successes++
		}
	}

	if successes == 0 {
		return errors.New("failed to write to any fallback backends")
	}

	return nil
}

// multiSourceRead ... reads from a set of backends and returns the first successfully read blob
func (r *Router) multiSourceRead(ctx context.Context, commitment []byte, fallback bool) ([]byte, error) {
	sources := r.caches
	if fallback {
		sources = r.fallbacks
	}

	if len(sources) == 0 {
		return nil, errors.New("no fallback backends configured")
	}

	key := crypto.Keccak256(commitment)
	for _, fb := range sources {
		data, err := fb.Get(ctx, key)
		if err == nil {
			return data, nil
		}
	}
	return nil, errors.New("no data found in any fallback backend")
}

// PutWithoutKey ... inserts a value into a storage backend that computes the key on-demand
func (r *Router) PutWithoutKey(ctx context.Context, value []byte) ([]byte, error) {
	if r.eigenda != nil {
		r.log.Debug("Storing data to EigenDA backend")
		return r.eigenda.Put(ctx, value)
	}

	if r.s3 != nil {
		r.log.Debug("Storing data to S3 backend")
		commitment := crypto.Keccak256(value)

		err := r.s3.Put(ctx, commitment, value)
		if err != nil {
			return nil, err
		}
	}

	return nil, errors.New("no DA storage backend found")
}

// PutWithKey ... only supported for S3 storage backends using OP's alt-da keccak256 commitment type
func (r *Router) PutWithKey(ctx context.Context, key []byte, value []byte) ([]byte, error) {
	if r.s3 == nil {
		return nil, errors.New("S3 is disabled but is only supported for posting known commitment keys")
	}
	// key should be a hash of the pre-image value
	if actualHash := crypto.Keccak256(value); !utils.EqualSlices(actualHash, key) {
		return nil, fmt.Errorf("provided key isn't the result of Keccak256(preimage); expected: %s, actual: %s", hexutil.Encode(key), crypto.Keccak256(value))
	}

	return key, r.s3.Put(ctx, key, value)
}

func (r *Router) fallbackEnabled() bool {
	return len(r.fallbacks) > 0
}

func (r *Router) cacheEnabled() bool {
	return len(r.caches) > 0
}

// GetEigenDAStore ...
func (r *Router) GetEigenDAStore() KeyGeneratedStore {
	return r.eigenda
}

// GetS3Store ...
func (r *Router) GetS3Store() PrecomputedKeyStore {
	return r.s3
}
