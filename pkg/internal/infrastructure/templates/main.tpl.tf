provider "azurerm" {
  subscription_id = var.SUBSCRIPTION_ID
  tenant_id       = var.TENANT_ID
  client_id       = var.CLIENT_ID

  {{- if .useWorkloadIdentity }}
  use_oidc        = true
  use_cli         = false
  oidc_token_file_path = "/var/run/secrets/gardener.cloud/workload-identity/token"
  {{- else }}
  client_secret   = var.CLIENT_SECRET
  {{- end }}

  skip_provider_registration = "true"
  features {
    resource_group {
      prevent_deletion_if_contains_resources = false
    }
  }
}

#===============================================
#= Resource Group
#===============================================
{{ if .create.resourceGroup -}}
resource "azurerm_resource_group" "rg" {
  name     = "{{ .resourceGroup.name }}"
  location = "{{ .azure.region }}"
}
{{- else -}}
data "azurerm_resource_group" "rg" {
  name     = "{{ .resourceGroup.name }}"
}
{{- end }}

#===============================================
#= VNet, Subnets, Route Table, Security Groups
#===============================================

# VNet
{{ if .create.vnet -}}
resource "azurerm_virtual_network" "vnet" {
  name                = "{{ .resourceGroup.vnet.name }}"
  resource_group_name = {{ template "resource-group-reference" $ }}
  location            = "{{ .azure.region }}"
  address_space       = ["{{ .resourceGroup.vnet.cidr }}"]
  {{- if .resourceGroup.vnet.ddosProtectionPlanID }}
  ddos_protection_plan {
    id = "{{ .resourceGroup.vnet.ddosProtectionPlanID }}"
    enable = true
  }
  {{- end }}
}
{{- else -}}
data "azurerm_virtual_network" "vnet" {
  name                = "{{ .resourceGroup.vnet.name }}"
  resource_group_name = "{{ .resourceGroup.vnet.resourceGroup }}"
}
{{- end }}

# RouteTable
resource "azurerm_route_table" "workers" {
  name                = "worker_route_table"
  location            = "{{ .azure.region }}"
  resource_group_name = {{ template "resource-group-reference" $ }}
}

# SecurityGroup
resource "azurerm_network_security_group" "workers" {
  name                = "{{ .clusterName }}-workers"
  location            = "{{ .azure.region }}"
  resource_group_name = {{ template "resource-group-reference" $ }}
}

{{- range $subnet := .networks.subnets }}
{{- $workers := "workers" }}
{{- $subnetName := printf "%s-nodes" $.clusterName }}
{{- $subnetOutput := $.outputKeys.subnetName }}

{{- if hasKey $subnet "migrated" }}
{{- if not $subnet.migrated}}
{{- $workers = printf "%s-z%d" $workers $subnet.name }}
{{- $subnetName = printf "%s-z%d" $subnetName $subnet.name }}
{{- $subnetOutput = printf "%s%d" $.outputKeys.subnetNamePrefix $subnet.name }}
{{- end }}
{{- end }}

#===============================================
#= Subnets {{ $subnetName }}
#===============================================

resource "azurerm_subnet" "{{ $workers }}" {
  name                      = "{{ $subnetName }}"
{{- if $.create.vnet }}
  virtual_network_name      = azurerm_virtual_network.vnet.name
  resource_group_name       = azurerm_virtual_network.vnet.resource_group_name
{{- else }}
  virtual_network_name      = data.azurerm_virtual_network.vnet.name
  resource_group_name       = data.azurerm_virtual_network.vnet.resource_group_name
{{- end }}
  address_prefixes          = ["{{ $subnet.cidr }}"]
  service_endpoints         = [{{ range $index, $serviceEndpoint := $subnet.serviceEndpoints }}{{ if $index }},{{ end }}"{{$serviceEndpoint}}"{{end}}]
}

resource "azurerm_subnet_route_table_association" "{{ $workers }}-rt-subnet-association" {
  subnet_id      = azurerm_subnet.{{ $workers }}.id
  route_table_id = azurerm_route_table.workers.id
}

resource "azurerm_subnet_network_security_group_association" "{{ $workers }}-nsg-subnet-association" {
  subnet_id                 = azurerm_subnet.{{ $workers }}.id
  network_security_group_id = azurerm_network_security_group.workers.id
}

output "{{ $subnetOutput }}" {
  value = azurerm_subnet.{{ $workers }}.name
}

{{- if $subnet.natGateway.enabled }}
{{- $natName := "nat" }}
{{- $natResourceName := printf "%s-nat-gateway" $.clusterName }}

{{- if hasKey $subnet "migrated" }}
{{- if not $subnet.migrated }}
{{- $natName = printf "%s-z%d" $natName $subnet.name }}
{{- $natResourceName = printf "%s-z%d" $natResourceName $subnet.name }}
{{- end }}
{{- end }}
#===============================================
#= NAT Gateway {{ $natName }}
#===============================================
resource "azurerm_nat_gateway" "{{ $natName }}" {
  name                    = "{{ $natResourceName }}"
  location                = "{{ $.azure.region }}"
  resource_group_name     = {{ template "resource-group-reference" $ }}
  sku_name                = "Standard"
{{ if hasKey $subnet.natGateway "idleConnectionTimeoutMinutes" -}}
  idle_timeout_in_minutes = {{ $subnet.natGateway.idleConnectionTimeoutMinutes }}
{{- end }}
{{- if hasKey $subnet.natGateway "zone" }}
  zones = [{{ $subnet.natGateway.zone | quote }}]
{{- end }}
}

resource "azurerm_subnet_nat_gateway_association" "{{ $natName }}-worker-subnet-association" {
  subnet_id      = azurerm_subnet.{{ $workers }}.id
  nat_gateway_id = azurerm_nat_gateway.{{ $natName }}.id
}

{{ if and (hasKey $subnet.natGateway "ipAddresses") (hasKey $subnet.natGateway "zone") -}}
#===============================================
#= NAT Gateway User provided IP
#===============================================
{{ range $ipIndex, $ip := .natGateway.ipAddresses }}
data "azurerm_public_ip" "{{ $natName }}-ip-user-provided-{{ $ipIndex }}" {
  name                = "{{ $ip.name }}"
  resource_group_name = "{{ $ip.resourceGroup }}"
}

resource "azurerm_nat_gateway_public_ip_association" "{{ $natName }}-ip-user-provided-association-{{ $ipIndex }}" {
  nat_gateway_id       = azurerm_nat_gateway.{{ $natName }}.id
  public_ip_address_id = data.azurerm_public_ip.{{ $natName }}-ip-user-provided-{{ $ipIndex }}.id
}
{{ end }}
{{- else -}}
#===============================================
#= NAT Gateway managed IP
#===============================================
{{- $natIpName := printf "%s-ip" $natName}}
resource "azurerm_public_ip" "{{ $natIpName }}" {
  name                = "{{ $natResourceName }}-ip"
  location            = "{{ $.azure.region }}"
  resource_group_name = {{ template "resource-group-reference" $ }}
  allocation_method   = "Static"
  sku                 = "Standard"
{{- if hasKey .natGateway "zone" }}
  zones = [{{ .natGateway.zone | quote }}]
{{- end }}
}

resource "azurerm_nat_gateway_public_ip_association" "{{ $natName }}-ip-association" {
  nat_gateway_id       = azurerm_nat_gateway.{{ $natName }}.id
  public_ip_address_id = azurerm_public_ip.{{ $natIpName }}.id
}
{{- end }}
{{- end }}
{{- end }}

{{ if .identity -}}
#===============================================
#= Identity
#===============================================

data "azurerm_user_assigned_identity" "identity" {
  name                = "{{ .identity.name }}"
  resource_group_name = "{{ .identity.resourceGroup }}"
}
{{- end }}

{{ if .create.availabilitySet -}}
#===============================================
#= Availability Set
#===============================================

resource "azurerm_availability_set" "workers" {
  name                         = "{{ .clusterName }}-avset-workers"
  location                     = "{{ .azure.region }}"
  resource_group_name          = {{ template "resource-group-reference" $ }}
  platform_update_domain_count = "{{ .azure.countUpdateDomains }}"
  platform_fault_domain_count  = "{{ .azure.countFaultDomains }}"
  managed                      = true
}
{{- end}}


#===============================================
//= Output variables
#===============================================

output "{{ .outputKeys.resourceGroupName }}" {
  value = {{ template "resource-group-reference" $ }}
}

{{ if .create.vnet -}}
output "{{ .outputKeys.vnetName }}" {
  value = azurerm_virtual_network.vnet.name
}
{{- else -}}
output "{{ .outputKeys.vnetName }}" {
  value = data.azurerm_virtual_network.vnet.name
}
output "{{ .outputKeys.vnetResourceGroup }}" {
  value = data.azurerm_virtual_network.vnet.resource_group_name
}
{{- end}}

output "{{ .outputKeys.routeTableName }}" {
  value = azurerm_route_table.workers.name
}

output "{{ .outputKeys.securityGroupName }}" {
  value = azurerm_network_security_group.workers.name
}

{{ if .create.availabilitySet -}}
output "{{ .outputKeys.availabilitySetID }}" {
  value = azurerm_availability_set.workers.id
}

output "{{ .outputKeys.availabilitySetName }}" {
  value = azurerm_availability_set.workers.name
}

output "{{ .outputKeys.countFaultDomains }}" {
  value = azurerm_availability_set.workers.platform_fault_domain_count
}

output "{{ .outputKeys.countUpdateDomains }}" {
  value = azurerm_availability_set.workers.platform_update_domain_count
}
{{- end }}
{{ if .identity -}}
output "{{ .outputKeys.identityID }}" {
  value = data.azurerm_user_assigned_identity.identity.id
}

output "{{ .outputKeys.identityClientID }}" {
  value = data.azurerm_user_assigned_identity.identity.client_id
}
{{- end }}


{{- /* Helper functions */ -}}
{{- define "resource-group-reference" -}}
{{- if .create.resourceGroup -}}
azurerm_resource_group.rg.name
{{- else -}}
data.azurerm_resource_group.rg.name
{{- end}}
{{- end -}}
