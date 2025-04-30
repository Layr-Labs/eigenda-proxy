package commitments

type EigenDACertVersion byte

const (
	// EigenDA V1
	CertV0 EigenDACertVersion = iota
	// EigenDA V2
	CertV1
)

type EigenDAVersionedCert struct {
	certVersion    EigenDACertVersion
	serializedCert []byte
}

// NewEigenDAVersionedCert creates a new EigenDAVersionedCert that holds the certVersion
// and a serialized certificate of that version.
func NewEigenDAVersionedCert(serializedCert []byte, certVersion EigenDACertVersion) EigenDAVersionedCert {
	return EigenDAVersionedCert{
		certVersion:    certVersion,
		serializedCert: serializedCert,
	}
}

// Encode adds a commitment type prefix self describing the commitment.
func (c EigenDAVersionedCert) Encode() []byte {
	return append([]byte{byte(c.certVersion)}, c.serializedCert...)
}
