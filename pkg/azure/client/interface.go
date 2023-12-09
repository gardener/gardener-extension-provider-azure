//  Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package client

import (
	"context"
)

// GetFunc gets a resource of type T.
type GetFunc[T any] interface {
	Get(ctx context.Context, resourceGroupName string, resourceName string) (*T, error)
}

// SubResourceGetFunc gets a subresource of type T.
type SubResourceGetFunc[T any] interface {
	Get(ctx context.Context, resourceGroupName string, parentResourceName string, resourceName string) (*T, error)
}

// GetWithExpandFunc gets a resource and allows expansion of other referenced resources.
type GetWithExpandFunc[T, E any] interface {
	Get(ctx context.Context, resourceGroupName string, resourceName string, expand E) (*T, error)
}

// SubResourceGetWithExpandFunc gets a subresource and allows expansion of other referenced resources.
type SubResourceGetWithExpandFunc[T, E any] interface {
	Get(ctx context.Context, resourceGroupName string, parentResourceName string, resourceName string, expand E) (*T, error)
}

// ListFunc gets a list of resources is the target resource group.
type ListFunc[T any] interface {
	List(ctx context.Context, resourceGroupName string) (result []*T, err error)
}

// SubResourceListFunc gets a list of subresources is the target resource.
type SubResourceListFunc[T any] interface {
	List(ctx context.Context, resourceGroupName string, parentResourceName string) ([]*T, error)
}

// CreateOrUpdateFunc creates or updates a resource.
type CreateOrUpdateFunc[T any] interface {
	CreateOrUpdate(ctx context.Context, resourceGroupName string, resourceName string, resourceParam T) (*T, error)
}

// SubResourceCreateOrUpdateFunc creates or updates a subresource.
type SubResourceCreateOrUpdateFunc[T any] interface {
	CreateOrUpdate(ctx context.Context, resourceGroupName string, parentResourceName string, resourceName string, resourceParam T) (*T, error)
}

// DeleteFunc deletes a resource.
type DeleteFunc[T any] interface {
	Delete(ctx context.Context, resourceGroupName string, resourceName string) error
}

// DeleteWithOptsFunc deletes a resource with the specified deleteOpts.
type DeleteWithOptsFunc[T, O any] interface {
	Delete(ctx context.Context, resourceGroupName string, resourceName string, opts O) error
}

// SubResourceDeleteFunc deletes a resource.
type SubResourceDeleteFunc[T any] interface {
	Delete(ctx context.Context, resourceGroupName string, parentResourceName string, resourceName string) error
}

// ContainerCreateOrUpdateFunc creates or updates a container resource for example resource groups.
type ContainerCreateOrUpdateFunc[T any] interface {
	CreateOrUpdate(ctx context.Context, container string, resourceParam T) (*T, error)
}

// ContainerGetFunc retrieves a container resource.
type ContainerGetFunc[T any] interface {
	Get(ctx context.Context, container string) (*T, error)
}

// ContainerDeleteFunc deletes the specified container resource.
type ContainerDeleteFunc[T any] interface {
	Delete(ctx context.Context, container string) error
}

// ContainerCheckExistenceFunc checks if the container resource exists in the infrastructure.
type ContainerCheckExistenceFunc[T any] interface {
	CheckExistence(ctx context.Context, container string) (bool, error)
}
