package memconfig

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Layr-Labs/eigensdk-go/logging"
	"github.com/gorilla/mux"
)

// JSON bodies received by the PATCH /memstore/config endpoint are deserialized into this struct,
// which is then used to update the memstore configuration.
type ConfigUpdate struct {
	MaxBlobSizeBytes        *uint64 `json:"MaxBlobSizeBytes,omitempty"`
	PutLatency              *string `json:"PutLatency,omitempty"`
	GetLatency              *string `json:"GetLatency,omitempty"`
	PutReturnsFailoverError *bool   `json:"PutReturnsFailoverError,omitempty"`
	BlobExpiration          *string `json:"BlobExpiration,omitempty"`
	GetReturnsStatusCode    *int    `json:"GetReturnsStatusCode,omitempty"`
}

// HandlerHTTP is an admin HandlerHTTP for GETting and PATCHing the memstore configuration.
// It adds routes to the proxy's main router (to be served on same port as the main proxy routes):
// - GET /memstore/config: returns the current memstore configuration
// - PATCH /memstore/config: updates the memstore configuration
type HandlerHTTP struct {
	log        logging.Logger
	safeConfig *SafeConfig
}

func NewHandlerHTTP(log logging.Logger, safeConfig *SafeConfig) HandlerHTTP {
	return HandlerHTTP{
		log:        log,
		safeConfig: safeConfig,
	}
}

func (api HandlerHTTP) RegisterMemstoreConfigHandlers(r *mux.Router) {
	memstore := r.PathPrefix("/memstore").Subrouter()
	memstore.HandleFunc("/config", api.handleGetConfig).Methods("GET")
	memstore.HandleFunc("/config", api.handleUpdateConfig).Methods("PATCH")
	memstore.HandleFunc("/config/status-code", api.handleGetStatusCode).Methods("GET")
}

// Returns the config of the memstore in json format.
// TODO: we prob want to use out custom Duration type instead of time.Duration
// since time.Duration serializes to nanoseconds, which is hard to read.
func (api HandlerHTTP) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	// Return the current configuration
	err := json.NewEncoder(w).Encode(api.safeConfig.Config())
	if err != nil {
		api.log.Error("failed to encode config", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Returns the status code in the instructed mode of the memstore in json format.
// If instructed mode is not activated, return an error
func (api HandlerHTTP) handleGetStatusCode(w http.ResponseWriter, _ *http.Request) {
	// Return the current configuration
	config := api.safeConfig.Config()
	if !config.currMode.IsActivated {
		http.Error(w, "memstore is not configured with the instructed mode", http.StatusInternalServerError)
	}

	payload := struct {
		GetReturnsStatusCode int
	}{
		GetReturnsStatusCode: config.currMode.GetReturnsStatusCode,
	}

	err := json.NewEncoder(w).Encode(payload)
	if err != nil {
		api.log.Error("failed to encode config", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (api HandlerHTTP) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var update ConfigUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		// TODO: wrap this error?
		api.log.Info("received bad update memstore config update", "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Only update fields that were included in the request
	if update.PutLatency != nil {
		duration, err := time.ParseDuration(*update.PutLatency)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		api.safeConfig.SetLatencyPUTRoute(duration)
	}

	if update.GetLatency != nil {
		duration, err := time.ParseDuration(*update.GetLatency)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		api.safeConfig.SetLatencyGETRoute(duration)
	}

	if update.PutReturnsFailoverError != nil {
		api.safeConfig.SetPUTReturnsFailoverError(*update.PutReturnsFailoverError)
	}

	if update.MaxBlobSizeBytes != nil {
		api.safeConfig.SetMaxBlobSizeBytes(*update.MaxBlobSizeBytes)
	}

	if update.BlobExpiration != nil {
		duration, err := time.ParseDuration(*update.BlobExpiration)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		api.safeConfig.SetBlobExpiration(duration)
	}

	// This activates the instructive mode in mem store and set the proper status code
	if update.GetReturnsStatusCode != nil {
		err := api.safeConfig.SetGETReturnsStatusCode(*update.GetReturnsStatusCode)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Return the current configuration
	err := json.NewEncoder(w).Encode(api.safeConfig.Config())
	if err != nil {
		api.log.Error("failed to encode config", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
