//go:generate mockgen -package mocks --destination ../mocks/manager.go . IManager

package store

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/Layr-Labs/eigenda-proxy/commitments"
	"github.com/Layr-Labs/eigenda-proxy/common"
	"github.com/Layr-Labs/eigensdk-go/logging"
)

// IManager ... read/write interface
type IManager interface {
	Get(ctx context.Context, key []byte, cm commitments.CommitmentMeta) ([]byte, error)
	Put(ctx context.Context, cm commitments.CommitmentMode, key, value []byte) ([]byte, error)
	SetDisperseToV2(disperseToV2 bool)
	DisperseToV2() bool
}

// Manager ... storage backend routing layer
type Manager struct {
	log logging.Logger

	s3 common.PrecomputedKeyStore // for op keccak256 commitment
	// For op generic commitments & standard commitments
	eigenda      common.GeneratedKeyStore // v0 da commitment version
	eigendaV2    common.GeneratedKeyStore // v1 da commitment version
	disperseToV2 *atomic.Bool             // write blobs to EigenDAV2 backend

	// secondary storage backends (caching and fallbacks)
	secondary ISecondary
}

var _ IManager = &Manager{}

// DisperseToV2 returns whether v2 dispersal is enabled
func (m *Manager) DisperseToV2() bool {
	return m.disperseToV2.Load()
}

// SetDisperseToV2 sets whether v2 dispersal is enabled.
//
// If set to true, the manager will disperse to eigenDA v2. If false, it will disperse to eigenDA v1
func (m *Manager) SetDisperseToV2(disperseToV2 bool) {
	m.disperseToV2.Store(disperseToV2)
}

// NewManager ... Init
func NewManager(
	eigenda common.GeneratedKeyStore,
	eigenDAV2 common.GeneratedKeyStore,
	s3 common.PrecomputedKeyStore,
	l logging.Logger,
	secondary ISecondary,
	disperseToV2 bool,
) (*Manager, error) {
	// Enforce invariants
	if disperseToV2 && eigenDAV2 == nil {
		return nil, fmt.Errorf("EigenDA V2 dispersal enabled but no v2 store provided")
	}

	if !disperseToV2 && eigenda == nil {
		return nil, fmt.Errorf("EigenDA dispersal enabled but no store provided")
	}

	disperseToV2Atomic := &atomic.Bool{}
	disperseToV2Atomic.Store(disperseToV2)

	return &Manager{
		log:          l,
		eigenda:      eigenda,
		eigendaV2:    eigenDAV2,
		s3:           s3,
		secondary:    secondary,
		disperseToV2: disperseToV2Atomic,
	}, nil
}

// Get ... fetches a value from a storage backend based on the (commitment mode, type)
func (m *Manager) Get(ctx context.Context, key []byte, cm commitments.CommitmentMeta) ([]byte, error) {
	switch cm.Mode {
	case commitments.OptimismKeccak:

		if m.s3 == nil {
			return nil, errors.New("expected S3 backend for OP keccak256 commitment type, but none configured")
		}

		// 1 - read blob from S3 backend
		m.log.Debug("Retrieving data from S3 backend")
		value, err := m.s3.Get(ctx, key)
		if err != nil {
			return nil, err
		}

		// 2 - verify blob hash against commitment key digest
		err = m.s3.Verify(ctx, key, value)
		if err != nil {
			return nil, err
		}
		return value, nil

	case commitments.Standard, commitments.OptimismGeneric:
		if cm.Version == commitments.CertV0 && m.eigenda == nil {
			return nil, errors.New("expected EigenDA V1 backend for DA commitment type with CertV0")
		}
		if cm.Version == commitments.CertV1 && m.eigendaV2 == nil {
			return nil, errors.New("expected EigenDA V2 backend for DA commitment type with CertV1")
		}

		verifyMethod, err := m.getVerifyMethod(cm.Version)
		if err != nil {
			return nil, fmt.Errorf("get verify method: %w", err)
		}

		// 1 - read blob from cache if enabled
		if m.secondary.CachingEnabled() {
			m.log.Debug("Retrieving data from cached backends")
			data, err := m.secondary.MultiSourceRead(ctx, key, false, verifyMethod)
			if err == nil {
				return data, nil
			}

			m.log.Warn("Failed to read from cache targets", "err", err)
		}

		// 2 - read blob from EigenDA
		data, err := m.getEigenDAMode(ctx, cm.Version, key)
		if err == nil {
			return data, nil
		}

		m.log.Error(err.Error())

		// 3 - read blob from fallbacks if enabled and data is non-retrievable from EigenDA
		if m.secondary.FallbackEnabled() {
			data, err = m.secondary.MultiSourceRead(ctx, key, true, verifyMethod)
			if err != nil {
				m.log.Error("Failed to read from fallback targets", "err", err)
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
func (m *Manager) Put(ctx context.Context, cm commitments.CommitmentMode, key, value []byte) ([]byte, error) {
	var commit []byte
	var err error

	// 1 - Put blob into primary storage backend
	switch cm {
	case commitments.OptimismKeccak: // caching and fallbacks are unsupported for this commitment mode
		return m.putKeccak256Mode(ctx, key, value)
	case commitments.OptimismGeneric, commitments.Standard:
		commit, err = m.putEigenDAMode(ctx, value)
	default:
		return nil, fmt.Errorf("unknown commitment mode")
	}

	if err != nil {
		return nil, err
	}

	// 2 - Put blob into secondary storage backends
	if m.secondary.Enabled() &&
		m.secondary.AsyncWriteEntry() { // publish put notification to secondary's subscription on PutNotify topic
		m.log.Debug("Publishing data to async secondary stores")
		m.secondary.Topic() <- PutNotify{
			Commitment: commit,
			Value:      value,
		}
		// secondary is available only for synchronous writes
	} else if m.secondary.Enabled() && !m.secondary.AsyncWriteEntry() {
		m.log.Debug("Publishing data to single threaded secondary stores")
		err := m.secondary.HandleRedundantWrites(ctx, commit, value)
		if err != nil {
			m.log.Error("Secondary insertions failed", "error", err.Error())
		}
	}

	return commit, nil
}

// getVerifyMethod returns the correct verify method based on commitment type
func (m *Manager) getVerifyMethod(commitmentType commitments.EigenDACommitmentType) (
	func(context.Context, []byte, []byte) error,
	error,
) {
	switch commitmentType {
	case commitments.CertV0:
		return m.eigenda.Verify, nil
	case commitments.CertV1:
		return m.eigendaV2.Verify, nil
	default:
		return nil, fmt.Errorf("commitment version unknown: %b", commitmentType)
	}
}

// putEigenDAMode ... disperses blob to EigenDA backend
func (m *Manager) putEigenDAMode(ctx context.Context, value []byte) ([]byte, error) {
	if !m.disperseToV2.Load() { // disperse v1
		m.log.Info("Storing data to EigenDA V1 backend")
		return m.eigenda.Put(ctx, value)
	}

	return m.eigendaV2.Put(ctx, value)
}

func (m *Manager) getEigenDAMode(
	ctx context.Context,
	commitmentType commitments.EigenDACommitmentType,
	key []byte,
) ([]byte, error) {
	switch commitmentType {
	case commitments.CertV0:
		m.log.Debug("Reading blob from EigenDAV1 backend")
		data, err := m.eigenda.Get(ctx, key)
		if err == nil {
			// verify v1 (payload, cert)
			err = m.eigenda.Verify(ctx, key, data)
			if err != nil {
				return nil, err
			}
			return data, nil
		}

		return nil, err

	case commitments.CertV1:
		// the cert must be verified before attempting to get the data, since the get logic assumes the cert is valid
		// verify v2 doesn't require a value, the "key" is the full cert
		err := m.eigendaV2.Verify(ctx, key, nil)
		if err != nil {
			return nil, fmt.Errorf("verify EigenDACert: %w", err)
		}

		m.log.Debug("Reading blob from EigenDAV2 backend")
		data, err := m.eigendaV2.Get(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("get data from V2 backend: %w", err)
		}

		return data, nil
	default:
		return nil, fmt.Errorf("commitment version unknown: %b", commitmentType)
	}
}

// putKeccak256Mode ... put blob into S3 compatible backend
func (m *Manager) putKeccak256Mode(ctx context.Context, key []byte, value []byte) ([]byte, error) {
	if m.s3 == nil {
		return nil, errors.New("S3 is disabled but is only supported for posting known commitment keys")
	}

	err := m.s3.Verify(ctx, key, value)
	if err != nil {
		return nil, err
	}

	return key, m.s3.Put(ctx, key, value)
}
