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
In Azure we have the option to restore data disk from a custom image which is more convenient and flexible. 

### Goals

1. Extend the provider specific [WorkerConfig](https://github.com/gardener/gardener-extension-provider-azure/blob/master/docs/usage/usage.md#workerconfig) section in the shoot YAML 
 and support provider configuration for data-disks to support data-disk creation based on an existing image.
 

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
    imageReference: imgRef # ID or URN (publisher:offer:sku:version) of the image from which to create a disk
    resourceGroup: # (optional) Name of resource group. Will take default if omitted
  nodeTemplate: # (to be specified only if the node capacity would be different from cloudprofile info during runtime)
   capacity:
     cpu: 2
     gpu: 1
     memory: 50Gi
```

In the above `imgRef` specified via `providerConfig.dataVolumes.imageReference` represents the image ID or URN of the image from which to create a disk. This image should already exist.
See [az image create](https://learn.microsoft.com/en-us/cli/azure/image?view=azure-cli-latest#az-image-create).
An optional `resourceGroup` can be specified if the image is associated with a non-default Azure resource group.

The [MCM Azure Provider](https://github.com/gardener/machine-controller-manager-provider-azure) will ensure that the data 
disk is created with the _image reference_ set to the provided `imgRef`. See [az disk create](https://learn.microsoft.com/en-us/cli/azure/disk?view=azure-cli-latest#az-disk-create). 
The mechanics of this is left to MCM Azure provider.


