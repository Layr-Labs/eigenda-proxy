package server

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/commitments"
	"github.com/Layr-Labs/eigenda-proxy/metrics"
	"github.com/Layr-Labs/eigenda-proxy/store"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/gorilla/mux"
)

var (
	ErrNotFound = errors.New("not found")
)

const (
	Put = "put"

	CommitmentModeKey = "commitment_mode"
)

type Server struct {
	log        log.Logger
	endpoint   string
	router     store.IRouter
	m          metrics.Metricer
	httpServer *http.Server
	listener   net.Listener
}

func NewServer(host string, port int, router store.IRouter, log log.Logger,
	m metrics.Metricer) *Server {
	endpoint := net.JoinHostPort(host, strconv.Itoa(port))
	return &Server{
		m:        m,
		log:      log,
		endpoint: endpoint,
		router:   router,
		httpServer: &http.Server{
			Addr:              endpoint,
			ReadHeaderTimeout: 10 * time.Second,
			// aligned with existing blob finalization times
			WriteTimeout: 40 * time.Minute,
		},
	}
}

// withMetrics is a middleware that records metrics for the route path.
func withMetrics(
	handleFn func(http.ResponseWriter, *http.Request) error,
	m metrics.Metricer,
	mode commitments.CommitmentMode,
) func(http.ResponseWriter, *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		recordDur := m.RecordRPCServerRequest(r.Method)

		err := handleFn(w, r)
		if err != nil {
			var metaErr MetaError
			if errors.As(err, &metaErr) {
				recordDur(w.Header().Get("status"), string(metaErr.Meta.Mode), string(metaErr.Meta.CertVersion))
			} else {
				recordDur(w.Header().Get("status"), string("NoCommitmentMode"), string("NoCertVersion"))
			}
			return err
		}
		// we assume that every route will set the status header
		versionByte, err := parseVersionByte(r)
		if err != nil {
			return fmt.Errorf("metrics middleware: error parsing version byte: %w", err)
		}
		recordDur(w.Header().Get("status"), string(mode), string(versionByte))
		return nil
	}
}

// withLogging is a middleware that logs information related to each request.
// It does not write anything to the response, that is the job of the handlers.
// Currently we cannot log the status code because go's default ResponseWriter interface does not expose it.
// TODO: implement a ResponseWriter wrapper that saves the status code: see https://github.com/golang/go/issues/18997
func withLogging(
	handleFn func(http.ResponseWriter, *http.Request) error,
	log log.Logger,
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		err := handleFn(w, r)
		var metaErr MetaError
		if errors.As(err, &metaErr) {
			log.Info("request", "method", r.Method, "url", r.URL, "duration", time.Since(start),
				"err", err, "status", w.Header().Get("status"),
				"commitment_mode", metaErr.Meta.Mode, "cert_version", metaErr.Meta.CertVersion)
		} else if err != nil {
			log.Info("request", "method", r.Method, "url", r.URL, "duration", time.Since(start), "err", err)
		} else {
			log.Info("request", "method", r.Method, "url", r.URL, "duration", time.Since(start))
		}
	}
}

func (svr *Server) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("testtesttest")
	})
	subrouterGET := r.Methods("GET").PathPrefix("/get").Subrouter()

	// simple commitments (for nitro)
	subrouterGET.HandleFunc("/"+
		"{optional_prefix:(?:0x)?}"+ // commitments can be prefixed with 0x
		"{version_byte_hex:[0-9a-fA-F]{2}}"+ // should always be 0x00 for now but we let others through to return a 404
		"{raw_commitment_hex}",
		withLogging(withMetrics(svr.handleGetSimpleCommitment, svr.m, commitments.SimpleCommitmentMode), svr.log),
	).Queries("commitment_mode", "simple")
	// op keccak256 commitments (write to S3)
	subrouterGET.HandleFunc("/"+
		"{optional_prefix:(?:0x)?}"+ // commitments can be prefixed with 0x
		"{commit_type_byte_hex:00}"+ // 00 for keccak256 commitments
		// TODO: should these be present..?? README says they should but server_test didn't have them
		// "{da_layer_byte:[0-9a-fA-F]{2}}"+ // should always be 0x00 for eigenDA but we let others through to return a 404
		// "{version_byte_hex:[0-9a-fA-F]{2}}"+ // should always be 0x00 for now but we let others through to return a 404
		"{raw_commitment_hex}",
		withLogging(withMetrics(svr.handleGetOPKeccakCommitment, svr.m, commitments.OptimismKeccak), svr.log),
	)
	// op generic commitments (write to EigenDA)
	subrouterGET.HandleFunc("/"+
		"{optional_prefix:(?:0x)?}"+ // commitments can be prefixed with 0x
		"{commit_type_byte_hex:01}"+ // 01 for generic commitments
		"{da_layer_byte:[0-9a-fA-F]{2}}"+ // should always be 0x00 for eigenDA but we let others through to return a 404
		"{version_byte_hex:[0-9a-fA-F]{2}}"+ // should always be 0x00 for now but we let others through to return a 404
		"{raw_commitment_hex}",
		withLogging(withMetrics(svr.handleGetOPGenericCommitment, svr.m, commitments.OptimismGeneric), svr.log),
	)
	// unrecognized op commitment type (not 00 or 01)
	subrouterGET.HandleFunc("/"+
		"{optional_prefix:(?:0x)?}"+ // commitments can be prefixed with 0x
		"{commit_type_byte_hex:[0-9a-fA-F]{2}}",
		func(w http.ResponseWriter, r *http.Request) {
			svr.log.Info("unrecognized commitment type", "commit_type_byte_hex", mux.Vars(r)["commit_type_byte_hex"])
			commitType := mux.Vars(r)["commit_type_byte_hex"]
			http.Error(w, fmt.Sprintf("unsupported commitment type %s", commitType), http.StatusBadRequest)
		},
	)
	// we need to handle both: see https://github.com/ethereum-optimism/optimism/pull/12081
	// /put is for generic commitments, and /put/ for keccak256 commitments
	// TODO: we should probably separate their handlers?
	// r.HandleFunc("/put", WithLogging(WithMetrics(svr.handlePut, svr.m), svr.log)).Methods("POST")
	// r.HandleFunc("/put/", WithLogging(WithMetrics(svr.handlePut, svr.m), svr.log)).Methods("POST")
	r.HandleFunc("/health", withLogging(svr.Health, svr.log)).Methods("GET")
}

func (svr *Server) Start() error {
	r := mux.NewRouter()
	svr.RegisterRoutes(r)
	svr.httpServer.Handler = r

	listener, err := net.Listen("tcp", svr.endpoint)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	svr.listener = listener

	svr.endpoint = listener.Addr().String()

	svr.log.Info("Starting DA server", "endpoint", svr.endpoint)
	errCh := make(chan error, 1)
	go func() {
		if err := svr.httpServer.Serve(svr.listener); err != nil {
			errCh <- err
		}
	}()

	// verify that the server comes up
	tick := time.NewTimer(10 * time.Millisecond)
	defer tick.Stop()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server failed: %w", err)
	case <-tick.C:
		return nil
	}
}

func (svr *Server) Endpoint() string {
	return svr.listener.Addr().String()
}

func (svr *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svr.httpServer.Shutdown(ctx); err != nil {
		svr.log.Error("Failed to shutdown proxy server", "err", err)
		return err
	}
	return nil
}
func (svr *Server) Health(w http.ResponseWriter, _ *http.Request) error {
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

	svr.WriteResponse(w, input)
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
		svr.WriteResponse(w, responseCommit)
	}
	return meta, nil
}

func (svr *Server) WriteResponse(w http.ResponseWriter, data []byte) {
	if _, err := w.Write(data); err != nil {
		http.Error(w, fmt.Sprintf("failed to write response: %v", err), http.StatusInternalServerError)
	}
}

func (svr *Server) Port() int {
	// read from listener
	_, portStr, _ := net.SplitHostPort(svr.listener.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return port
}

// Read both commitment mode and version
func readCommitmentMeta(r *http.Request) (commitments.CommitmentMeta, error) {
	// label requests with commitment mode and version
	ct, err := readCommitmentMode(r)
	if err != nil {
		return commitments.CommitmentMeta{}, fmt.Errorf("failed to read commitment mode: %w", err)
	}
	if ct == "" {
		return commitments.CommitmentMeta{}, fmt.Errorf("commitment mode is empty")
	}
	cv, err := readCommitmentVersion(r, ct)
	if err != nil {
		// default to version 0
		return commitments.CommitmentMeta{Mode: ct, CertVersion: cv}, err
	}
	return commitments.CommitmentMeta{Mode: ct, CertVersion: cv}, nil
}

func readCommitmentMode(r *http.Request) (commitments.CommitmentMode, error) {
	query := r.URL.Query()
	// if commitment mode is provided in the query params, use it
	// eg. /get/0x123..?commitment_mode=simple
	// TODO: should we only allow simple commitment to be set in the query params?
	key := query.Get(CommitmentModeKey)
	if key != "" {
		return commitments.StringToCommitmentMode(key)
	}

	// else, we need to parse the first byte of the commitment
	commit := path.Base(r.URL.Path)
	if len(commit) > 0 && commit != Put { // provided commitment in request params (op keccak256)
		if !strings.HasPrefix(commit, "0x") {
			commit = "0x" + commit
		}

		decodedCommit, err := hexutil.Decode(commit)
		if err != nil {
			return "", err
		}

		if len(decodedCommit) < 3 {
			return "", fmt.Errorf("commitment is too short")
		}

		switch decodedCommit[0] {
		case byte(commitments.GenericCommitmentType):
			return commitments.OptimismGeneric, nil

		case byte(commitments.Keccak256CommitmentType):
			return commitments.OptimismKeccak, nil

		default:
			return commitments.SimpleCommitmentMode, fmt.Errorf("unknown commit byte prefix")
		}
	}
	return commitments.OptimismGeneric, nil
}

func readCommitmentVersion(r *http.Request, mode commitments.CommitmentMode) (byte, error) {
	commit := path.Base(r.URL.Path)
	if len(commit) > 0 && commit != Put { // provided commitment in request params (op keccak256)
		if !strings.HasPrefix(commit, "0x") {
			commit = "0x" + commit
		}

		decodedCommit, err := hexutil.Decode(commit)
		if err != nil {
			return 0, err
		}

		if len(decodedCommit) < 3 {
			return 0, fmt.Errorf("commitment is too short")
		}

		if mode == commitments.OptimismGeneric || mode == commitments.SimpleCommitmentMode {
			return decodedCommit[2], nil
		}

		return decodedCommit[0], nil
	}
	return 0, nil
}

func (svr *Server) GetEigenDAStats() *store.Stats {
	return svr.router.GetEigenDAStore().Stats()
}

func (svr *Server) GetS3Stats() *store.Stats {
	return svr.router.GetS3Store().Stats()
}

func (svr *Server) GetStoreStats(bt store.BackendType) (*store.Stats, error) {
	// first check if the store is a cache
	for _, cache := range svr.router.Caches() {
		if cache.BackendType() == bt {
			return cache.Stats(), nil
		}
	}

	// then check if the store is a fallback
	for _, fallback := range svr.router.Fallbacks() {
		if fallback.BackendType() == bt {
			return fallback.Stats(), nil
		}
	}

	return nil, fmt.Errorf("store not found")
}

func parseVersionByte(r *http.Request) (byte, error) {
	vars := mux.Vars(r)
	// decode version byte
	versionByteHex, ok := vars["version_byte_hex"]
	if !ok {
		return 0, fmt.Errorf("version byte not found in path: %s", r.URL.Path)
	}
	versionByte, err := hex.DecodeString(versionByteHex)
	if err != nil {
		return 0, fmt.Errorf("failed to decode version byte %s: %w", versionByteHex, err)
	}
	if len(versionByte) != 1 {
		return 0, fmt.Errorf("version byte is not a single byte: %s", versionByteHex)
	}
	switch versionByte[0] {
	case byte(commitments.CertV0):
		return versionByte[0], nil
	default:
		return 0, fmt.Errorf("unsupported version byte %x", versionByte)
	}
}
