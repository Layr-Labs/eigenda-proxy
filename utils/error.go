package utils

import (
	"fmt"

	"github.com/google/uuid"
)

type APIError struct {
	InternalMsg string `json:"internal_msg"`
	ExternalMsg string `json:"user_msg"`
	StatusCode  int    `json:"status_code"`
	UUID        string `json:"uuid"`
}

// Error serializes the error to json structured string
func (e *APIError) Error() string {
	return fmt.Sprintf(`"internal_msg": "%s", "user_msg": "%s", "status_code": %d, "uuid": "%s"}`,
		e.InternalMsg, e.ExternalMsg, e.StatusCode, e.UUID)
}

func (e *APIError) SerializeResponse() []byte {
	if e.ExternalMsg == "" {
		e.ExternalMsg = e.InternalMsg
	}

	return []byte(fmt.Sprintf(`{"error_msg": "%s", "status_code": %d, "uuid": "%s"}`,
		e.ExternalMsg, e.StatusCode, e.UUID))
}

func (e *APIError) WithExternalMsg(msg string) *APIError {
	e.ExternalMsg = msg
	return e
}

func (e *APIError) WithStatusCode(code int) *APIError {
	e.StatusCode = code
	return e
}

func NewError(internalMsg string) *APIError {
	return &APIError{
		InternalMsg: internalMsg,
		UUID:        uuid.NewString(),
	}
}
