package server

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"

	"github.com/Layr-Labs/eigenda-proxy/commitments"
	"github.com/Layr-Labs/eigenda-proxy/store"
	"github.com/gorilla/mux"
)

func (svr *Server) handleHealth(w http.ResponseWriter, _ *http.Request) error {
	w.WriteHeader(http.StatusOK)
	return nil
}

func (svr *Server) handleGetSimpleCommitment(w http.ResponseWriter, r *http.Request) error {
	versionByte, err := parseVersionByte(r)
	if err != nil {
		return fmt.Errorf("error parsing version byte: %w", err)
	}
	commitmentMeta := commitments.CommitmentMeta{
		Mode:        commitments.SimpleCommitmentMode,
		CertVersion: versionByte,
	}

	rawCommitmentHex, ok := mux.Vars(r)["raw_commitment_hex"]
	if !ok {
		return fmt.Errorf("commitment not found in path: %s", r.URL.Path)
	}
	commitment, err := hex.DecodeString(rawCommitmentHex)
	if err != nil {
		return fmt.Errorf("failed to decode commitment %s: %w", rawCommitmentHex, err)
	}

	svr.log.Info("Processing simple commitment", "commitment", rawCommitmentHex, "commitmentMeta", commitmentMeta)
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
		Mode:        commitments.OptimismKeccak,
		CertVersion: byte(commitments.CertV0),
	}

	rawCommitmentHex, ok := mux.Vars(r)["raw_commitment_hex"]
	if !ok {
		return fmt.Errorf("commitment not found in path: %s", r.URL.Path)
	}
	commitment, err := hex.DecodeString(rawCommitmentHex)
	if err != nil {
		return fmt.Errorf("failed to decode commitment %s: %w", rawCommitmentHex, err)
	}

	svr.log.Info("Processing op keccak commitment", "commitment", rawCommitmentHex, "commitmentMeta", commitmentMeta)
	return svr.handleGetShared(r.Context(), w, commitment, commitmentMeta)
}

func (svr *Server) handleGetOPGenericCommitment(w http.ResponseWriter, r *http.Request) error {
	versionByte, err := parseVersionByte(r)
	if err != nil {
		return fmt.Errorf("error parsing version byte: %w", err)
	}
	commitmentMeta := commitments.CommitmentMeta{
		Mode:        commitments.OptimismGeneric,
		CertVersion: versionByte,
	}

	rawCommitmentHex, ok := mux.Vars(r)["raw_commitment_hex"]
	if !ok {
		return fmt.Errorf("commitment not found in path: %s", r.URL.Path)
	}
	commitment, err := hex.DecodeString(rawCommitmentHex)
	if err != nil {
		return fmt.Errorf("failed to decode commitment %s: %w", rawCommitmentHex, err)
	}

	svr.log.Info("Processing op keccak commitment", "commitment", rawCommitmentHex, "commitmentMeta", commitmentMeta)
	return svr.handleGetShared(r.Context(), w, commitment, commitmentMeta)
}

func (svr *Server) handleGetShared(ctx context.Context, w http.ResponseWriter, comm []byte, meta commitments.CommitmentMeta) error {
	input, err := svr.router.Get(ctx, comm, meta.Mode)
	if err != nil {
		err = MetaError{
			Err:  fmt.Errorf("get request failed with commitment %v: %w", comm, err),
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

// handlePut handles the PUT request for commitments.
// Note: even when an error is returned, the commitment meta is still returned,
// because it is needed for metrics (see the WithMetrics middleware).
// TODO: we should change this behavior and instead use a custom error that contains the commitment meta.
func (svr *Server) handlePut(w http.ResponseWriter, r *http.Request) (commitments.CommitmentMeta, error) {
	meta, err := readCommitmentMeta(r)
	if err != nil {
		err = fmt.Errorf("invalid commitment mode: %w", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return commitments.CommitmentMeta{}, err
	}
	// ReadCommitmentMeta function invoked inside HandlePut will not return a valid certVersion
	// Current simple fix is using the hardcoded default value of 0 (also the only supported value)
	//TODO: smarter decode needed when there's more than one version
	meta.CertVersion = byte(commitments.CertV0)

	input, err := io.ReadAll(r.Body)
	if err != nil {
		err = MetaError{
			Err:  fmt.Errorf("failed to read request body: %w", err),
			Meta: meta,
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return commitments.CommitmentMeta{}, err
	}

	key := path.Base(r.URL.Path)
	var comm []byte

	if len(key) > 0 && key != Put { // commitment key already provided (keccak256)
		comm, err = commitments.StringToDecodedCommitment(key, meta.Mode)
		if err != nil {
			err = MetaError{
				Err:  fmt.Errorf("failed to decode commitment from key %v (commitment mode %v): %w", key, meta.Mode, err),
				Meta: meta,
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return commitments.CommitmentMeta{}, err
		}
	}

	commitment, err := svr.router.Put(r.Context(), meta.Mode, comm, input)
	if err != nil {
		err = MetaError{
			Err:  fmt.Errorf("put request failed with commitment %v (commitment mode %v): %w", comm, meta.Mode, err),
			Meta: meta,
		}
		if errors.Is(err, store.ErrEigenDAOversizedBlob) || errors.Is(err, store.ErrProxyOversizedBlob) {
			// we add here any error that should be returned as a 400 instead of a 500.
			// currently only includes oversized blob requests
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return commitments.CommitmentMeta{}, err
	}

	responseCommit, err := commitments.EncodeCommitment(commitment, meta.Mode)
	if err != nil {
		err = MetaError{
			Err:  fmt.Errorf("failed to encode commitment %v (commitment mode %v): %w", commitment, meta.Mode, err),
			Meta: meta,
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return commitments.CommitmentMeta{}, err
	}

	svr.log.Info(fmt.Sprintf("response commitment: %x\n", responseCommit))
	// write commitment to resp body if not in OptimismKeccak mode
	if meta.Mode != commitments.OptimismKeccak {
		svr.writeResponse(w, responseCommit)
	}
	return meta, nil
}
