// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/dns/mgmt/2018-05-01/dns"
	"github.com/Azure/go-autorest/autorest/to"
)

var _ DNSZone = &DNSZoneClient{}
var resourceGroupRegex = regexp.MustCompile("/resourceGroups/([^/]+)/")

// DNSZoneClient is an implementation of DNSZone for a DNS zone k8sClient.
type DNSZoneClient struct {
	client dns.ZonesClient
}

// List returns a map of all zone names mapped to their IDs.
func (c *DNSZoneClient) List(ctx context.Context) (map[string]string, error) {
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
