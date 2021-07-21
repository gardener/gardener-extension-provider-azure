// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"
	"regexp"
	"strings"

	"github.com/Azure/go-autorest/autorest/to"
)

// GetAll returns a map of all zone names mapped to their IDs.
func (c DNSZoneClient) GetAll(ctx context.Context) (map[string]string, error) {
	zones := make(map[string]string)

	results, err := c.client.ListComplete(ctx, nil)
	if err != nil {
		return nil, err
	}
	for results.NotDone() {
		zone := results.Value()
		resourceGroupName, err := getResourceGroupName(to.String(zone.ID))
		if err != nil {
			return nil, err
		}
		zones[to.String(zone.Name)] = zoneID(resourceGroupName, to.String(zone.Name))
		if err := results.NextWithContext(ctx); err != nil {
			return nil, err
		}
	}

	return zones, nil
}

var resourceGroupRegex = regexp.MustCompile("/resourceGroups/([^/]+)/")

func getResourceGroupName(zoneID string) (string, error) {
	submatches := resourceGroupRegex.FindStringSubmatch(zoneID)
	if len(submatches) != 2 {
		return "", fmt.Errorf("unexpected DNS zone ID %s", zoneID)
	}
	return submatches[1], nil
}

func zoneID(resourceGroupName, zoneName string) string {
	return resourceGroupName + "/" + zoneName
}

func resourceGroupAndZoneNames(zoneID string) (string, string) {
	parts := strings.Split(zoneID, "/")
	if len(parts) != 2 {
		return "", zoneID
	}
	return parts[0], parts[1]
}
