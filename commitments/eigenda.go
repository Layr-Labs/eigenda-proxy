package commitments

import "fmt"

type CertEncodingCommitment byte

const (
	CertV0 CertEncodingCommitment = 0
	CertV1 CertEncodingCommitment = 1
)

// CertCommitment is the binary representation of a commitment.
type CertCommitment interface {
	CommitmentType() CertEncodingCommitment
	Encode() []byte
}

type CertCommitmentV0 []byte
type CertCommitmentV1 []byte

func NewCertCommitment(input []byte, version CertEncodingCommitment) (CertCommitment, error) {
	switch version {
	case CertV0:
		return NewV0CertCommitment(input), nil

	case CertV1:
		return NewV1CertCommitment(input), nil

	default:
		return nil, fmt.Errorf("Invalid cert version provided")
	}
}

// NewV0CertCommitment creates a new commitment from the given input.
func NewV0CertCommitment(input []byte) CertCommitmentV0 {
	return CertCommitmentV0(input)
}

// CommitmentType returns the commitment type of Keccak256.
func (c CertCommitmentV0) CommitmentType() CertEncodingCommitment {
	return CertV0
}

// Encode adds a commitment type prefix self describing the commitment.
func (c CertCommitmentV0) Encode() []byte {
	return append([]byte{byte(CertV0)}, c...)
}

// NewV1CertCommitment creates a new commitment from the given input.
func NewV1CertCommitment(input []byte) CertCommitmentV1 {
	return CertCommitmentV1(input)
}

// CommitmentType returns the commitment type of Keccak256.
func (c CertCommitmentV1) CommitmentType() CertEncodingCommitment {
	return CertV0
}

// Encode adds a commitment type prefix self describing the commitment.
func (c CertCommitmentV1) Encode() []byte {
	return append([]byte{byte(CertV0)}, c...)
}
