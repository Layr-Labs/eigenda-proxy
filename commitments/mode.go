package commitments

import (
	"fmt"
)

type CommitmentMode string

const (
	OptimismKeccakCommitmentMode  CommitmentMode = "optimism_keccak256"
	OptimismGenericCommitmentMode CommitmentMode = "optimism_generic"
	StandardCommitmentMode        CommitmentMode = "standard"
)

func EncodeCommitment(
	versionedCert EigenDAVersionedCert,
	commitmentMode CommitmentMode,
) ([]byte, error) {
	switch commitmentMode {
	case OptimismKeccakCommitmentMode:
		return OPKeccak256Commitment(versionedCert.SerializedCert).Encode(), nil
	case OptimismGenericCommitmentMode:
		// Proxy returns an altDACommitment, which doesn't contain the first op version_byte
		// (from https://specs.optimism.io/experimental/alt-da.html#example-commitments)
		// This is because the version_byte is added by op-alt-da when calling TxData() right before submitting the tx:
		// https://github.com/Layr-Labs/optimism/blob/89ac40d0fddba2e06854b253b9f0266f36350af2/op-alt-da/commitment.go#L158-L160
		return NewOPEigenDAGenericCommitment(versionedCert).Encode(), nil
	case StandardCommitmentMode:
		return NewStandardCommitment(versionedCert).Encode(), nil
	}
	return nil, fmt.Errorf("unknown commitment mode")
}

type DAServiceOPCommitmentType byte

const (
	EigenDAOPCommitmentType DAServiceOPCommitmentType = 0
)

type EigenDASvcCommitment []byte

// Encode adds a commitment type prefix self describing the commitment.
func (c EigenDASvcCommitment) Encode() []byte {
	return append([]byte{byte(EigenDAOPCommitmentType)}, c...)
}
