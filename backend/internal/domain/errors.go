// Package domain declares errors shared across service and persistence boundaries.
package domain

import "errors"

var (
	// ErrConflict indicates that a unique domain value already exists.
	ErrConflict = errors.New("domain conflict")
	// ErrNotFound indicates that the requested persisted value does not exist.
	ErrNotFound = errors.New("domain value not found")
)
