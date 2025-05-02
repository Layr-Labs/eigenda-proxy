package commitments

// StandardCommitment is the default commitment used by arbitrum nitro stack, AVSs,
// and any stack that doesn't need any specific bytes prefix.
// Its encoding simply returns the serialized versionedCert.
type StandardCommitment struct {
	versionedCert EigenDAVersionedCert
}

// NewOPEigenDAGenericCommitment creates a new commitment from the given input.
func NewStandardCommitment(versionedCert EigenDAVersionedCert) StandardCommitment {
	return StandardCommitment{versionedCert}
}
func (c StandardCommitment) Encode() []byte {
	return c.versionedCert.Encode()
}
