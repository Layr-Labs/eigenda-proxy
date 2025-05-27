package middleware

import (
	"net/http"
	"time"

	"github.com/Layr-Labs/eigensdk-go/logging"
)

// WithLogging is a middleware that logs information related to each request.
// It does not write anything to the response, that is the job of the handlers.
// Currently we cannot log the status code because go's default ResponseWriter interface does not expose it.
// TODO: implement a ResponseWriter wrapper that saves the status code: see https://github.com/golang/go/issues/18997
func WithLogging(
	handleFn func(http.ResponseWriter, *http.Request) error,
	log logging.Logger,
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		scw := newStatusCaptureWriter(w)
		err := handleFn(scw, r)

		ctx := GetRequestContext(r)
		if ctx == nil {
			log.Error("logging middleware: request context not found")
		}

		args := []any{
			"method", r.Method, "url", r.URL,
			"status", scw.status, "duration", time.Since(start),
			"commitment_mode", ctx.CommitmentMode, "cert_version", ctx.CertVersion,
		}
		if err != nil {
			args = append(args, "error", err.Error())
			log.Error("request completed with error", args...)
		} else {
			log.Info("request completed", args...)
		}
	}
}
