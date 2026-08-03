// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v10"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	logtesting "github.com/go-logr/logr/testing"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apisazure "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azureclientmock "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client/mock"
)

// TestConfigValidator_UserManagedEgress covers acceptance criteria C8-C13.
func TestConfigValidator_UserManagedEgress(t *testing.T) {
	const (
		vnetRG        = "byo-network-rg"
		vnetName      = "byo-vnet"
		subnetName    = "byo-workers"
		shootSubID    = "aaaaaaaa-1111-2222-3333-444444444444"
		otherSubID    = "bbbbbbbb-9999-8888-7777-666666666666"
		nsgIDSameSub  = "/subscriptions/aaaaaaaa-1111-2222-3333-444444444444/resourceGroups/central-sec-rg/providers/Microsoft.Network/networkSecurityGroups/team-nsg"
		rtIDSameSub   = "/subscriptions/aaaaaaaa-1111-2222-3333-444444444444/resourceGroups/central-sec-rg/providers/Microsoft.Network/routeTables/team-rt"
		nsgIDOtherSub = "/subscriptions/bbbbbbbb-9999-8888-7777-666666666666/resourceGroups/central-sec-rg/providers/Microsoft.Network/networkSecurityGroups/team-nsg"
	)

	baseCluster := &extensionscontroller.Cluster{
		Shoot: &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Networking: &gardencorev1beta1.Networking{
					Nodes:    to.Ptr("10.250.0.0/16"),
					Pods:     to.Ptr("100.96.0.0/11"),
					Services: to.Ptr("100.64.0.0/13"),
				},
			},
		},
	}
	baseInfraCfg := &apisazure.InfrastructureConfig{
		Networks: apisazure.NetworkConfig{
			VNet: apisazure.VNet{
				Name:          to.Ptr(vnetName),
				ResourceGroup: to.Ptr(vnetRG),
			},
			Subnet: &apisazure.SubnetReference{Name: subnetName},
		},
	}

	// happy-path subnet payload (satisfies C9 + C10 + C13 + C11 + C12).
	goodSubnet := &armnetwork.Subnet{
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix:        to.Ptr("10.250.0.0/24"),
			NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: to.Ptr(nsgIDSameSub)},
			RouteTable:           &armnetwork.RouteTable{ID: to.Ptr(rtIDSameSub)},
		},
	}

	tests := []struct {
		name           string
		cfgMutator     func(cfg *apisazure.InfrastructureConfig)
		clusterMutator func(cluster *extensionscontroller.Cluster)
		subnet         *armnetwork.Subnet
		subnetErr      error
		wantErrField   string
		wantErrType    field.ErrorType
		wantOK         bool
	}{
		{
			name:   "happy path",
			subnet: goodSubnet,
			wantOK: true,
		},
		{
			name:         "C8: subnet does not exist",
			subnet:       nil, // Get returns (nil, nil) after NotFound filter
			wantErrField: "spec.providerConfig.networks.subnet.name",
			wantErrType:  field.ErrorTypeNotFound,
		},
		{
			name: "C9: no NSG attached",
			subnet: &armnetwork.Subnet{
				Properties: &armnetwork.SubnetPropertiesFormat{
					AddressPrefix: to.Ptr("10.250.0.0/24"),
					RouteTable:    &armnetwork.RouteTable{ID: to.Ptr(rtIDSameSub)},
				},
			},
			wantErrField: "spec.providerConfig.networks.subnet.name",
			wantErrType:  field.ErrorTypeInvalid,
		},
		{
			name: "C10: no RT attached and non-overlay CNI (default)",
			subnet: &armnetwork.Subnet{
				Properties: &armnetwork.SubnetPropertiesFormat{
					AddressPrefix:        to.Ptr("10.250.0.0/24"),
					NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: to.Ptr(nsgIDSameSub)},
				},
			},
			clusterMutator: func(cluster *extensionscontroller.Cluster) {
				cluster.Shoot.Spec.Networking.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"overlay":{"enabled":false}}`),
				}
			},
			wantErrField: "spec.providerConfig.networks.subnet.name",
			wantErrType:  field.ErrorTypeInvalid,
		},
		{
			name: "C10 relaxed: no RT is fine when overlay CNI is enabled on the shoot networking",
			clusterMutator: func(cluster *extensionscontroller.Cluster) {
				cluster.Shoot.Spec.Networking.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"overlay":{"enabled":true}}`),
				}
			},
			subnet: &armnetwork.Subnet{
				Properties: &armnetwork.SubnetPropertiesFormat{
					AddressPrefix:        to.Ptr("10.250.0.0/24"),
					NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: to.Ptr(nsgIDSameSub)},
				},
			},
			wantOK: true,
		},
		{
			name: "C11: subnet CIDR not inside nodes CIDR",
			subnet: &armnetwork.Subnet{
				Properties: &armnetwork.SubnetPropertiesFormat{
					AddressPrefix:        to.Ptr("10.99.0.0/24"),
					NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: to.Ptr(nsgIDSameSub)},
					RouteTable:           &armnetwork.RouteTable{ID: to.Ptr(rtIDSameSub)},
				},
			},
			wantErrType: field.ErrorTypeInvalid,
		},
		{
			name: "C12: subnet CIDR overlaps pods CIDR",
			subnet: &armnetwork.Subnet{
				Properties: &armnetwork.SubnetPropertiesFormat{
					AddressPrefix:        to.Ptr("100.96.0.0/24"),
					NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: to.Ptr(nsgIDSameSub)},
					RouteTable:           &armnetwork.RouteTable{ID: to.Ptr(rtIDSameSub)},
				},
			},
			wantErrType: field.ErrorTypeInvalid,
		},
		{
			name: "C13: NSG in a different subscription",
			subnet: &armnetwork.Subnet{
				Properties: &armnetwork.SubnetPropertiesFormat{
					AddressPrefix:        to.Ptr("10.250.0.0/24"),
					NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: to.Ptr(nsgIDOtherSub)},
					RouteTable:           &armnetwork.RouteTable{ID: to.Ptr(rtIDSameSub)},
				},
			},
			wantErrField: "spec.providerConfig.networks.subnet.name",
			wantErrType:  field.ErrorTypeInvalid,
		},
		{
			name:         "ARM lookup fails with transient error",
			subnetErr:    errors.New("ARM returned 500"),
			wantErrType:  field.ErrorTypeInternal,
			wantErrField: "spec.providerConfig.networks.subnet.name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSubnets := azureclientmock.NewMockSubnet(ctrl)
			mockSubnets.EXPECT().
				Get(gomock.Any(), vnetRG, vnetName, subnetName, gomock.Nil()).
				Return(tc.subnet, tc.subnetErr).
				Times(1)

			cfg := baseInfraCfg.DeepCopy()
			if tc.cfgMutator != nil {
				tc.cfgMutator(cfg)
			}
			cluster := &extensionscontroller.Cluster{
				Shoot: baseCluster.Shoot.DeepCopy(),
			}
			if tc.clusterMutator != nil {
				tc.clusterMutator(cluster)
			}
			cv := &configValidator{logger: logtesting.NewTestLogger(t)}
			errs := cv.validateUserManagedEgress(context.Background(), cfg, cluster, shootSubID, mockSubnets, field.NewPath("spec", "providerConfig", "networks"))

			if tc.wantOK {
				if len(errs) != 0 {
					t.Fatalf("expected no errors, got: %v", errs)
				}
				return
			}
			if len(errs) == 0 {
				t.Fatalf("expected an error but got none")
			}
			if tc.wantErrType != "" {
				matched := false
				for _, e := range errs {
					if e.Type == tc.wantErrType {
						if tc.wantErrField == "" || e.Field == tc.wantErrField {
							matched = true
							break
						}
					}
				}
				if !matched {
					t.Fatalf("expected error type=%q field=%q, got errors: %v", tc.wantErrType, tc.wantErrField, errs)
				}
			}
		})
	}
	_ = otherSubID
}
