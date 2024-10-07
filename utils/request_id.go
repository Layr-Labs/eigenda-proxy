package utils

import (
	"context"

	"github.com/ethereum/go-ethereum/log"
	"github.com/google/uuid"
)

type ctxKey string

const (
	// requestIDKey is the context key for the request ID
	// used both in the context and in the logger
	requestIDKey ctxKey = "requestID"
)

// getRequestID retrieves the request ID from the context
func getRequestID(ctx context.Context) (string, bool) {
	requestID, ok := ctx.Value(requestIDKey).(string)
	return requestID, ok
}

// RequestLogger returns a new logger with the requestID added as a key-value pair
func RequestLogger(ctx context.Context, log log.Logger) log.Logger {
	requestID, ok := getRequestID(ctx)
	if !ok {
		return log
	}
	return log.With("requestID", requestID)
}

func ContextWithNewRequestID(ctx context.Context) context.Context {
	requestID := uuid.New().String()
	return context.WithValue(ctx, requestIDKey, requestID)
}
