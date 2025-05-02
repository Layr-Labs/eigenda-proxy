package commitments

import (
	"github.com/Layr-Labs/eigenda-proxy/common/types/certs"
)

// StandardCommitment is the default commitment used by arbitrum nitro stack, AVSs,
// and any stack that doesn't need any specific bytes prefix.
// Its encoding simply returns the serialized versionedCert.
type StandardCommitment struct {
	versionedCert certs.EigenDAVersionedCert
}

// NewOPEigenDAGenericCommitment creates a new commitment from the given input.
func NewStandardCommitment(versionedCert certs.EigenDAVersionedCert) StandardCommitment {
	return StandardCommitment{versionedCert}
}
func (c StandardCommitment) Encode() []byte {
	return c.versionedCert.Encode()
}
