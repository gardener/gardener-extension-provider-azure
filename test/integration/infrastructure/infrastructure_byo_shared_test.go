// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure_test

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v10"
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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	azurev1alpha1 "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/v1alpha1"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	. "github.com/gardener/gardener-extension-provider-azure/test/utils"
)

// BYOSharedFixture describes a pre-provisioned BYO network layout in which two shoots each own a
// distinct subnet (+ per-shoot NSG) but reuse one or both of an NSG-external resource across their
// subnets: a NAT gateway (B6) and/or a route table (B7). Everything lives inside a single fixture
// resource group so cleanup is a single group-delete.
type BYOSharedFixture struct {
	// FixtureRG holds every user resource — VNet, both subnets, both NSGs, the shared NAT / RT,
	// and the NAT's PIP.
	FixtureRG string
	Location  string

	VNetName string
	VNetCIDR string

	SubnetAName string
	SubnetACIDR string
	NSGAName    string

	SubnetBName string
	SubnetBCIDR string
	NSGBName    string

	// SharedNATName, SharedNATPIPName — pre-provisioned when non-empty; both subnets get this NAT
	// gateway attached. Exercises B6.
	SharedNATName    string
	SharedNATPIPName string

	// SharedRTName — pre-provisioned when non-empty; both subnets get this RT attached. Exercises
	// B7 (must be combined with OverlayEnabled=true so the seed CCM's route controller is off and
	// no shoot writes to the shared RT).
	SharedRTName string

	// OverlayEnabled stamps `overlay.enabled=true` into each shoot's networking provider config.
	// Required for B7; optional for B6 (both CNI modes work but overlay keeps the fixture leaner).
	OverlayEnabled bool
}

var _ = Describe("Infrastructure tests - BYO subnet, resources shared across shoots", func() {
	AfterEach(func() {
		framework.RunCleanupActions()
	})

	// B6: two BYO shoots sharing one user-owned NAT gateway across their distinct subnets.
	It("B6: should reconcile two BYO shoots that share a single NAT gateway across their subnets", func() {
		nsA, err := generateName()
		Expect(err).ToNot(HaveOccurred())
		nsB, err := generateName()
		Expect(err).ToNot(HaveOccurred())

		fx := &BYOSharedFixture{
			FixtureRG:        nsA + "-byo-shared",
			Location:         *region,
			VNetName:         "byo-shared-vnet",
			VNetCIDR:         "10.250.0.0/16",
			SubnetAName:      "byo-shared-a",
			SubnetACIDR:      "10.250.0.0/24",
			NSGAName:         "byo-shared-a-nsg",
			SubnetBName:      "byo-shared-b",
			SubnetBCIDR:      "10.250.1.0/24",
			NSGBName:         "byo-shared-b-nsg",
			SharedNATName:    "byo-shared-nat",
			SharedNATPIPName: "byo-shared-nat-pip",
			OverlayEnabled:   true, // orthogonal to B6, keeps the fixture lean (no per-shoot RT)
		}
		byoRunSharedTest(ctx, log, c, clientSet, nsA, nsB, fx, func(statusA, statusB *azurev1alpha1.InfrastructureStatus) {
			By("both shoots have no route table in status (overlay case)")
			Expect(statusA.RouteTables).To(BeEmpty())
			Expect(statusB.RouteTables).To(BeEmpty())

			By("shared NAT gateway is still attached to both subnets after reconcile")
			natGWID, err := natGatewayID(*subscriptionId, fx.FixtureRG, fx.SharedNATName)
			Expect(err).ToNot(HaveOccurred())
			verifySubnetHasNAT(ctx, clientSet, fx.FixtureRG, fx.VNetName, fx.SubnetAName, natGWID)
			verifySubnetHasNAT(ctx, clientSet, fx.FixtureRG, fx.VNetName, fx.SubnetBName, natGWID)
		})
	})

	// B7: two BYO overlay-Cilium shoots sharing one user-owned route table across their subnets.
	It("B7: should reconcile two BYO overlay shoots that share a single route table across their subnets", func() {
		nsA, err := generateName()
		Expect(err).ToNot(HaveOccurred())
		nsB, err := generateName()
		Expect(err).ToNot(HaveOccurred())

		fx := &BYOSharedFixture{
			FixtureRG:      nsA + "-byo-shared",
			Location:       *region,
			VNetName:       "byo-shared-vnet",
			VNetCIDR:       "10.250.0.0/16",
			SubnetAName:    "byo-shared-a",
			SubnetACIDR:    "10.250.0.0/24",
			NSGAName:       "byo-shared-a-nsg",
			SubnetBName:    "byo-shared-b",
			SubnetBCIDR:    "10.250.1.0/24",
			NSGBName:       "byo-shared-b-nsg",
			SharedRTName:   "byo-shared-rt",
			OverlayEnabled: true, // required for B7 — non-overlay would trigger CCM route-writer conflict
		}
		byoRunSharedTest(ctx, log, c, clientSet, nsA, nsB, fx, func(statusA, statusB *azurev1alpha1.InfrastructureStatus) {
			By("both shoots reference the same shared route table in their status")
			Expect(statusA.RouteTables).To(HaveLen(1))
			Expect(statusA.RouteTables[0].Name).To(Equal(fx.SharedRTName))
			Expect(statusA.RouteTables[0].ResourceGroup).To(PointTo(Equal(fx.FixtureRG)))

			Expect(statusB.RouteTables).To(HaveLen(1))
			Expect(statusB.RouteTables[0].Name).To(Equal(fx.SharedRTName))
			Expect(statusB.RouteTables[0].ResourceGroup).To(PointTo(Equal(fx.FixtureRG)))

			By("shared RT still attached to both subnets after reconcile")
			rtID, err := routeTableID(*subscriptionId, fx.FixtureRG, fx.SharedRTName)
			Expect(err).ToNot(HaveOccurred())
			verifySubnetHasRT(ctx, clientSet, fx.FixtureRG, fx.VNetName, fx.SubnetAName, rtID)
			verifySubnetHasRT(ctx, clientSet, fx.FixtureRG, fx.VNetName, fx.SubnetBName, rtID)
		})
	})
})

// byoRunSharedTest is the two-shoot variant of byoRunTest: it provisions a shared fixture, creates
// two Infrastructure CRs (each in its own namespace), waits for both to reconcile, runs the
// per-scenario verifier, then deletes both and asserts that every shared user-owned resource
// survives.
func byoRunSharedTest(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	az *azureClientSet,
	nsA, nsB string,
	fx *BYOSharedFixture,
	extraVerify func(statusA, statusB *azurev1alpha1.InfrastructureStatus),
) {
	log.Info("BYO shared-fixture integration test running", "namespaceA", nsA, "namespaceB", nsB, "fixtureRG", fx.FixtureRG)

	By("provisioning shared BYO network fixture")
	Expect(provisionBYOSharedFixture(ctx, log, az, fx)).To(Succeed())

	// Fixture RG must survive both shoot deletions and is torn down here at test cleanup.
	framework.AddCleanupAction(func() {
		By("teardown fixture RG")
		if err := teardownResourceGroup(ctx, az, fx.FixtureRG); err != nil {
			log.Info("teardownResourceGroup returned error (may be already deleted)", "rg", fx.FixtureRG, "error", err.Error())
		}
	})

	var (
		nsAObj, nsBObj     *corev1.Namespace
		clusterA, clusterB *extensionsv1alpha1.Cluster
		infraA, infraB     *extensionsv1alpha1.Infrastructure
		statusA, statusB   *azurev1alpha1.InfrastructureStatus
	)

	defer func() {
		By("delete both infrastructures and wait for teardown")
		for _, infra := range []*extensionsv1alpha1.Infrastructure{infraA, infraB} {
			if infra == nil {
				continue
			}
			Expect(client.IgnoreNotFound(c.Delete(ctx, infra))).To(Succeed())
			Expect(extensions.WaitUntilExtensionObjectDeleted(
				ctx, c, log, infra,
				extensionsv1alpha1.InfrastructureResource,
				10*time.Second, 16*time.Minute,
			)).To(Succeed())
		}

		By("verify shared BYO resources still exist after both shoots' deletion")
		verifyBYOSharedDeletion(ctx, az, fx, nsA, nsB)

		for _, ns := range []*corev1.Namespace{nsAObj, nsBObj} {
			if ns != nil {
				Expect(client.IgnoreNotFound(c.Delete(ctx, ns))).To(Succeed())
			}
		}
		for _, cl := range []*extensionsv1alpha1.Cluster{clusterA, clusterB} {
			if cl != nil {
				Expect(client.IgnoreNotFound(c.Delete(ctx, cl))).To(Succeed())
			}
		}
	}()

	// Create per-shoot namespace + cluster + credentials secret + infrastructure CR.
	nsAObj, clusterA, infraA = createBYOShoot(ctx, c, log, nsA, fx.VNetName, fx.FixtureRG, fx.SubnetAName, fx.OverlayEnabled)
	nsBObj, clusterB, infraB = createBYOShoot(ctx, c, log, nsB, fx.VNetName, fx.FixtureRG, fx.SubnetBName, fx.OverlayEnabled)

	By("wait until both infrastructures are ready")
	for _, infra := range []*extensionsv1alpha1.Infrastructure{infraA, infraB} {
		Expect(extensions.WaitUntilExtensionObjectReady(
			ctx, c, log, infra,
			extensionsv1alpha1.InfrastructureResource,
			10*time.Second, 30*time.Second, 16*time.Minute, nil,
		)).To(Succeed())
	}

	By("decode both infrastructure statuses")
	statusA = decodeInfraStatus(ctx, c, infraA)
	statusB = decodeInfraStatus(ctx, c, infraB)

	By("verify per-shoot invariants for both infrastructures")
	verifyBYOSharedCreation(ctx, az, infraA, fx, statusA, fx.SubnetAName, fx.NSGAName)
	verifyBYOSharedCreation(ctx, az, infraB, fx, statusB, fx.SubnetBName, fx.NSGBName)

	if extraVerify != nil {
		extraVerify(statusA, statusB)
	}
}

// createBYOShoot spins up the per-shoot resources (namespace, cluster CR, cloudprovider secret,
// infrastructure CR) that a single BYO shoot needs. Returns the created objects for the caller's
// cleanup path.
func createBYOShoot(
	ctx context.Context,
	c client.Client,
	log logr.Logger,
	namespaceName, vnetName, vnetRG, subnetName string,
	overlayEnabled bool,
) (*corev1.Namespace, *extensionsv1alpha1.Cluster, *extensionsv1alpha1.Infrastructure) {
	By(fmt.Sprintf("[%s] create namespace", namespaceName))
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
	Expect(c.Create(ctx, ns)).To(Succeed())

	By(fmt.Sprintf("[%s] create cluster CR", namespaceName))
	cluster, err := newCluster(namespaceName, *region, false)
	Expect(err).ToNot(HaveOccurred())
	if overlayEnabled {
		Expect(injectOverlayNetworking(cluster)).To(Succeed())
	}
	Expect(c.Create(ctx, cluster)).To(Succeed())

	By(fmt.Sprintf("[%s] create cloudprovider secret", namespaceName))
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

	By(fmt.Sprintf("[%s] create BYO infrastructure resource", namespaceName))
	providerConfig := &azurev1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureConfig",
		},
		Zoned: true,
		Networks: azurev1alpha1.NetworkConfig{
			VNet: azurev1alpha1.VNet{
				Name:          ptr.To(vnetName),
				ResourceGroup: ptr.To(vnetRG),
			},
			Subnet: &azurev1alpha1.SubnetReference{Name: subnetName},
		},
	}
	infra, err := newInfrastructure(namespaceName, providerConfig)
	Expect(err).ToNot(HaveOccurred())
	Expect(c.Create(ctx, infra)).To(Succeed())

	return ns, cluster, infra
}

// decodeInfraStatus refetches the given Infrastructure and decodes its providerStatus.
func decodeInfraStatus(ctx context.Context, c client.Client, infra *extensionsv1alpha1.Infrastructure) *azurev1alpha1.InfrastructureStatus {
	Expect(c.Get(ctx, client.ObjectKey{Namespace: infra.Namespace, Name: infra.Name}, infra)).To(Succeed())
	status := &azurev1alpha1.InfrastructureStatus{}
	_, _, err := decoder.Decode(infra.Status.ProviderStatus.Raw, nil, status)
	Expect(err).ToNot(HaveOccurred())
	return status
}

// provisionBYOSharedFixture provisions every resource named in the fixture: one RG, one VNet, two
// NSGs, optionally a shared NAT gateway (with its PIP) and/or a shared route table, and finally
// two subnets each attached to its own NSG plus the shared resources.
func provisionBYOSharedFixture(ctx context.Context, log logr.Logger, az *azureClientSet, fx *BYOSharedFixture) error {
	if err := prepareNewResourceGroup(ctx, log, az, fx.FixtureRG, fx.Location); err != nil {
		return fmt.Errorf("create fixture RG %q: %w", fx.FixtureRG, err)
	}

	if err := prepareNewVNet(ctx, log, az, fx.FixtureRG, fx.VNetName, fx.Location, fx.VNetCIDR); err != nil {
		return fmt.Errorf("create fixture VNet: %w", err)
	}

	if err := prepareNewNetworkSecurityGroup(ctx, log, az, fx.FixtureRG, fx.NSGAName, fx.Location); err != nil {
		return fmt.Errorf("create NSG A: %w", err)
	}
	if err := prepareNewNetworkSecurityGroup(ctx, log, az, fx.FixtureRG, fx.NSGBName, fx.Location); err != nil {
		return fmt.Errorf("create NSG B: %w", err)
	}

	var sharedNATID, sharedRTID string

	if fx.SharedNATName != "" {
		if err := prepareNewNatIp(ctx, log, az, fx.FixtureRG, fx.SharedNATPIPName, fx.Location, "1"); err != nil {
			return fmt.Errorf("create shared NAT PIP: %w", err)
		}
		if err := prepareNewNatGateway(ctx, log, az, fx.FixtureRG, fx.SharedNATName, fx.SharedNATPIPName, fx.Location); err != nil {
			return fmt.Errorf("create shared NAT gateway: %w", err)
		}
		id, err := natGatewayID(*subscriptionId, fx.FixtureRG, fx.SharedNATName)
		if err != nil {
			return err
		}
		sharedNATID = id
	}

	if fx.SharedRTName != "" {
		if err := prepareNewRouteTable(ctx, log, az, fx.FixtureRG, fx.SharedRTName, fx.Location, false); err != nil {
			return fmt.Errorf("create shared route table: %w", err)
		}
		id, err := routeTableID(*subscriptionId, fx.FixtureRG, fx.SharedRTName)
		if err != nil {
			return err
		}
		sharedRTID = id
	}

	nsgAID, err := securityGroupID(*subscriptionId, fx.FixtureRG, fx.NSGAName)
	if err != nil {
		return err
	}
	if err := prepareSubnetWithAllAttachments(ctx, log, az, fx.FixtureRG, fx.VNetName, fx.SubnetAName, fx.SubnetACIDR, nsgAID, sharedRTID, sharedNATID); err != nil {
		return fmt.Errorf("create subnet A: %w", err)
	}

	nsgBID, err := securityGroupID(*subscriptionId, fx.FixtureRG, fx.NSGBName)
	if err != nil {
		return err
	}
	if err := prepareSubnetWithAllAttachments(ctx, log, az, fx.FixtureRG, fx.VNetName, fx.SubnetBName, fx.SubnetBCIDR, nsgBID, sharedRTID, sharedNATID); err != nil {
		return fmt.Errorf("create subnet B: %w", err)
	}

	return nil
}

// prepareNewNatGateway creates a Standard SKU NAT gateway attached to the given PIP.
func prepareNewNatGateway(ctx context.Context, log logr.Logger, az *azureClientSet, rg, name, pipName, location string) error {
	log.Info("generating new NAT gateway", "rg", rg, "name", name, "pip", pipName)
	pipID, err := publicIPAddressID(*subscriptionId, rg, pipName)
	if err != nil {
		return err
	}
	poller, err := az.nat.BeginCreateOrUpdate(ctx, rg, name, armnetwork.NatGateway{
		Location: ptr.To(location),
		SKU:      &armnetwork.NatGatewaySKU{Name: ptr.To(armnetwork.NatGatewaySKUNameStandard)},
		Properties: &armnetwork.NatGatewayPropertiesFormat{
			IdleTimeoutInMinutes: ptr.To(int32(4)),
			PublicIPAddresses: []*armnetwork.SubResource{
				{ID: ptr.To(pipID)},
			},
		},
	}, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

// prepareSubnetWithAllAttachments is the shared-fixture variant of prepareSubnetWithAttachments:
// it optionally attaches a NAT gateway in addition to the NSG and (optional) RT.
func prepareSubnetWithAllAttachments(ctx context.Context, log logr.Logger, az *azureClientSet, vnetRG, vnetName, subnetName, cidr, nsgID, rtID, natID string) error {
	log.Info("generating BYO subnet", "vnet", vnetName, "subnet", subnetName, "attachRT", rtID != "", "attachNAT", natID != "")
	props := &armnetwork.SubnetPropertiesFormat{
		AddressPrefix:        ptr.To(cidr),
		NetworkSecurityGroup: &armnetwork.SecurityGroup{ID: ptr.To(nsgID)},
	}
	if rtID != "" {
		props.RouteTable = &armnetwork.RouteTable{ID: ptr.To(rtID)}
	}
	if natID != "" {
		props.NatGateway = &armnetwork.SubResource{ID: ptr.To(natID)}
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

func natGatewayID(subscription, rg, name string) (string, error) {
	if subscription == "" || rg == "" || name == "" {
		return "", fmt.Errorf("invalid inputs for NAT gateway ID: subscription=%q rg=%q name=%q", subscription, rg, name)
	}
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/natGateways/%s", subscription, rg, name), nil
}

func publicIPAddressID(subscription, rg, name string) (string, error) {
	if subscription == "" || rg == "" || name == "" {
		return "", fmt.Errorf("invalid inputs for PIP ID: subscription=%q rg=%q name=%q", subscription, rg, name)
	}
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/publicIPAddresses/%s", subscription, rg, name), nil
}

// verifyBYOSharedCreation runs the per-shoot invariants that must hold for every sharing shoot
// (identical shape to verifyBYOCreation but parameterized so both shoots can be checked).
func verifyBYOSharedCreation(
	ctx context.Context,
	az *azureClientSet,
	infra *extensionsv1alpha1.Infrastructure,
	fx *BYOSharedFixture,
	status *azurev1alpha1.InfrastructureStatus,
	expectedSubnetName, expectedNSGName string,
) {
	By(fmt.Sprintf("[%s] verifying status shape", infra.Namespace))
	Expect(status.Networks.OutboundAccessType).To(Equal(azurev1alpha1.OutboundAccessTypeUserManaged))
	Expect(status.Networks.Layout).To(Equal(azurev1alpha1.NetworkLayoutSingleSubnet))
	Expect(status.Networks.VNet.Name).To(Equal(fx.VNetName))
	Expect(status.Networks.VNet.ResourceGroup).To(PointTo(Equal(fx.FixtureRG)))
	Expect(status.Networks.Subnets).To(HaveLen(1))
	Expect(status.Networks.Subnets[0].Name).To(Equal(expectedSubnetName))

	Expect(status.SecurityGroups).To(HaveLen(1))
	Expect(status.SecurityGroups[0].Name).To(Equal(expectedNSGName))

	By(fmt.Sprintf("[%s] verifying cluster RG has no Gardener-owned worker NSG / RT / NAT", infra.Namespace))
	clusterRG := infra.Namespace
	_, nsgErr := az.securityGroups.Get(ctx, clusterRG, clusterRG+"-workers", nil)
	Expect(nsgErr).To(HaveOccurred())
	Expect(nsgErr).To(BeNotFoundError())

	_, rtErr := az.routeTable.Get(ctx, clusterRG, "worker_route_table", nil)
	Expect(rtErr).To(HaveOccurred())
	Expect(rtErr).To(BeNotFoundError())

	natPager := az.nat.NewListPager(clusterRG, nil)
	for natPager.More() {
		page, err := natPager.NextPage(ctx)
		Expect(err).ToNot(HaveOccurred())
		Expect(page.Value).To(BeEmpty())
	}
}

// verifyBYOSharedDeletion asserts that after both shoots are deleted, both cluster RGs are gone
// but every shared user resource in the fixture RG still exists.
func verifyBYOSharedDeletion(ctx context.Context, az *azureClientSet, fx *BYOSharedFixture, nsA, nsB string) {
	By("both cluster RGs must be deleted")
	for _, rg := range []string{nsA, nsB} {
		_, err := az.groups.Get(ctx, rg, nil)
		Expect(err).To(HaveOccurred())
		Expect(err).To(BeNotFoundError())
	}

	By("shared BYO fixture resources must survive")
	_, err := az.vnet.Get(ctx, fx.FixtureRG, fx.VNetName, nil)
	Expect(err).ToNot(HaveOccurred())
	_, err = az.subnets.Get(ctx, fx.FixtureRG, fx.VNetName, fx.SubnetAName, nil)
	Expect(err).ToNot(HaveOccurred())
	_, err = az.subnets.Get(ctx, fx.FixtureRG, fx.VNetName, fx.SubnetBName, nil)
	Expect(err).ToNot(HaveOccurred())
	_, err = az.securityGroups.Get(ctx, fx.FixtureRG, fx.NSGAName, nil)
	Expect(err).ToNot(HaveOccurred())
	_, err = az.securityGroups.Get(ctx, fx.FixtureRG, fx.NSGBName, nil)
	Expect(err).ToNot(HaveOccurred())
	if fx.SharedNATName != "" {
		_, err = az.nat.Get(ctx, fx.FixtureRG, fx.SharedNATName, nil)
		Expect(err).ToNot(HaveOccurred())
	}
	if fx.SharedRTName != "" {
		_, err = az.routeTable.Get(ctx, fx.FixtureRG, fx.SharedRTName, nil)
		Expect(err).ToNot(HaveOccurred())
	}
}

// verifySubnetHasNAT reads the given subnet and asserts its natGateway association matches the
// expected NAT gateway ID.
func verifySubnetHasNAT(ctx context.Context, az *azureClientSet, vnetRG, vnetName, subnetName, expectedNATID string) {
	resp, err := az.subnets.Get(ctx, vnetRG, vnetName, subnetName, nil)
	Expect(err).ToNot(HaveOccurred())
	Expect(resp.Properties).ToNot(BeNil())
	Expect(resp.Properties.NatGateway).ToNot(BeNil())
	Expect(resp.Properties.NatGateway.ID).To(PointTo(Equal(expectedNATID)))
}

// verifySubnetHasRT reads the given subnet and asserts its routeTable association matches the
// expected route-table ID.
func verifySubnetHasRT(ctx context.Context, az *azureClientSet, vnetRG, vnetName, subnetName, expectedRTID string) {
	resp, err := az.subnets.Get(ctx, vnetRG, vnetName, subnetName, nil)
	Expect(err).ToNot(HaveOccurred())
	Expect(resp.Properties).ToNot(BeNil())
	Expect(resp.Properties.RouteTable).ToNot(BeNil())
	Expect(resp.Properties.RouteTable.ID).To(PointTo(Equal(expectedRTID)))
}
