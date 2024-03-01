// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/dns/mgmt/2018-05-01/dns"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
)

var _ DNSRecordSet = &DNSRecordSetClient{}

// DNSRecordSetClient is an implementation of DNSRecordSet for a DNS recordset k8sClient.
type DNSRecordSetClient struct {
	client dns.RecordSetsClient
}

// CreateOrUpdate creates or updates the recordset with the given name, record type, values, and TTL in the zone with the given zone ID.
func (c *DNSRecordSetClient) CreateOrUpdate(ctx context.Context, zoneID string, name string, recordType string, values []string, ttl int64) error {
	resourceGroupName, zoneName := resourceGroupAndZoneNames(zoneID)
	relativeRecordSetName, err := getRelativeRecordSetName(name, zoneName)
	if err != nil {
		return err
	}
	params := dns.RecordSet{
		RecordSetProperties: newRecordSetProperties(dns.RecordType(recordType), values, ttl),
	}
	_, err = c.client.CreateOrUpdate(ctx, resourceGroupName, zoneName, relativeRecordSetName, dns.RecordType(recordType), params, "", "")
	return err
}

// Delete deletes the recordset with the given name and record type in the zone with the given zone ID.
func (c *DNSRecordSetClient) Delete(ctx context.Context, zoneID string, name string, recordType string) error {
	resourceGroupName, zoneName := resourceGroupAndZoneNames(zoneID)
	relativeRecordSetName, err := getRelativeRecordSetName(name, zoneName)
	if err != nil {
		return err
	}
	_, err = c.client.Delete(ctx, resourceGroupName, zoneName, relativeRecordSetName, dns.RecordType(recordType), "")
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

func newRecordSetProperties(recordType dns.RecordType, values []string, ttl int64) *dns.RecordSetProperties {
	rrp := &dns.RecordSetProperties{
		TTL: to.Int64Ptr(ttl),
	}
	switch recordType {
	case dns.A:
		var aRecords []dns.ARecord
		for _, value := range values {
			aRecords = append(aRecords, dns.ARecord{
				Ipv4Address: to.StringPtr(value),
			})
		}
		rrp.ARecords = &aRecords
	case dns.CNAME:
		rrp.CnameRecord = &dns.CnameRecord{
			Cname: to.StringPtr(values[0]),
		}
	case dns.TXT:
		var txtRecords []dns.TxtRecord
		for _, value := range values {
			txtRecords = append(txtRecords, dns.TxtRecord{
				Value: &[]string{value},
			})
		}
		rrp.TxtRecords = &txtRecords
	}
	return rrp
}

func ignoreAzureNotFoundError(err error) error {
	if err == nil {
		return nil
	}
	if e, ok := err.(autorest.DetailedError); ok && e.StatusCode == http.StatusNotFound {
		return nil
	}
	return err
}
