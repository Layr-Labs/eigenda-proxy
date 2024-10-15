package server

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
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

func (svr *Server) Start() error {
	r := mux.NewRouter()
	svr.registerRoutes(r)
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

func (svr *Server) writeResponse(w http.ResponseWriter, data []byte) {
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
