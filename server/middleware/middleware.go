package middleware

import (
	"net/http"

	"github.com/Layr-Labs/eigenda-proxy/common/types/commitments"
	"github.com/Layr-Labs/eigenda-proxy/metrics"
	"github.com/Layr-Labs/eigensdk-go/logging"
)

// Helper function to chain middlewares in the correct order
// Context -> Logging -> Metrics -> Error Handling -> Handler
func ChainMiddlewares(
	handler func(http.ResponseWriter, *http.Request) error,
	log logging.Logger,
	m metrics.Metricer,
	mode commitments.CommitmentMode,
) http.HandlerFunc {
	return withRequestContext(
		WithLogging(
			withMetrics(
				WithErrorHandling(handler),
				m,
				mode,
			),
			log,
		),
		mode,
	)
}
