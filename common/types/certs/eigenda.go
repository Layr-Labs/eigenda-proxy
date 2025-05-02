package certs

import "fmt"

// Version byte that prefixes serialized EigenDACert to identify their type.
type VersionByte byte

const (
	// EigenDA V1
	V0VersionByte VersionByte = iota
	// All future CertVersions will be against EigenDA V2 Blazar (https://docs.eigenda.xyz/releases/blazar)
	V1VersionByte
)

func ByteToVersion(b byte) (VersionByte, error) {
	switch b {
	case byte(V0VersionByte):
		return V0VersionByte, nil
	case byte(V1VersionByte):
		return V1VersionByte, nil
	default:
		return 0, fmt.Errorf("unknown EigenDA cert version: %d", b)
	}
}

type EigenDAVersionedCert struct {
	Version        VersionByte
	SerializedCert []byte
}

// NewEigenDAVersionedCert creates a new EigenDAVersionedCert that holds the certVersion
// and a serialized certificate of that version.
func NewEigenDAVersionedCert(serializedCert []byte, certVersion VersionByte) EigenDAVersionedCert {
	return EigenDAVersionedCert{
		Version:        certVersion,
		SerializedCert: serializedCert,
	}
}

// Encode adds a commitment type prefix self describing the commitment.
func (c EigenDAVersionedCert) Encode() []byte {
	return append([]byte{byte(c.Version)}, c.SerializedCert...)
}
