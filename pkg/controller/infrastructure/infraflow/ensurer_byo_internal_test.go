// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v10"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr/testr"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azureclientmock "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client/mock"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
)

// TestEnsureUserSubnet covers the read-only BYO subnet discovery path exercised by the reconcile
// flow in BYO mode. The scenarios mirror the acceptance-criteria matrix from
// docs/proposals/flexible-network-configuration-proposal.md: happy path, missing NSG (F-series),
// missing RT with overlay off vs. on, subnet-not-found, malformed NSG/RT IDs, and Azure lookup
// errors.
func TestEnsureUserSubnet(t *testing.T) {
	const (
		shootSubID = "aaaaaaaa-1111-2222-3333-444444444444"
		vnetRG     = "byo-network-rg"
		vnetName   = "byo-vnet"
		subnetName = "byo-workers"
		nsgRG      = "central-sec-rg"
		nsgName    = "team-nsg"
		rtRG       = "central-net-rg"
		rtName     = "team-rt"
		subnetCIDR = "10.250.0.0/24"
	)

	nsgID := "/subscriptions/" + shootSubID + "/resourceGroups/" + nsgRG + "/providers/Microsoft.Network/networkSecurityGroups/" + nsgName
	rtID := "/subscriptions/" + shootSubID + "/resourceGroups/" + rtRG + "/providers/Microsoft.Network/routeTables/" + rtName

	goodSubnet := &armnetwork.Subnet{
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix:        to.Ptr(subnetCIDR),
			NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: to.Ptr(nsgID)},
			RouteTable:           &armnetwork.RouteTable{ID: to.Ptr(rtID)},
		},
	}

	tests := []struct {
		name           string
		subnet         *armnetwork.Subnet
		subnetErr      error
		overlayEnabled bool // false -> shoot networking has overlay disabled; true -> overlay enabled

		wantErr             bool
		wantErrContains     string
		wantNSGName         string
		wantNSGResourceGrp  string
		wantRTName          string
		wantRTResourceGroup string
		wantCIDR            string
		wantNoRT            bool
	}{
		{
			name:                "happy path with NSG and RT attached",
			subnet:              goodSubnet,
			wantNSGName:         nsgName,
			wantNSGResourceGrp:  nsgRG,
			wantRTName:          rtName,
			wantRTResourceGroup: rtRG,
			wantCIDR:            subnetCIDR,
		},
		{
			name: "happy path with overlay enabled and no RT attached",
			subnet: &armnetwork.Subnet{
				Properties: &armnetwork.SubnetPropertiesFormat{
					AddressPrefix:        to.Ptr(subnetCIDR),
					NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: to.Ptr(nsgID)},
				},
			},
			overlayEnabled:     true,
			wantNSGName:        nsgName,
			wantNSGResourceGrp: nsgRG,
			wantCIDR:           subnetCIDR,
			wantNoRT:           true,
		},
		{
			name: "reads addressPrefixes[0] when addressPrefix is unset",
			subnet: &armnetwork.Subnet{
				Properties: &armnetwork.SubnetPropertiesFormat{
					AddressPrefixes:      []*string{to.Ptr(subnetCIDR), to.Ptr("10.250.1.0/24")},
					NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: to.Ptr(nsgID)},
					RouteTable:           &armnetwork.RouteTable{ID: to.Ptr(rtID)},
				},
			},
			wantNSGName:         nsgName,
			wantNSGResourceGrp:  nsgRG,
			wantRTName:          rtName,
			wantRTResourceGroup: rtRG,
			wantCIDR:            subnetCIDR,
		},
		{
			name:            "subnet not found (nil response) fails",
			subnet:          nil,
			wantErr:         true,
			wantErrContains: "was not found",
		},
		{
			name: "subnet with nil Properties fails",
			subnet: &armnetwork.Subnet{
				Properties: nil,
			},
			wantErr:         true,
			wantErrContains: "no properties",
		},
		{
			name: "no NSG attached fails",
			subnet: &armnetwork.Subnet{
				Properties: &armnetwork.SubnetPropertiesFormat{
					AddressPrefix: to.Ptr(subnetCIDR),
					RouteTable:    &armnetwork.RouteTable{ID: to.Ptr(rtID)},
				},
			},
			wantErr:         true,
			wantErrContains: "no network security group attached",
		},
		{
			name: "NSG with nil ID fails",
			subnet: &armnetwork.Subnet{
				Properties: &armnetwork.SubnetPropertiesFormat{
					AddressPrefix:        to.Ptr(subnetCIDR),
					NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: nil},
					RouteTable:           &armnetwork.RouteTable{ID: to.Ptr(rtID)},
				},
			},
			wantErr:         true,
			wantErrContains: "no network security group attached",
		},
		{
			name: "malformed NSG ID fails",
			subnet: &armnetwork.Subnet{
				Properties: &armnetwork.SubnetPropertiesFormat{
					AddressPrefix:        to.Ptr(subnetCIDR),
					NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: to.Ptr("not-an-arm-id")},
				},
			},
			wantErr:         true,
			wantErrContains: "failed to parse discovered NSG resource ID",
		},
		{
			name: "no RT with overlay disabled fails",
			subnet: &armnetwork.Subnet{
				Properties: &armnetwork.SubnetPropertiesFormat{
					AddressPrefix:        to.Ptr(subnetCIDR),
					NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: to.Ptr(nsgID)},
				},
			},
			overlayEnabled:  false,
			wantErr:         true,
			wantErrContains: "no route table attached",
		},
		{
			name: "malformed RT ID fails",
			subnet: &armnetwork.Subnet{
				Properties: &armnetwork.SubnetPropertiesFormat{
					AddressPrefix:        to.Ptr(subnetCIDR),
					NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: to.Ptr(nsgID)},
					RouteTable:           &armnetwork.RouteTable{ID: to.Ptr("not-an-arm-id")},
				},
			},
			wantErr:         true,
			wantErrContains: "failed to parse discovered route-table resource ID",
		},
		{
			name:            "Azure lookup error surfaces to caller",
			subnetErr:       errors.New("ARM returned 500"),
			wantErr:         true,
			wantErrContains: "failed to look up BYO subnet",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSubnet := azureclientmock.NewMockSubnet(ctrl)
			mockSubnet.EXPECT().
				Get(gomock.Any(), vnetRG, vnetName, subnetName, gomock.Nil()).
				Return(tc.subnet, tc.subnetErr).
				Times(1)

			mockFactory := azureclientmock.NewMockFactory(ctrl)
			mockFactory.EXPECT().Subnet().Return(mockSubnet, nil).Times(1)

			fctx := newBYOFlowContext(t, mockFactory, tc.overlayEnabled)

			err := fctx.EnsureUserSubnet(context.Background())
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErrContains)
				}
				if tc.wantErrContains != "" && !strings.Contains(err.Error(), tc.wantErrContains) {
					t.Fatalf("expected error containing %q, got %q", tc.wantErrContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			byo := fctx.whiteboard.GetChild(ChildKeyBYO)
			if got := ptr.Deref(byo.Get(KeyBYONSGName), ""); got != tc.wantNSGName {
				t.Errorf("NSG name: got %q, want %q", got, tc.wantNSGName)
			}
			if got := ptr.Deref(byo.Get(KeyBYONSGResourceGroup), ""); got != tc.wantNSGResourceGrp {
				t.Errorf("NSG resource group: got %q, want %q", got, tc.wantNSGResourceGrp)
			}
			if tc.wantNoRT {
				if byo.Get(KeyBYORTName) != nil {
					t.Errorf("RT name: expected unset, got %q", *byo.Get(KeyBYORTName))
				}
			} else {
				if got := ptr.Deref(byo.Get(KeyBYORTName), ""); got != tc.wantRTName {
					t.Errorf("RT name: got %q, want %q", got, tc.wantRTName)
				}
				if got := ptr.Deref(byo.Get(KeyBYORTResourceGroup), ""); got != tc.wantRTResourceGroup {
					t.Errorf("RT resource group: got %q, want %q", got, tc.wantRTResourceGroup)
				}
			}
			if tc.wantCIDR != "" {
				if got := ptr.Deref(byo.Get(KeyBYOSubnetCIDR), ""); got != tc.wantCIDR {
					t.Errorf("subnet CIDR: got %q, want %q", got, tc.wantCIDR)
				}
			}
		})
	}
}

// TestEnsureUserSubnet_ManagedModeIsNoOp verifies that when the config is a managed-mode config
// (no VNet.ResourceGroup, no Networks.Subnet), EnsureUserSubnet returns immediately without
// touching the factory. The mock factory records no expected calls; any call would fail the test.
func TestEnsureUserSubnet_ManagedModeIsNoOp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := azureclientmock.NewMockFactory(ctrl) // no EXPECTs -> any call fails

	fctx := &FlowContext{
		log:        testr.New(t),
		factory:    mockFactory,
		whiteboard: shared.NewWhiteboard(),
		cfg: &azure.InfrastructureConfig{
			Networks: azure.NetworkConfig{
				VNet:    azure.VNet{CIDR: ptr.To("10.250.0.0/16")},
				Workers: ptr.To("10.250.0.0/19"),
			},
		},
		cluster: &extensionscontroller.Cluster{
			Shoot: &gardencorev1beta1.Shoot{},
		},
	}
	if err := fctx.EnsureUserSubnet(context.Background()); err != nil {
		t.Fatalf("expected no-op in managed mode, got error: %v", err)
	}
}

// newBYOFlowContext builds a FlowContext suitable for exercising EnsureUserSubnet with the given
// factory and shoot-side overlay setting.
func newBYOFlowContext(t *testing.T, factory *azureclientmock.MockFactory, overlayEnabled bool) *FlowContext {
	t.Helper()

	const (
		vnetRG     = "byo-network-rg"
		vnetName   = "byo-vnet"
		subnetName = "byo-workers"
	)

	cfg := &azure.InfrastructureConfig{
		Networks: azure.NetworkConfig{
			VNet: azure.VNet{
				Name:          ptr.To(vnetName),
				ResourceGroup: ptr.To(vnetRG),
			},
			Subnet: &azure.SubnetReference{Name: subnetName},
		},
	}

	infra := &extensionsv1alpha1.Infrastructure{
		Spec: extensionsv1alpha1.InfrastructureSpec{
			Region: "westeurope",
		},
	}
	shoot := &gardencorev1beta1.Shoot{
		Spec: gardencorev1beta1.ShootSpec{
			Networking: &gardencorev1beta1.Networking{
				Nodes:    to.Ptr("10.250.0.0/16"),
				Pods:     to.Ptr("100.96.0.0/11"),
				Services: to.Ptr("100.64.0.0/13"),
			},
		},
	}
	// overlay=true / overlay=false: encode as a JSON blob understood by helper.IsOverlayEnabled.
	// Absence of the field means "overlay enabled" (Gardener default); the missing-RT test cases
	// therefore explicitly disable overlay to force the strict path.
	if overlayEnabled {
		shoot.Spec.Networking.ProviderConfig = &runtime.RawExtension{
			Raw: []byte(`{"overlay":{"enabled":true}}`),
		}
	} else {
		shoot.Spec.Networking.ProviderConfig = &runtime.RawExtension{
			Raw: []byte(`{"overlay":{"enabled":false}}`),
		}
	}
	cluster := &extensionscontroller.Cluster{Shoot: shoot}

	adapter, err := NewInfrastructureAdapter(infra, cfg, nil, nil, cluster)
	if err != nil {
		t.Fatalf("NewInfrastructureAdapter: %v", err)
	}

	return &FlowContext{
		log:        testr.New(t),
		factory:    factory,
		infra:      infra,
		cfg:        cfg,
		cluster:    cluster,
		adapter:    adapter,
		whiteboard: shared.NewWhiteboard(),
	}
}
