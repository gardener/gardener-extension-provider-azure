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

package dnsrecord

import (
	"context"
	"fmt"
	"time"

	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
	v1 "k8s.io/api/core/v1"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// NewAzureClientFactory initializes a new AzureClientFactory. Exposed for testing.
	NewAzureClientFactory = newAzureClientFactoryNew
)

func newAzureClientFactoryNew(ctx context.Context, client client.Client, secretRef v1.SecretReference) (azureclient.Factory, error) {
	return azureclient.NewAzureClientFactoryWithSecretReference(ctx, client, secretRef)
}

const (
	// requeueAfterOnProviderError is a value for RequeueAfter to be returned on provider errors
	// in order to prevent quick retries that could quickly exhaust the account rate limits in case of e.g.
	// configuration issues.
	requeueAfterOnProviderError = 30 * time.Second
)

type actuator struct {
	client client.Client
}

// NewActuator creates a new dnsrecord.Actuator.
func NewActuator(client client.Client, azureClientFactory azureclient.Factory) dnsrecord.Actuator {
	return &actuator{
		client: client,
	}
}

func (a *actuator) InjectClient(client client.Client) error {
	a.client = client
	return nil
}

// Reconcile reconciles the DNSRecord.
func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, dns *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	// Create Azure DNS zone and recordset clients
	factory, err := NewAzureClientFactory(ctx, a.client, dns.Spec.SecretRef)
	if err != nil {
		return err
	}
	dnsZoneClient, err := factory.DNSZone()
	if err != nil {
		return fmt.Errorf("could not create Azure DNS zone client: %+v", err)
	}
	dnsRecordSetClient, err := factory.DNSRecordSet()
	if err != nil {
		return fmt.Errorf("could not create Azure DNS recordset client: %+v", err)
	}

	// Determine DNS zone ID
	zone, err := a.getZone(ctx, log, dns, dnsZoneClient)
	if err != nil {
		return err
	}

	// Create or update DNS recordset
	ttl := extensionsv1alpha1helper.GetDNSRecordTTL(dns.Spec.TTL)
	log.Info("Creating or updating DNS recordset", "zone", zone, "name", dns.Spec.Name, "type", dns.Spec.RecordType, "values", dns.Spec.Values, "dnsrecord", kutil.ObjectName(dns))
	if err := dnsRecordSetClient.CreateOrUpdate(ctx, zone, dns.Spec.Name, string(dns.Spec.RecordType), dns.Spec.Values, ttl); err != nil {
		return &reconcilerutils.RequeueAfterError{
			Cause:        fmt.Errorf("could not create or update DNS recordset in zone %s with name %s, type %s, and values %v: %+v", zone, dns.Spec.Name, dns.Spec.RecordType, dns.Spec.Values, err),
			RequeueAfter: requeueAfterOnProviderError,
		}
	}

	// Delete meta DNS recordset if exists
	if dns.Status.LastOperation == nil || dns.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeCreate {
		name, recordType := dnsrecord.GetMetaRecordName(dns.Spec.Name), "TXT"
		log.Info("Deleting meta DNS recordset", "zone", zone, "name", name, "type", recordType, "dnsrecord", kutil.ObjectName(dns))
		if err := dnsRecordSetClient.Delete(ctx, zone, name, recordType); err != nil {
			return &reconcilerutils.RequeueAfterError{
				Cause:        fmt.Errorf("could not delete meta DNS recordset in zone %s with name %s and type %s: %+v", zone, name, recordType, err),
				RequeueAfter: requeueAfterOnProviderError,
			}
		}
	}

	// Update resource status
	patch := client.MergeFrom(dns.DeepCopy())
	dns.Status.Zone = &zone
	return a.client.Status().Patch(ctx, dns, patch)
}

// Delete deletes the DNSRecord.
func (a *actuator) Delete(ctx context.Context, log logr.Logger, dns *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	factory, err := NewAzureClientFactory(ctx, a.client, dns.Spec.SecretRef)
	if err != nil {
		return err
	}
	// Create Azure DNS zone and recordset clients
	dnsZoneClient, err := factory.DNSZone()
	if err != nil {
		return fmt.Errorf("could not create Azure DNS zone client: %+v", err)
	}
	dnsRecordSetClient, err := factory.DNSRecordSet()
	if err != nil {
		return fmt.Errorf("could not create Azure DNS recordset client: %+v", err)
	}

	// Determine DNS zone ID
	zone, err := a.getZone(ctx, log, dns, dnsZoneClient)
	if err != nil {
		return err
	}

	// Delete DNS recordset
	log.Info("Deleting DNS recordset", "zone", zone, "name", dns.Spec.Name, "type", dns.Spec.RecordType, "dnsrecord", kutil.ObjectName(dns))
	if err := dnsRecordSetClient.Delete(ctx, zone, dns.Spec.Name, string(dns.Spec.RecordType)); err != nil {
		return &reconcilerutils.RequeueAfterError{
			Cause:        fmt.Errorf("could not delete DNS recordset in zone %s with name %s and type %s: %+v", zone, dns.Spec.Name, dns.Spec.RecordType, err),
			RequeueAfter: requeueAfterOnProviderError,
		}
	}

	return nil
}

// Restore restores the DNSRecord.
func (a *actuator) Restore(ctx context.Context, log logr.Logger, dns *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return a.Reconcile(ctx, log, dns, cluster)
}

// Migrate migrates the DNSRecord.
func (a *actuator) Migrate(ctx context.Context, log logr.Logger, dns *extensionsv1alpha1.DNSRecord, cluster *extensionscontroller.Cluster) error {
	return nil
}

func (a *actuator) getZone(ctx context.Context, log logr.Logger, dns *extensionsv1alpha1.DNSRecord, dnsZoneClient azureclient.DNSZone) (string, error) {
	switch {
	case dns.Spec.Zone != nil && *dns.Spec.Zone != "":
		return *dns.Spec.Zone, nil
	case dns.Status.Zone != nil && *dns.Status.Zone != "":
		return *dns.Status.Zone, nil
	default:
		// The zone is not specified in the resource status or spec. Try to determine the zone by
		// getting all zones of the account and searching for the longest zone name that is a suffix of dns.spec.Name
		zones, err := dnsZoneClient.GetAll(ctx)
		if err != nil {
			return "", &reconcilerutils.RequeueAfterError{
				Cause:        fmt.Errorf("could not get DNS zones: %+v", err),
				RequeueAfter: requeueAfterOnProviderError,
			}
		}
		log.Info("Got DNS zones", "zones", zones, "dnsrecord", kutil.ObjectName(dns))
		zone := dnsrecord.FindZoneForName(zones, dns.Spec.Name)
		if zone == "" {
			return "", fmt.Errorf("could not find DNS zone for name %s", dns.Spec.Name)
		}
		return zone, nil
	}
}
