// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"fmt"
)

// SpecMismatchError is an error to indicate that the reconciliation cannot proceed or the operation requested is not supported.
type SpecMismatchError struct {
	// AzureResourceMetadata describe uniquely an Azure resource
	AzureResourceMetadata
	// Field is the name of field that could not be reconciled.
	Field string
	// Expected is the value of the field that was expected.
	Expected any
	// Found is the actual value of Field.
	Found any
	// Info contains additional information or instruction to the user.
	Info *string
}

// NewSpecMismatchError creates a TerminalSpecMismatch error.
func NewSpecMismatchError(identifier AzureResourceMetadata, offender string, expected, found any, info *string) *SpecMismatchError {
	return &SpecMismatchError{AzureResourceMetadata: identifier, Field: offender, Expected: expected, Found: found, Info: info}
}

func (t *SpecMismatchError) Error() string {
	s := fmt.Sprintf("differences between the current and target spec require the object to be deleted."+
		"Resource: %s, Name: %s, Field: %s, Expected: %v, Found: %v", t.Kind, t.Name, t.Field, t.Expected, t.Found)
	if t.Info != nil {
		s = fmt.Sprintf("%s. Additional info: %s", s, *t.Info)
	}
	return s
}

// TerminalConditionError is an error to mark cases where the reconciliation cannot continue.
type TerminalConditionError struct {
	AzureResourceMetadata
	error
}

// NewTerminalConditionError creates a TerminalConditionError.
func NewTerminalConditionError(identifier AzureResourceMetadata, err error) *TerminalConditionError {
	return &TerminalConditionError{identifier, err}
}

func (t *TerminalConditionError) Error() string {
	return fmt.Sprintf("terminal error prevents successful reconciliation. Resource: %s, Name: %s, Error: %s", t.Kind, t.Name, t.error)
}

func (t *TerminalConditionError) Unwrap() error {
	return t.error
}
