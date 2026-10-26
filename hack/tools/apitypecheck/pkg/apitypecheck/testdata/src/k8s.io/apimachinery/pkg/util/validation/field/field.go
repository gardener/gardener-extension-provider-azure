// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

// Package field provides stub types for testdata.
package field

// Path tracks the field path for error reporting.
type Path struct{}

// NewPath creates a new root Path.
func NewPath(name string, moreNames ...string) *Path { return &Path{} }

// Child returns a new Path with the given name appended.
func (p *Path) Child(name string, moreNames ...string) *Path { return &Path{} }

// Index returns a new Path with an index appended.
func (p *Path) Index(index int) *Path { return &Path{} }

// ErrorList is a list of validation errors.
type ErrorList []*Error

// Error is a validation error.
type Error struct {
	Type   ErrorType
	Field  string
	Detail string
}

// ErrorType is the type of a validation error.
type ErrorType string

const (
	ErrorTypeRequired ErrorType = "FieldValueRequired"
	ErrorTypeInvalid  ErrorType = "FieldValueInvalid"
)

// Required returns an error indicating a required field is missing.
func Required(field *Path, detail string) *Error { return &Error{} }

// Invalid returns an error indicating a field has an invalid value.
func Invalid(field *Path, value any, detail string) *Error { return &Error{} }
