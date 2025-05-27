package middleware

import (
	"context"
	"net/http"

	"github.com/Layr-Labs/eigenda-proxy/common/types/commitments"
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

// RequestContext holds request-specific data that middlewares need to share
type RequestContext struct {
	CommitmentMode commitments.CommitmentMode
	CertVersion    string
}

// ContextKey is used to store RequestContext in the request context
type ContextKey string

const RequestContextKey ContextKey = "request_context"

// GetRequestContext retrieves the RequestContext from the request
func GetRequestContext(r *http.Request) *RequestContext {
	if ctx, ok := r.Context().Value(RequestContextKey).(*RequestContext); ok {
		return ctx
	}
	return nil
}

// SetCertVersion allows handlers to set the certificate version
func SetCertVersion(r *http.Request, certVersion string) {
	if ctx := GetRequestContext(r); ctx != nil {
		ctx.CertVersion = certVersion
	}
}

func GetCertVersion(r *http.Request) string {
	if ctx := GetRequestContext(r); ctx != nil {
		return ctx.CertVersion
	}
	return "unknown"
}

// withRequestContext initializes the request context (outermost middleware)
func withRequestContext(
	handleFn func(http.ResponseWriter, *http.Request),
	mode commitments.CommitmentMode,
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := &RequestContext{
			CommitmentMode: mode,
			// CertVersion is only known and set after parsing the request,
			// so we initialize it to a default value.
			// TODO: should this flow via some other means..?
			CertVersion: "unknown",
		}

		// Add context to request
		r = r.WithContext(context.WithValue(r.Context(), RequestContextKey, ctx))

		scw := newStatusCaptureWriter(w)
		handleFn(scw, r)

		// RequestContext middleware is the outermost middleware,
		// so there is nothing to do after the handler is called.
	}
}
