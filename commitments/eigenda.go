package commitments

import "fmt"

type EigenDACertVersion byte

const (
	// EigenDA V1
	CertV0 EigenDACertVersion = iota
	// EigenDA V2
	CertV1
)

func ByteToEigenDACertVersion(b byte) (EigenDACertVersion, error) {
	switch b {
	case byte(CertV0):
		return CertV0, nil
	case byte(CertV1):
		return CertV1, nil
	default:
		return 0, fmt.Errorf("unknown EigenDA cert version: %d", b)
	}
}

type EigenDAVersionedCert struct {
	CertVersion    EigenDACertVersion
	SerializedCert []byte
}

// NewEigenDAVersionedCert creates a new EigenDAVersionedCert that holds the certVersion
// and a serialized certificate of that version.
func NewEigenDAVersionedCert(serializedCert []byte, certVersion EigenDACertVersion) EigenDAVersionedCert {
	return EigenDAVersionedCert{
		CertVersion:    certVersion,
		SerializedCert: serializedCert,
	}
}

// Encode adds a commitment type prefix self describing the commitment.
func (c EigenDAVersionedCert) Encode() []byte {
	return append([]byte{byte(c.CertVersion)}, c.SerializedCert...)
}
