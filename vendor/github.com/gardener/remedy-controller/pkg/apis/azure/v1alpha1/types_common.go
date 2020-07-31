// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// OperationType is a string alias.
type OperationType string

// Operation types
const (
	OperationTypeGetPublicIPAddress    OperationType = "GetPublicIPAddress"
	OperationTypeCleanPublicIPAddress  OperationType = "CleanPublicIPAddress"
	OperationTypeGetVirtualMachine     OperationType = "GetVirtualMachine"
	OperationTypeReapplyVirtualMachine OperationType = "ReapplyVirtualMachine"
)

// FailedOperation describes a failed Azure operation that has been attempted a certain number of times.
type FailedOperation struct {
	// Type is the operation type.
	Type OperationType `json:"type"`
	// Attempts is the number of times the operation was attempted so far.
	Attempts int `json:"attempts"`
	// ErrorMessage is a the error message from the last operation failure.
	ErrorMessage string `json:"errorMessage"`
	// Timestamp is the timestamp of the last operation failure.
	Timestamp metav1.Time `json:"timestamp"`
}

// AddOrUpdateFailedOperation adds a new or updates an existing FailedOperation of the given type in the given slice.
func AddOrUpdateFailedOperation(failedOperations *[]FailedOperation, opType OperationType, errorMessage string, timestamp metav1.Time) *FailedOperation {
	for i, op := range *failedOperations {
		if op.Type == opType {
			op = FailedOperation{
				Type:         opType,
				Attempts:     op.Attempts + 1,
				ErrorMessage: errorMessage,
				Timestamp:    timestamp,
			}
			(*failedOperations)[i] = op
			return &op
		}
	}
	op := FailedOperation{
		Type:         opType,
		Attempts:     1,
		ErrorMessage: errorMessage,
		Timestamp:    timestamp,
	}
	*failedOperations = append(*failedOperations, op)
	return &op
}

// DeleteFailedOperation deletes the FailedOperation of the given type from the given slice, if found.
func DeleteFailedOperation(failedOperations *[]FailedOperation, opType OperationType) {
	for i, op := range *failedOperations {
		if op.Type == opType {
			*failedOperations = append((*failedOperations)[:i], (*failedOperations)[i+1:]...)
			break
		}
	}
}
