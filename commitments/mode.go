package commitments

import (
	"fmt"
)

// EncodingType represents the serialization format for the certificate
type EncodingType byte

const (
	// RLPEncoding represents RLP encoding (default)
	RLPEncoding EncodingType = iota
	// ABIVerifyV2CertEncoding represents ABI encoding
	ABIVerifyV2CertEncoding
)

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

func StringToEncodingType(s string) (EncodingType, error) {
	switch s {
	case "rlp", "RLP", "0":
		return RLPEncoding, nil
	case "abi", "ABI", "1":
		return ABIVerifyV2CertEncoding, nil
	default:
		return RLPEncoding, fmt.Errorf("unknown encoding type: %s, using default RLP encoding", s)
	}
}

// DecodeCommitmentType returns the encoding type from a certificate commitment
func DecodeCommitmentType(cert []byte) (EigenDACommitmentType, EncodingType, []byte, error) {
	if len(cert) == 0 {
		return 0, 0, nil, fmt.Errorf("commitment is empty")
	}

	// Extract version byte (always present)
	version := EigenDACommitmentType(cert[0])
	var encoding EncodingType
	var payload []byte

	// For CertV2+, we have an encoding byte
	if version >= CertV2 {
		if len(cert) < 2 {
			return 0, 0, nil, fmt.Errorf("commitment too short for CertV2")
		}
		encoding = EncodingType(cert[1])
		payload = cert[2:] // Skip version and encoding bytes
	} else {
		// For CertV0/V1, there's no encoding byte (default to RLP)
		encoding = RLPEncoding
		payload = cert[1:] // Skip only version byte
	}

	return version, encoding, payload, nil
}

func EncodeCommitment(
	bytes []byte,
	commitmentMode CommitmentMode,
	commitmentType EigenDACommitmentType,
	encodingType EncodingType,
) ([]byte, error) {
	switch commitmentMode {
	case OptimismKeccak:
		return Keccak256Commitment(bytes).Encode(), nil

	case OptimismGeneric:
		// For V2 certificates with encoding specified, add encoding byte
		if commitmentType >= CertV2 {
			certCommit := NewEigenDACommitmentWithEncoding(bytes, commitmentType, encodingType).Encode()
			svcCommit := EigenDASvcCommitment(certCommit).Encode()
			altDACommit := NewGenericCommitment(svcCommit).Encode()
			return altDACommit, nil
		}

		// Fallback to previous behavior for V0/V1
		certCommit := NewEigenDACommitment(bytes, commitmentType).Encode()
		svcCommit := EigenDASvcCommitment(certCommit).Encode()
		altDACommit := NewGenericCommitment(svcCommit).Encode()
		return altDACommit, nil

	case Standard:
		// For V2 certificates with encoding specified, add encoding byte
		if commitmentType >= CertV2 {
			return NewEigenDACommitmentWithEncoding(bytes, commitmentType, encodingType).Encode(), nil
		}

		// Fallback to previous behavior for V0/V1
		return NewEigenDACommitment(bytes, commitmentType).Encode(), nil
	}

	return nil, fmt.Errorf("unknown commitment mode")
}
