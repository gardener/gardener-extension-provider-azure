// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package client

import (
	"context"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

// StorageAuth represents a Azure storage auth.
type StorageAuth struct {
	// StorageAccount is the data field in a secret where the storage account is stored at.
	StorageAccount []byte
	// StorageKey is the data field in a secret where the storage key is stored at.
	StorageKey []byte
}

// StorageClient represents a Azure storage client.
type StorageClient struct {
	// serviceURL is azure storage serviceURL object configured with storage credentials and pipeline.
	serviceURL azblob.ServiceURL
}

// Storage represents a Azure storage client.
type Storage interface {
	DeleteObjectsWithPrefix(ctx context.Context, container, prefix string) error
	CreateContainerIfNotExists(ctx context.Context, container string) error
	DeleteContainerIfExists(ctx context.Context, container string) error
}
