# User-Managed Egress via BYO Subnet

> **Companion document**: an implementation checklist derived from this proposal lives in [`flexible-network-configuration-spec.md`](./flexible-network-configuration-spec.md). This proposal is the source of truth for design intent and acceptance criteria; the spec captures the concrete file paths, code sketches, and testing surface a coding agent (or engineer) needs to implement it.

<!-- toc -->

- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Background: today's egress in `provider-azure`](#background-todays-egress-in-provider-azure)
- [Background: Azure egress patterns](#background-azure-egress-patterns)
- [Proposal](#proposal)
  - [API changes](#api-changes)
  - [Derived mode](#derived-mode)
  - [Validation rules](#validation-rules)
  - [Reconciler behavior](#reconciler-behavior)
  - [Status shape](#status-shape)
  - [Cloud-provider config (`azure.json`)](#cloud-provider-config-azurejson)
  - [`allow-egress` dummy Services](#allow-egress-dummy-services)
  - [Route-controller and overlay-CNI opt-out](#route-controller-and-overlay-cni-opt-out)
  - [NSG mutation contract](#nsg-mutation-contract)
  - [Route-table ownership](#route-table-ownership)
  - [Bastion](#bastion)
- [Configuration patterns](#configuration-patterns)
- [Migration and immutability](#migration-and-immutability)
- [User responsibilities](#user-responsibilities)
- [Deletion / teardown semantics](#deletion--teardown-semantics)
- [Documentation](#documentation)
- [Acceptance criteria](#acceptance-criteria)
- [Risks and upstream conflicts](#risks-and-upstream-conflicts)
- [Alternatives considered](#alternatives-considered)
- [Resolved questions](#resolved-questions)
- [Out of scope (future work)](#out-of-scope-future-work)

<!-- /toc -->

## Summary

Give shoot owners full control over Azure egress by allowing them to bring their own worker **subnet** — pre-attached to their own route table (typically with a `0.0.0.0/0` route to a firewall/NVA, or no default route at all for network-isolated clusters). In this mode Gardener's _infrastructure reconciler_ stops creating and managing the subnet, the route table, and the NAT Gateway, and stops deploying the LB-egress workaround Services. The user pre-provisions the subnet + route table (and optionally a subnet-level NSG); Gardener only _discovers and references_ them.

The network security group is a separate concern. Gardener creates a NSG in the shoot's cluster resource group in every mode. In **managed mode** that NSG is attached to the worker subnet (unchanged from today; existing shoots keep working). In **BYO-subnet mode** the same NSG is instead attached to worker node **NICs** — because the subnet is user-owned and Gardener must not touch it. Either way the NSG is what the Azure CCM manages at runtime for `Service type=LoadBalancer` ingress rules (see [NSG mutation contract](#nsg-mutation-contract)). NIC-level attachment in BYO mode keeps the CCM-managed NSG inside the resource group Gardener owns, so its lifecycle is tied to the cluster RG cascade delete — no orphan cleanup on shoot teardown. A subnet-level NSG owned by the user in BYO mode is optional and independent; if present, it stacks with the Gardener-owned NIC-level NSG per Azure's normal two-layer evaluation.

> [!IMPORTANT]
> **The BYO subnet must not be shared across shoots in v1.** The route table attached to the subnet is on the seed CCM's write path in non-overlay mode (per-node pod-CIDR routes), and Azure only allows one RT association per subnet. Two shoots sharing the subnet therefore share the RT, and their CCMs mutually delete each other's routes. Overlay-CNI shoots don't have this specific writer conflict, but v1 keeps the unique-per-shoot subnet rule uniform across CNI modes to avoid confusion. See [Route-table ownership](#route-table-ownership) for the full options analysis. Lifting the constraint is deferred to a follow-up that either forces overlay in BYO or moves the extension to a route-controller-free routing architecture.

This covers two related egress topologies: (a) a user-owned route table with a `0.0.0.0/0` route to a firewall or virtual appliance, and (b) a user-owned subnet with no default route at all — for network-isolated shoots that terminate all traffic via Private Endpoints. Both are collapsed into a single "user-managed egress" Gardener mode signaled by the presence of a BYO subnet reference.

## Motivation

Today, `provider-azure` locks shoot owners into one of two outbound topologies:

1. NAT Gateway (with optional BYO public IPs), or
2. A Standard Load Balancer forced into existence by two synthetic `allow-{tcp,udp}-egress` `Service type=LoadBalancer` objects in `kube-system` — a workaround for Standard SLB's default-deny-outbound behavior (chart at `charts/internal/shoot-system-components/charts/allow-egress`).

Common enterprise requirements dictate additional options:

- **Central firewall egress** — send all `0.0.0.0/0` from workers to an Azure Firewall / third-party NVA sitting in a hub VNet. Requires a user-owned route table on the worker subnet with a `VirtualAppliance` next hop.
- **No egress at all** — network-isolated / air-gapped shoots that terminate all traffic via Private Endpoints; no public IP anywhere.
- **BYO subnet inside a BYO VNet** — many enterprises pre-provision every subnet through their platform team and hand the shoot owner a subnet ID.

### Goals

- Support the two "user-managed egress" topologies described in the Summary — firewall-based egress via a user-owned route table with a default route to a virtual appliance, and no-egress (network-isolated) shoots with no default route.
- Support **BYO subnet** in the single-subnet (non-zoned) layout, requiring BYO VNet.
- Skip creation of the route table, NAT Gateway, and the LB-egress dummy Services when the user has taken over egress. The worker NSG creation is unchanged in both modes — Gardener continues to create a NSG in the shoot's cluster RG (see [NSG mutation contract](#nsg-mutation-contract)). Only its attachment point differs: subnet-level in managed mode, NIC-level in BYO-subnet mode.
- Zero breaking changes for existing shoots: opting in is purely additive.

### Non-Goals

- **No new `OutboundType` enum.** Per the design decision, mode is derived from BYO field presence.
- **No BYO subnet in the multi-subnet (zoned) layout** in v1. `Networks.Zones[i].Subnet` is deferred.
- **No BYO NAT Gateway** (the resource) in v1. Users needing a pre-existing NAT Gateway can attach it to their BYO subnet themselves; Gardener will not create or reference one in BYO-subnet mode.
- **No BYO Route Table as a separate API field.** The route table is discovered from the subnet's existing `RouteTable` association; the user attaches it out-of-band.
- **No BYO NSG.** Gardener always creates and owns the CCM-facing NSG, in every mode. In BYO-subnet mode the same NSG creation logic runs; the resulting NSG lives in the shoot's cluster resource group and is attached to worker node NICs (rather than to the user's subnet, which Gardener must not touch). In managed mode the NSG attachment is unchanged (subnet-level). See [NSG mutation contract](#nsg-mutation-contract) for the rationale (short version: the CCM's `RetainSecurityGroup` behavior forbids sharing a single NSG across clusters, so we need per-shoot NSG ownership; NIC-level attachment gives us that in BYO where we can't attach to the user's subnet). A user-owned NSG on the subnet in BYO mode is optional and independent — Gardener neither creates nor touches it.
- **No shared BYO subnet across shoots in v1.** The route table is on the CCM's write path in non-overlay mode and Azure supports only one RT association per subnet, so shared subnet = shared RT = writer conflict. See [Route-table ownership](#route-table-ownership) for the options analysis. Deferred lifting is future work.
- **No cleanup of orphan routes on shoot deletion in v1.** If the CCM has written per-node pod-CIDR routes into the user's RT and the graceful teardown fails to remove them (CCM crash, force-delete, transient Azure API errors), those routes remain in the user's RT pointing at IPs that no longer exist ("blackhole" routes). Targeted cleanup via a snapshot-and-filter approach (as in `provider-aws` / `provider-gcp`) is [future work](#out-of-scope-future-work).

## Background: today's egress in `provider-azure`

- `InfrastructureConfig` API: `pkg/apis/azure/types_infrastructure.go`. BYO surface today = VNet (`VNet.Name`+`VNet.ResourceGroup`), user-assigned managed identity, DDoS plan, and public IPs consumed by a Gardener-managed NAT Gateway.
- Reconciler is flow-based: `pkg/controller/infrastructure/infraflow/flow_context.go:150-177`. Task graph creates RG → VNet → Identity → RouteTable → NSG → PublicIPs → NatGateways → Subnets. The route table (`worker_route_table`) is created **empty**; the seed-hosted CCM populates per-node pod-CIDR routes via `--configure-cloud-routes=true` (chart `charts/internal/seed-controlplane/charts/cloud-controller-manager/templates/cloud-controller-manager.yaml:48`).
- `cloud-provider-config` chart hard-codes `loadBalancerSku=standard` (`charts/internal/cloud-provider-config/templates/cloud-provider-config.tpl:12`). Neither `outboundType` nor `disableOutboundSNAT` is ever set.
- Egress via SLB works today only because of the two `allow-{tcp,udp}-egress` Services pinned into `kube-system` (deployment gated at `pkg/controller/controlplane/valuesprovider.go:766-771`; opt-out annotation `azure.provider.extensions.gardener.cloud/skip-allow-egress`).
- Existing derived status field `InfrastructureStatus.Networks.OutboundAccessType` currently takes `NATGateway` | `LoadBalancer`. This proposal adds a third value.

## Background: Azure egress patterns

Azure workloads inside a VNet can reach the internet through several distinct topologies. From the standpoint of the shoot's infrastructure, they are:

| Egress path                                                                     | Automation creates            | User provides                                             |
| ------------------------------------------------------------------------------- | ----------------------------- | --------------------------------------------------------- |
| Standard Load Balancer (frontend PIP + outbound rule)                           | LB + PIP + backend pool       | —                                                         |
| NAT Gateway (`Standard` or `StandardV2`, zone-redundant) attached to the subnet | NAT + PIP, attached to subnet | —                                                         |
| Pre-existing NAT Gateway attached to a pre-existing subnet                      | nothing                       | BYO VNet + subnet + NAT (attached to subnet)              |
| Route table on the subnet with a `0.0.0.0/0` route to a firewall / NVA          | nothing                       | BYO VNet + subnet + user route table with a default route |
| No egress infrastructure at all (network-isolated)                              | nothing                       | BYO VNet + subnet; no default route required              |

Critical fact confirmed from upstream `kubernetes-sigs/cloud-provider-azure` source: the CCM has **zero awareness of any high-level "outbound type" abstraction** — no such field is read from `azure.json`. The CCM only creates LB + PIP reactively for `Service type=LoadBalancer`, never proactively at startup. Any outbound-topology decisions must be made by whatever provisions the subnet, route table, NAT, and load balancer — i.e. Gardener's infrastructure reconciler. The only related knob the CCM does honor is `disableOutboundSNAT` (per-`LoadBalancingRule` flag; it should be set to `true` whenever the SLB is not the intended egress path).

Consequence for Gardener: implementing user-managed egress is entirely a matter of what Gardener chooses **not** to create at reconcile time, plus a small `azure.json` tweak (`disableOutboundSNAT: true`) plus deciding whether to skip the allow-egress dummy Services.

## Proposal

### API changes

Two additions on `InfrastructureConfig.Networks`:

1. **`Subnet` — an optional reference to an existing subnet inside the (also user-provided) VNet.** When set, Gardener switches into user-managed-egress mode. The reference carries only a subnet name, not a resource group — in ARM, a subnet is a child resource of its VNet, so the VNet's resource group (already available on `Networks.VNet.ResourceGroup`) plus VNet name plus subnet name uniquely identify it.
2. **`Subnet.SkipRouteReconciliation` — an optional boolean, default `false`.** When `true`, the seed CCM's route controller is disabled and Gardener does not require the BYO subnet to have a route table attached. Intended for shoots using an overlay CNI (Cilium/Calico with VXLAN or Geneve) where pod-CIDR routes are not needed in the underlying VNet.

No new mode/enum is added. Presence of `Networks.Subnet` alone signals user-managed-egress mode (see [Derived mode](#derived-mode)).

One status-only addition: the existing `OutboundAccessType` enum on `InfrastructureStatus.Networks` gains a third value **`UserManaged`**, set when the user brought their own subnet and did not enable a NAT Gateway. The API user does not set this; it is derived at reconcile time.

Concrete Go type definitions are in the companion spec document — see [`flexible-network-configuration-spec.md`](./flexible-network-configuration-spec.md).

### Derived mode

The mode is signaled solely by the presence of `Networks.Subnet`. No enum, no marker field, no annotation. The reconciler branches on `Networks.Subnet != nil`; the resulting status carries `OutboundAccessType=UserManaged` and downstream code (allow-egress gating, azure.json field emission, etc.) branches on the status value.

### Validation rules

Added in `pkg/apis/azure/validation/infrastructure.go`.

When `Networks.Subnet != nil`:

| Rule                                                                    | Rationale                                                                                                      |
| ----------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| `Networks.VNet.Name` and `Networks.VNet.ResourceGroup` must both be set | User-managed egress requires a BYO VNet — Gardener cannot own the VNet if it doesn't own the subnet inside it. |
| `Networks.VNet.CIDR` must be unset                                      | BYO VNet — CIDR is discovered, not declared.                                                                   |
| `Networks.VNet.DDosProtectionPlanID` must be unset                      | BYO VNet — user manages the plan on their side.                                                                |
| `Networks.Workers` must be unset                                        | Workers CIDR is discovered from the actual subnet.                                                             |
| `Networks.Zones` must be empty                                          | Single-subnet layout only in v1.                                                                               |
| `Networks.NatGateway` must be nil                                       | Gardener does not manage a NAT in this mode; if the user wants one, they attach it to their subnet themselves. |
| `Networks.ServiceEndpoints` must be empty                               | User manages service endpoints on their own subnet.                                                            |
| `Networks.Subnet.Name` non-empty (RFC-1123-ish per Azure naming rules)  | Standard field validation.                                                                                     |

**Runtime validation** — a new pre-flight validator runs before reconcile and checks the referenced ARM resources actually exist and are compatible. It complements the API-level rules above; API-level rules catch static mistakes at admission time, runtime rules catch mismatches with the live cloud state:

| Rule                                                                                                           | Rationale                                                                                                                                                                                                                   |
| -------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The referenced subnet must exist in the BYO VNet                                                               | Fail fast with a clear error.                                                                                                                                                                                               |
| The subnet must have a `RouteTable` association (`subnet.properties.routeTable.id != nil`), unless `Networks.Subnet.SkipRouteReconciliation=true` | The seed CCM runs the route controller by default and needs somewhere to write per-node pod-CIDR routes. Relaxed for overlay CNI shoots that opt out (see [Route-controller and overlay-CNI opt-out](#route-controller-and-overlay-cni-opt-out)).                                                                                                                                                                              |
| The subnet's CIDR must be a subset of `shoot.spec.networking.nodes` and non-overlapping with pods/services     | Same rule that validates managed subnets today.                                                                                                                                                                             |
| The discovered route table must be in the same subscription as the shoot                                       | Cross-subscription references aren't supported by CCM's `azure.json`. There is **no** cluster-RG-or-VNet-RG constraint: any RG in the subscription is fine (see [Cloud-provider config](#cloud-provider-config-azurejson)). |

The subnet is _permitted_ to have a `NetworkSecurityGroup` association pointing at a user-owned NSG. Gardener does not require it, does not read it, and does not touch it. If the user attaches one, they take responsibility for permitting the traffic flows Kubernetes requires — see [Configuration patterns](#configuration-patterns) for the specifics.

**Immutability** (`ValidateInfrastructureConfigUpdate`):

- `Networks.Subnet.Name` is immutable once set.
- Transitioning to or from user-managed-egress mode after cluster creation is **forbidden** in v1 (i.e. `Networks.Subnet` cannot be added or removed on an existing shoot). Such a transition would recreate/delete route tables, NSGs, NAT gateways, and PIPs while workloads run — high operational blast radius (egress IPs change, connections drop). Deferred to a follow-up.

### Reconciler behavior

Modifications live in `pkg/controller/infrastructure/infraflow/`.

**Task-gating matrix**:

| Task                    | Condition                                                                                                                                |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| `EnsureResourceGroup`   | unchanged                                                                                                                                |
| `EnsureVirtualNetwork`  | unchanged (BYO VNet path already exists)                                                                                                 |
| `EnsureManagedIdentity` | unchanged                                                                                                                                |
| `EnsureRouteTable`      | **skip if `IsUsingUserManagedEgress()`**                                                                                                 |
| `EnsureSecurityGroup`   | **unchanged** — runs in every mode, creates the NSG in the shoot's cluster RG. In managed mode the NSG is attached to the subnet by `EnsureSubnets` (unchanged from today). In BYO mode the NSG is not attached to the subnet (Gardener does not touch the user's subnet); it is attached to worker NICs by MCM via the machine class. See [NSG mutation contract](#nsg-mutation-contract). |
| `EnsurePublicIps`       | already naturally skipped (no NAT configured)                                                                                            |
| `EnsureNatGateways`     | already naturally skipped                                                                                                                |
| `EnsureSubnets`         | **new branch: `EnsureUserSubnet`** — verify existence, fetch RT ID/name, populate whiteboard; do NOT patch any subnet properties         |
| `EnsureBYOResourceTags` | **new, BYO-only** — best-effort tag add on VNet and RT for human observability (see [metadata-tags subsection below](#reconciler-behavior))                    |

The flow-level gating:

```mermaid
flowchart TD
    Cfg[InfrastructureConfig] --> Ck{Networks.Subnet<br/>set?}
    Ck -->|No — managed mode| M1[EnsureResourceGroup]
    M1 --> M2[EnsureVirtualNetwork]
    M2 --> M3[EnsureManagedIdentity]
    M3 --> M4[EnsureRouteTable]
    M4 --> M5[EnsureSecurityGroup]
    M5 --> M6[EnsurePublicIps]
    M6 --> M7[EnsureNatGateways]
    M7 --> M8[EnsureSubnets]
    M8 --> MZ[Status: NATGateway or LoadBalancer]

    Ck -->|Yes — user-managed egress| B1[EnsureResourceGroup]
    B1 --> B2[EnsureVirtualNetwork<br/>BYO path — verify only]
    B2 --> B3[EnsureManagedIdentity]
    B3 --> B4[EnsureSecurityGroup<br/>NIC-level NSG in cluster RG]
    B4 --> BSk[/skip: RouteTable,<br/>PublicIps, NatGateways/]
    BSk --> BUS[EnsureUserSubnet<br/>read-only discovery of RT]
    BUS --> BT[EnsureBYOResourceTags<br/>best-effort tag on VNet/RT]
    BT --> BZ[Status: UserManaged<br/>+ discovered RT name+RG<br/>+ Gardener NSG name in cluster RG]
```

**New reconciler task** `EnsureUserSubnet` — active only in BYO mode. It:

1. Reads the referenced subnet from Azure.
2. Verifies the subnet exists in the BYO VNet, that its CIDR is compatible with the shoot's networking config, and that it has a route table attached unless `SkipRouteReconciliation=true`.
3. Parses the route table ARM ID into `(resourceGroup, name)` and stores it for status emission and `azure.json` rendering.
4. Never issues a `PUT`/`PATCH` on the subnet itself — the discovery is read-only. The Gardener-owned NSG created by `EnsureSecurityGroup` is _not_ associated with the subnet in this mode; it lives in the shoot's cluster RG and is attached to worker NICs directly by MCM (see [NSG mutation contract](#nsg-mutation-contract)).

**Metadata tags on the BYO resources.** For human observability, the reconciler adds a single tag to the BYO VNet and route table:

```
kubernetes.io/cluster/<shoot-technical-name> = shared
```

Standard cloud-provider convention. `shared` signals that the resource is BYO (shared with the shoot, not owned by it). Multiple shoots on the same resource each add their own key. On shoot deletion, only this shoot's key is removed; other tags are untouched. The Gardener-owned NSG in the cluster RG does not receive this tag — it's owned end-to-end by the shoot's cluster RG and dies with the cascade delete.

Tag operations are **best-effort**: if the Azure principal lacks tag-write permission on either resource (e.g. under Azure Policy locking), the reconciler logs a warning and continues. Tags are informational only — no Gardener code reads them back for correctness.

Implementation: a new task `EnsureBYOResourceTags` runs after `EnsureUserSubnet` in the reconcile flow, and a corresponding `RemoveBYOResourceTags` runs in the delete flow. Both merge into the resource's existing tags (never replace) and only manage the one shoot-scoped key.

### Status shape

The `RouteTable` status type gains an optional `ResourceGroup` field — additive and backward-compatible (nil in existing managed shoots, where the RT lives in the cluster RG by convention). Only populated in BYO-subnet mode, where the RT may live in any RG within the shoot's subscription. The `SecurityGroup` status type keeps its shape; `ResourceGroup` remains nil in both managed and BYO modes because the NSG is always in the shoot's cluster RG (that's the point of the NIC-level ownership model).

`InfrastructureStatus.Networks` mapping in BYO-subnet mode:

| Status field                         | In BYO-subnet mode                                                                                           |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------ |
| `Networks.VNet.{Name,ResourceGroup}` | user-provided values, unchanged                                                                              |
| `Networks.Subnets[]`                 | one entry: `{Purpose: PurposeNodes, Name: <BYO subnet name>, Zone: nil, Migrated: false, NatGatewayID: nil}` |
| `Networks.Layout`                    | `SingleSubnet`                                                                                               |
| `Networks.OutboundAccessType`        | **new value** `UserManaged`                                                                                  |
| `RouteTables[]`                      | one entry with the discovered RT `Name` + `ResourceGroup`; **omitted entirely** if `SkipRouteReconciliation=true` and no RT is attached |
| `SecurityGroups[]`                   | one entry `{Purpose: PurposeNodes, Name: <Gardener-created NSG>, ResourceGroup: nil}`. Same shape as managed mode. Consumers (worker/machine-class rendering, bastion, `azure.json`) read the ARM resource ID from cluster RG + this name. |
| `EgressCIDRs`                        | **nil** (Gardener has no knowledge of the user's egress IPs)                                                 |

The last point matters: today `shoot.status.provider.egressCIDRs` is populated from the NAT Gateway's public IPs. In user-managed egress mode, Gardener has no reliable way to know what IPs the user's firewall / NVA egresses through. Downstream consumers that rely on it must handle nil; document that reliably knowing egress IPs in this mode requires out-of-band information from the user.

### Cloud-provider config (`azure.json`)

Three concerns for how Gardener renders the shoot's `azure.json`:

1. **Existing fields already work.** `resourceGroup`, `vnetName`, `vnetResourceGroup`, `subnetName`, `routeTableName`, `securityGroupName` are already sourced from `InfrastructureStatus`; once the status is populated correctly in BYO mode, they pick up the correct values automatically. `securityGroupName` in particular points at the Gardener-created NIC-level NSG in the cluster RG — same shape as managed mode.
2. **Resource-group overrides.** When the discovered route table lives in an RG other than the shoot's cluster RG, `azure.json` must additionally emit the upstream CCM field `routeTableResourceGroup`. It defaults to `resourceGroup` if unset, so today's managed-mode shoots remain unchanged. Emitting it lets the RT live in **any resource group in the same subscription** as the shoot. `securityGroupResourceGroup` is not emitted — the NSG is always in the shoot's cluster RG.
3. **Outbound SNAT suppression.** When `OutboundAccessType == UserManaged`, `azure.json` sets `disableOutboundSNAT: true`. This is a cluster-wide instruction to the upstream CCM: every `Service type=LoadBalancer` reconcile will produce LB rules with `DisableOutboundSnat=true`, ensuring the LB frontend never becomes an accidental egress path bypassing the user's route table.

A fourth concern applies in the [route-controller opt-out](#route-controller-and-overlay-cni-opt-out) case: the seed-controlplane CCM's `--configure-cloud-routes` flag becomes `false`. This is a separate lever from `azure.json` — it's a CCM command-line flag rendered by the seed-controlplane chart.

### `allow-egress` dummy Services

The gating in `deployAllowEgressChart` is extended: the two dummy Services are deployed only when `OutboundAccessType == LoadBalancer`. In `UserManaged` mode they are skipped. In `NATGateway` mode they were already skipped by the existing logic. The pre-existing `skip-allow-egress` shoot annotation still overrides the gating for advanced LB-mode users who want to manage egress themselves.

### Route-controller and overlay-CNI opt-out

There are two operating modes for pod-CIDR routing on Azure:

**Default (route-controller enabled)** — the seed CCM runs with `--configure-cloud-routes=true` and writes per-node pod-CIDR routes into the route table associated with the worker subnet. This is what today's shoots do; it matches the historic kubenet-style routing model. In BYO-subnet mode the CCM writes those routes into the user's route table, which has three implications the user must accept:

1. The user must not run competing automation (Terraform / policy engines) against the route table that reverts CCM's per-node routes.
2. Every node created by MCM results in a new route in the route table. Very large clusters may approach the Azure route-table limit (400 routes per table, extensible to 1000 via a support case).
3. The user's default route (`0.0.0.0/0` to firewall / NVA) coexists peacefully with per-node routes because the per-node routes are more specific (pod CIDR /24). No conflict.

**Overlay-CNI (route-controller disabled)** — shoots using an overlay CNI (Cilium with VXLAN or Geneve, Calico with VXLAN, etc.) do not need pod-CIDR routes in the underlying VNet: pod-to-pod traffic is encapsulated at the node level. For those shoots Gardener should be able to leave the user's route table completely untouched.

The proposal introduces an opt-out signal that turns the route controller off:

- A new optional field, `Networks.Subnet.SkipRouteReconciliation *bool` (default `false` — preserves today's behavior).
- When `true`, the seed CCM runs with `--configure-cloud-routes=false` and the reconciler does not require the BYO subnet to have a route table attached at all. If a route table is attached, Gardener still discovers and references it in `azure.json` (for consistency and for the CCM's LB code paths that read route-table state), but no per-node routes are written.
- Validation adjusts accordingly: the "subnet must have a `RouteTable` association" rule from [Validation rules](#validation-rules) is relaxed when `SkipRouteReconciliation=true`.

The user takes ownership of using an overlay CNI. Gardener does not introspect the shoot's `networking.type` or the CNI's configuration to auto-derive this — the field is an explicit opt-in with clear semantics, and misconfiguration (setting it `true` while using a non-overlay CNI) is the user's responsibility.

### NSG mutation contract

**Who owns the NSG.** Gardener owns exactly one NSG per shoot — created by `EnsureSecurityGroup` in the shoot's cluster resource group. This is the CCM-facing NSG (`securityGroupName` in `azure.json`). Its **attachment point** depends on the mode:

- **Managed mode**: attached to the worker subnet (`Subnet.Properties.NetworkSecurityGroup` — set by `EnsureSubnets`, unchanged from today).
- **BYO-subnet mode**: attached to every worker NIC by MCM at machine creation time (via the `securityGroupID` field on the machine class network config; see `machine-controller-manager-provider-azure`). Never attached to the subnet — the subnet is user-owned and Gardener must not touch it.

Rationale for the BYO-side NIC attachment (vs. attaching to the user's subnet):

1. **Ownership boundary matches the resource-group boundary.** The NSG lives inside the shoot's cluster RG, which Gardener owns end-to-end. Cluster-RG cascade delete on shoot teardown wipes the NSG cleanly. No BYO-mode orphan cleanup problem for the NSG.
2. **Sharing is impossible by construction.** Every shoot has its own NSG in its own cluster RG. Two shoots sharing the user's BYO subnet each still have a distinct NSG on their own NICs. The upstream CCM's `RetainSecurityGroup` behavior (`pkg/provider/azure_loadbalancer.go:3520` → `RetainDestinationFromRules` in the `securitygroup` package) iterates every managed rule on the NSG and strips destination IPs it does not recognize as belonging to _this_ cluster. On a shared NSG this pattern mutually destroys rules across clusters. Per-shoot NSG ownership avoids that class of failure entirely.

**Why not migrate managed mode to NIC attachment too?** It's a rolling change on every running shoot: the machine class would change, but because MCM does not reconcile the NIC of an already-running machine, we would end up with a fleet split (existing machines still with subnet-only evaluation, new machines with subnet + NIC evaluation) that is unsafe to reason about without a controlled full-fleet rollout. That migration is deferred to a follow-up if and when Gardener chooses to converge managed mode to NIC-level attachment. Until then, managed mode stays subnet-attached.

**What the infrastructure reconciler does in BYO-subnet mode**: creates the NSG in the cluster RG (unchanged from managed mode). Emits its name in `InfrastructureStatus.SecurityGroups[0]`. Does not attach it to the subnet. Does not touch any subnet-level NSG the user may have attached themselves. Deletes it on shoot teardown (via cluster-RG cascade delete; no per-resource delete step).

**What the Azure CCM does to the NSG at runtime** — implemented in upstream `reconcileSecurityGroup` at `pkg/provider/azure_loadbalancer.go:3401`, invoked from `EnsureLoadBalancer` (`:163`) and `EnsureLoadBalancerDeleted` (`:482`). For each `Service type=LoadBalancer` reconcile:

| Step | Operation                                     | Upstream ref | When                                                               | Effect on NSG                                                                                                                                                                    |
| ---- | --------------------------------------------- | ------------ | ------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1    | `nsgRepo.GetSecurityGroup(ctx)`               | `:3424`      | always                                                             | Read-only: `GET` on the NSG in `securityGroupResourceGroup` (or falls back to `resourceGroup`).                                                                                  |
| 2    | `accessControl.CleanSecurityGroup(...)`       | `:3491`      | Service delete only                                                | Drops the rules previously added for this Service (name-matched by Service hash).                                                                                                |
| 3    | `accessControl.PatchSecurityGroup(...)`       | `:3498`      | Service create/update, iff `!disableLoadBalancerNSGRule` (`:3497`) | Adds allow-rule: source = `Internet` service tag or `spec.loadBalancerSourceRanges`; destination = LB frontend IPs (or backend IPs if `disableFloatingIP`); port = Service port. |
| 4    | `accessControl.RetainSecurityGroup(...)`      | `:3520`      | Service create/update                                              | Sweeps stale CCM-authored rules whose target destinations no longer exist. Safe because the NSG is exclusive to this cluster.                                                    |
| 5    | `ensureSecurityGroupTagged(rv)`               | `:3532`      | always                                                             | Adds cluster tags to the NSG iff `az.Tags`/`az.TagsMap` are set. **No-op for Gardener** — Gardener does not populate `Tags`/`TagsMap` in `cloud-provider-config`.                |
| 6    | `nsgRepo.CreateOrUpdateSecurityGroup(ctx,rv)` | `:3539`      | if steps 2–5 mutated the NSG                                       | Writes the updated NSG back to ARM.                                                                                                                                              |

**What the bastion controller does to the NSG**: for each Bastion resource lifecycle, adds/removes 4 IP-scoped rules on the same NSG — 2 SSH-in from operator CIDRs, 1 SSH-out to worker CIDRs, and 1 deny-all-out that ensures the bastion has no internet egress. Rule names are prefixed `<bastion-instance-name>-` and removed on Bastion delete.

Both flows are additive, name-scoped, and self-cleaning. They never touch the user's subnet-level NSG (if any) — that's a different NSG at a different Azure layer.

**Optional user-owned NSG at the subnet layer**: in BYO-subnet mode, the user is free to attach their own NSG to the BYO subnet. Gardener does not create, read, mutate, or delete it. Both the subnet-level NSG and Gardener's NIC-level NSG are evaluated by Azure for every packet — subnet-first for inbound, NIC-first for outbound. This is standard Azure two-layer NSG evaluation and requires no coordination between the layers. (In managed mode there is only one Gardener NSG, attached at the subnet layer; a second user-owned NSG at the subnet layer is not possible since a subnet supports at most one `NetworkSecurityGroup` reference.)

If the user does attach a subnet-level NSG, they must permit the flows Kubernetes needs:

| Destination                           | Source                | Protocol      | Port      | Use                                                            |
| ------------------------------------- | --------------------- | ------------- | --------- | -------------------------------------------------------------- |
| Kubernetes API server (seed-hosted)   | Cluster subnet CIDR   | TCP           | 443       | Node → apiserver                                               |
| Node CIDR                             | Node CIDR             | any           | any       | Node ↔ node                                                    |
| Node CIDR                             | Pod CIDR              | any           | any       | Service traffic (kube-proxy / kube-apiserver / kubelet health) |
| Pod CIDR                              | Pod CIDR              | any           | any       | Pod ↔ pod (only relevant if flat CNI is used)                  |

For overlay CNIs (Calico with VXLAN, Cilium with VXLAN/Geneve — the Gardener default) the pod↔pod flow is encapsulated inside node↔node traffic; only the node↔node row is required in that case. See [Route-controller and overlay-CNI opt-out](#route-controller-and-overlay-cni-opt-out) for the CNI-mode discussion.

Misconfiguration of the user's subnet-level NSG (denying required flows) breaks the cluster; Gardener has no way to detect this at reconcile time. This is the user's responsibility.

**Escape hatch for CCM NSG mutation** — the annotation `service.beta.kubernetes.io/azure-disable-load-balancer-nsg-rule: "true"` on a `Service type=LoadBalancer`:

- Skips step 3 (`PatchSecurityGroup`) — no rules added for that Service.
- Steps 4, 5, 6 still run, but each is a no-op when nothing has been added:
  - `RetainSecurityGroup` only removes stale CCM-authored rules; if we never added any, none to remove.
  - `ensureSecurityGroupTagged` is already a no-op for Gardener.
  - `CreateOrUpdateSecurityGroup` is only invoked if steps 2–5 mutated the NSG.
- If **every** LB Service in the shoot carries the annotation, the CCM's NSG interaction becomes read-only (one `GET` per reconcile; zero writes).

There is no cluster-wide upstream equivalent. If we later want to force this cluster-wide (e.g. for shoots whose NSGs are hard-locked by Azure Policy), a mutating webhook that stamps the annotation on every LB Service would suffice — deferred as future work.

**Permission implication**: Gardener's Azure principal needs `Microsoft.Network/networkSecurityGroups/*` on the shoot's cluster resource group. This is the same permission set that managed mode already requires; no additional user-side privilege grants are needed for BYO mode.

### Route-table ownership

The route table is fundamentally different from the NSG. Azure allows exactly one `RouteTable` association per subnet, and the subnet is user-owned in BYO mode, so we cannot use the "put it in our cluster RG and attach elsewhere" trick that solved the NSG. Any RT that lives on the BYO subnet is user-attached and Gardener cannot substitute a shadow RT of its own. That constraint drives everything else in this section.

The seed CCM's route controller writes one pod-CIDR route per node into the subnet's RT in non-overlay CNI shoots (`--configure-cloud-routes=true`, kubenet-style). In overlay CNI shoots (Calico or Cilium with VXLAN/Geneve — the Gardener default) the route controller is disabled and no CCM writes happen to the RT.

Three ownership models were considered:

1. **Gardener creates and owns the RT in cluster RG, attaches it to the user's subnet.** Rejected. The user cannot then place their own routes on the RT (e.g. `0.0.0.0/0 → Azure Firewall`) without either (a) sharing write access to a resource Gardener claims to own, or (b) racing with Gardener's per-node route writes on every reconcile. The whole point of BYO is that the user drives egress. This option defeats it.
2. **Gardener does not manage the RT; the user brings it, attached to the subnet before shoot creation.** Chosen. The RT is entirely user-owned. Gardener only _discovers_ its name and resource group (from `subnet.properties.routeTable.id`) at reconcile time and threads them into `azure.json` so the seed CCM knows where to write per-node routes.
3. **Force overlay CNI in BYO — reject `overlay.enabled: false` at admission time.** Considered and rejected for v1. It would sidestep the RT question entirely (no CCM writes → RT is untouched → RT can be shared safely) but it removes non-overlay CNI shoots as a first-class use case. Some users legitimately want kubenet-style flat/UDR routing for observability or interop reasons. Kept open as a future opt-in tightening if we discover the shared-RT footgun in the wild is bad enough.

Consequences of option 2 (chosen):

- **The RT is on the seed CCM's write path** in non-overlay shoots. Two shoots writing to the same RT contend on route names (`<nodeName>` or `<nodeName>-<cidr>`) and prune each other's entries on every reconcile (same mutual-destruction shape as the NSG `RetainSecurityGroup` case).
- **Since the RT is 1:1 with the subnet in Azure**, "no shared RT across shoots" implies "**no shared subnet across shoots**" for non-overlay shoots. In overlay shoots the CCM does not write to the RT, so subnet sharing would be technically safe — but v1 keeps the constraint uniform across CNI modes to avoid a subtle mode-dependent rule that users can trip on if they toggle overlay later.
- **Enforcement is docs-only in v1**, not admission-time. Cross-shoot inspection ("is this subnet already referenced by another Azure shoot?") is not a pattern this extension uses today; adding it is a bigger lift than it looks (race conditions between concurrent shoot creations, coverage across seeds, etc.). Doc surface is [User responsibilities](#user-responsibilities) and [Configuration patterns](#configuration-patterns).
- **On shoot deletion, orphan routes are left behind if the CCM's graceful teardown fails to clean them up.** Targeted cleanup via IP-snapshot-and-filter (as in `provider-aws` / `provider-gcp`) is [out of scope for v1](#out-of-scope-future-work).

**Note on user-owned NSG at subnet layer under this constraint.** Because BYO subnets are already unique-per-shoot in v1 (as a consequence of the RT rule above), a user _could_ also attach a per-shoot subnet-level NSG. Nothing in this design requires or prevents that; it's the user's choice. See the "Optional user-owned NSG" paragraph in [NSG mutation contract](#nsg-mutation-contract).

**Permission implication**: Gardener's Azure principal needs at minimum `Microsoft.Network/routeTables/read` on the user's RG hosting the RT, plus the write-path permissions that the seed CCM requires to write per-node routes (typically `Microsoft.Network/routeTables/routes/write`). In overlay-CNI shoots, only read is needed.

### Bastion

Bastion is **fully supported** in BYO-subnet mode. No new API surface is required.

The bastion controller reads the worker subnet from `InfrastructureStatus.Networks.Subnets[0]` and the NSG name from `InfrastructureStatus.SecurityGroups[0]`. Both are populated uniformly across modes; only the attachment point of the NSG differs (subnet-level in managed mode, NIC-level in BYO). Bastion adds/removes its four IP-scoped rules on that NSG the same way in both modes.

**Bastion's NSG mutation footprint** — four IP-scoped rules per bastion (see [NSG mutation contract](#nsg-mutation-contract)). The bastion has no internet egress by design (its own NSG rules include a deny-all-outbound), so no firewall allowlisting is required on the user's side for bastion egress.

**Bastion resource placement**: VM, disk, NIC, and the bastion's public IP are created in the shoot's cluster RG (unchanged). The NIC lives inside the BYO worker subnet. **Follow-up work**: in BYO mode the bastion's own NIC should also carry the Gardener NSG at the NIC layer (same rationale as worker NICs — the user's subnet is not Gardener-owned). The bastion controller today does not set an NSG on its NIC; the CCM-authored rules on the shoot NSG therefore currently rely on evaluation via the user's subnet NSG (if any). Tracked as a follow-up commit in the same PR series.

## Configuration patterns

### Pattern 1 — Firewall-based egress

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

User pre-provisions:

- The VNet `hub-spoke-workers-vnet` in `platform-network-rg`.
- A subnet `shoot-a-workers` inside that VNet.
- A route table attached to the subnet with a `0.0.0.0/0` route pointing to their Azure Firewall / NVA (next-hop = `VirtualAppliance` + firewall IP).
- **Optional**: an NSG attached to the subnet. If present, it must permit the flows listed in [NSG mutation contract](#nsg-mutation-contract). Gardener does not create, read, or mutate this NSG.
- Any firewall rules needed to reach the Azure endpoints required for a Kubernetes cluster to function — at minimum the `AzureCloud` service tag (ARM, IMDS, storage), the `AzureContainerRegistry` service tag, and the public MCR endpoint (`mcr.microsoft.com` and its CDN backends).

Gardener will:

- Verify the VNet, subnet, and route-table associations exist.
- Skip creating a route table, NAT, LB, or allow-egress services.
- Create the shoot NSG in the shoot's cluster RG (same as managed mode) and attach it to worker node NICs via the MCM machine class (BYO-only; managed mode continues to attach the NSG to the subnet).
- Discover the RT association from the subnet at reconcile time and emit `cloud-provider-config` with `subnetName`, `routeTableName` from the BYO resources, `securityGroupName` from the Gardener-created NSG, `vnetResourceGroup` from the BYO VNet, plus `disableOutboundSNAT: true`.
- Continue to allow the CCM and bastion controller to add/remove narrowly-scoped rules on Gardener's NSG at runtime (see [NSG mutation contract](#nsg-mutation-contract)).

### Pattern 2 — No egress (network-isolated)

Identical `InfrastructureConfig` to Pattern 1. The user simply doesn't put a `0.0.0.0/0` route in their route table. Gardener cannot and does not validate the absence — that's the user's choice.

Prerequisites for this to actually work as a cluster:

- Private Endpoints for the API server (Gardener's private-cluster feature).
- Private Endpoints or Service Endpoints for any Azure services the workload needs.
- No dependency on public MCR, no dependency on public NTP, etc.

Documented as a supported topology; the user takes ownership of making the cluster viable.

### Pattern 3 — Managed VNet + managed NAT + managed subnet (unchanged)

Today's default. `Networks.Subnet` unset. Everything works exactly as before. Zero migration required for existing shoots.

## Migration and immutability

- **Existing shoots** keep working. `Networks.Subnet` is `+optional`.
- **New shoots** may opt in at creation time. Opting in requires setting `Networks.VNet.{Name,ResourceGroup}` (already supported).
- **In-place transition**: forbidden in v1. Once created with `Networks.Subnet` set, it must stay set; once created without, it must stay unset. Enforced in `ValidateInfrastructureConfigUpdate` at `pkg/apis/azure/validation/infrastructure.go:469-513`. Rationale: transitioning between managed and user-managed egress mid-flight means recreating/deleting the route table + NSG + NAT + PIPs while workloads are running — disruptive, error-prone, and unnecessary for the initial delivery. Users who need to migrate can create a fresh shoot.
- **`Networks.Subnet.Name` immutable** once set.

## User responsibilities

Explicitly documented in `docs/usage/`:

The user MUST provide, before shoot creation:

1. A VNet.
2. A subnet inside that VNet, whose CIDR fits inside `shoot.spec.networking.nodes` and does not overlap `shoot.spec.networking.{pods,services}`. **The subnet must not be reused across shoots** — see the [Route-table ownership](#route-table-ownership) section for the rationale. Concretely: no other Gardener Azure shoot may reference this subnet in its own `InfrastructureConfig.Networks.Subnet.Name`.
3. A **route table** attached to that subnet, unique to this shoot (not reused by any other shoot's subnet). For firewall-based egress usage, this route table should contain a `0.0.0.0/0` route to the user's firewall / NVA. For network-isolated usage, it may be empty (Gardener will still write pod-CIDR routes into it in non-overlay shoots). In overlay-CNI shoots (Gardener default), the route table may be omitted entirely.
4. Any firewall rules required for Azure control-plane traffic and container image pulls.
5. The route table must be in the same Azure subscription as the shoot; it may live in any RG within that subscription.

The user MAY (optional, not required):

6. Attach an NSG to the subnet. If they do, they take responsibility for permitting the traffic flows Kubernetes needs — see [NSG mutation contract](#nsg-mutation-contract) for the specific rules. Gardener neither creates nor mutates this subnet-level NSG. Gardener's own NIC-level NSG (in the shoot's cluster RG) stacks on top of the user's NSG and covers the CCM-managed LB ingress rules.
7. (Non-blocking) For observability tags to appear on the VNet and route table, Gardener's Azure principal needs `Microsoft.Network/virtualNetworks/write` and `Microsoft.Network/routeTables/write` on the corresponding RGs. If any of these are denied, tagging is silently skipped for that resource — the shoot still reconciles green.

The user MUST NOT:

- Reuse the referenced subnet or its route table across multiple Gardener Azure shoots. See [Route-table ownership](#route-table-ownership) for the CCM route-controller mutual-destruction hazard.
- Run competing automation (Terraform, policy engines) against the discovered route table — the seed cloud-controller-manager writes per-node pod-CIDR routes there in non-overlay shoots. Overlay-CNI shoots avoid this constraint because the CCM's route controller is disabled.
- If they attached a subnet-level NSG, block the flows listed in [NSG mutation contract](#nsg-mutation-contract). Doing so breaks the cluster; Gardener cannot detect this.
- Rely on `shoot.status.provider.egressCIDRs` for firewall allowlisting on the receiving side — this field is empty in user-managed egress mode.
- Expect Gardener to clean up orphan pod-CIDR routes from the route table on shoot deletion. In non-overlay shoots, some routes may remain after teardown if the CCM's graceful pruning failed — see [Deletion / teardown semantics](#deletion--teardown-semantics).

## Deletion / teardown semantics

- BYO subnet is **never** deleted by Gardener.
- BYO VNet is **never** deleted by Gardener (already the case today).
- The BYO route table (and any subnet-level NSG the user attached) are **never** deleted by Gardener.
- The Gardener-owned NIC-level NSG lives in the shoot's cluster RG and dies with the cluster-RG cascade delete on shoot teardown. No separate cleanup step is required for it.
- The observability tag `kubernetes.io/cluster/<shoot-technical-name>: shared` that the reconciler added to the BYO VNet and route table is removed on shoot deletion by the `RemoveBYOResourceTags` task. Tag removal is best-effort — a failure (e.g. permission revoked, resource already gone) logs a warning and does not block deletion. Other tags on the resource are untouched.
- Named rules added by the CCM (for LB Services) and the bastion controller (for Bastion resources) on the Gardener-owned NSG are cleaned up on their owning resource's delete — same as for any Gardener Azure shoot today. On shoot deletion, all LB Services and all Bastion resources are deleted first, which triggers the corresponding NSG rule removal before Gardener stops managing anything. Any rules that slip through are wiped along with the NSG itself when the cluster RG is cascade-deleted.
- **Orphan pod-CIDR routes on the user's RT are not cleaned up in v1.** In non-overlay shoots the seed CCM writes one route per node into the user's route table. Graceful teardown removes them as MCM scales workers down and as the CCM observes Node deletions. Failure modes that leave orphans behind:
    - CCM crash during ControlPlane teardown.
    - API server unreachable → CCM can't observe Node deletions.
    - Force-delete of the shoot → skips graceful teardown; the CCM never gets a chance.
    - Transient Azure API throttling / errors past the CCM's retry budget.
  These orphaned routes point at IPs that no longer exist ("blackhole" routes). Gardener will not clean them up in v1; the user is responsible for pruning them from the RT if they matter. Targeted cleanup via a snapshot-and-filter approach is planned as [future work](#out-of-scope-future-work). Overlay-CNI shoots are not affected because the CCM's route controller is disabled — no per-node routes are ever written.
- Any load balancers created by CCM for `Service type=LoadBalancer` are cleaned up by the existing LB-in-foreign-VNet-RG cleanup path, which already handles the BYO-VNet case today.

## Documentation

Two documents added:

1. `docs/usage/user-managed-egress.md` — end-user how-to with:
   - Step-by-step Azure CLI / portal instructions to pre-provision VNet + subnet + RT (NSG on the subnet is optional and treated as a side note with the required flow list).
   - Example route table configurations for firewall-based egress.
   - Discussion of the network-isolated variant (no default route) and its cluster-viability prerequisites (private API server, Private Endpoints for MCR, etc.).
   - The list of Azure endpoints that the cluster needs to reach (control plane, container registries, etc.), cross-linked to Microsoft's authoritative outbound-rules documentation.
   - Explanation of the two-layer NSG evaluation (subnet-level user NSG stacks with Gardener's NIC-level NSG), with the required flows if the user attaches a subnet NSG.
   - Explicit warning that `shoot.status.provider.egressCIDRs` will be empty.
2. `docs/usage/usage.md` — new subsection under the existing "Networking" section, cross-linking to (1).

## Acceptance criteria

Any implementation must pass the following scenarios end-to-end. They are grouped into regression scenarios (existing behavior must not change), new BYO-mode scenarios that must succeed, validation scenarios that must fail with clear errors, immutability scenarios, and runtime-behavior scenarios. Each scenario states its pre-conditions and its observable pass criteria.

### Group A — Regression (existing behavior unchanged)

| ID  | Configuration                                                                                     | Pass criteria                                                                                                                                                                                                                                  |
| --- | ------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A1  | Managed VNet + managed subnet, no `NatGateway`                                                    | Shoot reconciles green; `worker_route_table` created; `<technicalName>-workers` NSG created; two `allow-{tcp,udp}-egress` Services present in `kube-system`; `azure.json` has no `disableOutboundSNAT`; egress functional via LB frontend PIP. |
| A2  | Managed VNet + managed subnet + `NatGateway.Enabled=true`                                         | Shoot reconciles green; NAT Gateway created; no `allow-egress` Services; `InfrastructureStatus.EgressCIDRs` populated with NAT PIP `/32` entries; egress functional via NAT PIP.                                                               |
| A3  | BYO VNet (`Networks.VNet.{Name,ResourceGroup}` set) + managed subnet + NAT with `IPAddresses` set | Shoot reconciles green; NAT uses user's public IPs (no new PIP created); user's VNet unmodified except for Gardener's subnet/RT/NSG lifecycle.                                                                                                 |
| A4  | Multi-zone managed subnet layout (`Networks.Zones[]` non-empty)                                   | Shoot reconciles green; one subnet + one NAT (if enabled) per zone; per-zone NSG rules and route tables.                                                                                                                                       |
| A5  | Existing single-subnet shoot migrated to multi-subnet layout                                      | Migration webhook stamps the `migration.azure.provider.extensions.gardener.cloud/zone` annotation; reconcile succeeds; no data-plane disruption.                                                                                               |

### Group B — Valid new BYO configurations (must reconcile green)

| ID  | Configuration                                                                                                     | Pass criteria                                                                                                                                                                                                                                                                                          |
| --- | ----------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| B1  | BYO VNet in RG `hub-network`; BYO subnet with RT in `hub-network`; no subnet-level NSG                            | Shoot reconciles green. `<technicalName>-workers` NSG is created in the shoot's cluster RG and attached to worker NICs. `azure.json` contains `subnetName`, `routeTableName`, `securityGroupName` (Gardener's NSG in cluster RG), `vnetResourceGroup=hub-network`, `disableOutboundSNAT: true`. Neither `securityGroupResourceGroup` nor `routeTableResourceGroup` are emitted (defaults suffice). |
| B2  | BYO VNet in `hub-network`; RT in `central-network-rg`                                                             | `azure.json` emits `routeTableResourceGroup: central-network-rg`. `securityGroupResourceGroup` remains unset (NSG is in cluster RG, same as `resourceGroup`). Reconcile green.                                                                                                                          |
| B3  | BYO VNet in `hub-network`; BYO subnet with a subnet-level NSG owned by the user in `security-team-rg`             | Shoot reconciles green. Gardener does not read, tag, or mutate the user's subnet-level NSG. Gardener's NIC-level NSG is created in cluster RG as usual and stacks over the user's subnet-level NSG.                                                                                                    |
| B4  | BYO subnet with a route table whose `0.0.0.0/0` route points at an Azure Firewall (next-hop = `VirtualAppliance`) | Egress from worker nodes flows via the firewall. Per-node pod-CIDR routes appear in the user's RT (written by the seed CCM's route controller). The user's `0.0.0.0/0` route is preserved.                                                                                                             |
| B5  | BYO subnet with an empty route table (no `0.0.0.0/0`)                                                             | Shoot reconciles green. Workers can reach in-VNet resources. Internet egress fails (expected; this is the "no egress" pattern). Per-node pod-CIDR routes still appear in the RT.                                                                                                                       |

### Group C — Rejected configurations (must fail validation with a clear error)

| ID  | Configuration                                                                     | Pass criteria                                                                             |
| --- | --------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| C1  | `Networks.Subnet` set, but `Networks.VNet.ResourceGroup` unset                    | API validation rejects: BYO subnet requires BYO VNet.                                     |
| C2  | `Networks.Subnet` set + `Networks.Zones` non-empty                                | API validation rejects: multi-subnet layout not supported with BYO subnet in v1.          |
| C3  | `Networks.Subnet` set + `Networks.Workers` non-empty                              | API validation rejects: workers CIDR is discovered, not declared, in BYO mode.            |
| C4  | `Networks.Subnet` set + `Networks.NatGateway` non-nil (any variant)               | API validation rejects: NAT is user-managed in BYO mode.                                  |
| C5  | `Networks.Subnet` set + `Networks.ServiceEndpoints` non-empty                     | API validation rejects: service endpoints managed by user in BYO mode.                    |
| C6  | `Networks.Subnet` set + `Networks.VNet.CIDR` set                                  | API validation rejects: CIDR is discovered from the actual VNet.                          |
| C7  | `Networks.Subnet` set + `Networks.VNet.DDosProtectionPlanID` set                  | API validation rejects: DDoS plan managed by user on BYO VNet.                            |
| C8  | `Networks.Subnet.Name` refers to a subnet that does not exist inside the BYO VNet | Pre-flight (runtime) validator rejects with error containing subnet name + VNet identity. |
| C9  | Referenced subnet has no `RouteTable` association, and `SkipRouteReconciliation` is not set              | Pre-flight validator rejects: route table must be pre-attached to the BYO subnet, or overlay opt-out must be selected. |
| C10 | Referenced subnet's CIDR is not a subset of `shoot.spec.networking.nodes`         | Pre-flight validator rejects.                                                             |
| C11 | Referenced subnet's CIDR overlaps `shoot.spec.networking.pods` or `.services`     | Pre-flight validator rejects.                                                             |
| C12 | Referenced route table lives in a different Azure subscription than the shoot     | Pre-flight validator rejects (cross-subscription references unsupported by CCM).          |

### Group D — Update / immutability

| ID  | Change on an existing shoot                                                  | Pass criteria                                                         |
| --- | ---------------------------------------------------------------------------- | --------------------------------------------------------------------- |
| D1  | Managed-mode shoot: attempt to add `Networks.Subnet` on update               | API validation rejects: mode transitions forbidden in v1.             |
| D2  | User-managed-egress shoot: attempt to remove `Networks.Subnet` on update     | API validation rejects.                                               |
| D3  | User-managed-egress shoot: change `Networks.Subnet.Name`                     | API validation rejects: `Networks.Subnet.Name` is immutable once set. |
| D4  | User-managed-egress shoot: change any BYO field on `Networks.VNet` (name/RG) | API validation rejects (existing VNet immutability rules apply).      |

### Group E — Runtime behavior (invariants after successful reconcile in BYO mode)

| ID  | Assertion                                                                                                                                                                                                                                                                                                                    |
| --- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| E1  | `InfrastructureStatus.Networks.OutboundAccessType == UserManaged`.                                                                                                                                                                                                                                                           |
| E2  | `InfrastructureStatus.Networks.EgressCIDRs` is nil (or empty).                                                                                                                                                                                                                                                               |
| E3  | The reconciler makes zero `PUT`/`PATCH` calls against the BYO subnet, the BYO route table, or the BYO VNet during reconcile (verified via ARM audit log or mock client assertions). The Gardener-owned NIC-level NSG in the cluster RG is reconciled as usual.                                                              |
| E4  | The `worker_route_table` name does not appear in the shoot's cluster RG.                                                                                                                                                                                                                                                     |
| E5  | The `<technicalName>-workers` NSG **does** appear in the shoot's cluster RG (unchanged from managed mode).                                                                                                                                                                                                                   |
| E6  | The subnet's `.properties.networkSecurityGroup` is left as the user configured it (either nil, or pointing at the user's own NSG). Gardener does not associate the cluster-RG NSG with the subnet.                                                                                                                          |
| E7  | The `allow-tcp-egress` and `allow-udp-egress` Services are **not** present in `kube-system` of the shoot cluster.                                                                                                                                                                                                            |
| E8  | The `azure.json` emitted to the CCM contains, at minimum: `subnetName`, `routeTableName`, `securityGroupName`, `vnetResourceGroup`, `disableOutboundSNAT: true`. If RT lives outside the cluster RG, `routeTableResourceGroup` is also emitted. `securityGroupResourceGroup` is not emitted (NSG is in cluster RG).          |
| E9  | In BYO-subnet mode: every worker node NIC has `properties.networkSecurityGroup.id` pointing at the Gardener-owned NSG in the shoot's cluster RG (asserted via `armnetwork.InterfacesClient.Get`). In managed mode: worker NICs have no NIC-level NSG association (unchanged); the shoot NSG is applied via the subnet-level association instead.                                                     |
| E10 | Creating a `Service type=LoadBalancer` (default settings) → CCM creates SLB + Public IP; adds NSG rules on Gardener's NSG in the shoot's cluster RG; LB rule has `disableOutboundSnat=true`.                                                                                                                                 |
| E11 | Creating a `Service type=LoadBalancer` with annotation `service.beta.kubernetes.io/azure-disable-load-balancer-nsg-rule: "true"` → CCM creates SLB but adds **no** NSG rules on the shoot NSG.                                                                                                                               |
| E12 | Deleting the `Service type=LoadBalancer` → CCM removes SLB, Public IP, and any NSG rules it added.                                                                                                                                                                                                                           |
| E13 | Creating a `Bastion` resource → 4 rules named `<bastion-instance-name>-*` appear on the Gardener NSG (in cluster RG); a bastion VM appears in the shoot's cluster RG with its NIC in the BYO subnet and its NSG association pointing at the same Gardener NSG; bastion has no internet egress by design.                     |
| E14 | Deleting the `Bastion` resource → all 4 rules are removed from the Gardener NSG; bastion VM/NIC/PIP/Disk removed from cluster RG.                                                                                                                                                                                            |

### Group F — Deletion / teardown

| ID  | Action                                                                                    | Pass criteria                                                                                                                                                                                                                                                                                        |
| --- | ----------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| F1  | Delete the shoot                                                                          | The cluster RG is deleted (all Gardener-owned resources — VMs, NICs, disks, PIPs, the LB, and the shoot NSG — go with it). The BYO VNet, subnet, RT, and any user-owned subnet-level NSG remain untouched. The `kubernetes.io/cluster/<technicalName>: shared` tag is removed from the VNet and RT. Other tags on those resources are unchanged.                                                                                                                             |
| F2  | Delete the shoot while a `Service type=LoadBalancer` and a `Bastion` resource still exist | CCM/bastion controller remove their respective NSG rules before Gardener stops managing. On successful teardown the shoot NSG is gone (cluster RG cascade); on graceful teardown its rules were already emptied. LB resources in a foreign-VNet RG cleanup path apply as today.                                                                                                                                                                                             |
| F3  | Delete the shoot in a VNet where other shoots or workloads share the BYO subnet           | The BYO subnet and its RT remain functional for the other consumers. Only the deleted shoot's cluster RG resources go away. Other shoots' `kubernetes.io/cluster/*` tags on the shared VNet/RT are preserved.                                                                                                                                                                                                                                                              |
| F4  | Delete the shoot when tag-write permission on the VNet/RT has been revoked mid-life       | Tag removal fails; a warning is logged; shoot deletion completes successfully. Stale `kubernetes.io/cluster/<technicalName>` tags on the resources are the user's responsibility to clean up out-of-band.                                                                                                                                                                                                                                                                    |

### Group G — Metadata tagging

| ID  | Action / Configuration                                                                                   | Pass criteria                                                                                                                                                                              |
| --- | -------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| G1  | Create a BYO shoot with tag-write permission on both resources                                           | After reconcile, the BYO VNet and RT each carry the tag `kubernetes.io/cluster/<technicalName>: shared`. Any pre-existing tags on those resources are preserved.                          |
| G2  | Create a second BYO shoot sharing the same BYO subnet (different technicalName)                          | Both shoots' tags coexist on the shared VNet and RT: `kubernetes.io/cluster/<shootA>: shared` AND `kubernetes.io/cluster/<shootB>: shared`. Neither shoot's tags interfere with the other's. |
| G3  | Create a BYO shoot where the Azure principal lacks `Microsoft.Network/virtualNetworks/write` on the VNet | Shoot still reconciles green. The VNet is not tagged. RT still gets tagged (if permission is present). A warning is logged for the VNet tag failure.                                       |
| G4  | Create a BYO shoot where the principal lacks tag-write on both resources                                 | Shoot still reconciles green. No tags are added. Two warnings are logged.                                                                                                                  |
| G5  | Re-reconcile an existing BYO shoot (nothing changed)                                                     | Tag operations are idempotent — no ARM writes are made if the correct tag is already present.                                                                                              |

## Risks and upstream conflicts

Design-level concerns identified during investigation of the upstream Azure cloud-controller-manager and of the operational surface this proposal exposes. Listed roughly in decreasing severity. Anything here that is not addressed in this proposal is called out explicitly as "accepted risk" or "user responsibility".

| # | Category                | Concern                                                                                                                                                                                                                                                                                                                                                                          | Mitigation / stance                                                                                                                                                                                                                                                                                                          |
| - | ----------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 | Shared-chart regression | Making `--configure-cloud-routes` values-driven in `charts/internal/seed-controlplane/charts/cloud-controller-manager` affects every Azure shoot on the seed. If the default is not preserved as `true`, existing managed-mode shoots silently stop programming pod-CIDR routes and node-to-pod connectivity breaks.                                                             | Default the values-key to `true`. Acceptance criteria A1 and A2 verify managed-mode behavior. Ship the chart change as an isolated commit with explicit regression testing before wiring it up to the new API field.                                                                                                          |
| 2 | Shared BYO subnet / RT  | In non-overlay CNI shoots the seed CCM writes per-node pod-CIDR routes into the user's RT. Two shoots referencing the same subnet share its RT (Azure supports only one RT association per subnet) and their CCMs mutually delete each other's routes on every reconcile. In overlay shoots the CCM route controller is disabled and the risk does not apply. | **Docs-only in v1** — [Route-table ownership](#route-table-ownership) and [User responsibilities](#user-responsibilities) state clearly that the BYO subnet must not be reused across shoots. No admission-time enforcement (cross-shoot inspection is not a pattern this extension uses today). Follow-up options: force overlay in BYO, or move to a route-controller-free architecture — see [Out of scope](#out-of-scope-future-work). |
| 3 | Cross-subscription BYO  | Users with hub/spoke topologies that put the VNet in a "networking" subscription and workers in a "workload" subscription cannot use this design. The Azure CCM's `azure.json` is auth-scoped to one subscription; there is no field for resolving the RT in a different subscription.                                                                                           | **Not addressed in v1**. Documented as a user prerequisite (single-subscription BYO). Runtime validator rejects cross-subscription references (`C12`). If demand emerges, a follow-up can introduce a secondary-credential mechanism, but this is a substantial extension well beyond the current scope.                     |
| 4 | Orphan pod-CIDR routes on delete | In non-overlay shoots the seed CCM writes one route per node to the user's RT. Graceful teardown removes them; CCM crashes, force-delete, or transient Azure errors leave orphans pointing at now-dead node IPs (blackhole routes). | **Not cleaned up in v1**. User is responsible for pruning. Targeted cleanup via IP snapshot + filter is [future work](#out-of-scope-future-work). Overlay shoots are not affected. |
| 5 | Silent-failure UX       | `disableOutboundSNAT: true` in `azure.json` makes CCM-created LB rules ingress-only. A user who expects `Service type=LoadBalancer` to also SNAT node egress (as it does today in Gardener's LB-egress workaround mode) will find their pods have no working egress path via the LB. Debugging is non-obvious because the LB reports healthy and rules exist — they just don't SNAT. | Documented under [Configuration patterns](#configuration-patterns) and [User responsibilities](#user-responsibilities). Consider surfacing a status condition on the shoot when `OutboundAccessType==UserManaged` to make the mode obvious to operators. Accepted risk in v1.                                                 |
| 6 | User subnet NSG stale rules | If the user attaches a subnet-level NSG and later modifies it (e.g. tightens rules) in a way that breaks Kubernetes' required flows (kubelet ↔ apiserver, node ↔ node, LB health probes), the cluster degrades or fails outright. Gardener has no visibility into the user's NSG changes and cannot re-validate at runtime.                                                       | User responsibility. Documented in [User responsibilities](#user-responsibilities) and [NSG mutation contract](#nsg-mutation-contract) with the exact required flows. The subnet-level NSG is entirely operator-managed.                                                                                     |
| 7 | Platform default drift  | Azure's ongoing retirement of default outbound access means new subnets created in 2025+ get `defaultOutboundAccess=false`. For network-isolated mode this is the intended state; for firewall-egress mode the user still needs their `0.0.0.0/0` route configured — if they miss it, there is no fallback egress at all. Not a bug in this design; a change in the platform default. | Documented in [User responsibilities](#user-responsibilities). Runtime validator does not check the subnet's `defaultOutboundAccess` state or the route table's contents; the user takes ownership.                                                                                                                          |
| 8 | Retry storm             | If Gardener's Azure principal loses NSG write permission on the shoot's cluster RG mid-flight (Policy change, IAM revocation), both the CCM's `Service type=LoadBalancer` reconcile loop and the extension's Bastion reconcile loop enter retry-with-backoff against ARM indefinitely. Noisy at the API level; user sees a stream of confusing reconcile-failed statuses.            | Accepted; matches the existing behavior when any Gardener component loses permission on the cluster RG. This is not BYO-specific — managed-mode shoots have the same failure mode. Users are advised to gate permission changes through a controlled change-management flow.                                                                        |
| 9 | Upstream CCM versioning | `routeTableResourceGroup` and `disableOutboundSNAT` are all long-standing fields in `kubernetes-sigs/cloud-provider-azure`. If Gardener supports a CCM version that predates any of them, that field is silently ignored and BYO breaks (RT lookup falls back to the cluster RG, egress SNAT falls back to enabled, etc.).                                                          | Verify the fields exist in every CCM version Gardener ships across its supported Kubernetes minor versions before merging. Add a compile-time or start-up sanity check if practical.                                                                                                                                          |

## Alternatives considered

### 1. Explicit `OutboundType` enum

Add `Networks.OutboundType: string` with values `loadBalancer` / `managedNATGateway` / `userAssignedNATGateway` / `userDefinedRouting` / `none`. Best diagnostics, best future compatibility for later additions such as `block` / `managedNATGatewayV2` / a true `userAssignedNATGateway`.

**Rejected in favor of derived mode** because:

- We're currently scoping only to UDR+None (collapsed). A five-value enum where four values are "TBD" is worse than no enum.
- Consistency across Gardener providers is preferred; other providers derive similar modes from field presence rather than exposing enums.
- If we later want the enum, we can add it as a status-only field derived from the same inputs (a "computed outbound type" for observability) without changing the input surface.

### 2. Reuse the two `OutboundAccessType` status values

Instead of adding `UserManaged`, reuse `LoadBalancer` and let downstream code differentiate via `Networks.Subnet != nil`.

**Rejected** because `OutboundAccessType` is a widely-consumed status signal; conflating "user-managed" with "we deployed the allow-egress LB workaround" would silently break dashboards, alerts, and monitoring queries that key off it.

### 3. BYO Route Table as a separate top-level field

Add `Networks.RouteTable.{Name,ResourceGroup}` alongside `Networks.Subnet`, letting users mix-and-match.

**Rejected** because in Azure, an RT is _attached to_ a subnet. If the user brings a subnet, they've already made the RT choice at the ARM level. Discovering it from the subnet is unambiguous and produces less API surface / less validation. If a future use case needs "managed subnet with BYO route table" we can add it then.

### 4. BYO NSG as a separate top-level field, or discovered from the subnet

Two related alternatives were considered:

- Adding a top-level `Networks.SecurityGroup.{Name,ResourceGroup}` field that the user sets;
- Or discovering the NSG from the subnet's existing `NetworkSecurityGroup` association (as the earlier draft of this proposal did).

**Both rejected** because they hand the CCM a user-owned NSG for it to write rules into, and the upstream CCM's `RetainSecurityGroup` behavior (`pkg/provider/azure_loadbalancer.go:3520`) treats its configured NSG as exclusively owned. Two shoots pointed at the same NSG would mutually delete each other's LB rules on every reconcile. The design that avoids the problem entirely is to keep the CCM-facing NSG per-shoot in the shoot's cluster RG, attached at NIC level rather than at subnet level. This gives a clean lifecycle without cross-cluster sharing hazards. See [NSG mutation contract](#nsg-mutation-contract).

### 4. Programmatically detect and preserve foreign-RG resources (extend the existing heuristic)

The existing preservation heuristic in the infrastructure reconciler already leaves foreign-RG NAT associations alone. Extend the same approach to RT and NSG, making BYO implicit.

**Rejected**: implicit behavior with no API surface is impossible to validate, document, or reason about. Users need an explicit way to say "I own this."

### 5. Support BYO in the zoned (multi-subnet) layout too

Add `Networks.Zones[i].Subnet` as a per-zone BYO subnet reference to support the multi-subnet (zoned) layout.

**Rejected for v1** per design decision. Deferred. The single-subnet layout covers the vast majority of user-managed-egress use cases (a hub-spoke topology with one worker subnet is the norm). Multi-subnet BYO adds a large validation matrix (all-BYO vs all-managed homogeneity, per-zone RT resolution, migration rules from single→multi with BYO on both sides) that's not worth the complexity in v1.

## Resolved questions

- **CCM-facing NSG ownership** — Resolved. Gardener creates and owns the CCM-facing NSG in every mode, including BYO. It lives in the shoot's cluster RG and is attached to worker NICs, not the subnet. See [NSG mutation contract](#nsg-mutation-contract) for the rationale (short: the CCM's `RetainSecurityGroup` behavior is incompatible with shared NSGs across clusters).
- **Route table ownership** — Resolved. Three options were considered (Gardener-created RT, user-BYO RT, force overlay in BYO). Chosen: user-BYO RT, discovered from the subnet at reconcile time, not shareable across shoots. See [Route-table ownership](#route-table-ownership).
- **Route table resource group** — Resolved. May live in any RG within the same subscription as the shoot. Populated in `azure.json` via `routeTableResourceGroup`. `securityGroupResourceGroup` is not needed because the NSG is always in the shoot's cluster RG.
- **`disableOutboundSNAT` semantics** — Resolved. Cluster-wide setting in `azure.json` is applied by upstream CCM to every LB rule on every reconcile (`pkg/provider/azure_loadbalancer.go:3360` reads `az.DisableLoadBalancerOutboundSNAT()` inside `EnsureLoadBalancer`, so both create and update paths apply it). Setting it to `true` at the cluster level is sufficient; no per-rule work needed. The value is `true` for user-managed-egress mode; unset (CCM default `false`) for all existing modes to preserve the `allow-egress` LB workaround, which requires SNAT-capable LB rules to function.
- **Bastion in BYO-subnet mode** — Resolved. Fully supported; see [Bastion](#bastion). The bastion controller uses the Gardener-owned NSG in the cluster RG (same NSG as the CCM), unchanged from managed mode.
- **NSG "read-only" promise** — Not applicable in the current design: the CCM-facing NSG is entirely Gardener-owned. The user's optional subnet-level NSG is entirely user-owned; Gardener does not touch it.

## Out of scope (future work)

- Explicit `OutboundType` enum (either as input or as an observability-only status field).
- In-place transition between managed and user-managed egress on an existing shoot.
- **Targeted cleanup of orphan pod-CIDR routes** on the user's RT during shoot deletion. In v1 these are left behind; the user is responsible for pruning them. The intended follow-up is a snapshot-and-filter approach (as in `provider-aws` and `provider-gcp`): snapshot the shoot's cluster-RG NIC IPs at the top of the delete flow, then iterate routes on the BYO RT and drop those whose `nextHopIpAddress` matches a snapshotted IP. Best-effort per resource. Deferred because it needs a delete-flow refactor and additional integration test surface that isn't in v1's critical path.
- **Lifting the "no shared BYO subnet across shoots" constraint.** Two paths converge on this:
  1. Force overlay CNI in BYO — reject `overlay.enabled: false` at admission time. Simple, restrictive.
  2. Move the extension to a route-controller-free architecture even for non-overlay CNIs (e.g. by allocating pod IPs directly from the VNet subnet via NIC `IPConfiguration` arrays, similar to Azure CNI Node Subnet). Much bigger scope but preserves flat CNI use cases.
  Neither is in v1's scope.
