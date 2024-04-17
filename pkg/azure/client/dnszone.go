// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

var _ DNSZone = &DNSZoneClient{}
var resourceGroupRegex = regexp.MustCompile("/resourceGroups/([^/]+)/")

// DNSZoneClient is an implementation of DNSZone for a DNS zone k8sClient.
type DNSZoneClient struct {
	client *armdns.ZonesClient
}

// NewDnsZoneClient creates a new DnsZoneClient
func NewDnsZoneClient(auth *internal.ClientAuth, tc azcore.TokenCredential, opts *policy.ClientOptions) (*DNSZoneClient, error) {
	client, err := armdns.NewZonesClient(auth.SubscriptionID, tc, opts)
	return &DNSZoneClient{client}, err
}

// List returns a map of all zone names mapped to their IDs.
func (c *DNSZoneClient) List(ctx context.Context) (map[string]string, error) {
	zones := make(map[string]string)

	results := c.client.NewListPager(nil)
	for results.More() {
		nextResult, err := results.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, zone := range nextResult.Value {
			resourceGroupName, err := getResourceGroupName(*zone.ID)
			if err != nil {
				return nil, err
			}
			zoneName := *zone.Name
			zones[zoneName] = zoneID(resourceGroupName, zoneName)
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
