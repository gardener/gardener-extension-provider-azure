// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v10"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/test/framework"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	azurev1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	. "github.com/gardener/gardener-extension-provider-azure/test/utils"
)

// BYONetworkFixture references the resources that make up a pre-provisioned BYO worker network.
// The naming convention keeps a single "fixture group name" from which the individual resource
// names are derived, so that a single teardownResourceGroup call cleans them all up.
type BYONetworkFixture struct {
	// FixtureRG is the resource group that holds the VNet, subnet, and (unless the test overrides)
	// the NSG and RT. Deleting this RG cleans up every fixture resource in one operation.
	FixtureRG string
	// NSGResourceGroup / RTResourceGroup override the RG that holds the NSG / RT for
	// foreign-RG scenarios (B2, B3). Empty means "use FixtureRG".
	NSGResourceGroup string
	RTResourceGroup  string

	VNetName        string
	SubnetName      string
	SubnetCIDR      string
	NSGName         string
	RTName          string // Empty means "no route table attached" (requires overlay CNI on the shoot).
	AddDefaultRoute bool   // if true, adds a 0.0.0.0/0 route to a stub virtual appliance IP.

	// OverlayEnabled, if true, sets `overlay.enabled=true` in the shoot's networking provider
	// config. Overlay CNIs (Cilium/Calico with VXLAN or Geneve) encapsulate pod-to-pod traffic at
	// the node level, so the CCM's route controller can be disabled and the BYO subnet is not
	// required to have a route table attached.
	OverlayEnabled bool
}

var _ = Describe("Infrastructure tests - BYO subnet (user-managed egress)", func() {
	AfterEach(func() {
		framework.RunCleanupActions()
	})

	// B1: everything co-located in a single user-owned RG.
	It("B1: should reconcile a BYO shoot with VNet + subnet + NSG + RT co-located in one RG", func() {
		namespace, err := generateName()
		Expect(err).ToNot(HaveOccurred())

		fx := &BYONetworkFixture{
			FixtureRG:  namespace + "-byo",
			VNetName:   "byo-vnet",
			SubnetName: "byo-workers",
			SubnetCIDR: "10.250.0.0/24",
			NSGName:    "byo-workers-nsg",
			RTName:     "byo-workers-rt",
		}
		byoRunTest(ctx, log, c, clientSet, namespace, fx, nil)
	})

	// B2: NSG in a foreign RG.
	It("B2: should reconcile a BYO shoot with NSG in a separate resource group", func() {
		namespace, err := generateName()
		Expect(err).ToNot(HaveOccurred())

		fx := &BYONetworkFixture{
			FixtureRG:        namespace + "-byo",
			NSGResourceGroup: namespace + "-security",
			VNetName:         "byo-vnet",
			SubnetName:       "byo-workers",
			SubnetCIDR:       "10.250.0.0/24",
			NSGName:          "team-owned-nsg",
			RTName:           "byo-workers-rt",
		}
		byoRunTest(ctx, log, c, clientSet, namespace, fx, nil)
	})

	// B3: both NSG and RT in a third RG.
	It("B3: should reconcile a BYO shoot with NSG and RT in a third resource group", func() {
		namespace, err := generateName()
		Expect(err).ToNot(HaveOccurred())

		fx := &BYONetworkFixture{
			FixtureRG:        namespace + "-byo",
			NSGResourceGroup: namespace + "-netops",
			RTResourceGroup:  namespace + "-netops",
			VNetName:         "byo-vnet",
			SubnetName:       "byo-workers",
			SubnetCIDR:       "10.250.0.0/24",
			NSGName:          "network-team-nsg",
			RTName:           "network-team-rt",
		}
		byoRunTest(ctx, log, c, clientSet, namespace, fx, nil)
	})

	// B4: route table with a 0.0.0.0/0 route to a virtual appliance (firewall).
	It("B4: should reconcile a BYO shoot with a firewall-egress route table", func() {
		namespace, err := generateName()
		Expect(err).ToNot(HaveOccurred())

		fx := &BYONetworkFixture{
			FixtureRG:       namespace + "-byo",
			VNetName:        "byo-vnet",
			SubnetName:      "byo-workers",
			SubnetCIDR:      "10.250.0.0/24",
			NSGName:         "byo-workers-nsg",
			RTName:          "byo-workers-rt",
			AddDefaultRoute: true,
		}
		byoRunTest(ctx, log, c, clientSet, namespace, fx, func(status *azurev1alpha1.InfrastructureStatus) {
			By("verifying the user's 0.0.0.0/0 route is still present after reconcile")
			rtRG := fx.RTResourceGroup
			if rtRG == "" {
				rtRG = fx.FixtureRG
			}
			rt, err := clientSet.routeTable.Get(ctx, rtRG, fx.RTName, nil)
			Expect(err).ToNot(HaveOccurred())
			foundDefault := false
			for _, r := range rt.Properties.Routes {
				if r.Properties != nil && r.Properties.AddressPrefix != nil && *r.Properties.AddressPrefix == "0.0.0.0/0" {
					foundDefault = true
					Expect(r.Properties.NextHopType).To(PointTo(Equal(armnetwork.RouteNextHopTypeVirtualAppliance)))
					break
				}
			}
			Expect(foundDefault).To(BeTrue(), "user's default route to the firewall must not be clobbered by the reconciler")
		})
	})

	// B5: empty route table (no default route -> "no egress" pattern).
	It("B5: should reconcile a BYO shoot with an empty route table (no default route)", func() {
		namespace, err := generateName()
		Expect(err).ToNot(HaveOccurred())

		fx := &BYONetworkFixture{
			FixtureRG:  namespace + "-byo",
			VNetName:   "byo-vnet",
			SubnetName: "byo-workers",
			SubnetCIDR: "10.250.0.0/24",
			NSGName:    "byo-workers-nsg",
			RTName:     "byo-workers-rt",
		}
		byoRunTest(ctx, log, c, clientSet, namespace, fx, nil)
	})

	// Overlay-CNI variant of B5: no route table at all, overlay enabled on the shoot networking.
	It("overlay-CNI: should reconcile a BYO shoot with no route table when the shoot's networking has overlay enabled", func() {
		namespace, err := generateName()
		Expect(err).ToNot(HaveOccurred())

		fx := &BYONetworkFixture{
			FixtureRG:      namespace + "-byo",
			VNetName:       "byo-vnet",
			SubnetName:     "byo-workers",
			SubnetCIDR:     "10.250.0.0/24",
			NSGName:        "byo-workers-nsg",
			OverlayEnabled: true,
		}
		byoRunTest(ctx, log, c, clientSet, namespace, fx, func(status *azurev1alpha1.InfrastructureStatus) {
			By("verifying no RouteTables entry is emitted when no RT is attached")
			Expect(status.RouteTables).To(BeEmpty())
		})
	})
})

// nsgRG returns the RG hosting the NSG, defaulting to the fixture RG.
func (fx *BYONetworkFixture) nsgRG() string {
	if fx.NSGResourceGroup != "" {
		return fx.NSGResourceGroup
	}
	return fx.FixtureRG
}

// rtRG returns the RG hosting the route table, defaulting to the fixture RG.
func (fx *BYONetworkFixture) rtRG() string {
	if fx.RTResourceGroup != "" {
		return fx.RTResourceGroup
	}
	return fx.FixtureRG
}

// byoRunTest is the BYO-mode variant of runTest: it provisions the BYO fixture, creates the
// Infrastructure resource, waits for reconcile, runs the BYO-specific verifier, then deletes the
// infrastructure and verifies that only Gardener-owned resources were removed while the user-owned
// resources survive intact.
func byoRunTest(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	az *azureClientSet,
	namespaceName string,
	fx *BYONetworkFixture,
	extraVerify func(status *azurev1alpha1.InfrastructureStatus),
) {
	log.Info("BYO integration test running", "namespace", namespaceName, "fixtureRG", fx.FixtureRG)

	By("provisioning BYO network fixture (RG + VNet + NSG + RT + subnet)")
	Expect(provisionBYOFixture(ctx, log, az, fx, *region)).To(Succeed())

	// The user-owned fixture RGs must survive the shoot's deletion and are torn down here at the
	// end of the test. Order: fixture RGs first (which may host the NSG/RT), then any foreign-RG
	// overrides. Deletion is idempotent (ignore NotFound).
	framework.AddCleanupAction(func() {
		By("teardown fixture RGs")
		for _, rg := range fx.uniqueResourceGroups() {
			if err := teardownResourceGroup(ctx, az, rg); err != nil {
				log.Info("teardownResourceGroup returned error (may be already deleted)", "rg", rg, "error", err.Error())
			}
		}
	})

	var (
		namespace *corev1.Namespace
		cluster   *extensionsv1alpha1.Cluster
		infra     *extensionsv1alpha1.Infrastructure
	)

	// Cleanup for Gardener-owned resources.
	defer func() {
		if infra != nil {
			By("delete infrastructure")
			Expect(client.IgnoreNotFound(c.Delete(ctx, infra))).To(Succeed())

			By("wait until infrastructure is deleted")
			Expect(extensions.WaitUntilExtensionObjectDeleted(
				ctx,
				c,
				log,
				infra,
				extensionsv1alpha1.InfrastructureResource,
				10*time.Second,
				16*time.Minute,
			)).To(Succeed())

			By("verify BYO resources still exist after shoot deletion (F1)")
			verifyBYODeletion(ctx, az, fx, namespaceName)
		}
		if namespace != nil {
			Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())
		}
		if cluster != nil {
			Expect(client.IgnoreNotFound(c.Delete(ctx, cluster))).To(Succeed())
		}
	}()

	By("create namespace for test execution")
	namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
	Expect(c.Create(ctx, namespace)).To(Succeed())

	By("create cluster CR")
	var err error
	cluster, err = newCluster(namespaceName, *region, false)
	Expect(err).ToNot(HaveOccurred())
	if fx.OverlayEnabled {
		Expect(injectOverlayNetworking(cluster)).To(Succeed())
	}
	Expect(c.Create(ctx, cluster)).To(Succeed())

	By("deploy cloudprovider secret into namespace")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.SecretNameCloudProvider,
			Namespace: namespaceName,
		},
		Data: map[string][]byte{
			azure.SubscriptionIDKey: []byte(*subscriptionId),
			azure.TenantIDKey:       []byte(*tenantId),
			azure.ClientIDKey:       []byte(*clientId),
			azure.ClientSecretKey:   []byte(*clientSecret),
		},
	}
	Expect(c.Create(ctx, secret)).To(Succeed())

	By("create BYO infrastructure resource")
	providerConfig := newBYOInfrastructureConfig(fx)
	infra, err = newInfrastructure(namespaceName, providerConfig)
	Expect(err).ToNot(HaveOccurred())
	Expect(c.Create(ctx, infra)).To(Succeed())

	By("wait until infrastructure is created")
	Expect(extensions.WaitUntilExtensionObjectReady(
		ctx, c, log, infra,
		extensionsv1alpha1.InfrastructureResource,
		10*time.Second, 30*time.Second, 16*time.Minute, nil,
	)).To(Succeed())

	By("decode infrastructure status")
	Expect(c.Get(ctx, client.ObjectKey{Namespace: infra.Namespace, Name: infra.Name}, infra)).To(Succeed())
	status := &azurev1alpha1.InfrastructureStatus{}
	_, _, decodeErr := decoder.Decode(infra.Status.ProviderStatus.Raw, nil, status)
	Expect(decodeErr).ToNot(HaveOccurred())

	By("verify BYO infrastructure creation")
	verifyBYOCreation(ctx, az, infra, fx, status)

	if extraVerify != nil {
		extraVerify(status)
	}
}

// uniqueResourceGroups returns each distinct fixture RG that must be torn down at test cleanup.
func (fx *BYONetworkFixture) uniqueResourceGroups() []string {
	seen := map[string]bool{}
	var out []string
	for _, rg := range []string{fx.FixtureRG, fx.NSGResourceGroup, fx.RTResourceGroup} {
		if rg == "" || seen[rg] {
			continue
		}
		seen[rg] = true
		out = append(out, rg)
	}
	return out
}

// newBYOInfrastructureConfig builds a minimal BYO-mode InfrastructureConfig referencing the fixture.
func newBYOInfrastructureConfig(fx *BYONetworkFixture) *azurev1alpha1.InfrastructureConfig {
	return &azurev1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureConfig",
		},
		Zoned: true,
		Networks: azurev1alpha1.NetworkConfig{
			VNet: azurev1alpha1.VNet{
				Name:          ptr.To(fx.VNetName),
				ResourceGroup: ptr.To(fx.FixtureRG),
			},
			Subnet: &azurev1alpha1.SubnetReference{Name: fx.SubnetName},
		},
	}
}

// provisionBYOFixture creates every resource named by the fixture in the right RGs, then attaches
// the NSG and RT to the subnet. It uses the existing helpers wherever possible.
func provisionBYOFixture(ctx context.Context, log logr.Logger, az *azureClientSet, fx *BYONetworkFixture, location string) error {
	// 1. Fixture RGs.
	for _, rg := range fx.uniqueResourceGroups() {
		if err := prepareNewResourceGroup(ctx, log, az, rg, location); err != nil {
			return fmt.Errorf("create fixture RG %q: %w", rg, err)
		}
	}

	// 2. VNet in FixtureRG. The VNet must be sized to contain the subnet CIDR; use a /16 wrapper.
	if err := prepareNewVNet(ctx, log, az, fx.FixtureRG, fx.VNetName, location, "10.250.0.0/16"); err != nil {
		return fmt.Errorf("create fixture VNet: %w", err)
	}

	// 3. NSG (in its own RG, which may equal FixtureRG).
	if err := prepareNewNetworkSecurityGroup(ctx, log, az, fx.nsgRG(), fx.NSGName, location); err != nil {
		return fmt.Errorf("create fixture NSG: %w", err)
	}

	// 4. RT (in its own RG, may equal FixtureRG). Optionally add a default route to a stub NVA.
	// Skipped entirely when the fixture opts out of a route table (overlay-CNI scenario).
	var rtID string
	if fx.RTName != "" {
		if err := prepareNewRouteTable(ctx, log, az, fx.rtRG(), fx.RTName, location, fx.AddDefaultRoute); err != nil {
			return fmt.Errorf("create fixture route table: %w", err)
		}
		id, err := routeTableID(*subscriptionId, fx.rtRG(), fx.RTName)
		if err != nil {
			return err
		}
		rtID = id
	}

	// 5. Subnet attached to the NSG + (optionally) RT.
	nsgID, err := securityGroupID(*subscriptionId, fx.nsgRG(), fx.NSGName)
	if err != nil {
		return err
	}
	if err := prepareSubnetWithAttachments(ctx, log, az, fx.FixtureRG, fx.VNetName, fx.SubnetName, fx.SubnetCIDR, nsgID, rtID); err != nil {
		return fmt.Errorf("create fixture subnet: %w", err)
	}
	return nil
}

func prepareNewNetworkSecurityGroup(ctx context.Context, log logr.Logger, az *azureClientSet, rg, name, location string) error {
	log.Info("generating new NSG", "rg", rg, "name", name)
	poller, err := az.securityGroups.BeginCreateOrUpdate(ctx, rg, name, armnetwork.SecurityGroup{
		Location: ptr.To(location),
	}, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

func prepareNewRouteTable(ctx context.Context, log logr.Logger, az *azureClientSet, rg, name, location string, addDefaultRoute bool) error {
	log.Info("generating new route table", "rg", rg, "name", name, "addDefaultRoute", addDefaultRoute)
	rt := armnetwork.RouteTable{
		Location:   ptr.To(location),
		Properties: &armnetwork.RouteTablePropertiesFormat{},
	}
	if addDefaultRoute {
		rt.Properties.Routes = []*armnetwork.Route{
			{
				Name: ptr.To("default-egress"),
				Properties: &armnetwork.RoutePropertiesFormat{
					AddressPrefix:    ptr.To("0.0.0.0/0"),
					NextHopType:      ptr.To(armnetwork.RouteNextHopTypeVirtualAppliance),
					NextHopIPAddress: ptr.To("10.99.0.4"), // stub firewall / NVA IP; the test never sends traffic through it.
				},
			},
		}
	}
	poller, err := az.routeTable.BeginCreateOrUpdate(ctx, rg, name, rt, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

func prepareSubnetWithAttachments(ctx context.Context, log logr.Logger, az *azureClientSet, vnetRG, vnetName, subnetName, cidr, nsgID, rtID string) error {
	log.Info("generating BYO subnet", "vnet", vnetName, "subnet", subnetName, "attachRT", rtID != "")
	props := &armnetwork.SubnetPropertiesFormat{
		AddressPrefix:        ptr.To(cidr),
		NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: ptr.To(nsgID)},
	}
	if rtID != "" {
		props.RouteTable = &armnetwork.RouteTable{ID: ptr.To(rtID)}
	}
	poller, err := az.subnets.BeginCreateOrUpdate(ctx, vnetRG, vnetName, subnetName, armnetwork.Subnet{
		Properties: props,
	}, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

func securityGroupID(subscription, rg, name string) (string, error) {
	if subscription == "" || rg == "" || name == "" {
		return "", fmt.Errorf("invalid inputs for NSG ID: subscription=%q rg=%q name=%q", subscription, rg, name)
	}
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/networkSecurityGroups/%s", subscription, rg, name), nil
}

func routeTableID(subscription, rg, name string) (string, error) {
	if subscription == "" || rg == "" || name == "" {
		return "", fmt.Errorf("invalid inputs for RT ID: subscription=%q rg=%q name=%q", subscription, rg, name)
	}
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/routeTables/%s", subscription, rg, name), nil
}

// verifyBYOCreation asserts every invariant that must hold for a successfully reconciled BYO shoot:
// status shape (E1, E2), no cluster-RG Gardener-owned network resources (E4, E5), and the
// observability tags (G1).
func verifyBYOCreation(
	ctx context.Context,
	az *azureClientSet,
	infra *extensionsv1alpha1.Infrastructure,
	fx *BYONetworkFixture,
	status *azurev1alpha1.InfrastructureStatus,
) {
	By("verifying status shape (E1, E2)")
	Expect(status.Networks.OutboundAccessType).To(Equal(azurev1alpha1.OutboundAccessTypeUserManaged))
	Expect(status.Networks.Layout).To(Equal(azurev1alpha1.NetworkLayoutSingleSubnet))
	Expect(status.Networks.VNet.Name).To(Equal(fx.VNetName))
	Expect(status.Networks.VNet.ResourceGroup).To(PointTo(Equal(fx.FixtureRG)))
	Expect(status.Networks.Subnets).To(HaveLen(1))
	Expect(status.Networks.Subnets[0].Name).To(Equal(fx.SubnetName))
	Expect(status.Networks.Subnets[0].Purpose).To(Equal(azurev1alpha1.PurposeNodes))

	Expect(status.SecurityGroups).To(HaveLen(1))
	Expect(status.SecurityGroups[0].Name).To(Equal(fx.NSGName))
	// ResourceGroup is populated only when the NSG lives in a foreign RG.
	if fx.NSGResourceGroup != "" {
		Expect(status.SecurityGroups[0].ResourceGroup).To(PointTo(Equal(fx.NSGResourceGroup)))
	} else {
		Expect(status.SecurityGroups[0].ResourceGroup).To(PointTo(Equal(fx.FixtureRG)))
	}

	Expect(status.RouteTables).To(HaveLen(func() int {
		if fx.RTName == "" {
			return 0
		}
		return 1
	}()))
	if fx.RTName != "" {
		Expect(status.RouteTables[0].Name).To(Equal(fx.RTName))
		if fx.RTResourceGroup != "" {
			Expect(status.RouteTables[0].ResourceGroup).To(PointTo(Equal(fx.RTResourceGroup)))
		} else {
			Expect(status.RouteTables[0].ResourceGroup).To(PointTo(Equal(fx.FixtureRG)))
		}
	}

	By("verifying cluster resource group has no worker NSG / RT / NAT (E4, E5)")
	// The cluster RG matches the shoot's technical name (== infra.Namespace).
	clusterRG := infra.Namespace
	nsgResp, nsgErr := az.securityGroups.Get(ctx, clusterRG, clusterRG+"-workers", nil)
	Expect(nsgErr).To(HaveOccurred(), "the cluster RG must NOT contain a Gardener-owned worker NSG in BYO mode; got: %v", nsgResp)
	Expect(nsgErr).To(BeNotFoundError())

	rtResp, rtErr := az.routeTable.Get(ctx, clusterRG, "worker_route_table", nil)
	Expect(rtErr).To(HaveOccurred(), "the cluster RG must NOT contain a Gardener-owned route table in BYO mode; got: %v", rtResp)
	Expect(rtErr).To(BeNotFoundError())

	// The cluster RG list of NAT gateways must be empty.
	natPager := az.nat.NewListPager(clusterRG, nil)
	for natPager.More() {
		page, err := natPager.NextPage(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(page.Value).To(BeEmpty(), "the cluster RG must NOT contain a Gardener-owned NAT gateway in BYO mode")
	}

	By("verifying observability tag was applied to the BYO VNet, NSG, and RT (G1)")
	tagKey := "kubernetes.io/cluster/" + clusterRG // technicalName == cluster RG name
	expectResourceHasTag(ctx, az, "vnet", func(ctx context.Context) (map[string]*string, error) {
		resp, err := az.vnet.Get(ctx, fx.FixtureRG, fx.VNetName, nil)
		if err != nil {
			return nil, err
		}
		return resp.Tags, nil
	}, tagKey, "shared")

	expectResourceHasTag(ctx, az, "nsg", func(ctx context.Context) (map[string]*string, error) {
		resp, err := az.securityGroups.Get(ctx, fx.nsgRG(), fx.NSGName, nil)
		if err != nil {
			return nil, err
		}
		return resp.Tags, nil
	}, tagKey, "shared")

	if fx.RTName != "" {
		expectResourceHasTag(ctx, az, "rt", func(ctx context.Context) (map[string]*string, error) {
			resp, err := az.routeTable.Get(ctx, fx.rtRG(), fx.RTName, nil)
			if err != nil {
				return nil, err
			}
			return resp.Tags, nil
		}, tagKey, "shared")
	}
}

// verifyBYODeletion asserts the invariants after the shoot has been deleted: cluster RG is gone
// (managed by the framework), but every BYO fixture resource is still present, and the
// observability tag added by this shoot is gone.
func verifyBYODeletion(ctx context.Context, az *azureClientSet, fx *BYONetworkFixture, technicalName string) {
	By("cluster RG must be deleted")
	_, err := az.groups.Get(ctx, technicalName, nil)
	Expect(err).To(HaveOccurred())
	Expect(err).To(BeNotFoundError())

	By("BYO VNet, subnet, NSG, and RT must survive")
	_, err = az.vnet.Get(ctx, fx.FixtureRG, fx.VNetName, nil)
	Expect(err).ToNot(HaveOccurred())
	_, err = az.subnets.Get(ctx, fx.FixtureRG, fx.VNetName, fx.SubnetName, nil)
	Expect(err).ToNot(HaveOccurred())
	_, err = az.securityGroups.Get(ctx, fx.nsgRG(), fx.NSGName, nil)
	Expect(err).ToNot(HaveOccurred())
	if fx.RTName != "" {
		_, err = az.routeTable.Get(ctx, fx.rtRG(), fx.RTName, nil)
		Expect(err).ToNot(HaveOccurred())
	}

	By("observability tag was removed from BYO resources (F1)")
	tagKey := "kubernetes.io/cluster/" + technicalName
	vnetResp, err := az.vnet.Get(ctx, fx.FixtureRG, fx.VNetName, nil)
	Expect(err).ToNot(HaveOccurred())
	Expect(vnetResp.Tags).ToNot(HaveKey(tagKey))

	nsgResp, err := az.securityGroups.Get(ctx, fx.nsgRG(), fx.NSGName, nil)
	Expect(err).ToNot(HaveOccurred())
	Expect(nsgResp.Tags).ToNot(HaveKey(tagKey))

	if fx.RTName != "" {
		rtResp, err := az.routeTable.Get(ctx, fx.rtRG(), fx.RTName, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(rtResp.Tags).ToNot(HaveKey(tagKey))
	}
}

// expectResourceHasTag pulls the given resource's tags via `getTags` and asserts the key/value
// pair is present. Small closure-based indirection avoids duplicating the three different get
// signatures (vnet, nsg, rt).
func expectResourceHasTag(ctx context.Context, _ *azureClientSet, label string, getTags func(ctx context.Context) (map[string]*string, error), key, value string) {
	tags, err := getTags(ctx)
	Expect(err).ToNot(HaveOccurred(), "failed to get tags on %s", label)
	Expect(tags).To(HaveKey(key), "%s must carry the observability tag %q", label, key)
	Expect(tags[key]).To(PointTo(Equal(value)), "observability tag on %s has wrong value", label)
}

// injectOverlayNetworking stamps `spec.networking.providerConfig.overlay.enabled=true` on the
// shoot embedded in the given Cluster resource. Used by overlay-CNI BYO scenarios so the
// infrastructure reconciler's `helper.IsOverlayEnabled` check returns true regardless of the
// package's default.
func injectOverlayNetworking(cluster *extensionsv1alpha1.Cluster) error {
	shoot := &gardencorev1beta1.Shoot{}
	if err := json.Unmarshal(cluster.Spec.Shoot.Raw, shoot); err != nil {
		return fmt.Errorf("decode shoot from cluster: %w", err)
	}
	if shoot.Spec.Networking == nil {
		shoot.Spec.Networking = &gardencorev1beta1.Networking{}
	}
	shoot.Spec.Networking.ProviderConfig = &runtime.RawExtension{
		Raw: []byte(`{"overlay":{"enabled":true}}`),
	}
	raw, err := json.Marshal(shoot)
	if err != nil {
		return fmt.Errorf("encode shoot back to raw: %w", err)
	}
	cluster.Spec.Shoot.Raw = raw
	return nil
}
