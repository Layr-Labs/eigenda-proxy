package utils

import (
	"sync"
)

type AtomicRef[T any] struct {
	value   T
	rwMutex *sync.RWMutex
}

func NewAtomicRef[T any](v T) *AtomicRef[T] {
	return &AtomicRef[T]{
		value:   v,
		rwMutex: &sync.RWMutex{},
	}
}

func (ar *AtomicRef[T]) Update(newValue T) {
	ar.rwMutex.Lock()
	ar.value = newValue
	ar.rwMutex.Unlock()
}

func (ar *AtomicRef[T]) Value() T {
	ar.rwMutex.RLock()
	defer ar.rwMutex.RUnlock()
	return ar.value
}
