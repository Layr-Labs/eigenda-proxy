package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/Layr-Labs/eigenda-proxy/commitments"
	"github.com/Layr-Labs/eigenda-proxy/metrics"
	"github.com/Layr-Labs/eigenda-proxy/store"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
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

// WithMetrics is a middleware that records metrics for the route path.
func WithMetrics(handleFn func(http.ResponseWriter, *http.Request) (commitments.CommitmentMeta, error),
	m metrics.Metricer) func(http.ResponseWriter, *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		recordDur := m.RecordRPCServerRequest(r.Method)

		meta, err := handleFn(w, r)
		// we assume that every route will set the status header
		recordDur(w.Header().Get("status"), string(meta.Mode), string(meta.CertVersion))
		return err
	}
}

func (svr *Server) Start() error {

	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(echoprometheus.NewMiddleware("myapp"))
	// TODO: probably want to server this on a different echo server so that it uses the metrics port instead of server port
	e.GET("/metrics", echoprometheus.NewHandler())

	// Routes: see https://specs.optimism.io/experimental/alt-da.html#da-server
	e.GET("/get/", svr.HandleGet)
	// TODO: prob want different handlers for these
	// one difference in the spec for eg which we aren't accounting for currently is that /put should not return a commitment
	e.POST("/put/:hex_encoded_commitment", svr.HandlePut)
	e.POST("/put", svr.HandlePut)
	e.GET("/health", svr.Health)

	listener, err := net.Listen("tcp", svr.endpoint)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	svr.endpoint = listener.Addr().String()
	e.Listener = listener

	svr.log.Info("Starting DA server", "endpoint", svr.endpoint)

	errCh := make(chan error, 1)
	go func() {
		errCh <- e.Start("")
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
func (svr *Server) Health(c echo.Context) error {
	return c.NoContent(http.StatusOK)
}

// HandleGet handles the GET request for commitments.
func (svr *Server) HandleGet(c echo.Context) error {
	meta, err := ReadCommitmentMeta(c.Request())
	if err != nil {
		err = fmt.Errorf("invalid commitment mode: %w", err)
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}
	key := path.Base(c.Path())
	comm, err := commitments.StringToDecodedCommitment(key, meta.Mode)
	if err != nil {
		err = fmt.Errorf("failed to decode commitment from key %v (commitment mode %v): %w", key, meta.Mode, err)
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	input, err := svr.router.Get(c.Request().Context(), comm, meta.Mode)
	if err != nil {
		err = fmt.Errorf("get request failed with commitment %v (commitment mode %v): %w", comm, meta.Mode, err)
		if errors.Is(err, ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, err)
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.Blob(http.StatusOK, "application/octet-stream", input)
}

// HandlePut handles the PUT request for commitments.
func (svr *Server) HandlePut(c echo.Context) error {
	meta, err := ReadCommitmentMeta(c.Request())
	if err != nil {
		err = fmt.Errorf("invalid commitment mode: %w", err)
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}
	// ReadCommitmentMeta function invoked inside HandlePut will not return a valid certVersion
	// Current simple fix is using the hardcoded default value of 0 (also the only supported value)
	//TODO: smarter decode needed when there's more than one version
	meta.CertVersion = byte(commitments.CertV0)

	input, err := io.ReadAll(c.Request().Body)
	if err != nil {
		err = fmt.Errorf("failed to read request body: %w", err)
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	key := path.Base(c.Path())
	var comm []byte

	if len(key) > 0 && key != Put { // commitment key already provided (keccak256)
		comm, err = commitments.StringToDecodedCommitment(key, meta.Mode)
		if err != nil {
			err = fmt.Errorf("failed to decode commitment from key %v (commitment mode %v): %w", key, meta.Mode, err)
			return echo.NewHTTPError(http.StatusBadRequest, err)
		}
	}

	commitment, err := svr.router.Put(c.Request().Context(), meta.Mode, comm, input)
	if err != nil {
		err = fmt.Errorf("put request failed with commitment %v (commitment mode %v): %w", comm, meta.Mode, err)
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	responseCommit, err := commitments.EncodeCommitment(commitment, meta.Mode)
	if err != nil {
		err = fmt.Errorf("failed to encode commitment %v (commitment mode %v): %w", commitment, meta.Mode, err)
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.Blob(http.StatusOK, "application/octet-stream", responseCommit)
}

func (svr *Server) WriteResponse(w http.ResponseWriter, data []byte) {
	if _, err := w.Write(data); err != nil {
		svr.WriteInternalError(w, err)
	}
}

func (svr *Server) WriteInternalError(w http.ResponseWriter, err error) {
	svr.log.Error("internal server error", "err", err)
	w.WriteHeader(http.StatusInternalServerError)
}

func (svr *Server) WriteNotFound(w http.ResponseWriter, err error) {
	svr.log.Info("not found", "err", err)
	w.WriteHeader(http.StatusNotFound)
}

func (svr *Server) WriteBadRequest(w http.ResponseWriter, err error) {
	svr.log.Info("bad request", "err", err)
	w.WriteHeader(http.StatusBadRequest)
}

func (svr *Server) Port() int {
	// read from listener
	_, portStr, _ := net.SplitHostPort(svr.listener.Addr().String())
	port, _ := strconv.Atoi(portStr)
	return port
}

// Read both commitment mode and version
func ReadCommitmentMeta(r *http.Request) (commitments.CommitmentMeta, error) {
	// label requests with commitment mode and version
	ct, err := ReadCommitmentMode(r)
	if err != nil {
		return commitments.CommitmentMeta{}, err
	}
	if ct == "" {
		return commitments.CommitmentMeta{}, fmt.Errorf("commitment mode is empty")
	}
	cv, err := ReadCommitmentVersion(r, ct)
	if err != nil {
		// default to version 0
		return commitments.CommitmentMeta{Mode: ct, CertVersion: cv}, err
	}
	return commitments.CommitmentMeta{Mode: ct, CertVersion: cv}, nil
}

func ReadCommitmentMode(r *http.Request) (commitments.CommitmentMode, error) {
	query := r.URL.Query()
	key := query.Get(CommitmentModeKey)
	if key != "" {
		return commitments.StringToCommitmentMode(key)
	}

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
			return commitments.OptimismAltDA, nil

		case byte(commitments.Keccak256CommitmentType):
			return commitments.OptimismGeneric, nil

		default:
			return commitments.SimpleCommitmentMode, fmt.Errorf("unknown commit byte prefix")
		}
	}
	return commitments.OptimismAltDA, nil
}

func ReadCommitmentVersion(r *http.Request, mode commitments.CommitmentMode) (byte, error) {
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

		if mode == commitments.OptimismAltDA || mode == commitments.SimpleCommitmentMode {
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
