package commitments

type DAServiceOPCommitmentType byte

const (
	EigenDAOPCommitmentType DAServiceOPCommitmentType = 0
)

type EigenDASvcCommitment []byte


// Encode adds a commitment type prefix self describing the commitment.
func (c EigenDASvcCommitment) Encode() []byte {
	return append([]byte{byte(EigenDAOPCommitmentType)}, c...)
}
