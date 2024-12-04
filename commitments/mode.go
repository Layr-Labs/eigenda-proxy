package commitments

import (
	"fmt"
)

type CommitmentMeta struct {
	Mode CommitmentMode
	// CertVersion is shared for all modes and denotes version of the EigenDA certificate
	CertVersion uint8
}

type CommitmentMode string

const (
	OptimismKeccak       CommitmentMode = "optimism_keccak256"
	OptimismGeneric      CommitmentMode = "optimism_generic"
	SimpleCommitmentMode CommitmentMode = "simple"
)

func StringToCommitmentMode(s string) (CommitmentMode, error) {
	switch s {
	case string(OptimismKeccak):
		return OptimismKeccak, nil
	case string(OptimismGeneric):
		return OptimismGeneric, nil
	case string(SimpleCommitmentMode):
		return SimpleCommitmentMode, nil
	default:
		return "", fmt.Errorf("unknown commitment mode: %s", s)
	}
}

func EncodeCommitment(b []byte, c CommitmentMode, v CertEncodingCommitment) ([]byte, error) {
	switch c {
	case OptimismKeccak:
		return Keccak256Commitment(b).Encode(), nil

	case OptimismGeneric:
		certCommit, err := NewCertCommitment(b, v)
		if err != nil {
			return nil, err
		}
		svcCommit := EigenDASvcCommitment(certCommit.Encode()).Encode()
		altDACommit := NewGenericCommitment(svcCommit).Encode()
		return altDACommit, nil

	case SimpleCommitmentMode:
		certCommit, err := NewCertCommitment(b, v)
		if err != nil {
			return nil, err
		}

		return certCommit.Encode(), nil
	}

	return nil, fmt.Errorf("unknown commitment mode")
}
