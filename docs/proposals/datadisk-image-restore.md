---
title: Data Disk Restore From Image
creation-date: 2025-04-05
status: implementable
authors:
- "@elankath"
reviewers:
- "@rishabh-11"
- "@unmarshall"
- "@kon-angelo "
---

# Data Disk Restore From Image

## Table of Contents

- [Summary](#summary)
- [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
- [Proposal](#proposal)
- [Alternatives](#alternatives)

## Summary

Currently, we have no support either in the shoot spec or in the [MCM Azure](https://github.com/gardener/machine-controller-manager-provider-azure) for restoring Azure Data Disks from a user created image.

## Motivation
The primary motivation is to support [Integration of vSMP MemeoryOne in Azure #](https://github.com/gardener/gardener-extension-provider-azure/issues/788). 
We implemented support for this in AWS via [Support for data volume snapshot ID ](https://github.com/gardener/gardener-extension-provider-aws/pull/112).
In Azure we have the option to restore data disk from an image which is more convenient and flexible. 

### Goals

1. Extend the provider specific [WorkerConfig](https://github.com/gardener/gardener-extension-provider-azure/blob/master/docs/usage/usage.md#workerconfig) section in the shoot YAML and support provider configuration for data-disks to support data-disk creation based from a snapshot id.
 

## Proposal

### Shoot Specification

At this current time, there is no support for provider specific configuration of data disks in an azure shoot spec.
The below shows an example configuration at the time of this proposal:
```yaml
providerConfig:
  apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
  kind: WorkerConfig
  nodeTemplate: # (to be specified only if the node capacity would be different from cloudprofile info during runtime)
    capacity:
      cpu: 2
      gpu: 1
      memory: 50Gi
```
We propose that the worker config section be enahnced to support data disk configuration
```yaml
providerConfig:
  apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
  kind: WorkerConfig
  dataVolumes: # <-- NEW SUB_SECTION
  - name: vsmp1
    snapshotName: snap-1234
  nodeTemplate: # (to be specified only if the node capacity would be different from cloudprofile info during runtime)
   capacity:
     cpu: 2
     gpu: 1
     memory: 50Gi
```

In the above `snap-1234` represents the snapshot name created by an external process/tool.
See [az-snapshot-create](https://learn.microsoft.com/en-us/cli/azure/snapshot?view=azure-cli-latest#az-snapshot-create).

The Azure disk `snapshotName` is distinct from the azure `snapshotID`. The azure disk `snapshotID` is a full qualified hierarchical
identifier that includes the `snapshotName`, Azure subscription ID and resource group name: 
like `/subscriptions/<AzureSubscriptionID>/resourceGroups/<resourceGroupName>/providers/Microsoft.Compute/disks/<snapshotName>`

It would be painful and errorprone to specify this in the shoot `WorkerConfig` section, so it is best that the MCM Azure provider take care
of forming the fully qualified azure disk `snapshotID` and forming the `MachineClass` for azure which is then operated on by MCM Azure Provider.


