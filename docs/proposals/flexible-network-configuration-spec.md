# Implementation Spec: Bring-Your-Own Subnet for User-Managed Egress on Azure

**Status**: implementation checklist derived from [flexible-network-configuration-proposal.md](./flexible-network-configuration-proposal.md). The proposal is the source of truth for design intent and constraints; this spec captures the concrete file changes, code sketches, and test surface needed to satisfy the proposal's acceptance criteria.

**Audience**: coding agents and implementing engineers. Anything in this document may be revised during implementation as long as the changes still satisfy the proposal's acceptance criteria (referenced below by their proposal IDs — `A1`–`A5`, `B1`–`B5`, `C1`–`C13`, `D1`–`D4`, `E1`–`E12`, `F1`–`F4`, `G1`–`G5`).

<!-- toc -->

- [Scope and non-scope](#scope-and-non-scope)
- [API type changes](#api-type-changes)
- [Validation](#validation)
- [Reconciler](#reconciler)
- [Cloud-provider config](#cloud-provider-config)
- [CCM route-controller flag](#ccm-route-controller-flag)
- [`allow-egress` gating](#allow-egress-gating)
- [Bastion controller](#bastion-controller)
- [Metadata tagging](#metadata-tagging)
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
    // or manage the worker subnet, its route table, or its network security group; it
    // discovers them from the referenced subnet at reconcile time. The NSG's securityRules
    // will still be mutated at runtime by the Azure cloud-controller-manager (for
    // Service type=LoadBalancer) and by the bastion controller (for Bastion resources).
    // Requires VNet.Name and VNet.ResourceGroup to be set. Not compatible with Zones,
    // Workers, NatGateway, or ServiceEndpoints.
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

`RouteTable` and `SecurityGroup` status types gain an optional `ResourceGroup`. Fields are pointers so absence in existing (managed-mode) shoots continues to round-trip cleanly:

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
    // ResourceGroup is the resource group hosting this security group. If nil, the shoot's
    // cluster resource group is assumed. Only populated in BYO-subnet mode.
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
- The subnet has a `NetworkSecurityGroup` association (`C9`).
- The subnet has a `RouteTable` association, unless `Networks.Subnet.SkipRouteReconciliation=true` (`C10`).
- The subnet's CIDR is a subset of `shoot.spec.networking.nodes` and does not overlap `shoot.spec.networking.{pods,services}` (`C11`, `C12`).
- The discovered NSG and RT ARM IDs resolve to the same subscription as the shoot (`C13`).

Errors must include the subnet name and VNet identity so the user can debug without inspecting logs.

## Reconciler

**File**: `pkg/controller/infrastructure/infraflow/flow_context.go:150-177` — the task graph.

Add branching on `IsUsingUserManagedEgress()`:

- **Skip** `EnsureRouteTable` and `EnsureSecurityGroup` in BYO mode.
- **Replace** `EnsureSubnets` with a new `EnsureUserSubnet` in BYO mode.
- **Add** `EnsureBYOResourceTags` after `EnsureUserSubnet` in the reconcile flow.
- **Add** `RemoveBYOResourceTags` at the start of the deletion flow (before any actual resource deletion, since removal is on the BYO resources not on Gardener-owned ones).

**New helper** `EnsureUserSubnet` — put adjacent to the existing `ensureUserVirtualNetwork` at `ensurer.go:136-156`:

```go
// EnsureUserSubnet verifies the user-referenced subnet exists inside the BYO VNet
// and discovers the associated route table and security group, storing their names
// and resource groups on the whiteboard for status building and cloud-provider-config
// emission.
func (fctx *FlowContext) EnsureUserSubnet(ctx context.Context) error {
    // 1. GET the subnet from ARM.
    // 2. Verify the subnet's CIDR is compatible with shoot networking.
    // 3. Parse subnet.Properties.NetworkSecurityGroup.ID -> (rg, name); store on the whiteboard.
    // 4. Parse subnet.Properties.RouteTable.ID -> (rg, name); store on the whiteboard.
    //    RT may be absent if SkipRouteReconciliation=true — then RouteTables[] is not populated in status.
    // 5. Do NOT PUT the subnet back — discovery must be read-only. Satisfies E3.
}
```

Status builder (`ensurer.go:641-708` `EnsureInfrastructureStatus`):

- Read the discovered NSG and RT identifiers from the whiteboard.
- Emit `Networks.OutboundAccessType = UserManaged`.
- Emit exactly one entry each in `Networks.Subnets[]`, `SecurityGroups[]`, and `RouteTables[]` (the RT entry only if a RT was discovered).
- Set `Networks.Layout = SingleSubnet`.
- Leave `EgressCIDRs` nil.

## Cloud-provider config

**Template**: `charts/internal/cloud-provider-config/templates/cloud-provider-config.tpl` — three conditional fields:

```yaml
{{- if .Values.securityGroupResourceGroup }}
securityGroupResourceGroup: {{ .Values.securityGroupResourceGroup }}
{{- end }}
{{- if .Values.routeTableResourceGroup }}
routeTableResourceGroup: {{ .Values.routeTableResourceGroup }}
{{- end }}
{{- if .Values.disableOutboundSNAT }}
disableOutboundSNAT: true
{{- end }}
```

Upstream references for the field semantics:

- `SecurityGroupResourceGroup` — `pkg/provider/config/azure.go:58` in `kubernetes-sigs/cloud-provider-azure`. Falls back to `resourceGroup` when unset (`azure.go:282-283`).
- `RouteTableResourceGroup` — `azure.go:62` upstream, fallback at `:278-279`.
- `DisableOutboundSNAT` — `azure.go:123-125` upstream, per-LB-rule applied at `azure_loadbalancer.go:3360`.

**Value provider**: `pkg/controller/controlplane/valuesprovider.go` (`getConfigChartValues` at `:428-476`):

```go
if infraStatus.Networks.OutboundAccessType == azureapi.OutboundAccessTypeUserManaged {
    values["disableOutboundSNAT"] = true
}
for _, sg := range infraStatus.SecurityGroups {
    if sg.Purpose == azureapi.PurposeNodes && sg.ResourceGroup != nil {
        values["securityGroupResourceGroup"] = *sg.ResourceGroup
    }
}
for _, rt := range infraStatus.RouteTables {
    if rt.Purpose == azureapi.PurposeNodes && rt.ResourceGroup != nil {
        values["routeTableResourceGroup"] = *rt.ResourceGroup
    }
}
```

Verifies `E7`.

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

Verifies `E6`.

## Bastion controller

**Files**: `pkg/controller/bastion/options.go`, `pkg/controller/bastion/actuator.go`.

- Replace the hard-coded `NSGName(clusterName)` (`options.go:82`) with a lookup from `InfrastructureStatus.SecurityGroups[0]` — read `Name` and optionally `ResourceGroup`. Fall back to the hardcoded name only if the status list is empty (defensive; shouldn't happen after this PR).
- `actuator.go:181-188` already handles the BYO-VNet case for the subnet lookup; no change needed there.
- Wire the NSG's optional resource group through to the client calls in `getNetworkSecurityGroup` (`actuator.go:120-135`) and `ensureNetworkSecurityGroups` (`actuator_reconcile.go:205`).

Verifies `E11`, `E12`.

## Metadata tagging

**Convention**: `kubernetes.io/cluster/<technicalName>: shared` on the BYO VNet, NSG, and RT.

**New helper** `EnsureBYOResourceTags`:

- Reads the current tags on VNet, NSG, RT.
- If the shoot's cluster tag is already present with value `shared`, no-op.
- Otherwise merges the tag in and PUTs the resource.
- Best-effort per resource: catches the AuthorizationFailed / Forbidden error, logs a warning with the resource ID and the operator's principal name, and continues.
- Does not touch any other tags.

**New helper** `RemoveBYOResourceTags`:

- For each of VNet, NSG, RT: read tags, drop only the `kubernetes.io/cluster/<technicalName>` key if present, PUT back if the tag was removed.
- If the resource no longer exists (user deleted it), skip and log.
- Best-effort: on permission failure, log and continue — deletion must not be blocked.

Verifies `G1`–`G5`, `F1`, `F4`.

## Testing plan

**Unit tests** — add or extend:

- `pkg/apis/azure/validation/infrastructure_test.go` — cover every case in `C1`–`C7` and `D1`–`D4`. Use a table-driven test structure.
- `pkg/controller/infrastructure/configvalidator_test.go` (new) — cover `C8`–`C13`. Mock the Azure subnet/network client.
- `pkg/controller/infrastructure/infraflow/ensurer_test.go` — cover `EnsureUserSubnet` (happy path + missing NSG + missing RT + cross-subscription reference).
- `pkg/controller/infrastructure/infraflow/ensurer_test.go` — cover `EnsureBYOResourceTags` and `RemoveBYOResourceTags` including permission-failure paths (`G3`, `G4`, `F4`).
- `pkg/controller/controlplane/valuesprovider_test.go` — cover `E6`, `E7`.
- `pkg/controller/bastion/bastion_test.go` — cover the NSG lookup path in BYO mode (`E11`).

**Integration tests** — `test/integration/infrastructure/`:

- Extend the existing integration harness so that BYO-mode shoots can be created against a real subscription. Requires pre-provisioned VNet + subnet + NSG + RT in a test resource group; a script under `hack/` to create these on-demand.
- New scenarios: `B1`, `B2`, `B3`, `B4`, `B5`, `E3` (mock ARM audit alternative may be needed), `F1`, `F2`, `F3`.

**E2E** — the existing shoot-creation e2e in Gardener core covers the LB Service creation surface (`E8`, `E10`). Add one BYO-mode variant if capacity allows; otherwise cover in a follow-up.

**Regression** — `A1`–`A5` must continue to pass unchanged. Run the existing infrastructure integration suite against master then against this branch to confirm parity.

## Suggested implementation order

Minimizes risk by getting the machine-checkable parts (types, validation, unit tests) in first, then reconciler behavior, then integration coverage:

1. **API types + generated code** (deep-copy, conversion, defaulter). No behavior change; unit tests can compile and be added.
2. **API-level validation** in `pkg/apis/azure/validation/infrastructure.go`. Unit tests for `C1`–`C7`, `D1`–`D4`.
3. **Status-shape changes** — extend `RouteTable`/`SecurityGroup` with `ResourceGroup`. Regenerate. Adjust existing status-builder code to always leave `ResourceGroup` nil (backward-compat baseline).
4. **Pre-flight `ConfigValidator`**. Wire it into the infrastructure controller. Unit tests for `C8`–`C13`.
5. **Reconciler task-graph branching** — add `EnsureUserSubnet`. Manual smoke test in a scratch shoot with a hand-crafted BYO subnet.
6. **`cloud-provider-config` template + valuesprovider changes**. Unit tests for `E7`.
7. **`allow-egress` gating change**. Unit test for `E6`.
8. **CCM route-controller flag** — chart change + valuesprovider wiring. Manual test with `SkipRouteReconciliation=true`.
9. **Metadata tagging** — new `EnsureBYOResourceTags` / `RemoveBYOResourceTags`. Unit tests for `G1`–`G5`.
10. **Bastion controller refactor**. Unit test for `E11`.
11. **Integration test harness updates**. Add scenarios `B1`–`B5`, `F1`–`F4`.
12. **Documentation** — `docs/usage/user-managed-egress.md` and the pointer from `docs/usage/usage.md`.

Steps 1–4 are review-safe and can land in a small PR. Steps 5–8 form the behavioral core. Steps 9–12 are additive polish.
