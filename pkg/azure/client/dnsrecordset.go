// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/dns/armdns"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-azure/pkg/internal"
)

var _ DNSRecordSet = &DNSRecordSetClient{}

// DNSRecordSetClient is an implementation of DNSRecordSet for a DNS recordset k8sClient.
type DNSRecordSetClient struct {
	client *armdns.RecordSetsClient
}

// NewDnsRecordSetClient creates a new DnsRecordSetClient
func NewDnsRecordSetClient(auth *internal.ClientAuth, tc azcore.TokenCredential, opts *policy.ClientOptions) (*DNSRecordSetClient, error) {
	client, err := armdns.NewRecordSetsClient(auth.SubscriptionID, tc, opts)
	return &DNSRecordSetClient{client}, err
}

// CreateOrUpdate creates or updates the recordset with the given name, record type, values, and TTL in the zone with the given zone ID.
func (c *DNSRecordSetClient) CreateOrUpdate(ctx context.Context, zoneID string, name string, recordType string, values []string, ttl int64) error {
	resourceGroupName, zoneName := resourceGroupAndZoneNames(zoneID)
	relativeRecordSetName, err := getRelativeRecordSetName(name, zoneName)
	if err != nil {
		return err
	}
	params := armdns.RecordSet{
		Properties: newRecordSetProperties(armdns.RecordType(recordType), values, ttl),
	}
	_, err = c.client.CreateOrUpdate(ctx, resourceGroupName, zoneName, relativeRecordSetName, armdns.RecordType(recordType), params, nil)
	return err
}

// Delete deletes the recordset with the given name and record type in the zone with the given zone ID.
func (c *DNSRecordSetClient) Delete(ctx context.Context, zoneID string, name string, recordType string) error {
	resourceGroupName, zoneName := resourceGroupAndZoneNames(zoneID)
	relativeRecordSetName, err := getRelativeRecordSetName(name, zoneName)
	if err != nil {
		return err
	}
	_, err = c.client.Delete(ctx, resourceGroupName, zoneName, relativeRecordSetName, armdns.RecordType(recordType), nil)
	return ignoreAzureNotFoundError(err)
}

func getRelativeRecordSetName(name, zoneName string) (string, error) {
	if name == zoneName {
		return "@", nil
	}
	suffix := "." + zoneName
	if !strings.HasSuffix(name, suffix) {
		return "", fmt.Errorf("name %s does not match zone name %s", name, zoneName)
	}
	return strings.TrimSuffix(name, suffix), nil
}

func newRecordSetProperties(recordType armdns.RecordType, values []string, ttl int64) *armdns.RecordSetProperties {
	rrp := &armdns.RecordSetProperties{
		TTL: ptr.To[int64](ttl),
	}
	switch recordType {
	case armdns.RecordTypeA:
		var aRecords []*armdns.ARecord
		for _, value := range values {
			aRecords = append(aRecords, &armdns.ARecord{
				IPv4Address: ptr.To(value),
			})
		}
		rrp.ARecords = aRecords
	case armdns.RecordTypeCNAME:
		rrp.CnameRecord = &armdns.CnameRecord{
			Cname: ptr.To(values[0]),
		}
	case armdns.RecordTypeTXT:
		var txtRecords []*armdns.TxtRecord
		for _, value := range values {
			txtRecords = append(txtRecords, &armdns.TxtRecord{
				Value: []*string{ptr.To(value)},
			})
		}
		rrp.TxtRecords = txtRecords
	}
	return rrp
}

func ignoreAzureNotFoundError(err error) error {
	if err == nil {
		return nil
	}
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) && respErr.StatusCode == http.StatusNotFound {
		return nil
	}
	return err
}
