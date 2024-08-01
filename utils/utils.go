package utils

import "sync/atomic"

// Define a struct with an atomic integer
type AtomicInt64 struct {
	value int64
}

func NewAtomicInt64(value int64) *AtomicInt64 {
	return &AtomicInt64{value: value}
}

// Increment the atomic integer
func (a *AtomicInt64) Increment() {
	atomic.AddInt64(&a.value, 1)
}

// Decrement the atomic integer
func (a *AtomicInt64) Decrement() {
	atomic.AddInt64(&a.value, -1)
}

// Get the value of the atomic integer
func (a *AtomicInt64) Value() int64 {
	return atomic.LoadInt64(&a.value)
}

func (a *AtomicInt64) ValueUnsignedInt() uint {
	return uint(atomic.LoadInt64(&a.value))
}