package metrics

import (
	"fmt"
	"sort"
	"sync"

	"github.com/ethereum-optimism/optimism/op-service/metrics"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

func keyLabels(labels []string) (common.Hash, error) {
	sort.Strings(labels) // in-place sort strings so keys are order agnostic

	encodedBytes, err := rlp.EncodeToBytes(labels)
	if err != nil {
		return common.Hash{}, err
	}

	hash := crypto.Keccak256Hash(encodedBytes)

	return hash, nil
}

type MetricCountMap struct {
	m *sync.Map
}

func NewCountMap() *MetricCountMap {
	return &MetricCountMap{
		m: new(sync.Map),
	}
}

func (mcm *MetricCountMap) insert(values ...string) error {
	key, err := keyLabels(values)

	if err != nil {
		return err
	}

	// update or add count entry
	value, exists := mcm.m.Load(key.Hex())
	if !exists {
		mcm.m.Store(key.Hex(), uint64(1))
		return nil
	}
	uint64Val, ok := value.(uint64)
	if !ok {
		return fmt.Errorf("could not read uint64 from sync map")
	}

	mcm.m.Store(key.Hex(), uint64Val+uint64(1))
	return nil
}

func (mcm *MetricCountMap) Find(values ...string) (uint64, error) {
	key, err := keyLabels(values)

	if err != nil {
		return 0, err
	}

	val, exists := mcm.m.Load(key.Hex())
	if !exists {
		return 0, fmt.Errorf("value doesn't exist")
	}
	uint64Val, ok := val.(uint64)
	if !ok {
		return 0, fmt.Errorf("could not read uint64 from sync map")
	}

	return uint64Val, nil
}

type InMemoryMetricer struct {
	HTTPServerRequestsTotal *MetricCountMap
	// secondary metrics
	SecondaryRequestsTotal *MetricCountMap
}

func NewInMemoryMetricer() *InMemoryMetricer {
	return &InMemoryMetricer{
		HTTPServerRequestsTotal: NewCountMap(),
		SecondaryRequestsTotal:  NewCountMap(),
	}
}

var _ Metricer = NewInMemoryMetricer()

func (n *InMemoryMetricer) Document() []metrics.DocumentedMetric {
	return nil
}

func (n *InMemoryMetricer) RecordInfo(_ string) {
}

func (n *InMemoryMetricer) RecordUp() {
}

func (n *InMemoryMetricer) RecordRPCServerRequest(endpoint string) func(status, mode, ver string) {
	return func(x string, y string, z string) {
		err := n.HTTPServerRequestsTotal.insert(endpoint, x, y, z)
		if err != nil {
			panic(err)
		}
	}
}

func (n *InMemoryMetricer) RecordSecondaryRequest(x string, y string) func(status string) {
	return func(z string) {
		err := n.SecondaryRequestsTotal.insert(x, y, z)
		if err != nil {
			panic(err)
		}
	}
}
