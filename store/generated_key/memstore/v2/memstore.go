package memstore

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/Layr-Labs/eigenda-proxy/common"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore/ephemeral_db"
	"github.com/Layr-Labs/eigenda-proxy/store/generated_key/memstore/memconfig"
	"github.com/Layr-Labs/eigenda/api/clients/codecs"
	"github.com/Layr-Labs/eigenda/api/clients/v2/verification"

	cert_verifier_binding "github.com/Layr-Labs/eigenda/contracts/bindings/EigenDACertVerifier"

	"github.com/Layr-Labs/eigensdk-go/logging"
)

const (
	BytesPerFieldElement = 32
)

// randomBytes ... Generates random byte slice provided
// size. Errors when generating are ignored since this is only
// used for constructing dummy certificates when testing insecure integrations.
// in the worst case it doesn't work and returns empty arrays which would only
// impact memstore operation in the event that two identical payloads are provided
// since they'd resolve to the same commitment and blob key. This shouldn't matter
// given this is typically used for testing standard E2E functionality against a rollup
// stack which SHOULD never submit an identical batch more than once.
func randomBytes(size uint) []byte {
	entropy := make([]byte, size)
	_, _ = rand.Read(entropy)
	return entropy
}

func randInt(max int64) *big.Int {
	randInt, _ := rand.Int(rand.Reader, big.NewInt(max))
	return randInt
}

func randUint32() uint32 {
	return uint32(randInt(32).Uint64())
}

/*
MemStore is a simple in-memory store for blobs which uses an expiration
time to evict blobs to best emulate the ephemeral nature of blobs dispersed to
EigenDA V2 operators.
*/
type MemStore struct {
	*ephemeral_db.DB
	log logging.Logger

	g1SRS []bn254.G1Affine
	codec codecs.BlobCodec
}

var _ common.GeneratedKeyStore = (*MemStore)(nil)

// New ... constructor
func New(
	ctx context.Context, log logging.Logger, config *memconfig.SafeConfig,
	g1SRS []bn254.G1Affine,
) (*MemStore, error) {
	return &MemStore{
		ephemeral_db.New(ctx, config, log),
		log,
		g1SRS,
		codecs.NewIFFTCodec(codecs.NewDefaultBlobCodec()),
	}, nil
}

// Get fetches a value from the store.
func (e *MemStore) Get(_ context.Context, commit []byte) ([]byte, error) {
	var cert verification.EigenDACert
	err := rlp.DecodeBytes(commit, &cert)
	if err != nil {
		return nil, fmt.Errorf("failed to decode DA cert to RLP format: %w", err)
	}

	key, err := json.Marshal(cert)
	if err != nil {
		return nil, err
	}

	encodedBlob, err := e.FetchEphemeralEntry(crypto.Keccak256Hash(key).Bytes())
	if err != nil {
		return nil, fmt.Errorf("fetching entry via v2 memstore: %w", err)
	}

	return e.codec.DecodeBlob(encodedBlob)
}

// Put inserts a value into the store.
// ephemeral db key = keccak256(pseudo_random_cert)
// this is done to verify that a rollup must be able to provide
// the same certificate used in dispersal for retrieval
func (e *MemStore) Put(_ context.Context, value []byte) ([]byte, error) {
	encodedVal, err := e.codec.EncodeBlob(value)
	if err != nil {
		return nil, err
	}

	// compute kzg data commitment. this is useful for testing
	// READPREIMAGE functionality in the arbitrum x eigenda integration since
	// preimage key is computed within the VM from hashing a recomputation of the data
	// commitment
	dataCommitment, err := verification.GenerateBlobCommitment(e.g1SRS, encodedVal)
	if err != nil {
		return nil, err
	}

	x := dataCommitment.X.BigInt(&big.Int{})
	y := dataCommitment.Y.BigInt(&big.Int{})

	g1CommitPoint := cert_verifier_binding.BN254G1Point{
		X: x,
		Y: y,
	}

	pseudoRandomBlobInclusionInfo := cert_verifier_binding.BlobInclusionInfo{
		BlobCertificate: cert_verifier_binding.BlobCertificate{
			BlobHeader: cert_verifier_binding.BlobHeaderV2{
				Version:       0,                            // only supported version as of now
				QuorumNumbers: []byte{byte(0x0), byte(0x1)}, // quorum 0 && quorum 1
				Commitment: cert_verifier_binding.BlobCommitment{
					LengthCommitment: cert_verifier_binding.BN254G2Point{
						X: [2]*big.Int{randInt(1000), randInt(1000)},
						Y: [2]*big.Int{randInt(1000), randInt(1000)},
					},
					LengthProof: cert_verifier_binding.BN254G2Point{
						X: [2]*big.Int{randInt(1), randInt(1)},
						Y: [2]*big.Int{randInt(1), randInt(1)},
					},
					Commitment: g1CommitPoint,
					Length:     uint32(len(encodedVal)),
				},
				PaymentHeaderHash: [32]byte(randomBytes(32)),
			},
			Signature: randomBytes(48), // 384 bits
			RelayKeys: []uint32{randUint32(), randUint32()},
		},
		BlobIndex:      uint32(randInt(1_000).Uint64()),
		InclusionProof: randomBytes(128),
	}

	randomBatchHeader := cert_verifier_binding.BatchHeaderV2{
		BatchRoot:            [32]byte(randomBytes(32)),
		ReferenceBlockNumber: randUint32(),
	}

	randomNonSignerStakesAndSigs := cert_verifier_binding.NonSignerStakesAndSignature{
		NonSignerQuorumBitmapIndices: []uint32{randUint32(), randUint32()},
		NonSignerPubkeys: []cert_verifier_binding.BN254G1Point{
			cert_verifier_binding.BN254G1Point{
				X: randInt(1000),
				Y: randInt(1000),
			},
		},
		QuorumApks: []cert_verifier_binding.BN254G1Point{
			cert_verifier_binding.BN254G1Point{
				X: randInt(1000),
				Y: randInt(1000),
			},
		},
		ApkG2: cert_verifier_binding.BN254G2Point{
			X: [2]*big.Int{randInt(1000), randInt(10000)},
			Y: [2]*big.Int{randInt(1000), randInt(1000)},
		},
		QuorumApkIndices:  []uint32{randUint32(), randUint32()},
		TotalStakeIndices: []uint32{randUint32(), randUint32(), randUint32()},
		NonSignerStakeIndices: [][]uint32{
			[]uint32{randUint32(), randUint32()},
			[]uint32{randUint32(), randUint32()},
		},
		Sigma: cert_verifier_binding.BN254G1Point{
			X: randInt(1000),
			Y: randInt(1000),
		},
	}

	artificialV2Cert := verification.EigenDACert{
		BlobInclusionInfo:           pseudoRandomBlobInclusionInfo,
		BatchHeader:                 randomBatchHeader,
		NonSignerStakesAndSignature: randomNonSignerStakesAndSigs,
	}

	b, err := json.Marshal(artificialV2Cert)
	if err != nil {
		return nil, fmt.Errorf("unmarshal V2 cert: %w", err)
	}

	err = e.InsertEphemeralEntry(crypto.Keccak256Hash(b).Bytes(), encodedVal)
	if err != nil { // don't wrap here so api.ErrorFailover{} isn't modified
		return nil, err
	}

	certBytes, err := rlp.EncodeToBytes(artificialV2Cert)
	if err != nil {
		return nil, fmt.Errorf("rlp decode v2 cert: %w", err)
	}

	return certBytes, nil
}

func (e *MemStore) Verify(_ context.Context, _, _ []byte) error {
	return nil
}

func (e *MemStore) BackendType() common.BackendType {
	return common.MemstoreV2BackendType
}
