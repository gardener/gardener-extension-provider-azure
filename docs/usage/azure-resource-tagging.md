# Azure Resource Tagging

This document describes which Azure resources are tagged by `gardener-extension-provider-azure` and what tags are applied to each resource type.

## Overview

The extension applies tags to Azure resources for two primary purposes:

1. **Ownership identification** — marking resources as Gardener-managed so they can be found, filtered, and reconciled correctly.
2. **Kubernetes node labeling** — propagating Shoot worker pool labels onto VM instances so that Kubernetes node labels are backed by Azure VM tags (used by the cloud controller manager).

## Tagged Resource Types

### Virtual Machines (Worker Nodes)

Worker node VMs receive tags derived from the worker pool configuration in the Shoot spec.

| Tag Key | Value | Source |
|---|---|---|
| `Name` | Technical ID of the Shoot | Shoot technical ID |
| `kubernetes.io-cluster-{technicalID}` | `"1"` | Shoot technical ID |
| `kubernetes.io-role-node` | `"1"` | Static |
| `{label-key}` | `{label-value}` | Each entry in `shoot.spec.provider.workers[].labels` |

### Virtual Machine Scale Sets / VMOs (Flex Orchestration)

Azure Virtual Machine Scale Set resources created for worker pools (VMO mode) receive tags that identify them as Gardener-managed and associate them with a specific worker pool.

| Tag Key | Value | Source |
|---|---|---|
| `machineset.azure.extensions.gardener.cloud` | `"1"` | Static — marks the VMSS as Gardener-managed |
| `machineset.azure.extensions.gardener.cloud.worker-name` | Worker pool name | `worker.Name` |

These tags are also used as a filter when listing VMSS resources to determine which ones belong to the extension.

### Public IP Addresses

Public IP addresses created for load balancers (Services of type `LoadBalancer`) are tagged so the extension can identify and manage them across reconciliation cycles.

| Tag Key | Value | Source |
|---|---|---|
| `managed-by-gardener` | `"true"` | Static |
| `gardener-shoot-name` | Technical name of the Shoot | Shoot technical ID |

During reconciliation, the extension filters Public IPs by requiring **both** of these tags to match. This prevents the extension from touching Public IPs it did not create.

### Bastion Resources

When a bastion host is created, the following resources are all tagged with the same set of tags:

- Bastion Virtual Machine
- Network Interface Card (NIC) attached to the bastion VM
- Public IP Address of the bastion

| Tag Key | Value | Source |
|---|---|---|
| `Name` | Bastion instance name (cluster name + bastion name + hash) | Derived from cluster and bastion metadata |
| `Type` | `"gardenctl"` | Static |

### Blob Storage Objects

Blobs in Azure Storage accounts may be tagged when they cannot be deleted immediately due to an immutability policy (e.g. WORM retention). In this case a tag is applied to mark the blob for deferred deletion via a storage lifecycle policy.

| Tag Key | Value | Source |
|---|---|---|
| `blob-marked-for-deletion` | `"true"` | Static |

## Tag Sanitization

Azure VM tags do not allow the characters `< > % \ & ? /` or spaces in tag keys. Worker pool label keys (and the Shoot technical ID used in `kubernetes.io-cluster-*` keys) are sanitized by replacing any of these characters with an underscore (`_`) before the tags are applied.
