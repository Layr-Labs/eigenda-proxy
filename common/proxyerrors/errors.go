package proxyerrors

import (
	"fmt"

	"github.com/Layr-Labs/eigenda-proxy/common/types/commitments"
)

// POSTError wraps an error with PUT query context (mode).
// Unlike GETError, POSTError does not have CertVersion, given that the cert version
// is fixed (always the same depending on which flags proxy was started with).
type POSTError struct {
	Err  error
	Mode commitments.CommitmentMode
}

func NewPOSTError(err error, mode commitments.CommitmentMode) POSTError {
	return POSTError{
		Err:  err,
		Mode: mode,
	}
}

func (me POSTError) Error() string {
	return fmt.Sprintf("Error in PUT route (Mode: %s): %s",
		me.Mode,
		me.Err.Error())
}

// Used to satisfy the error interface: https://pkg.go.dev/errors.
// This is needed to use errors.Is() and errors.As() to check for specific errors.
func (me POSTError) Unwrap() error {
	return me.Err
}
