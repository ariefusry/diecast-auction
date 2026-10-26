// Package service holds business logic. Services run raw SQL on pgx directly
// so multi-row operations can own their transaction and lock order.
package service

import "errors"

// Sentinel errors that handlers map to HTTP status codes.
var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrConflict           = errors.New("conflict")
	ErrForbidden          = errors.New("forbidden")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrNotFound           = errors.New("not found")
	// ErrUnauthenticated: a valid token whose user no longer exists.
	ErrUnauthenticated = errors.New("session expired")
)
