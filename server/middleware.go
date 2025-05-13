package server

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Layr-Labs/eigenda-proxy/common/types/commitments"
	"github.com/Layr-Labs/eigenda-proxy/config"
	"github.com/Layr-Labs/eigenda-proxy/metrics"
	"github.com/Layr-Labs/eigensdk-go/logging"
)

// Used to capture the status code of the response, so that we can use it in middlewares.
// See https://github.com/golang/go/issues/18997
// TODO: right now instantiating a separate scw for logging and metrics... is there a better way?
// TODO: should we capture more information about the response, like GET vs POST, etc?
type statusCaptureWriter struct {
	http.ResponseWriter
	status int
}

func (scw *statusCaptureWriter) WriteHeader(status int) {
	scw.status = status
	scw.ResponseWriter.WriteHeader(status)
}

func newStatusCaptureWriter(w http.ResponseWriter) *statusCaptureWriter {
	return &statusCaptureWriter{
		ResponseWriter: w,
		// 200 status code is only added to response by outer layer http framework,
		// since WriteHeader(200) is typically not called by handlers.
		// So we initialize status as 200, and assume that any other status code
		// will be set by the handler.
		status: http.StatusOK,
	}
}

// withMetrics is a middleware that records metrics for the route path.
// It does not write anything to the response, that is the job of the handlers.
func withMetrics(
	handleFn func(http.ResponseWriter, *http.Request) error,
	m metrics.Metricer,
	mode commitments.CommitmentMode,
) func(http.ResponseWriter, *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		recordDur := m.RecordRPCServerRequest(r.Method)

		scw := newStatusCaptureWriter(w)
		err := handleFn(scw, r)
		if err != nil {
			commitMode := "unknown"
			certVersion := "unknown"
			var getErr GETError
			if errors.As(err, &getErr) {
				commitMode = string(getErr.Mode)
				certVersion = string(getErr.CertVersion)
			}
			var postErr POSTError
			if errors.As(err, &postErr) {
				commitMode = string(postErr.Mode)
			}
			// Prob should use different metric for POST and GET errors.
			recordDur(strconv.Itoa(scw.status), commitMode, certVersion)
			return err
		}
		certVersion, err := parseCertVersion(w, r)
		if err != nil {
			recordDur(strconv.Itoa(scw.status), string(mode), "unknown")
			return fmt.Errorf("metrics middleware: parsing version byte: %w", err)
		}
		recordDur(strconv.Itoa(scw.status), string(mode), string(certVersion))
		return nil
	}
}

// withLogging is a middleware that logs information related to each request.
// It does not write anything to the response, that is the job of the handlers.
// Currently we cannot log the status code because go's default ResponseWriter interface does not expose it.
// TODO: implement a ResponseWriter wrapper that saves the status code: see https://github.com/golang/go/issues/18997
func withLogging(
	handleFn func(http.ResponseWriter, *http.Request) error,
	log logging.Logger,
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		scw := newStatusCaptureWriter(w)
		err := handleFn(scw, r)

		args := []any{
			"method", r.Method, "url", r.URL, "status", scw.status, "duration", time.Since(start),
		}
		if err != nil {
			args = append(args, "err", err)
		}
		var getErr GETError
		if errors.As(err, &getErr) {
			args = append(args, "commitment_mode", getErr.Mode, "cert_version", getErr.CertVersion)
		}
		log.Info("request", args...)
	}
}

// withCORS is a middleware that adds CORS headers to responses.
// It intercepts OPTIONS requests for handling CORS preflight requests.
// OPTIONS requests are needed to support cross-origin requests that use custom
// headers or methods other than simple methods (GET, POST).
func withCORS(
	handleFn func(http.ResponseWriter, *http.Request),
	corsConfig config.ServerConfig,
	log logging.Logger,
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// If CORS domains list is empty, CORS is disabled
		if len(corsConfig.CORSAllowedDomains) == 0 {
			handleFn(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		if origin == "" {
			// Not a CORS request
			handleFn(w, r)
			return
		}

		// Check if origin is allowed
		allowOrigin := ""
		allowAllOrigins := false

		// Check if wildcard is used
		for _, domain := range corsConfig.CORSAllowedDomains {
			trimmedDomain := strings.TrimSpace(domain)
			if trimmedDomain == "*" {
				allowAllOrigins = true
				break
			}
		}

		if allowAllOrigins {
			allowOrigin = "*"
		} else {
			// Check for specific domain match
			allowed := false
			for _, domain := range corsConfig.CORSAllowedDomains {
				trimmedDomain := strings.TrimSpace(domain)
				// Exact match or domain match (subdomain handling)
				if trimmedDomain == origin ||
					// Match pattern like "example.com" against "subdomain.example.com"
					strings.HasSuffix(origin, "."+trimmedDomain) ||
					// Match exact subdomain like "app.example.com" against "app.example.com"
					origin == trimmedDomain {
					allowed = true
					allowOrigin = origin // Use the actual origin for specific domain matches
					break
				}
			}

			if !allowed {
				log.Info("CORS rejected", "origin", origin)
				handleFn(w, r)
				return
			}
		}

		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers",
			"Content-Type, Accept, Content-Length, Accept-Encoding, Authorization")

		// Handle preflight request (OPTIONS method is used for preflight CORS requests)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the actual handler
		handleFn(w, r)
	}
}
