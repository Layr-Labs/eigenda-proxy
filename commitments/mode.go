package commitments

import (
	"fmt"
	"strings"
)

// EncodingType represents the serialization format for the certificate
type EncodingType byte

const (
	// RLPEncoding represents RLP encoding (default)
	RLPEncoding EncodingType = iota
	// ABIVerifyV2CertEncoding represents ABI encoding
	ABIVerifyV2CertEncoding
)

// The values used here should match the one in ParseEncodingQueryParam
func (e EncodingType) QueryParamValue() string {
	switch e {
	case RLPEncoding:
		return "rlp"
	case ABIVerifyV2CertEncoding:
		return "abi"
	default:
		return "unknown"
	}
}

func ParseEncodingQueryParam(s string) (EncodingType, error) {
	switch strings.ToLower(s) {
	case "rlp":
		return RLPEncoding, nil
	case "abi":
		return ABIVerifyV2CertEncoding, nil
	default:
		return RLPEncoding, fmt.Errorf("unknown encoding type: %s, using default RLP encoding", s)
	}
}

type CommitmentMeta struct {
	Mode CommitmentMode
	// version is shared for all modes and denotes version of the EigenDA certificate
	Version EigenDACommitmentType
	// encoding type for the certificate (defaults to RLPEncoding)
	Encoding EncodingType
}

type CommitmentMode string

const (
	OptimismKeccak  CommitmentMode = "optimism_keccak256"
	OptimismGeneric CommitmentMode = "optimism_generic"
	Standard        CommitmentMode = "standard"
)

func StringToCommitmentMode(s string) (CommitmentMode, error) {
	switch s {
	case string(OptimismKeccak):
		return OptimismKeccak, nil
	case string(OptimismGeneric):
		return OptimismGeneric, nil
	case string(Standard):
		return Standard, nil
	default:
		return "", fmt.Errorf("unknown commitment mode: %s", s)
	}
}

func EncodeCommitment(
	bytes []byte,
	cm CommitmentMeta,
) ([]byte, error) {
	switch cm.Mode {
	case OptimismKeccak:
		return Keccak256Commitment(bytes).Encode(), nil

	case OptimismGeneric:
		// For V2 certificates with encoding specified, add encoding byte
		if cm.Version >= CertV2 {
			certCommit := NewEigenDACommitmentWithEncoding(bytes, cm.Version, cm.Encoding).Encode()
			svcCommit := EigenDASvcCommitment(certCommit).Encode()
			altDACommit := NewGenericCommitment(svcCommit).Encode()
			return altDACommit, nil
		}

		// Fallback to previous behavior for V0/V1
		certCommit := NewEigenDACommitment(bytes, cm.Version).Encode()
		svcCommit := EigenDASvcCommitment(certCommit).Encode()
		altDACommit := NewGenericCommitment(svcCommit).Encode()
		return altDACommit, nil

	case Standard:
		// For V2 certificates with encoding specified, add encoding byte
		if cm.Version >= CertV2 {
			return NewEigenDACommitmentWithEncoding(bytes, cm.Version, cm.Encoding).Encode(), nil
		}

		// Fallback to previous behavior for V0/V1
		return NewEigenDACommitment(bytes, cm.Version).Encode(), nil
	}

	return nil, fmt.Errorf("unknown commitment mode")
}
