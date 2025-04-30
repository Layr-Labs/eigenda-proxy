package common

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Layr-Labs/eigenda-proxy/commitments"
	"github.com/Layr-Labs/eigenda/api/clients/v2/coretypes"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/rlp"

	binding "github.com/Layr-Labs/eigenda/contracts/bindings/EigenDACertVerifier"
)

var (
	v2VerifyMethodInputs abi.Arguments
)

func init() {
	certVerifierABI, err := binding.ContractEigenDACertVerifierMetaData.GetAbi()
	if err != nil {
		panic(err)
	}

	v2VerifyMethodInputs = certVerifierABI.Methods["verifyDACertV2"].Inputs
}

func DecodeCertFromCtx(ctx context.Context, commit []byte) (*coretypes.EigenDACert, error) {
	encodingAlg, ok := ctx.Value(EncodingCtxKey).(commitments.EncodingType)
	if !ok {
		return nil, fmt.Errorf("could not read encoding type from context")
	}
	cert, err := DecodeV2CertFromBytes(encodingAlg, commit)
	if err != nil {
		return nil, fmt.Errorf("decoding V2 cert from bytes: %w", err)
	}

	return cert, nil
}

func EncodeV2CertToBytes(encoding commitments.EncodingType, cert *coretypes.EigenDACert) ([]byte, error) {
	switch encoding {
	case commitments.ABIVerifyV2CertEncoding:
		return v2VerifyMethodInputs.Pack(
			cert.BatchHeader,
			cert.BlobInclusionInfo,
			cert.NonSignerStakesAndSignature,
			cert.SignedQuorumNumbers)

	case commitments.RLPEncoding:
		return rlp.EncodeToBytes(cert)

	default:
		return nil, fmt.Errorf("encoding v2 cert to bytes: %x", encoding)
	}
}

// DecodeV2CertFromBytes ... Decodes a raw DA commitment into an EigenDA V2 certificate
// provided the respective encoding version
func DecodeV2CertFromBytes(encoding commitments.EncodingType, commitment []byte) (*coretypes.EigenDACert, error) {
	switch encoding {
	case commitments.ABIVerifyV2CertEncoding:

		var abiMap map[string]interface{}
		err := v2VerifyMethodInputs.UnpackIntoMap(abiMap, commitment)
		if err != nil {
			return nil, fmt.Errorf("unpacking from encoding ABI: %w", err)
		}

		// use json as intermediary to cast abstract type to bytes to
		// then deserialize into structured certificate type
		bytes, err := json.Marshal(commitment)
		if err != nil {
			return nil, err
		}

		var cert *coretypes.EigenDACert
		err = json.Unmarshal(bytes, &cert)

		if err != nil {
			return nil, fmt.Errorf("json unmarshal v2 cert: %w", err)
		}

		return cert, nil

	case commitments.RLPEncoding:
		var cert *coretypes.EigenDACert

		err := rlp.DecodeBytes(commitment, &cert)
		if err != nil {
			return nil, fmt.Errorf("rlp decoding v2 cert: %w", err)
		}

		return cert, nil

	default:
		return nil, fmt.Errorf("encoding v2 cert to bytes: %x", encoding)
	}
}
