package server

import (
	"fmt"
	"net/http"

	"github.com/Layr-Labs/eigenda-proxy/commitments"
	"github.com/gorilla/mux"
)

func (svr *Server) registerRoutes(r *mux.Router) {
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
	r.HandleFunc("/health", withLogging(svr.handleHealth, svr.log)).Methods("GET")
}
