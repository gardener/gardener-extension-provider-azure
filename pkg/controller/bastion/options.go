// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"crypto/sha256"
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	extensionsbastion "github.com/gardener/gardener/extensions/pkg/bastion"
	"github.com/gardener/gardener/extensions/pkg/controller"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	"github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
)

// Maximum length for "base" name due to fact that we use this name to name other Azure resources,
// https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/resource-name-rules
const maxLengthForBaseName = 33

// BaseOptions contain the information needed for deleting a Bastion on Azure.
type BaseOptions struct {
	BastionInstanceName string
	ResourceGroupName   string
	DiskName            string
	PublicIPName        string
	NicName             string
	SecurityGroupName   string
	SecretReference     corev1.SecretReference
	Logr                logr.Logger
}

// Options contains provider-related information required for setting up
// a bastion instance. This struct combines precomputed values like the
// bastion instance name with the IDs of pre-existing cloud provider
// resources, like the VPC ID, subnet ID etc.
type Options struct {
	PrivateIPAddressV4 string
	PrivateIPAddressV6 string
	Location           string
	NicID              string
	WorkersCIDR        []string
	CIDRs              []string
	Tags               map[string]*string
	MachineType        string
	ImageRef           *armcompute.ImageReference
	// needed for creation and deletion
	BaseOptions
}

// NewBaseOpts determines base opts that are required for creating and deleting a Bastion.
func NewBaseOpts(bastion *extensionsv1alpha1.Bastion, cluster *controller.Cluster, resourceGroup string, log logr.Logger) (BaseOptions, error) {
	clusterName := cluster.ObjectMeta.Name
	baseResourceName, err := generateBastionBaseResourceName(clusterName, bastion.Name)
	if err != nil {
		return BaseOptions{}, err
	}

	secretReference := corev1.SecretReference{
		Namespace: cluster.ObjectMeta.Name,
		Name:      v1beta1constants.SecretNameCloudProvider,
	}

	return BaseOptions{
		BastionInstanceName: baseResourceName,
		ResourceGroupName:   resourceGroup,
		SecretReference:     secretReference,
		Logr:                log,
		DiskName:            DiskResourceName(baseResourceName),
		PublicIPName:        publicIPResourceName(baseResourceName),
		NicName:             NicResourceName(baseResourceName),
		SecurityGroupName:   NSGName(clusterName),
	}, nil
}

// NewOpts determines the information that is required to reconcile a Bastion.
func NewOpts(bastion *extensionsv1alpha1.Bastion, cluster *controller.Cluster, resourceGroup string, log logr.Logger) (Options, error) {
	baseOpts, err := NewBaseOpts(bastion, cluster, resourceGroup, log)
	if err != nil {
		return Options{}, err
	}

	cidrs, err := ingressPermissions(bastion)
	if err != nil {
		return Options{}, err
	}

	workersCidr, err := getWorkersCIDR(cluster)
	if err != nil {
		return Options{}, err
	}

	tags := map[string]*string{
		"Name": &baseOpts.BastionInstanceName,
		"Type": ptr.To("gardenctl"),
	}

	machineSpec, err := extensionsbastion.GetMachineSpecFromCloudProfile(cluster.CloudProfile)
	if err != nil {
		return Options{}, fmt.Errorf("failed to determine VM details for bastion host: %w", err)
	}

	cloudProfileConfig, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return Options{}, fmt.Errorf("failed to extract cloud provider config from cluster: %w", err)
	}

	imageRef, err := getProviderSpecificImage(cloudProfileConfig.MachineImages, machineSpec)
	if err != nil {
		return Options{}, fmt.Errorf("failed to extract image from provider config: %w", err)
	}

	return Options{
		CIDRs:       cidrs,
		WorkersCIDR: workersCidr,
		Location:    cluster.Shoot.Spec.Region,
		Tags:        tags,
		MachineType: machineSpec.MachineTypeName,
		ImageRef:    imageRef,
		BaseOptions: baseOpts,
	}, nil
}

func generateBastionBaseResourceName(clusterName string, bastionName string) (string, error) {
	if clusterName == "" {
		return "", fmt.Errorf("clusterName can't be empty")
	}
	if bastionName == "" {
		return "", fmt.Errorf("bastionName can't be empty")
	}

	staticName := clusterName + "-" + bastionName
	h := sha256.New()
	_, err := h.Write([]byte(staticName))
	if err != nil {
		return "", err
	}
	hash := fmt.Sprintf("%x", h.Sum(nil))
	if len([]rune(staticName)) > maxLengthForBaseName {
		staticName = staticName[:maxLengthForBaseName]
	}
	return fmt.Sprintf("%s-bastion-%s", staticName, hash[:5]), nil
}

func ingressPermissions(bastion *extensionsv1alpha1.Bastion) ([]string, error) {
	var cidrs []string
	for _, ingress := range bastion.Spec.Ingress {
		cidr := ingress.IPBlock.CIDR
		ip, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid ingress CIDR %q: %w", cidr, err)
		}

		normalisedCIDR := ipNet.String()

		if ip.To4() == nil && ip.To16() == nil {
			return nil, fmt.Errorf("ip address %s is not valid: %w", normalisedCIDR, err)
		}

		cidrs = append(cidrs, normalisedCIDR)
	}

	return cidrs, nil
}

// getProviderSpecificImage returns the provider specific MachineImageVersion that matches with the given MachineSpec
func getProviderSpecificImage(images []azure.MachineImages, vm extensionsbastion.MachineSpec) (*armcompute.ImageReference, error) {
	imageIndex := slices.IndexFunc(images, func(image azure.MachineImages) bool {
		return strings.EqualFold(image.Name, vm.ImageBaseName)
	})

	if imageIndex == -1 {
		return nil, fmt.Errorf("machine image with name %s not found in cloudProfileConfig", vm.ImageBaseName)
	}

	versions := images[imageIndex].Versions
	versionIndex := slices.IndexFunc(versions, func(version azure.MachineImageVersion) bool {
		return version.Version == vm.ImageVersion
	})

	if versionIndex == -1 {
		return nil, fmt.Errorf("version %s for arch %s of image %s not found in cloudProfileConfig",
			vm.ImageVersion, vm.Architecture, vm.ImageBaseName)
	}

	image := versions[versionIndex]

	var (
		publisher *string
		offer     *string
		sku       *string
		version   *string
	)
	if image.URN != nil {
		urnSplit := strings.Split(*image.URN, ":")
		if len(urnSplit) == 4 {
			publisher = &urnSplit[0]
			offer = &urnSplit[1]
			sku = &urnSplit[2]
			version = &urnSplit[3]
		}
	}

	return &armcompute.ImageReference{
		CommunityGalleryImageID: image.CommunityGalleryImageID,
		ID:                      image.ID,
		Publisher:               publisher,
		Offer:                   offer,
		SKU:                     sku,
		Version:                 version,
		SharedGalleryImageID:    image.SharedGalleryImageID,
	}, nil
}

func nodesResourceName(baseName string) string {
	return fmt.Sprintf("%s-nodes", baseName)
}

func publicIPResourceName(baseName string) string {
	return fmt.Sprintf("%s-public-ip", baseName)
}

// NSGIngressAllowSSHResourceNameIPv4 is network security group ingress allow ssh resource name
func NSGIngressAllowSSHResourceNameIPv4(baseName string) string {
	return fmt.Sprintf("%s-allow-ssh-ipv4", baseName)
}

// NSGIngressAllowSSHResourceNameIPv6 is network security group ingress allow ssh resource name
func NSGIngressAllowSSHResourceNameIPv6(baseName string) string {
	return fmt.Sprintf("%s-allow-ssh-ipv6", baseName)
}

// NSGEgressAllowOnlyResourceName is network security group egress allow only rule name
func NSGEgressAllowOnlyResourceName(baseName string) string {
	return fmt.Sprintf("%s-egress-worker", baseName)
}

// NSGEgressDenyAllResourceName is network security group egress deny all rule name
func NSGEgressDenyAllResourceName(baseName string) string {
	return fmt.Sprintf("%s-deny-all", baseName)
}

// NSGName is network security group resource name
func NSGName(baseName string) string {
	return fmt.Sprintf("%s-workers", baseName)
}

// DiskResourceName is Disk resource name
func DiskResourceName(baseName string) string {
	return fmt.Sprintf("%s-disk", baseName)
}

// NicResourceName is Nic resource name
func NicResourceName(baseName string) string {
	return fmt.Sprintf("%s-nic", baseName)
}
