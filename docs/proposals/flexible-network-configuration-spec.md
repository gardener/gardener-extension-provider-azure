# Implementation Spec: Bring-Your-Own Subnet for User-Managed Egress on Azure

**Status**: implementation checklist derived from [flexible-network-configuration-proposal.md](./flexible-network-configuration-proposal.md). The proposal is the source of truth for design intent and constraints; this spec captures the concrete file changes, code sketches, and test surface needed to satisfy the proposal's acceptance criteria.

**Audience**: coding agents and implementing engineers. Anything in this document may be revised during implementation as long as the changes still satisfy the proposal's acceptance criteria (referenced below by their proposal IDs — `A1`–`A5`, `B1`–`B5`, `C1`–`C12`, `D1`–`D4`, `E1`–`E14`, `F1`–`F4`, `G1`–`G5`).

<!-- toc -->

- [Scope and non-scope](#scope-and-non-scope)
- [API type changes](#api-type-changes)
- [Validation](#validation)
- [Reconciler](#reconciler)
- [Cloud-provider config](#cloud-provider-config)
- [CCM route-controller flag](#ccm-route-controller-flag)
- [`allow-egress` gating](#allow-egress-gating)
- [Bastion controller](#bastion-controller)
- [Testing plan](#testing-plan)
- [Suggested implementation order](#suggested-implementation-order)

<!-- /toc -->

## Scope and non-scope

**In scope for the initial PR**: everything below. Satisfies acceptance criteria groups A–G in the proposal.

**Out of scope, deferred to follow-up PRs**: explicit `OutboundType` enum surface; in-place transition between managed and user-managed egress on an existing shoot. Both are enumerated in the proposal's "Out of scope" section and must be actively rejected by validation in this PR (see `D1`, `D2`).

**Contract with the design**: if any of the following implementation details turn out to be wrong at coding time (e.g. a file path has shifted, an upstream API signature is different), revise the spec — do not silently deviate. If the deviation touches proposal-level design intent (a new mode, a different API shape, a different NSG contract), stop and get the proposal amended first.

## API type changes

**File**: `pkg/apis/azure/types_infrastructure.go`. Mirror in `pkg/apis/azure/v1alpha1/types_infrastructure.go` with JSON tags on the exported fields.

New nested field on `NetworkConfig`:

```go
// NetworkConfig holds information about the Kubernetes and infrastructure networks.
type NetworkConfig struct {
    // ... existing fields (VNet, Workers, NatGateway, ServiceEndpoints, Zones) ...

    // Subnet is an optional reference to an already-existing subnet inside the (also
    // user-provided) VNet. When set, Gardener's infrastructure reconciler will not create
    // or manage the worker subnet or its route table; it discovers the subnet's route-table
    // association at reconcile time. Gardener still creates its own NSG in the shoot's cluster
    // resource group, but attaches it to worker NICs (not to the user's subnet). A subnet-level
    // NSG owned by the user is optional and independent. Requires VNet.Name and VNet.ResourceGroup
    // to be set. Not compatible with Zones, Workers, NatGateway, or ServiceEndpoints.
    // +optional
    Subnet *SubnetReference `json:"subnet,omitempty"`
}

// SubnetReference references an existing subnet in an existing VNet.
type SubnetReference struct {
    // Name is the name of the subnet.
    Name string `json:"name"`

    // SkipRouteReconciliation disables the seed CCM's route controller for this shoot.
    // Intended for overlay-CNI shoots (Cilium/Calico with VXLAN or Geneve) where pod-CIDR
    // routes in the underlying VNet are not needed. When true, the reconciler does not
    // require the BYO subnet to have a route table attached.
    // +optional
    SkipRouteReconciliation *bool `json:"skipRouteReconciliation,omitempty"`
}
```

`OutboundAccessType` enum gains a third value:

```go
const (
    OutboundAccessTypeNATGateway   OutboundAccessType = "NATGateway"
    OutboundAccessTypeLoadBalancer OutboundAccessType = "LoadBalancer"
    // OutboundAccessTypeUserManaged indicates that the user is responsible for egress
    // (firewall-based egress via a user-owned route table, or network-isolated with no
    // default route). Set when the user brought their own subnet and did not enable a NAT
    // Gateway.
    OutboundAccessTypeUserManaged OutboundAccessType = "UserManaged"
)
```

`RouteTable` status type gains an optional `ResourceGroup`. The `SecurityGroup` status type does not need one — the NSG is always in the shoot's cluster RG in both managed and BYO mode. Both fields are pointers so absence in existing (managed-mode) shoots continues to round-trip cleanly:

```go
type RouteTable struct {
    Purpose Purpose
    Name    string
    // ResourceGroup is the resource group hosting this route table. If nil, the shoot's
    // cluster resource group is assumed. Only populated in BYO-subnet mode.
    // +optional
    ResourceGroup *string
}

type SecurityGroup struct {
    Purpose Purpose
    Name    string
    // ResourceGroup was reserved for a BYO-NSG design that was rejected. Always nil today
    // — the NSG lives in the shoot's cluster resource group in every mode.
    // +optional
    ResourceGroup *string
}
```

Helper on `InfrastructureConfig`:

```go
// IsUsingUserManagedEgress reports whether the user opted into managing their own egress
// path by bringing an existing subnet.
func (c *InfrastructureConfig) IsUsingUserManagedEgress() bool {
    return c.Networks.Subnet != nil
}
```

Regenerate deep-copy / conversion / defaulter code via the usual codegen make target.

## Validation

**API-level** — `pkg/apis/azure/validation/infrastructure.go`:

When `Networks.Subnet != nil`, enforce every rule in the proposal's `Validation rules` table:

- `Networks.VNet.Name` and `Networks.VNet.ResourceGroup` must be set (`C1`).
- `Networks.VNet.CIDR` must be unset (`C6`).
- `Networks.VNet.DDosProtectionPlanID` must be unset (`C7`).
- `Networks.Workers` must be unset (`C3`).
- `Networks.Zones` must be empty (`C2`).
- `Networks.NatGateway` must be nil (`C4`).
- `Networks.ServiceEndpoints` must be empty (`C5`).
- `Networks.Subnet.Name` must be non-empty and conform to Azure subnet naming.

Extend `ValidateInfrastructureConfigUpdate` with:

- `Networks.Subnet.Name` immutable once set (`D3`).
- Mode transition forbidden — `Networks.Subnet` cannot be added or removed on an existing shoot (`D1`, `D2`).

**Runtime pre-flight validator** — new file `pkg/controller/infrastructure/configvalidator.go`:

Azure does not have a `ConfigValidator` today (provider-aws and provider-gcp both do; use their pattern). Called from the infrastructure controller before reconcile. Uses the shoot's Azure credentials to hit ARM. Checks:

- The referenced subnet exists in the BYO VNet (`C8`).
- The subnet has a `RouteTable` association, unless `Networks.Subnet.SkipRouteReconciliation=true` (`C9`).
- The subnet's CIDR is a subset of `shoot.spec.networking.nodes` and does not overlap `shoot.spec.networking.{pods,services}` (`C10`, `C11`).
- The discovered RT ARM ID resolves to the same subscription as the shoot (`C12`).

Errors must include the subnet name and VNet identity so the user can debug without inspecting logs.

## Reconciler

**File**: `pkg/controller/infrastructure/infraflow/flow_context.go` — the task graph.

Add branching on `IsUsingUserManagedEgress()`:

- **Skip** `EnsureRouteTable` and `EnsureSecurityGroup` in BYO mode. Both resources are user-owned in that mode; Gardener neither creates nor deletes them.
- **Replace** `EnsureSubnets` with a new `EnsureUserSubnet` in BYO mode.

**New helper** `EnsureUserSubnet` — put adjacent to the existing `ensureUserVirtualNetwork` at `ensurer.go:136-156`:

```go
// EnsureUserSubnet verifies the user-referenced subnet exists inside the BYO VNet
// and discovers the associated NSG and route table, storing their names and resource
// groups on the whiteboard for status building and cloud-provider-config emission.
// Never writes to the subnet, NSG, or RT.
func (fctx *FlowContext) EnsureUserSubnet(ctx context.Context) error {
    // 1. GET the subnet from ARM.
    // 2. Verify the subnet's CIDR is compatible with shoot networking.
    // 3. Verify subnet.Properties.NetworkSecurityGroup.ID is set. Parse to (rg, name); store on whiteboard.
    // 4. Parse subnet.Properties.RouteTable.ID -> (rg, name); store on the whiteboard.
    //    RT may be absent if the shoot uses overlay CNI (helper.IsOverlayEnabled) — then
    //    RouteTables[] is not populated in status.
    // 5. Do NOT PUT the subnet back — discovery must be read-only. Satisfies E3.
}
```

Status builder (`ensurer.go:641-708` `EnsureInfrastructureStatus`):

- Read the discovered NSG and RT identifiers from the whiteboard.
- Emit `Networks.OutboundAccessType = UserManaged`.
- Emit exactly one entry each in `Networks.Subnets[]`, `SecurityGroups[]`, and `RouteTables[]` (RT only if discovered).
- Set `Networks.Layout = SingleSubnet`.
- Leave `EgressCIDRs` nil.

## Cloud-provider config

**Template**: `charts/internal/cloud-provider-config/templates/cloud-provider-config.tpl` — two conditional fields:

```yaml
{{- if .Values.routeTableResourceGroup }}
routeTableResourceGroup: {{ .Values.routeTableResourceGroup }}
{{- end }}
{{- if .Values.disableOutboundSNAT }}
disableOutboundSNAT: true
{{- end }}
```

No `securityGroupResourceGroup` — the NSG is always in the shoot's cluster RG, so the default (`securityGroupResourceGroup` falls back to `resourceGroup`) suffices.

Upstream references for the field semantics:

- `RouteTableResourceGroup` — `azure.go:62` upstream, fallback at `:278-279`.
- `DisableOutboundSNAT` — `azure.go:123-125` upstream, per-LB-rule applied at `azure_loadbalancer.go:3360`.

**Value provider**: `pkg/controller/controlplane/valuesprovider.go` (`getConfigChartValues` at `:428-476`):

```go
if infraStatus.Networks.OutboundAccessType == azureapi.OutboundAccessTypeUserManaged {
    values["disableOutboundSNAT"] = true
}
for _, rt := range infraStatus.RouteTables {
    if rt.Purpose == azureapi.PurposeNodes && rt.ResourceGroup != nil {
        values["routeTableResourceGroup"] = *rt.ResourceGroup
    }
}
```

Verifies `E8`.

## CCM route-controller flag

**File**: `charts/internal/seed-controlplane/charts/cloud-controller-manager/templates/cloud-controller-manager.yaml:48`.

Currently hard-codes `--configure-cloud-routes=true`. Make it values-driven, defaulting to `true` for backward compatibility. `valuesprovider.go` passes `false` when `Networks.Subnet.SkipRouteReconciliation == true`.

Example:

```yaml
- --configure-cloud-routes={{ .Values.configureCloudRoutes | default "true" }}
```

When `false`, the CCM does not run the route controller and does not touch any route table. Verifies acceptance criterion `B5` (and future overlay-CNI tests).

## `allow-egress` gating

**File**: `pkg/controller/controlplane/valuesprovider.go:766-771` (`deployAllowEgressChart`).

```go
func deployAllowEgressChart(cluster *extensions.Cluster, infraStatus *azureapi.InfrastructureStatus) bool {
    if metav1.HasAnnotation(cluster.Shoot.ObjectMeta, azure.AnnotationKeySkipAllowEgress) {
        return false
    }
    if infraStatus.Networks.OutboundAccessType != azureapi.OutboundAccessTypeLoadBalancer {
        return false
    }
    // ... existing zoned / VMO logic ...
}
```

Verifies `E7`.

## Bastion controller

**Files**: `pkg/controller/bastion/options.go`, `pkg/controller/bastion/actuator.go`.

- Replace the hard-coded `NSGName(clusterName)` (`options.go:82`) with a lookup from `InfrastructureStatus.SecurityGroups[0]` — read `Name`. Fall back to the hardcoded name only if the status list is empty (defensive; shouldn't happen after this PR).
- `actuator.go:181-188` already handles the BYO-VNet case for the subnet lookup; no change needed there.
- The NSG's `ResourceGroup` is nil in status (always cluster RG); no wiring change needed at the ARM client call sites (`actuator.go:120-135`, `actuator_reconcile.go:205`) beyond making them use the status-sourced NSG name.

Verifies `E13`, `E14`.

## Testing plan

**Unit tests** — add or extend:

- `pkg/apis/azure/validation/infrastructure_test.go` — cover every case in `C1`–`C7` and `D1`–`D4`. Use a table-driven test structure.
- `pkg/controller/infrastructure/configvalidator_test.go` (new) — cover `C8`–`C12`. Mock the Azure subnet/network client.
- `pkg/controller/infrastructure/infraflow/ensurer_test.go` — cover `EnsureUserSubnet` (happy path + missing NSG + missing RT with overlay off + missing RT with overlay on + cross-subscription reference).
- `pkg/controller/controlplane/valuesprovider_test.go` — cover `E7`, `E8`.
- `pkg/controller/bastion/bastion_test.go` — cover the NSG lookup path in BYO mode (`E13`).

**Integration tests** — `test/integration/infrastructure/`:

- Extend the existing integration harness so that BYO-mode shoots can be created against a real subscription. Requires pre-provisioned VNet + subnet + NSG (attached) + RT in a test resource group.
- New scenarios: `B1`, `B2`, `B3`, `B4`, `B5`, `E3`, `F1`, `F2`, `F3`.

**E2E** — the existing shoot-creation e2e in Gardener core covers the LB Service creation surface (`E10`, `E12`). Add one BYO-mode variant if capacity allows.

**Regression** — `A1`–`A5` must continue to pass unchanged. Run the existing infrastructure integration suite against master then against this branch to confirm parity.

## Suggested implementation order

Minimizes risk by getting the machine-checkable parts (types, validation, unit tests) in first, then reconciler behavior, then integration coverage:

1. **API types + generated code** (deep-copy, conversion, defaulter). No behavior change; unit tests can compile and be added.
2. **API-level validation** in `pkg/apis/azure/validation/infrastructure.go`. Unit tests for `C1`–`C7`, `D1`–`D4`.
3. **Status-shape changes** — extend `RouteTable` with `ResourceGroup`. Regenerate. Adjust existing status-builder code to always leave `ResourceGroup` nil (backward-compat baseline).
4. **Pre-flight `ConfigValidator`**. Wire it into the infrastructure controller. Unit tests for `C8`–`C12`.
5. **Reconciler task-graph branching** — add `EnsureUserSubnet`. Manual smoke test in a scratch shoot with a hand-crafted BYO subnet.
6. **`cloud-provider-config` template + valuesprovider changes**. Unit tests for `E8`.
7. **`allow-egress` gating change**. Unit test for `E7`.
8. **CCM route-controller flag** — chart change + valuesprovider wiring. Manual test with `SkipRouteReconciliation=true`.
9. **Bastion controller refactor**. Unit test for `E13`.
10. **Integration test harness updates**. Add scenarios `B1`–`B5`, `F1`–`F4`.
11. **Documentation** — `docs/usage/user-managed-egress.md` and the pointer from `docs/usage/usage.md`.
