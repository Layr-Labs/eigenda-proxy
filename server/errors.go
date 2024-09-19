package server

import (
	"fmt"

	"github.com/Layr-Labs/eigenda-proxy/commitments"
)

// HandlerError is an interface that can be either a simple error or a MetaError
type HandlerError interface {
	error
	isHandlerError() // marker method
}

// MetaError includes both an error and commitment metadata
type MetaError struct {
	Err  error
	Meta commitments.CommitmentMeta
}

func (me MetaError) Error() string {
	return fmt.Sprintf("Error: %s (Mode: %s, CertVersion: %b)",
		me.Err.Error(),
		me.Meta.Mode,
		me.Meta.CertVersion)
}

func (MetaError) isHandlerError() {}

// NewMetaError creates a new MetaError
func NewMetaError(err error, meta commitments.CommitmentMeta) HandlerError {
	return MetaError{Err: err, Meta: meta}
}
