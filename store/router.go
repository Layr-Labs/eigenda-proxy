//go:generate mockgen -package mocks --destination ../mocks/router.go . IRouter

package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/Layr-Labs/eigenda-proxy/commitments"
	"github.com/ethereum/go-ethereum/log"
)

type IRouter interface {
	Get(ctx context.Context, key []byte, cm commitments.CommitmentMode) ([]byte, error)
	Put(ctx context.Context, cm commitments.CommitmentMode, key, value []byte) ([]byte, error)

	GetEigenDAStore() GeneratedKeyStore
	GetS3Store() PrecomputedKeyStore
	Caches() []PrecomputedKeyStore
	Fallbacks() []PrecomputedKeyStore
}

// Router ... storage backend routing layer
type Router struct {
	log log.Logger
	// primary storage backends
	eigenda GeneratedKeyStore   // ALT DA commitment type for OP mode && simple commitment mode for standard /client
	s3      PrecomputedKeyStore // OP commitment mode && keccak256 commitment type

	// secondary storage backends (caching and fallbacks)
	secondary ISecondary
}

func NewRouter(eigenda GeneratedKeyStore, s3 PrecomputedKeyStore, l log.Logger,
	secondary ISecondary) (IRouter, error) {
	return &Router{
		log:       l,
		eigenda:   eigenda,
		s3:        s3,
		secondary: secondary,
	}, nil
}

// Get ... fetches a value from a storage backend based on the (commitment mode, type)
func (r *Router) Get(ctx context.Context, key []byte, cm commitments.CommitmentMode) ([]byte, error) {
	switch cm {
	case commitments.OptimismKeccak:

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

	case commitments.SimpleCommitmentMode, commitments.OptimismGeneric:
		if r.eigenda == nil {
			return nil, errors.New("expected EigenDA backend for DA commitment type, but none configured")
		}

		// 1 - read blob from cache if enabled
		if r.secondary.CachingEnabled() {
			r.log.Debug("Retrieving data from cached backends")
			data, err := r.secondary.MultiSourceRead(ctx, key, false, r.eigenda.Verify)
			if err == nil {
				return data, nil
			}

			r.log.Warn("Failed to read from cache targets", "err", err)
		}

		// 2 - read blob from EigenDA
		data, err := r.eigenda.Get(ctx, key)
		if err == nil {
			// verify
			err = r.eigenda.Verify(key, data)
			if err != nil {
				return nil, err
			}
			return data, nil
		}

		// 3 - read blob from fallbacks if enabled and data is non-retrievable from EigenDA
		if r.secondary.FallbackEnabled() {
			data, err = r.secondary.MultiSourceRead(ctx, key, true, r.eigenda.Verify)
			if err != nil {
				r.log.Error("Failed to read from fallback targets", "err", err)
				return nil, err
			}
		} else {
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
	case commitments.OptimismKeccak: // caching and fallbacks are unsupported for this commitment mode
		return r.putWithKey(ctx, key, value)
	case commitments.OptimismGeneric, commitments.SimpleCommitmentMode:
		commit, err = r.putWithoutKey(ctx, value)
	default:
		return nil, fmt.Errorf("unknown commitment mode")
	}

	if err != nil {
		return nil, err
	}

	if r.secondary.Enabled() {

		r.secondary.Ingress() <- PutNotif{
			Commitment: commit,
			Value:      value,
		}
	}

	return commit, nil
}

// putWithoutKey ... inserts a value into a storage backend that computes the key on-demand (i.e, EigenDA)
func (r *Router) putWithoutKey(ctx context.Context, value []byte) ([]byte, error) {
	if r.eigenda != nil {
		r.log.Debug("Storing data to EigenDA backend")
		return r.eigenda.Put(ctx, value)
	}

	return nil, errors.New("no DA storage backend found")
}

// putWithKey ... only supported for S3 storage backends using OP's alt-da keccak256 commitment type
func (r *Router) putWithKey(ctx context.Context, key []byte, value []byte) ([]byte, error) {
	if r.s3 == nil {
		return nil, errors.New("S3 is disabled but is only supported for posting known commitment keys")
	}

	err := r.s3.Verify(key, value)
	if err != nil {
		return nil, err
	}

	return key, r.s3.Put(ctx, key, value)
}

// GetEigenDAStore ...
func (r *Router) GetEigenDAStore() GeneratedKeyStore {
	return r.eigenda
}

// GetS3Store ...
func (r *Router) GetS3Store() PrecomputedKeyStore {
	return r.s3
}

// Caches ...
func (r *Router) Caches() []PrecomputedKeyStore {
	return r.secondary.Caches()
}

// Fallbacks ...
func (r *Router) Fallbacks() []PrecomputedKeyStore {
	return r.secondary.Fallbacks()
}
