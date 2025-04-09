package common

import (
	"strings"
)

// EigenDABackendValue implements cli.Generic for use with EigenDABackend flags
type EigenDABackendValue struct {
	Value *EigenDABackend
}

// Set converts the string to an EigenDABackend and stores it in Value
func (e *EigenDABackendValue) Set(value string) error {
	backend, err := StringToEigenDABackend(value)
	if err != nil {
		return err
	}
	*e.Value = backend
	return nil
}

// String returns the string representation of the EigenDABackend
func (e *EigenDABackendValue) String() string {
	return EigenDABackendToString(*e.Value)
}

// NewEigenDABackendValue creates a new EigenDABackendValue with the given default value
func NewEigenDABackendValue(value EigenDABackend) *EigenDABackendValue {
	return &EigenDABackendValue{Value: &value}
}

// EigenDABackendSliceValue implements cli.Generic for use with []EigenDABackend flags
type EigenDABackendSliceValue struct {
	Value *[]EigenDABackend
}

// Set converts a comma-separated string to a slice of EigenDABackend values
func (e *EigenDABackendSliceValue) Set(value string) error {
	if value == "" {
		*e.Value = []EigenDABackend{}
		return nil
	}

	// Split the comma-separated values
	values := strings.Split(value, ",")
	backends := make([]EigenDABackend, 0, len(values))

	for _, v := range values {
		backend, err := StringToEigenDABackend(v)
		if err != nil {
			return err
		}
		backends = append(backends, backend)
	}

	*e.Value = backends
	return nil
}

// String returns a comma-separated string representation of the []EigenDABackend
func (e *EigenDABackendSliceValue) String() string {
	if e.Value == nil || len(*e.Value) == 0 {
		return ""
	}

	var values []string
	for _, backend := range *e.Value {
		values = append(values, EigenDABackendToString(backend))
	}

	return strings.Join(values, ",")
}

// NewEigenDABackendSliceValue creates a new EigenDABackendSliceValue with the given default value
func NewEigenDABackendSliceValue(value []EigenDABackend) *EigenDABackendSliceValue {
	return &EigenDABackendSliceValue{Value: &value}
}
