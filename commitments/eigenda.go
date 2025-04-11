package commitments

type EigenDACommitmentType byte

const (
	// EigenDA V1
	CertV0 EigenDACommitmentType = iota
	// EigenDA V2
	CertV1
	// EigenDA V2 with encoding byte
	CertV2
)

// CertCommitment is the binary representation of a commitment.
type CertCommitment interface {
	CommitmentType() EigenDACommitmentType
	Encode() []byte
	Verify(input []byte) error
}

type EigenDACommitment struct {
	prefix   EigenDACommitmentType
	b        []byte
	encoding *EncodingType // Optional encoding type, used only for V2+
}

// NewEigenDACommitment creates a new commitment from the given input.
func NewEigenDACommitment(input []byte, commitmentType EigenDACommitmentType) EigenDACommitment {
	return EigenDACommitment{
		prefix: commitmentType,
		b:      input,
	}
}

// NewEigenDACommitmentWithEncoding creates a new commitment with encoding from the given input.
func NewEigenDACommitmentWithEncoding(input []byte, commitmentType EigenDACommitmentType,
	encoding EncodingType) EigenDACommitment {
	return EigenDACommitment{
		prefix:   commitmentType,
		b:        input,
		encoding: &encoding,
	}
}

// CommitmentType returns the commitment type of EigenDACommitment.
func (c EigenDACommitment) CommitmentType() EigenDACommitmentType {
	return c.prefix
}

// Encode adds a commitment type prefix self describing the commitment.
func (c EigenDACommitment) Encode() []byte {
	// For V2+ certificates with encoding type
	if c.prefix >= CertV2 && c.encoding != nil {
		return append([]byte{byte(c.prefix), byte(*c.encoding)}, c.b...)
	}

	// For V0/V1 certificates (backward compatibility)
	return append([]byte{byte(c.prefix)}, c.b...)
}
