package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/Layr-Labs/eigenda-proxy/commitments"
	"github.com/Layr-Labs/eigenda-proxy/common"
	"github.com/gorilla/mux"
)

const (
	// limit requests to only 32 mib to mitigate potential DoS attacks
	maxRequestBodySize int64 = 1024 * 1024 * 32

	// HTTP headers
	headerContentType = "Content-Type"

	// Content types
	contentTypeJSON = "application/json"
)

func (svr *Server) handleHealth(w http.ResponseWriter, _ *http.Request) error {
	w.WriteHeader(http.StatusOK)
	return nil
}

// =================================================================================================
// GET ROUTES
// =================================================================================================

// handleGetStdCommitment handles the GET request for std commitments.
func (svr *Server) handleGetStdCommitment(w http.ResponseWriter, r *http.Request) error {
	versionByte, err := parseVersionByte(w, r)
	if err != nil {
		return fmt.Errorf("error parsing version byte: %w", err)
	}

	commitmentMeta := commitments.CommitmentMeta{
		Mode:     commitments.OptimismGeneric,
		Version:  commitments.EigenDACommitmentType(versionByte),
		Encoding: commitments.RLPEncoding,
	}

	rawCommitmentHex, ok := mux.Vars(r)[routingVarNamePayloadHex]
	if !ok {
		return fmt.Errorf("commitment not found in path: %s", r.URL.Path)
	}
	commitment, err := hex.DecodeString(rawCommitmentHex)
	if err != nil {
		return fmt.Errorf("failed to decode commitment %s: %w", rawCommitmentHex, err)
	}

	if versionByte >= byte(commitments.CertV2) {
		commitmentMeta.Encoding = commitments.EncodingType(commitment[0])
		commitment = commitment[1:]
	}

	return svr.handleGetShared(r.Context(), w, commitment, commitmentMeta)
}

// handleGetOPKeccakCommitment handles the GET request for optimism keccak commitments.
func (svr *Server) handleGetOPKeccakCommitment(w http.ResponseWriter, r *http.Request) error {
	// TODO: do we use a version byte in OPKeccak commitments? README seems to say so, but server_test didn't
	// versionByte, err := parseVersionByte(r)
	// if err != nil {
	// 	err = fmt.Errorf("error parsing version byte: %w", err)
	// 	http.Error(w, err.Error(), http.StatusBadRequest)
	// 	return err
	// }
	commitmentMeta := commitments.CommitmentMeta{
		Mode:    commitments.OptimismKeccak,
		Version: commitments.CertV0,
	}

	rawCommitmentHex, ok := mux.Vars(r)[routingVarNamePayloadHex]
	if !ok {
		return fmt.Errorf("commitment not found in path: %s", r.URL.Path)
	}
	commitment, err := hex.DecodeString(rawCommitmentHex)
	if err != nil {
		return fmt.Errorf("failed to decode commitment %s: %w", rawCommitmentHex, err)
	}

	return svr.handleGetShared(r.Context(), w, commitment, commitmentMeta)
}

// handleGetOPGenericCommitment handles the GET request for optimism generic commitments.
func (svr *Server) handleGetOPGenericCommitment(w http.ResponseWriter, r *http.Request) error {
	versionByte, err := parseVersionByte(w, r)
	if err != nil {
		return fmt.Errorf("error parsing version byte: %w", err)
	}

	commitmentMeta := commitments.CommitmentMeta{
		Mode:     commitments.OptimismGeneric,
		Version:  commitments.EigenDACommitmentType(versionByte),
		Encoding: commitments.RLPEncoding,
	}

	rawCommitmentHex, ok := mux.Vars(r)[routingVarNamePayloadHex]
	if !ok {
		return fmt.Errorf("commitment not found in path: %s", r.URL.Path)
	}
	commitment, err := hex.DecodeString(rawCommitmentHex)
	if err != nil {
		return fmt.Errorf("failed to decode commitment %s: %w", rawCommitmentHex, err)
	}

	if versionByte >= byte(commitments.CertV2) {
		commitmentMeta.Encoding = commitments.EncodingType(commitment[0])
		commitment = commitment[1:]
	}

	return svr.handleGetShared(r.Context(), w, commitment, commitmentMeta)
}

func (svr *Server) handleGetShared(
	ctx context.Context,
	w http.ResponseWriter,
	comm []byte,
	meta commitments.CommitmentMeta,
) error {
	commitmentHex := hex.EncodeToString(comm)
	svr.log.Info("Processing GET request", "commitment", commitmentHex, "commitmentMeta", meta)
	input, err := svr.sm.Get(ctx, comm, meta)
	if err != nil {
		err = MetaError{
			Err:  fmt.Errorf("get request failed with commitment %v: %w", commitmentHex, err),
			Meta: meta,
		}
		if errors.Is(err, ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return err
	}

	svr.writeResponse(w, input)
	return nil
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
		http.Error(w, fmt.Sprintf("failed to encode response: %v", err), http.StatusInternalServerError)
		return err
	}

	return nil
}

// =================================================================================================
// POST ROUTES
// =================================================================================================

// handlePostStdCommitment handles the POST request for std commitments.
// parseEncodingQueryParamType parses the encoding query parameter
func parseEncodingQueryParamType(w http.ResponseWriter, r *http.Request) (commitments.EncodingType, bool, error) {
	encodingParam := r.URL.Query().Get(routingQueryParamEncoding)
	if encodingParam == "" {
		// If no encoding is provided, use default RLP encoding
		return commitments.RLPEncoding, false, nil
	}

	// Parse the encoding type
	encoding, err := commitments.StringToEncodingType(encodingParam)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid encoding type: %s", encodingParam), http.StatusBadRequest)
		return commitments.RLPEncoding, false, err
	}

	return encoding, true, nil
}

func (svr *Server) handlePostStdCommitment(w http.ResponseWriter, r *http.Request) error {
	// Parse encoding type from query parameter
	encodingType, hasEncoding, err := parseEncodingQueryParamType(w, r)
	if err != nil {
		return err
	}

	commitmentMeta := commitments.CommitmentMeta{
		Mode:     commitments.Standard,
		Version:  commitments.CertV0,
		Encoding: encodingType,
	}

	if svr.sm.GetDispersalBackend() == common.V2EigenDABackend {
		// If encoding is specified and we're using V2, use CertV2
		if hasEncoding {
			commitmentMeta.Version = commitments.CertV2
		} else {
			commitmentMeta.Version = commitments.CertV1
		}
	}

	return svr.handlePostShared(w, r, nil, commitmentMeta)
}

// handlePostOPKeccakCommitment handles the POST request for optimism keccak commitments.
func (svr *Server) handlePostOPKeccakCommitment(w http.ResponseWriter, r *http.Request) error {
	// TODO: do we use a version byte in OPKeccak commitments? README seems to say so, but server_test didn't
	// versionByte, err := parseVersionByte(r)
	// if err != nil {
	// 	err = fmt.Errorf("error parsing version byte: %w", err)
	// 	http.Error(w, err.Error(), http.StatusBadRequest)
	// 	return err
	// }

	// Parse encoding type from query parameter
	encodingType, _, err := parseEncodingQueryParamType(w, r)
	if err != nil {
		return err
	}

	commitmentMeta := commitments.CommitmentMeta{
		Mode:     commitments.OptimismKeccak,
		Version:  commitments.CertV0,
		Encoding: encodingType,
	}

	rawCommitmentHex, ok := mux.Vars(r)[routingVarNamePayloadHex]
	if !ok {
		return fmt.Errorf("commitment not found in path: %s", r.URL.Path)
	}
	commitment, err := hex.DecodeString(rawCommitmentHex)
	if err != nil {
		return fmt.Errorf("failed to decode commitment %s: %w", rawCommitmentHex, err)
	}

	return svr.handlePostShared(w, r, commitment, commitmentMeta)
}

// handlePostOPGenericCommitment handles the POST request for optimism generic commitments.
func (svr *Server) handlePostOPGenericCommitment(w http.ResponseWriter, r *http.Request) error {
	// Parse encoding type from query parameter
	encodingType, hasEncoding, err := parseEncodingQueryParamType(w, r)
	if err != nil {
		return err
	}

	commitmentMeta := commitments.CommitmentMeta{
		Mode:     commitments.OptimismGeneric,
		Version:  commitments.CertV0,
		Encoding: encodingType,
	}

	if svr.sm.GetDispersalBackend() == common.V2EigenDABackend {
		// If encoding is specified and we're using V2, use CertV2
		if hasEncoding {
			commitmentMeta.Version = commitments.CertV2
		} else {
			commitmentMeta.Version = commitments.CertV1
		}
	}

	return svr.handlePostShared(w, r, nil, commitmentMeta)
}

func (svr *Server) handlePostShared(
	w http.ResponseWriter,
	r *http.Request,
	comm []byte,
	meta commitments.CommitmentMeta,
) error {
	svr.log.Info("Processing POST request", "commitment", hex.EncodeToString(comm), "meta", meta)
	input, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBodySize))
	if err != nil {
		err = MetaError{
			Err:  fmt.Errorf("failed to read request body: %w", err),
			Meta: meta,
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	commitment, err := svr.sm.Put(r.Context(), meta, comm, input)
	if err != nil {
		err = MetaError{
			Err:  fmt.Errorf("put request failed with commitment %v (commitment mode %v): %w", comm, meta.Mode, err),
			Meta: meta,
		}
		switch {
		case is400(err):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case is429(err):
			http.Error(w, err.Error(), http.StatusTooManyRequests)
		case is503(err):
			// this tells the caller (batcher) to failover to ethda b/c eigenda is temporarily down
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return err
	}

	responseCommit, err := commitments.EncodeCommitment(commitment, meta.Mode, meta.Version, meta.Encoding)
	if err != nil {
		err = MetaError{
			Err:  fmt.Errorf("failed to encode commitment %v (commitment mode %v): %w", commitment, meta.Mode, err),
			Meta: meta,
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}

	svr.log.Info(fmt.Sprintf("response commitment: %x\n", responseCommit))
	// write commitment to resp body if not in OptimismKeccak mode
	if meta.Mode != commitments.OptimismKeccak {
		svr.writeResponse(w, responseCommit)
	}
	return nil
}

// handleSetEigenDADispersalBackend handles the PUT request to set the EigenDA backend used for dispersal.
// This endpoint configures which EigenDA backend version (v1 or v2) will be used for blob dispersal.
func (svr *Server) handleSetEigenDADispersalBackend(w http.ResponseWriter, r *http.Request) error {
	// Read request body to get the new value
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1024)) // Small limit since we only expect a string
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return err
	}

	// Parse the backend string value
	var setRequest struct {
		EigenDADispersalBackend string `json:"eigenDADispersalBackend"`
	}

	if err := json.Unmarshal(body, &setRequest); err != nil {
		http.Error(w, fmt.Sprintf("failed to parse JSON request: %v", err), http.StatusBadRequest)
		return err
	}

	// Convert the string to EigenDABackend enum
	backend, err := common.StringToEigenDABackend(setRequest.EigenDADispersalBackend)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid eigenDADispersalBackend value: %v", err), http.StatusBadRequest)
		return err
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
		http.Error(w, fmt.Sprintf("failed to encode response: %v", err), http.StatusInternalServerError)
		return err
	}

	return nil
}
