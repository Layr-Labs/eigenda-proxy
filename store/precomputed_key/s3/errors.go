package s3

// TODO: this doesn't belong in the s3 package, but currently the Verify function returns
// this error and is on S3. That also should be moved elsewhere.
type Keccak256KeyValueMismatchError struct {
	Key   string
	Value string
}

func NewKeccak256KeyValueMismatchErr(key, value string) Keccak256KeyValueMismatchError {
	return Keccak256KeyValueMismatchError{
		Key:   key,
		Value: value,
	}
}

func (e Keccak256KeyValueMismatchError) Error() string {
	return "key does not match value, expected: " + e.Key + " got: " + e.Value
}
