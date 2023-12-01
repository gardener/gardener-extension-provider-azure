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
	"crypto/tls"
	"math"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

const (
	// DefaultMaxRetries is the default value for max retries on retryable operations.
	DefaultMaxRetries = 3
	// DefaultMaxRetryDelay is the default maximum value for delay on retryable operations.
	DefaultMaxRetryDelay = math.MaxInt64
	// DefaultRetryDelay is the default value for the initial delay on retry for retryable operations.
	DefaultRetryDelay = 5 * time.Second
)

var (
	// DefaultAzureClientOpts generates clientOptions for the azure clients.
	DefaultAzureClientOpts func() *arm.ClientOptions
	once                   sync.Once
)

func init() {
	once.Do(func() {
		DefaultAzureClientOpts = getAzureClientOpts
	})
}

func getTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        100,
		MaxConnsPerHost:     100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
}
func getAzureClientOpts() *arm.ClientOptions {
	return &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{
			Retry: policy.RetryOptions{
				RetryDelay:    DefaultRetryDelay,
				MaxRetryDelay: DefaultMaxRetryDelay,
				MaxRetries:    DefaultMaxRetries,
				StatusCodes:   getRetriableStatusCode(),
			},
			Transport: &http.Client{
				Transport: getTransport(),
			},
		},
	}
}

func getRetriableStatusCode() []int {
	return []int{
		http.StatusRequestTimeout,      // 408
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout,      // 504
	}
}
