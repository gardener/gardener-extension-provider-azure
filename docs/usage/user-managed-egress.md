---
title: User-managed egress via BYO subnet
---

# User-managed egress via BYO subnet

Azure shoots may bring their own worker **subnet** inside their own **VNet** to take full control of
egress. In this mode Gardener stops creating and managing the worker subnet, its route table, the
NAT Gateway, and the `allow-{tcp,udp}-egress` loadbalancer workaround services. The user pre-provisions
the subnet and route table; Gardener discovers and references them.

The cloud-controller-manager-facing network security group is unchanged in the sense that Gardener
continues to create it in the shoot's cluster resource group. The difference is where it lives at
the Azure layer: managed-mode shoots have it attached at the **subnet** (as today); BYO-subnet
shoots have it attached at every worker **NIC** by the machine-controller-manager, since Gardener
must not touch the user's subnet. A subnet-level NSG owned by the user in BYO mode is optional and
independent — Gardener neither creates nor mutates it. See [NSG evaluation](#nsg-evaluation) below.

> [!IMPORTANT]
> **The BYO subnet (and its route table) must not be shared with any other Gardener shoot.**
> If you provision a subnet for a shoot, that subnet is dedicated to that one shoot.
>
> Why: in non-overlay CNI shoots (`overlay.enabled: false`), the seed cloud-controller-manager
> writes one route per node into the route table attached to your subnet. Azure allows exactly
> one route table per subnet. Two shoots pointing at the same subnet therefore share the same
> route table, and their CCMs *mutually delete* each other's routes on every reconcile —
> silently breaking pod-to-pod connectivity across nodes. Overlay-CNI shoots (Gardener default:
> Calico/Cilium with VXLAN) do not have this specific writer conflict, but the same
> "one-shoot-per-subnet" rule applies uniformly in v1 to keep the constraint simple to reason
> about and to prevent accidental toggling of `overlay.enabled` from silently breaking sharing.
>
> Gardener does not enforce this rule at admission time — it is your responsibility to keep
> subnets one-to-one with shoots. Lifting this constraint is planned for a future release.

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
- **Platform-team provisioned networking** — the enterprise's platform team pre-provisions the
  subnet and route table and hands the shoot owner the identifiers.

For every other scenario the default managed-mode shoot (with an optional NAT Gateway) is simpler
and unchanged.

## Prerequisites (user responsibilities)

Before creating the shoot, the user MUST provide the following in their Azure subscription:

1. A **VNet**.
2. A **subnet** inside that VNet, whose CIDR fits inside `shoot.spec.networking.nodes` and does not
   overlap `shoot.spec.networking.pods` or `.services`. **The subnet must be dedicated to this
   shoot** — no other Gardener Azure shoot may reference the same subnet. See the important
   callout above for the rationale.
3. A **route table** attached to that subnet, dedicated to this shoot (do not attach it to any
   other subnet used by another Gardener shoot). For firewall-based egress this route table should
   contain a `0.0.0.0/0` route to the user's firewall / NVA (next-hop = `VirtualAppliance` +
   firewall IP). For network-isolated shoots the route table may be empty. If the shoot uses an
   overlay CNI (Cilium/Calico with VXLAN or Geneve — i.e. `shoot.spec.networking.providerConfig`
   sets `overlay.enabled: true`), the route table can be omitted entirely; the seed CCM's route
   controller is disabled automatically in that case (same behavior as provider-gcp).
4. Any firewall rules required for Azure control-plane traffic and container image pulls —
   at minimum the `AzureCloud` service tag, the `AzureContainerRegistry` service tag, and the
   public MCR endpoint (`mcr.microsoft.com` and its CDN backends). Cross-link to the
   [outbound-connectivity docs](https://learn.microsoft.com/en-us/azure/aks/limit-egress-traffic)
   for the full list.
5. The route table must live in the **same Azure subscription** as the shoot. It may live in
   any resource group within that subscription — cluster RG, VNet RG, or a central network-team RG
   are all supported.

The user MAY (optional):

6. Attach an **NSG to the subnet**. Gardener does not create, read, mutate, or delete it. If you
   attach one, you take responsibility for permitting the traffic flows Kubernetes needs — see
   [NSG evaluation](#nsg-evaluation) below for the exact list.

The user MUST NOT:

- **Reuse the subnet or its route table across multiple Gardener Azure shoots** (see the callout
  at the top of this page).
- Run competing automation (Terraform, policy engines) against the discovered route table in
  non-overlay shoots — the seed cloud-controller-manager writes per-node pod-CIDR routes there.
  Overlay-CNI shoots (Gardener default) are unaffected because the CCM's route controller is
  disabled.
- If you attached a subnet-level NSG: block any of the flows listed in
  [NSG evaluation](#nsg-evaluation). Doing so breaks the cluster; Gardener has no way to detect the
  misconfiguration at reconcile time.
- Rely on `shoot.status.provider.egressCIDRs` for firewall allowlisting on the receiving side. That
  field is empty (`nil`) in BYO mode because Gardener has no reliable way to know the user's
  firewall / NVA egress IPs.
- Expect Gardener to prune orphan pod-CIDR routes from the route table on shoot deletion — in
  non-overlay shoots some routes may remain after teardown if the CCM's graceful pruning failed
  (crash during teardown, force-delete of the shoot, etc.). See [Deletion](#deletion) below.

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

- Verifies the referenced subnet exists in the BYO VNet and that its CIDR is compatible with the
  shoot's networking config. A route table association is required unless the shoot uses an overlay
  CNI (`spec.networking.providerConfig.overlay.enabled: true`).
- Skips creation of the worker subnet, route table, and NAT Gateway.
- **Creates** the worker NSG in the shoot's cluster resource group (same as managed mode). This NSG
  is attached to worker node NICs by the machine-controller-manager (BYO-only behavior); it is not
  attached to the subnet.
- Discovers the route-table association at reconcile time and threads its name (and resource group,
  if outside cluster RG) into the shoot's `azure.json` as `routeTableName`/`routeTableResourceGroup`.
- Emits the Gardener-owned NSG's name as `securityGroupName` in `azure.json`. `securityGroupResourceGroup`
  is not emitted because the NSG lives in the cluster RG (the CCM's default).
- Sets `disableOutboundSNAT: true` in `azure.json` so that any user-created
  `Service type=LoadBalancer` does not become an accidental egress path bypassing the user's route
  table.
- Skips deploying the `allow-tcp-egress` and `allow-udp-egress` services in the shoot's
  `kube-system` namespace.
- Applies the observability tag `kubernetes.io/cluster/<shoot-technical-name>=shared` to the BYO
  VNet and route table (best-effort; ignored if the principal lacks tag-write permission).

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

## NSG evaluation

In BYO-subnet mode, two layers of Azure network security groups may sit in front of a worker node:

1. **NIC-level NSG (Gardener-owned)** — always present in BYO mode. Created by the infrastructure
   reconciler in the shoot's cluster RG and attached to every worker NIC by the machine-controller-manager
   at machine creation time. The Azure cloud-controller-manager writes `Service type=LoadBalancer`
   ingress rules here. The bastion controller writes bastion SSH rules here. This is the CCM-facing
   NSG (`securityGroupName` in `azure.json`).
2. **Subnet-level NSG (user-owned, optional)** — only present if the user attached one to the
   BYO subnet. Gardener never touches it.

(In managed-mode shoots there is only one NSG: the Gardener-owned NSG at the subnet layer. NIC-level
NSG attachment is a BYO-specific behavior.)

Azure evaluates both layers logically as AND — traffic must be allowed by both to pass. For
inbound the subnet NSG is evaluated first, then the NIC NSG. For outbound the order is reversed.
A deny at either layer wins.

If you attach a subnet-level NSG, it must permit at minimum the following:

| Direction | Source        | Destination            | Protocol | Port      | Use                                              |
| --------- | ------------- | ---------------------- | -------- | --------- | ------------------------------------------------ |
| Inbound   | Node CIDR     | Node CIDR              | any      | any       | Node ↔ node                                      |
| Inbound   | Pod CIDR      | Node CIDR              | any      | any       | Service routing (kube-proxy, kubelet health)     |
| Inbound   | Pod CIDR      | Pod CIDR               | any      | any       | Pod ↔ pod (only relevant for flat-CNI shoots)    |
| Inbound   | AzureLoadBalancer service tag | Node CIDR | TCP | any     | LB health probes                                 |
| Outbound  | Node CIDR     | Kubernetes API server  | TCP      | 443, 4443 | Node → apiserver                                 |

For overlay CNIs (Calico with VXLAN, Cilium with VXLAN/Geneve — the Gardener default) the pod↔pod
flow is encapsulated inside node↔node traffic; only the node↔node row is required.

Misconfiguration of the subnet-level NSG (denying a required flow) breaks the cluster; Gardener
has no way to detect this at reconcile time. This is your responsibility.

## Bastion and `Service type=LoadBalancer`

The Azure cloud-controller-manager and the bastion controller mutate the Gardener-owned NIC-level
NSG at runtime — the same behavior every managed Azure shoot has today. Rule additions are
name-scoped and self-cleaning:

- `Service type=LoadBalancer` reconciliation adds allow-rules for the service's frontend IPs and
  ports; deletion removes them. Set the annotation
  `service.beta.kubernetes.io/azure-disable-load-balancer-nsg-rule: "true"` on a service to
  suppress NSG mutation for that specific service (the LB frontend is created but no NSG rules
  are added).
- `Bastion` resources add four rules (2 SSH-in from operator CIDRs, 1 SSH-out to worker CIDRs,
  1 deny-all-out). All rules are named `<bastion-instance-name>-*` and removed when the Bastion is
  deleted.

Because the NSG lives in the shoot's cluster RG, no user-side permission grant is needed for these
mutations — Gardener's Azure principal already has the necessary permissions on the cluster RG.

## Deletion

- The BYO VNet, subnet, route table, and any user-owned subnet-level NSG are never deleted by
  Gardener — they belong to you.
- The Gardener-owned NIC-level NSG in the cluster RG is deleted with the rest of the cluster RG
  (Azure cascade delete). No separate cleanup step is required.
- The observability tag `kubernetes.io/cluster/<technicalName>: shared` is removed from the VNet
  and route table on shoot deletion (best-effort; a failure logs a warning and does not block
  deletion). Other tags on those resources are untouched.
- Named rules added by the CCM (for LB services) and the bastion controller (for Bastion resources)
  on the Gardener NSG die with the NSG itself. Under graceful teardown the CCM/bastion controllers
  remove them first anyway; the cluster-RG cascade catches anything that slips through.
- **Orphan pod-CIDR routes on your route table are NOT cleaned up by Gardener.** In non-overlay
  shoots the seed CCM writes one route per node into your RT. Graceful teardown removes them as
  the CCM observes each Node deletion. Failure modes that leave orphans behind:
    - CCM crash during control-plane teardown.
    - API server becoming unreachable before all Nodes are drained.
    - Force-delete of the shoot (skips graceful teardown entirely).
    - Transient Azure API errors past the CCM's retry budget.

  Any orphan routes are yours to prune. They will point at private IPs that no longer exist (the
  worker VMs are gone with the cluster RG). A future release of this extension is planned to add
  targeted cleanup (snapshot the shoot's NIC IPs at the top of the delete flow, then drop matching
  routes).

  Overlay-CNI shoots are unaffected because the CCM's route controller is disabled and no per-node
  routes are ever written.
