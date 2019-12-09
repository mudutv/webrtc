// Package rtcerr implements the error wrappers defined throughout the
// WebRTC 1.0 specifications.
package rtcerr

import (
	"fmt"
)

// UnknownError indicates the operation failed for an unknown transient reason.
type UnknownError struct {
	Err error
}

func (e *UnknownError) Error() string {
	return fmt.Sprintf("UnknownError: %v", e.Err)
}

// InvalidStateError indicates the object is in an invalid state.
type InvalidStateError struct {
	Err error
}

func (e *InvalidStateError) Error() string {
	return fmt.Sprintf("InvalidStateError: %v", e.Err)
}

// InvalidAccessError indicates the object does not support the operation or
// argument.
type InvalidAccessError struct {
	Err error
}

func (e *InvalidAccessError) Error() string {
	return fmt.Sprintf("InvalidAccessError: %v", e.Err)
}

// NotSupportedError indicates the operation is not supported.
type NotSupportedError struct {
	Err error
}

func (e *NotSupportedError) Error() string {
	return fmt.Sprintf("NotSupportedError: %v", e.Err)
}

// InvalidModificationError indicates the object cannot be modified in this way.
type InvalidModificationError struct {
	Err error
}

func (e *InvalidModificationError) Error() string {
	return fmt.Sprintf("InvalidModificationError: %v", e.Err)
}

// SyntaxError indicates the string did not match the expected pattern.
type SyntaxError struct {
	Err error
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("SyntaxError: %v", e.Err)
}

// TypeError indicates an error when a value is not of the expected type.
type TypeError struct {
	Err error
}

func (e *TypeError) Error() string {
	return fmt.Sprintf("TypeError: %v", e.Err)
}

// OperationError indicates the operation failed for an operation-specific
// reason.
type OperationError struct {
	Err error
}

func (e *OperationError) Error() string {
	return fmt.Sprintf("OperationError: %v", e.Err)
}

// NotReadableError indicates the input/output read operation failed.
type NotReadableError struct {
	Err error
}

func (e *NotReadableError) Error() string {
	return fmt.Sprintf("NotReadableError: %v", e.Err)
}

// RangeError indicates an error when a value is not in the set or range
// of allowed values.
type RangeError struct {
	Err error
}

func (e *RangeError) Error() string {
	return fmt.Sprintf("RangeError: %v", e.Err)
}
