// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"reflect"
	"sort"
	"testing"
	"unsafe"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/go-logr/logr/testr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/controller/infrastructure/infraflow/shared"
)

// The graph-structure tests below assert the exact set of tasks registered by buildReconcileGraph
// in each mode (managed / BYO). They are internal (package infraflow) tests because the sub-
// builders and graphTaskNames() reflect over an unexported field on upstream flow.Graph, and both
// pieces are intentionally not exported to the public API of this package.

// graphTaskNames reads the (unexported) `tasks` field on flow.Graph and returns the sorted set of
// registered task names. This uses reflection with unsafe to bypass the field's unexported status;
// the alternative — public accessors on upstream flow.Graph — is not available in the current
// gardener/gardener version this repo pins. If a future upstream change renames or exports the
// field, this helper will need adjustment; the trade-off is worth it for the safety net.
func graphTaskNames(g *flow.Graph) []string {
	v := reflect.ValueOf(g).Elem().FieldByName("tasks")
	// use unsafe to read unexported map field
	v = reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
	names := make([]string, 0, v.Len())
	for _, key := range v.MapKeys() {
		names = append(names, key.String())
	}
	sort.Strings(names)
	return names
}

// newTestFlowContext builds a minimal FlowContext suitable for exercising the graph-build helpers.
// It does not populate an Azure client factory or a shoot cluster because the sub-builders never
// call into Azure or the shoot at graph-construction time — the payload functions are only invoked
// when the flow runs, and we do not run the flow in these tests.
func newTestFlowContext(t *testing.T, cfg *azure.InfrastructureConfig) *FlowContext {
	t.Helper()
	fctx := &FlowContext{
		log:        testr.New(t),
		cfg:        cfg,
		whiteboard: shared.NewWhiteboard(),
		infra:      &extensionsv1alpha1.Infrastructure{},
		cluster: &extensionscontroller.Cluster{
			Shoot: &gardencorev1beta1.Shoot{},
		},
	}
	fctx.BasicFlowContext = shared.NewBasicFlowContext().WithLogger(fctx.log)
	return fctx
}

var (
	managedConfig = &azure.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{Kind: "InfrastructureConfig"},
		Networks: azure.NetworkConfig{
			VNet:    azure.VNet{CIDR: ptr.To("10.250.0.0/16")},
			Workers: ptr.To("10.250.0.0/19"),
		},
	}
	byoConfig = &azure.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{Kind: "InfrastructureConfig"},
		Networks: azure.NetworkConfig{
			VNet: azure.VNet{
				Name:          ptr.To("byo-vnet"),
				ResourceGroup: ptr.To("byo-rg"),
			},
			Subnet: &azure.SubnetReference{Name: "byo-workers"},
		},
	}
)

// Managed-mode reconcile task expectations.
const (
	taskEnsureResourceGroup  = "ensure resource group"
	taskEnsureVirtualNetwork = "ensure vnet"
	taskEnsureManagedIdenty  = "ensure managed identity"
	taskEnsureRouteTable     = "ensure route table"
	taskEnsureSecurityGroup  = "ensure security group"
	taskEnsurePublicIPs      = "ensure public IPs"
	taskEnsureNATs           = "ensure nats"
	taskEnsureSubnets        = "ensure subnets"
	taskEnsureUserSubnet     = "ensure BYO subnet (read-only discovery)"
	taskEnsureBYOTags        = "ensure BYO resource tags"
)

func TestBuildReconcileGraph_Managed_RegistersOnlyManagedTasks(t *testing.T) {
	fctx := newTestFlowContext(t, managedConfig)
	g := fctx.buildReconcileGraph()

	got := graphTaskNames(g)
	want := []string{
		taskEnsureResourceGroup,
		taskEnsureVirtualNetwork,
		taskEnsureManagedIdenty,
		taskEnsureRouteTable,
		taskEnsureSecurityGroup,
		taskEnsurePublicIPs,
		taskEnsureNATs,
		taskEnsureSubnets,
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("managed-mode graph task set differs\n got: %v\nwant: %v", got, want)
	}

	// Sanity: BYO-only tasks must not appear in a managed-mode graph.
	for _, n := range got {
		if n == taskEnsureUserSubnet || n == taskEnsureBYOTags {
			t.Errorf("managed-mode graph must not register %q", n)
		}
	}
}

func TestBuildReconcileGraph_BYO_RegistersOnlyBYOTasks(t *testing.T) {
	fctx := newTestFlowContext(t, byoConfig)
	g := fctx.buildReconcileGraph()

	got := graphTaskNames(g)
	want := []string{
		taskEnsureResourceGroup,
		taskEnsureVirtualNetwork,
		taskEnsureManagedIdenty,
		taskEnsureSecurityGroup,
		taskEnsureUserSubnet,
		taskEnsureBYOTags,
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BYO graph task set differs\n got: %v\nwant: %v", got, want)
	}

	// Sanity: managed-only tasks must not appear in a BYO graph.
	// (The security group is shared between managed and BYO — Gardener creates it in the
	// shoot's cluster RG in both modes; see docs/proposals/flexible-network-configuration-proposal.md#nsg-mutation-contract.)
	managedOnly := []string{
		taskEnsureRouteTable,
		taskEnsurePublicIPs,
		taskEnsureNATs,
		taskEnsureSubnets,
	}
	for _, n := range got {
		for _, forbidden := range managedOnly {
			if n == forbidden {
				t.Errorf("BYO graph must not register managed-only task %q", forbidden)
			}
		}
	}
}

// TestBuildReconcileGraph_DependenciesCompile ensures the graph builds without upstream
// flow.Graph.Add panicking on a missing dependency; flow.Graph.Add is the choke point that
// catches dependency wiring errors at construction time. Regressions where a sub-builder wires a
// task against a dep that has not been added yet will surface as a test panic here.
func TestBuildReconcileGraph_DependenciesCompile(t *testing.T) {
	for name, cfg := range map[string]*azure.InfrastructureConfig{
		"managed": managedConfig,
		"byo":     byoConfig,
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("buildReconcileGraph(%s) panicked (likely a missing dependency): %v", name, r)
				}
			}()
			fctx := newTestFlowContext(t, cfg)
			_ = fctx.buildReconcileGraph()
		})
	}
}
