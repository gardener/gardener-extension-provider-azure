---
title: User-managed egress via BYO subnet
---

# User-managed egress via BYO subnet

Azure shoots may bring their own worker **subnet** inside their own **VNet** to take full control of
egress. In this mode Gardener stops creating and managing the worker subnet, its route table, its
network security group, the NAT Gateway, and the `allow-{tcp,udp}-egress` loadbalancer workaround
services. The user pre-provisions all of the above; Gardener only discovers and references them.

This document covers the shoot-owner's side of the workflow. For the full design intent and
acceptance criteria see the design proposal at
[`docs/proposals/flexible-network-configuration-proposal.md`](../proposals/flexible-network-configuration-proposal.md).

## When to use it

Reach for BYO subnet when any of the following applies:

- **Central firewall egress** — all `0.0.0.0/0` from workers should flow through an Azure Firewall
  or third-party NVA sitting in a hub VNet. The user attaches a route table with a
  `VirtualAppliance` next-hop to their worker subnet.
- **Network-isolated / air-gapped shoots** — the shoot must not have any internet egress. The user
  attaches a route table with no default route; Kubernetes traffic terminates via Private
  Endpoints (private API server, Private Endpoints for MCR, etc.).
- **Platform-team provisioned networking** — the enterprise's platform team pre-provisions every
  subnet, NSG, and route table and hands the shoot owner the identifiers.

For every other scenario the default managed-mode shoot (with an optional NAT Gateway) is simpler
and unchanged.

## Prerequisites (user responsibilities)

Before creating the shoot, the user MUST provide the following in their Azure subscription:

1. A **VNet**.
2. A **subnet** inside that VNet, whose CIDR fits inside `shoot.spec.networking.nodes` and does not
   overlap `shoot.spec.networking.pods` or `.services`.
3. A **network security group** attached to that subnet, permitting at minimum:
   - Intra-subnet traffic (pod-to-pod, node-to-node).
   - Inbound from the `AzureLoadBalancer` service tag (for LB health probes).
4. A **route table** attached to that subnet. For firewall-based egress this route table should
   contain a `0.0.0.0/0` route to the user's firewall / NVA (next-hop = `VirtualAppliance` +
   firewall IP). For network-isolated shoots the route table may be empty. If the shoot uses an
   overlay CNI (Cilium/Calico with VXLAN or Geneve — i.e. `shoot.spec.networking.providerConfig`
   sets `overlay.enabled: true`), the route table can be omitted entirely; the seed CCM's route
   controller is disabled automatically in that case (same behavior as provider-gcp).
5. Any firewall rules required for Azure control-plane traffic and container image pulls —
   at minimum the `AzureCloud` service tag, the `AzureContainerRegistry` service tag, and the
   public MCR endpoint (`mcr.microsoft.com` and its CDN backends). Cross-link to the
   [outbound-connectivity docs](https://learn.microsoft.com/en-us/azure/aks/limit-egress-traffic)
   for the full list.
6. Route table and NSG must live in the **same Azure subscription** as the shoot. They may live in
   any resource group within that subscription — cluster RG, VNet RG, or a central security-team RG
   are all supported.
7. The Gardener Azure principal must have write permission on the NSG's resource group
   (`Microsoft.Network/networkSecurityGroups/securityRules/write`). The Azure cloud-controller-manager
   adds/removes rules on the NSG for each `Service type=LoadBalancer` lifecycle event, and the
   bastion controller adds/removes rules for each `Bastion` resource lifecycle event.

The user MUST NOT:

- Run competing automation (Terraform, policy engines) against the discovered route table — the
  seed cloud-controller-manager writes per-node pod-CIDR routes there. Shoots using an overlay CNI
  (see below) avoid this constraint because the CCM's route controller is disabled.
- Run competing automation against the discovered NSG — the CCM and bastion controller
  add/remove named, scoped rules there.
- Rely on `shoot.status.provider.egressCIDRs` for firewall allowlisting on the receiving side. That
  field is empty (`nil`) in BYO mode because Gardener has no reliable way to know the user's
  firewall / NVA egress IPs.

## Configuring a BYO-subnet shoot

Set `networks.subnet.name` on the InfrastructureConfig and reference the pre-provisioned VNet:

```yaml
apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
kind: InfrastructureConfig
zoned: true
networks:
  vnet:
    name: hub-spoke-workers-vnet
    resourceGroup: platform-network-rg
  subnet:
    name: shoot-a-workers
```

Presence of `networks.subnet` alone signals user-managed-egress mode; no `outboundType` enum or
marker annotation is required. When the config is submitted Gardener:

- Verifies the referenced subnet exists in the BYO VNet, that its CIDR is compatible with the
  shoot's networking config, and that it has the required NSG association pre-attached. A route
  table association is also required unless the shoot uses an overlay CNI
  (`spec.networking.providerConfig.overlay.enabled: true`).
- Skips creation of the worker subnet, route table, worker NSG, and NAT Gateway.
- Discovers the RT and NSG associations at reconcile time and threads their names and resource
  groups into the shoot's `azure.json` (as `routeTableName`/`routeTableResourceGroup`,
  `securityGroupName`/`securityGroupResourceGroup`).
- Sets `disableOutboundSNAT: true` in `azure.json` so that any user-created
  `Service type=LoadBalancer` does not become an accidental egress path bypassing the user's route
  table.
- Skips deploying the `allow-tcp-egress` and `allow-udp-egress` services in the shoot's
  `kube-system` namespace.
- Applies the observability tag `kubernetes.io/cluster/<shoot-technical-name>=shared` to the BYO
  VNet, NSG, and route table (best-effort; ignored if the principal lacks tag-write permission).

The `zoned` field remains valid — the shoot can still be zonal even though the worker subnet is
single-subnet.

## Fields that are forbidden in BYO mode

The following fields must **not** be set when `networks.subnet` is set:

| Forbidden field                    | Rationale                                                                          |
| ---------------------------------- | ---------------------------------------------------------------------------------- |
| `networks.workers`                 | Worker CIDR is discovered from the BYO subnet.                                     |
| `networks.zones`                   | Multi-subnet (zoned) layout is not supported with BYO subnet in v1.                |
| `networks.natGateway`              | NAT is user-managed. Attach a NAT gateway to the BYO subnet out-of-band if needed. |
| `networks.serviceEndpoints`        | Service endpoints are managed by the user on their own subnet.                     |
| `networks.vnet.cidr`               | VNet CIDR is discovered from the actual VNet, not declared.                        |
| `networks.vnet.ddosProtectionPlanID` | DDoS plan is managed by the user on the BYO VNet.                                |

## Overlay-CNI shoots

Shoots using an overlay CNI (Cilium/Calico with VXLAN or Geneve) do not need pod-CIDR routes in
the underlying VNet: pod-to-pod traffic is encapsulated at the node level. Gardener automatically
disables the seed CCM's route controller (`--configure-cloud-routes=false`) whenever the shoot's
networking provider config sets `overlay.enabled: true`, using the same signal as `provider-gcp`.

In BYO mode, this also relaxes the "subnet must have a route table" precondition — an overlay-CNI
BYO shoot may attach no route table at all:

```yaml
spec:
  networking:
    type: calico
    nodes: 10.250.0.0/16
    pods: 100.96.0.0/11
    services: 100.64.0.0/13
    providerConfig:
      apiVersion: calico.networking.extensions.gardener.cloud/v1alpha1
      kind: NetworkConfig
      overlay:
        enabled: true
  provider:
    type: azure
    infrastructureConfig:
      apiVersion: azure.provider.extensions.gardener.cloud/v1alpha1
      kind: InfrastructureConfig
      networks:
        vnet:
          name: hub-spoke-workers-vnet
          resourceGroup: platform-network-rg
        subnet:
          name: shoot-a-workers   # may reference a subnet with no route table attached
```

The user takes ownership of ensuring the shoot really uses an overlay CNI. Gardener does not
verify that `networking.type` matches the overlay setting.

## Migration and immutability

- Existing shoots keep working. Opting in requires setting `networks.subnet` at shoot **creation**
  time.
- In-place transitions between managed and BYO mode are forbidden in v1. Once created with
  `networks.subnet` set, the field must stay set; once created without, it must stay unset. Users
  who need to migrate must create a fresh shoot.
- `networks.subnet.name` is immutable once set.

## Bastion, `Service type=LoadBalancer`, and NSG mutation

Gardener's infrastructure reconciler is read-only against the BYO NSG. The Azure
cloud-controller-manager and the bastion controller still mutate the NSG's `securityRules`
collection at runtime — the same behavior every managed Azure shoot has today. Rule additions are
name-scoped and self-cleaning:

- `Service type=LoadBalancer` reconciliation adds allow-rules for the service's frontend IPs and
  ports; deletion removes them. Set the annotation
  `service.beta.kubernetes.io/azure-disable-load-balancer-nsg-rule: "true"` on a service to
  suppress NSG mutation for that specific service (the LB frontend is created but no NSG rules
  are added).
- `Bastion` resources add four rules (2 SSH-in from operator CIDRs, 1 SSH-out to worker CIDRs,
  1 deny-all-out). All rules are named `<bastion-instance-name>-*` and removed when the Bastion is
  deleted.

If the user's Azure Policy locks down NSG mutation on the BYO NSG, LB services and bastion
creation will fail at reconcile time. This is a user-facing prerequisite.

## What is NOT deleted on shoot deletion

- The BYO VNet, subnet, route table, and NSG are never deleted by Gardener — they belong to the
  user.
- The observability tag `kubernetes.io/cluster/<technicalName>: shared` is removed from the VNet,
  NSG, and route table on shoot deletion (best-effort; a failure logs a warning and does not block
  deletion). Other tags on those resources are untouched.
- Named rules added by the CCM (for LB services) and the bastion controller (for Bastion resources)
  are cleaned up on their owning resource's delete — same as for every managed Azure shoot today.
