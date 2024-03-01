// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/gardener/gardener/pkg/utils"
	"k8s.io/utils/pointer"

	azureapi "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure"
	azureapihelper "github.com/gardener/gardener-extension-provider-azure/pkg/apis/azure/helper"
	"github.com/gardener/gardener-extension-provider-azure/pkg/azure"
	azureclient "github.com/gardener/gardener-extension-provider-azure/pkg/azure/client"
)

func (w *workerDelegate) reconcileVmoDependencies(ctx context.Context, infrastructureStatus *azureapi.InfrastructureStatus, workerProviderStatus *azureapi.WorkerStatus) ([]azureapi.VmoDependency, error) {
	var vmoDependencies = copyVmoDependencies(workerProviderStatus)

	vmoClient, err := w.clientFactory.Vmss()
	if err != nil {
		return vmoDependencies, err
	}

	faultDomainCount, err := azureapihelper.FindDomainCountByRegion(w.cloudProfileConfig.CountFaultDomains, w.worker.Spec.Region)
	if err != nil {
		return nil, err
	}

	// Deploy workerpool dependencies and store their status to be persistent in the worker provider status.
	for _, workerPool := range w.worker.Spec.Pools {
		vmoDependencyStatus, err := w.reconcileVMO(ctx, vmoClient, vmoDependencies, infrastructureStatus.ResourceGroup.Name, workerPool.Name, faultDomainCount)
		if err != nil {
			return vmoDependencies, err
		}
		vmoDependencies = appendVmoDependency(vmoDependencies, vmoDependencyStatus)
	}

	return vmoDependencies, nil
}

func (w *workerDelegate) reconcileVMO(ctx context.Context, client azureclient.Vmss, dependencies []azureapi.VmoDependency, resourceGroupName, workerPoolName string, faultDomainCount int32) (*azureapi.VmoDependency, error) {
	var (
		existingDependency *azureapi.VmoDependency
		vmo                *armcompute.VirtualMachineScaleSet
		err                error
	)

	// Check if there is already a VMO dependency object for the workerpool in the status.
	for _, dep := range dependencies {
		if dep.PoolName == workerPoolName {
			existingDependency = &dep
			break
		}
	}

	// Try to fetch the VMO from Azure as it exists in the status.
	if existingDependency != nil {
		vmo, err = client.Get(ctx, resourceGroupName, existingDependency.Name, to.Ptr(armcompute.ExpandTypesForGetVMScaleSetsUserData))
		if err != nil {
			return nil, err
		}
	}

	// VMO does not exists. Create it.
	if vmo == nil {
		newVMO, err := generateAndCreateVmo(ctx, client, workerPoolName, resourceGroupName, w.worker.Spec.Region, faultDomainCount)
		if err != nil {
			return nil, err
		}
		return newVMO, nil
	}

	// VMO already exists. Check if the fault domain count configuration has been changed.
	// If yes then it is required to create a new VMO with the correct configuration.
	if *vmo.Properties.PlatformFaultDomainCount != faultDomainCount {
		newVMO, err := generateAndCreateVmo(ctx, client, workerPoolName, resourceGroupName, w.worker.Spec.Region, faultDomainCount)
		if err != nil {
			return nil, err
		}
		return newVMO, nil
	}

	return generateVmoDependency(vmo, workerPoolName), nil
}

func (w *workerDelegate) cleanupVmoDependencies(ctx context.Context, infrastructureStatus *azureapi.InfrastructureStatus, workerProviderStatus *azureapi.WorkerStatus) ([]azureapi.VmoDependency, error) {
	var vmoDependencies = copyVmoDependencies(workerProviderStatus)

	vmoClient, err := w.clientFactory.Vmss()
	if err != nil {
		return vmoDependencies, err
	}

	// Cleanup VMO dependencies which are not tracked in the worker provider status anymore.
	if err := cleanupOrphanVMODependencies(ctx, vmoClient, workerProviderStatus.VmoDependencies, infrastructureStatus.ResourceGroup.Name); err != nil {
		return vmoDependencies, err
	}

	// Delete all vmo workerpool dependencies as the Worker is intended to be deleted.
	if w.worker.ObjectMeta.DeletionTimestamp != nil {
		for _, dependency := range workerProviderStatus.VmoDependencies {
			if err := vmoClient.Delete(ctx, infrastructureStatus.ResourceGroup.Name, dependency.Name, pointer.Bool(false)); err != nil {
				return vmoDependencies, err
			}
			vmoDependencies = removeVmoDependency(vmoDependencies, dependency)
		}
		return vmoDependencies, nil
	}

	for _, dependency := range workerProviderStatus.VmoDependencies {
		var workerPoolExists = false
		for _, pool := range w.worker.Spec.Pools {
			if pool.Name == dependency.PoolName {
				workerPoolExists = true
				break
			}
		}
		if workerPoolExists {
			continue
		}

		// Delete the dependency as no corresponding workerpool exist anymore.
		if err := vmoClient.Delete(ctx, infrastructureStatus.ResourceGroup.Name, dependency.Name, pointer.Bool(false)); err != nil {
			return vmoDependencies, err
		}
		vmoDependencies = removeVmoDependency(vmoDependencies, dependency)
	}
	return vmoDependencies, nil
}

func cleanupOrphanVMODependencies(ctx context.Context, client azureclient.Vmss, dependencies []azureapi.VmoDependency, resourceGroupName string) error {
	vmoListAll, err := client.List(ctx, resourceGroupName)
	if err != nil && !azureclient.IsAzureAPINotFoundError(err) {
		return err
	}
	vmoList := filterGardenerManagedVmos(vmoListAll)

	for _, vmo := range vmoList {
		vmoExists := false
		for _, dependency := range dependencies {
			if *vmo.ID == dependency.ID {
				vmoExists = true
				break
			}
		}
		if !vmoExists {
			if err := client.Delete(ctx, resourceGroupName, *vmo.Name, pointer.Bool(false)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *workerDelegate) determineWorkerPoolVmoDependency(ctx context.Context, infrastructureStatus *azureapi.InfrastructureStatus, workerStatus *azureapi.WorkerStatus, workerPoolName string) (*azureapi.VmoDependency, error) {
	if !azureapihelper.IsVmoRequired(infrastructureStatus) {
		return nil, nil
	}

	// First: Lookup the vmo dependency for the worker pool in the worker status.
	var dependencyInStatus *azureapi.VmoDependency
	for _, dep := range workerStatus.VmoDependencies {
		if dep.PoolName != workerPoolName {
			continue
		}
		if dependencyInStatus != nil {
			return nil, fmt.Errorf("found more then one vmo dependencies for workerpool %s in the worker provider status", workerPoolName)
		}
		depSnapshot := dep
		dependencyInStatus = &depSnapshot
	}
	if dependencyInStatus != nil {
		return dependencyInStatus, nil
	}

	// Second: The vmo dependency was not found in the worker status. Check if a corresponding vmo exists on Azure.
	vmoClient, err := w.clientFactory.Vmss()
	if err != nil {
		return nil, err
	}

	vmoListAll, err := vmoClient.List(ctx, infrastructureStatus.ResourceGroup.Name)
	if err != nil {
		return nil, err
	}
	vmoList := filterGardenerManagedVmos(vmoListAll)

	var existingVmo *armcompute.VirtualMachineScaleSet
	for _, vmo := range vmoList {
		if vmo.Name != nil && strings.Contains(*vmo.Name, workerPoolName) {
			if existingVmo != nil {
				return nil, fmt.Errorf("found multiple vmos for workerpool %q in resource group %q", workerPoolName, infrastructureStatus.ResourceGroup.Name)
			}
			vmoSnapshot := vmo
			existingVmo = vmoSnapshot
		}
	}
	if existingVmo != nil {
		existingVmoDependency := generateVmoDependency(existingVmo, workerPoolName)
		workerStatus.VmoDependencies = append(workerStatus.VmoDependencies, *existingVmoDependency)
		if err := w.updateWorkerProviderStatus(ctx, workerStatus); err != nil {
			return nil, err
		}
		return existingVmoDependency, nil
	}

	// Third: No vmo for the worker pool was found on Azure. Need to create it.
	faultDomainCount, err := azureapihelper.FindDomainCountByRegion(w.cloudProfileConfig.CountFaultDomains, w.worker.Spec.Region)
	if err != nil {
		return nil, err
	}

	newDependency, err := generateAndCreateVmo(ctx, vmoClient, workerPoolName, infrastructureStatus.ResourceGroup.Name, w.worker.Spec.Region, faultDomainCount)
	if err != nil {
		return nil, err
	}
	workerStatus.VmoDependencies = append(workerStatus.VmoDependencies, *newDependency)
	if err := w.updateWorkerProviderStatus(ctx, workerStatus); err != nil {
		return nil, err
	}
	return newDependency, nil
}

// VMO Helper

func generateAndCreateVmo(ctx context.Context, client azureclient.Vmss, workerPoolName, resourceGroupName, region string, faultDomainCount int32) (*azureapi.VmoDependency, error) {
	var properties = armcompute.VirtualMachineScaleSet{
		Location: &region,
		Properties: &armcompute.VirtualMachineScaleSetProperties{
			SinglePlacementGroup:     pointer.Bool(false),
			PlatformFaultDomainCount: &faultDomainCount,
		},
		Tags: map[string]*string{
			azure.MachineSetTagKey: pointer.String("1"),
		},
	}

	randomString, err := utils.GenerateRandomString(8)
	if err != nil {
		return nil, err
	}

	newVMO, err := client.CreateOrUpdate(ctx, resourceGroupName, fmt.Sprintf("vmo-%s-%s", workerPoolName, randomString), properties)
	if err != nil {
		return nil, err
	}

	return generateVmoDependency(newVMO, workerPoolName), nil
}

func copyVmoDependencies(workerStatus *azureapi.WorkerStatus) []azureapi.VmoDependency {
	statusCopy := workerStatus.DeepCopy()
	return statusCopy.VmoDependencies
}

// appendVmoDependency appends a new vmo to the dependency list.
// If the dependency list contains already a vmo for the workerpool then the
// existing vmo object will be replaced by the given vmo object.
func appendVmoDependency(dependencies []azureapi.VmoDependency, dependency *azureapi.VmoDependency) []azureapi.VmoDependency {
	var idx *int
	for i, dep := range dependencies {
		if dep.PoolName == dependency.PoolName {
			idx = &i
			break
		}
	}
	if idx != nil {
		dependencies[*idx] = *dependency
	} else {
		dependencies = append(dependencies, *dependency)
	}
	return dependencies
}

// removeVmoDependency will remove a given vmo dependency from the passed list of dependencies.
func removeVmoDependency(dependencies []azureapi.VmoDependency, dependency azureapi.VmoDependency) []azureapi.VmoDependency {
	var idx *int
	for i, dep := range dependencies {
		if reflect.DeepEqual(dependency, dep) {
			idx = &i
			break
		}
	}
	if idx != nil {
		return append(dependencies[:*idx], dependencies[*idx+1:]...)
	}
	return dependencies
}

func filterGardenerManagedVmos(list []*armcompute.VirtualMachineScaleSet) []*armcompute.VirtualMachineScaleSet {
	var filteredList = []*armcompute.VirtualMachineScaleSet{}
	for _, vmo := range list {
		if _, hasTag := vmo.Tags[azure.MachineSetTagKey]; hasTag {
			filteredList = append(filteredList, vmo)
		}
	}
	return filteredList
}

func generateVmoDependency(vmo *armcompute.VirtualMachineScaleSet, workerPoolName string) *azureapi.VmoDependency {
	return &azureapi.VmoDependency{
		ID:       *vmo.ID,
		Name:     *vmo.Name,
		PoolName: workerPoolName,
	}
}
