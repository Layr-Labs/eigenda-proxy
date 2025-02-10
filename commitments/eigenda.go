package commitments

type EigenDACommit byte

const (
	// EigenDA V1
	CertV0 EigenDACommit = iota
	// EigenDA V2
	CertV1
)

// CertCommitment is the binary representation of a commitment.
type CertCommitment interface {
	CommitmentType() EigenDACommit
	Encode() []byte
	Verify(input []byte) error
}

type EigenDACommitment struct {
	prefix EigenDACommit
	b      []byte
}

// NewEigenDACommitment creates a new commitment from the given input.
func NewEigenDACommitment(input []byte, v EigenDACommit) EigenDACommitment {
	return EigenDACommitment{
		prefix: v,
		b:      input,
	}
}

// CommitmentType returns the commitment type of EigenDACommitment.
func (c EigenDACommitment) CommitmentType() EigenDACommit {
	return c.prefix
}

// Encode adds a commitment type prefix self describing the commitment.
func (c EigenDACommitment) Encode() []byte {
	return append([]byte{byte(c.prefix)}, c.b...)
}
