package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/Layr-Labs/eigenda-proxy/common"
	"github.com/Layr-Labs/eigenda-proxy/common/proxyerrors"
	"github.com/Layr-Labs/eigenda-proxy/common/types/certs"
	"github.com/Layr-Labs/eigenda-proxy/common/types/commitments"
	"github.com/Layr-Labs/eigenda-proxy/server/middleware"
	"github.com/gorilla/mux"
)

const (
	// limit requests to only 32 MiB to mitigate potential DoS attacks
	maxPOSTRequestBodySize int64 = 1024 * 1024 * 32

	// HTTP headers
	headerContentType = "Content-Type"

	// Content types
	contentTypeJSON = "application/json"
)

func (svr *Server) handleHealth(w http.ResponseWriter, _ *http.Request) error {
	w.WriteHeader(http.StatusOK)
	return nil
}

func (svr *Server) logDispersalGetError(w http.ResponseWriter, _ *http.Request) error {
	svr.log.Warn(`GET method invoked on /put/ endpoint.
		This can occur due to 303 redirects when using incorrect slash ticks.`)
	w.WriteHeader(http.StatusMethodNotAllowed)
	return nil
}

// =================================================================================================
// GET ROUTES
// =================================================================================================

// handleGetOPKeccakCommitment handles GET requests for optimism keccak commitments.
func (svr *Server) handleGetOPKeccakCommitment(w http.ResponseWriter, r *http.Request) error {
	keccakCommitmentHex, ok := mux.Vars(r)[routingVarNameKeccakCommitmentHex]
	if !ok {
		return fmt.Errorf("keccak commitment not found in path: %s", r.URL.Path)
	}
	keccakCommitment, err := hex.DecodeString(keccakCommitmentHex)
	if err != nil {
		return fmt.Errorf("failed to decode hex keccak commitment %s: %w", keccakCommitmentHex, err)
	}
	svr.log.Info("Processing GET request", "commitmentMode", commitments.OptimismKeccakCommitmentMode,
		"keccakCommitment", keccakCommitmentHex)
	payload, err := svr.sm.GetOPKeccakValueFromS3(r.Context(), keccakCommitment)
	if err != nil {
		return fmt.Errorf("GET keccakCommitment %v: %w", keccakCommitmentHex, err)
	}

	svr.writeResponse(w, payload)
	return nil
}

// handleGetOPGenericCommitment handles the GET request for optimism generic commitments.
func (svr *Server) handleGetOPGenericCommitment(w http.ResponseWriter, r *http.Request) error {
	return svr.handleGetShared(r.Context(), w, r, commitments.OptimismGenericCommitmentMode)
}

// handleGetStdCommitment handles the GET request for std commitments.
func (svr *Server) handleGetStdCommitment(w http.ResponseWriter, r *http.Request) error {
	return svr.handleGetShared(r.Context(), w, r, commitments.StandardCommitmentMode)
}

func (svr *Server) handleGetShared(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	mode commitments.CommitmentMode,
) error {
	certVersion, err := parseCertVersion(w, r)
	if err != nil {
		return fmt.Errorf("error parsing version byte: %w", err)
	}
	// used in the metrics middleware... there's prob a better way to do this
	middleware.SetCertVersion(r, string(certVersion))
	serializedCertHex, ok := mux.Vars(r)[routingVarNamePayloadHex]
	if !ok {
		return fmt.Errorf("serializedDACert not found in path: %s", r.URL.Path)
	}
	serializedCert, err := hex.DecodeString(serializedCertHex)
	if err != nil {
		return proxyerrors.NewCertHexDecodingError(serializedCertHex, err)
	}
	versionedCert := certs.NewVersionedCert(serializedCert, certVersion)
	svr.log.Info("Processing GET request", "commitmentMode", mode,
		"certVersion", versionedCert.Version, "serializedCert", serializedCertHex)

	l1InclusionBlockNum, err := parseCommitmentInclusionL1BlockNumQueryParam(r)
	if err != nil {
		return err
	}
	input, err := svr.sm.Get(
		ctx,
		versionedCert,
		mode,
		common.CertVerificationOpts{L1InclusionBlockNum: l1InclusionBlockNum},
	)
	if err != nil {
		return fmt.Errorf("get request failed with serializedCert (version %v) %v: %w",
			versionedCert.Version, serializedCertHex, err)
	}

	svr.writeResponse(w, input)
	return nil
}

// Parses the l1_inclusion_block_number query param from the request.
// Happy path:
//   - if the l1_inclusion_block_number is provided, it returns the parsed value.
//
// Unhappy paths:
//   - if the l1_inclusion_block_number is not provided, it returns 0 (whose meaning is to skip the check).
//   - if the l1_inclusion_block_number is provided but isn't a valid integer, it returns an error.
func parseCommitmentInclusionL1BlockNumQueryParam(r *http.Request) (uint64, error) {
	l1BlockNumStr := r.URL.Query().Get("l1_inclusion_block_number")
	if l1BlockNumStr != "" {
		l1BlockNum, err := strconv.ParseUint(l1BlockNumStr, 10, 64)
		if err != nil {
			return 0, proxyerrors.NewL1InclusionBlockNumberParsingError(l1BlockNumStr, err)
		}
		return l1BlockNum, nil
	}
	return 0, nil
}

// handleGetEigenDADispersalBackend handles the GET request to check the current EigenDA backend used for dispersal.
// This endpoint returns which EigenDA backend version (v1 or v2) is currently being used for blob dispersal.
func (svr *Server) handleGetEigenDADispersalBackend(w http.ResponseWriter, _ *http.Request) error {
	w.Header().Set(headerContentType, contentTypeJSON)
	w.WriteHeader(http.StatusOK)

	backend := svr.sm.GetDispersalBackend()
	backendString := common.EigenDABackendToString(backend)

	response := struct {
		EigenDADispersalBackend string `json:"eigenDADispersalBackend"`
	}{
		EigenDADispersalBackend: backendString,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		return fmt.Errorf("failed to encode response: %w", err)
	}

	return nil
}

// =================================================================================================
// POST ROUTES
// =================================================================================================

// handlePostStdCommitment handles the POST request for std commitments.
func (svr *Server) handlePostStdCommitment(w http.ResponseWriter, r *http.Request) error {
	return svr.handlePostShared(w, r, nil, commitments.StandardCommitmentMode)
}

// handlePostOPKeccakCommitment handles the POST request for optimism keccak commitments.
func (svr *Server) handlePostOPKeccakCommitment(w http.ResponseWriter, r *http.Request) error {
	keccakCommitmentHex, ok := mux.Vars(r)[routingVarNameKeccakCommitmentHex]
	if !ok {
		return fmt.Errorf("keccak commitment not found in path: %s", r.URL.Path)
	}
	keccakCommitment, err := hex.DecodeString(keccakCommitmentHex)
	if err != nil {
		return fmt.Errorf("failed to decode hex keccak commitment %s: %w", keccakCommitmentHex, err)
	}
	svr.log.Info("Processing Keccak Commitment POST request",
		"mode", commitments.OptimismKeccakCommitmentMode, "commitment", keccakCommitmentHex)
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPOSTRequestBodySize))
	if err != nil {
		return proxyerrors.NewReadRequestBodyError(err, maxPOSTRequestBodySize)
	}
	err = svr.sm.PutOPKeccakPairInS3(r.Context(), keccakCommitment, payload)
	if err != nil {
		err = proxyerrors.NewPOSTError(
			fmt.Errorf("keccak POST request failed for commitment %v: %w", keccakCommitmentHex, err),
			commitments.OptimismKeccakCommitmentMode)
		return err
	}
	return nil
}

// handlePostOPGenericCommitment handles the POST request for optimism generic commitments.
func (svr *Server) handlePostOPGenericCommitment(w http.ResponseWriter, r *http.Request) error {
	return svr.handlePostShared(w, r, nil, commitments.OptimismGenericCommitmentMode)
}

// This is a shared function for handling POST requests for
func (svr *Server) handlePostShared(
	w http.ResponseWriter,
	r *http.Request,
	comm []byte, // only non-nil for OPKeccak commitments
	mode commitments.CommitmentMode,
) error {
	svr.log.Info("Processing POST request", "commitment", hex.EncodeToString(comm), "mode", mode)
	payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPOSTRequestBodySize))
	if err != nil {
		return proxyerrors.NewReadRequestBodyError(err, maxPOSTRequestBodySize)
	}

	serializedCert, err := svr.sm.Put(r.Context(), mode, comm, payload)
	if err != nil {
		err = proxyerrors.NewPOSTError(fmt.Errorf("post request failed with commitment %v: %w", comm, err), mode)
		return err
	}

	var certVersion certs.VersionByte
	switch svr.sm.GetDispersalBackend() {
	case common.V1EigenDABackend:
		certVersion = certs.V0VersionByte
	case common.V2EigenDABackend:
		certVersion = certs.V1VersionByte
	default:
		return fmt.Errorf("unknown dispersal backend: %v", svr.sm.GetDispersalBackend())
	}
	versionedCert := certs.NewVersionedCert(serializedCert, certVersion)

	responseCommit, err := commitments.EncodeCommitment(versionedCert, mode)
	if err != nil {
		err = proxyerrors.NewPOSTError(fmt.Errorf("failed to encode serializedCert %v: %w", serializedCert, err), mode)
		return err
	}

	svr.log.Info(fmt.Sprintf("response commitment: %x\n", responseCommit))
	// We write the commitment as bytes directly instead of hex encoded.
	// The spec https://specs.optimism.io/experimental/alt-da.html#da-server says it should be hex-encoded,
	// but the client expects it to be raw bytes.
	// See
	// https://github.com/Layr-Labs/optimism/blob/89ac40d0fddba2e06854b253b9f0266f36350af2/op-alt-da/daclient.go#L151
	svr.writeResponse(w, responseCommit)
	return nil
}

// handleSetEigenDADispersalBackend handles the PUT request to set the EigenDA backend used for dispersal.
// This endpoint configures which EigenDA backend version (v1 or v2) will be used for blob dispersal.
func (svr *Server) handleSetEigenDADispersalBackend(w http.ResponseWriter, r *http.Request) error {
	// Read request body to get the new value
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024)) // Small limit since we only expect a string
	if err != nil {
		return proxyerrors.NewReadRequestBodyError(err, 1024)
	}

	// Parse the backend string value
	var setRequest struct {
		EigenDADispersalBackend string `json:"eigenDADispersalBackend"`
	}

	if err := json.Unmarshal(body, &setRequest); err != nil {
		return proxyerrors.NewUnmarshalJSONError(fmt.Errorf("parsing eigenDADispersalBackend"))
	}

	// Convert the string to EigenDABackend enum
	backend, err := common.StringToEigenDABackend(setRequest.EigenDADispersalBackend)
	if err != nil {
		return err // already a structured error that error middleware knows how to handle
	}

	svr.SetDispersalBackend(backend)

	// Return the current value in the response
	w.Header().Set(headerContentType, contentTypeJSON)
	w.WriteHeader(http.StatusOK)

	currentBackend := svr.sm.GetDispersalBackend()
	backendString := common.EigenDABackendToString(currentBackend)

	response := struct {
		EigenDADispersalBackend string `json:"eigenDADispersalBackend"`
	}{
		EigenDADispersalBackend: backendString,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		return fmt.Errorf("failed to encode response: %w", err)
	}

	return nil
}
