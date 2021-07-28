provider "azurerm" {
  subscription_id = var.SUBSCRIPTION_ID
  tenant_id       = var.TENANT_ID
  client_id       = var.CLIENT_ID
  client_secret   = var.CLIENT_SECRET

  skip_provider_registration = "true"
  features {}
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
  resource_group_name = {{ template "resource-group-reference" . }}
  location            = "{{ .azure.region }}"
  address_space       = ["{{ .resourceGroup.vnet.cidr }}"]
}
{{- else -}}
data "azurerm_virtual_network" "vnet" {
  name                = "{{ .resourceGroup.vnet.name }}"
  resource_group_name = "{{ .resourceGroup.vnet.resourceGroup }}"
}
{{- end }}

# Subnet
resource "azurerm_subnet" "workers" {
  name                      = "{{ .clusterName }}-nodes"
  {{ if .create.vnet -}}
  virtual_network_name      = azurerm_virtual_network.vnet.name
  resource_group_name       = azurerm_virtual_network.vnet.resource_group_name
  {{- else -}}
  virtual_network_name      = data.azurerm_virtual_network.vnet.name
  resource_group_name       = data.azurerm_virtual_network.vnet.resource_group_name
  {{- end }}
  address_prefixes          = ["{{ .networks.worker }}"]
  service_endpoints         = [{{range $index, $serviceEndpoint := .resourceGroup.subnet.serviceEndpoints}}{{if $index}},{{end}}"{{$serviceEndpoint}}"{{end}}]
}

# RouteTable
resource "azurerm_route_table" "workers" {
  name                = "worker_route_table"
  location            = "{{ .azure.region }}"
  resource_group_name = {{ template "resource-group-reference" . }}
}
resource "azurerm_subnet_route_table_association" "workers-rt-subnet-association" {
  subnet_id      = azurerm_subnet.workers.id
  route_table_id = azurerm_route_table.workers.id
}

# SecurityGroup
resource "azurerm_network_security_group" "workers" {
  name                = "{{ .clusterName }}-workers"
  location            = "{{ .azure.region }}"
  resource_group_name = {{ template "resource-group-reference" . }}
}
resource "azurerm_subnet_network_security_group_association" "workers-nsg-subnet-association" {
  subnet_id                 = azurerm_subnet.workers.id
  network_security_group_id = azurerm_network_security_group.workers.id
}

{{ if .create.natGateway -}}
#===============================================
#= NAT Gateway
#===============================================

resource "azurerm_nat_gateway" "nat" {
  name                    = "{{ .clusterName }}-nat-gateway"
  location                = "{{ .azure.region }}"
  resource_group_name     = {{ template "resource-group-reference" . }}
  sku_name                = "Standard"
  {{ if .natGateway -}}
  {{ if hasKey .natGateway "idleConnectionTimeoutMinutes" -}}
  idle_timeout_in_minutes = {{ .natGateway.idleConnectionTimeoutMinutes }}
  {{- end }}
  {{ if hasKey .natGateway "zone" -}}
  zones = [{{ .natGateway.zone | quote }}]
  {{- end }}
  {{ if .natGateway.migrateNatGatewayToIPAssociation -}}
  # TODO(natipmigration) This can be removed in future versions when the ip migration has been completed.
  public_ip_address_ids   = []
  {{- end }}
  {{- end }}
}
resource "azurerm_subnet_nat_gateway_association" "nat-worker-subnet-association" {
  subnet_id      = azurerm_subnet.workers.id
  nat_gateway_id = azurerm_nat_gateway.nat.id
}

{{ if .natGateway -}}
{{ if and (hasKey .natGateway "ipAddresses") (hasKey .natGateway "zone") -}}
{{ template "natgateway-user-provided-public-ips" . }}
{{- else -}}
{{ template "natgateway-managed-ip" . }}
{{- end }}
{{- else -}}
{{ template "natgateway-managed-ip" . }}
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
  resource_group_name          = {{ template "resource-group-reference" . }}
  platform_update_domain_count = "{{ .azure.countUpdateDomains }}"
  platform_fault_domain_count  = "{{ .azure.countFaultDomains }}"
  managed                      = true
}
{{- end}}

#===============================================
//= Output variables
#===============================================

output "{{ .outputKeys.resourceGroupName }}" {
  value = {{ template "resource-group-reference" . }}
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

output "{{ .outputKeys.subnetName }}" {
  value = azurerm_subnet.workers.name
}

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

{{- define "natgateway-managed-ip" -}}
# Gardener managed public IP to be attached to the NatGateway.
resource "azurerm_public_ip" "natip" {
name                = "{{ .clusterName }}-nat-ip"
location            = "{{ .azure.region }}"
resource_group_name = {{ template "resource-group-reference" . }}
allocation_method   = "Static"
sku                 = "Standard"
{{ if .natGateway -}}{{ if hasKey .natGateway "zone" -}}
zones = [{{ .natGateway.zone | quote }}]
{{- end }}{{- end }}
}
resource "azurerm_nat_gateway_public_ip_association" "natip-association" {
nat_gateway_id       = azurerm_nat_gateway.nat.id
public_ip_address_id = azurerm_public_ip.natip.id
}
{{- end -}}

{{- define "natgateway-user-provided-public-ips" -}}
# User provided public IPs to be attached to the NatGateway.
{{ range $index, $ip := .natGateway.ipAddresses -}}
data "azurerm_public_ip" "natip-user-provided-{{ $index }}" {
name                = "{{ $ip.name }}"
resource_group_name = "{{ $ip.resourceGroup }}"
}
resource "azurerm_nat_gateway_public_ip_association" "natip-user-provided-{{ $index }}-association" {
nat_gateway_id       = azurerm_nat_gateway.nat.id
public_ip_address_id = data.azurerm_public_ip.natip-user-provided-{{ $index }}.id
}
{{ end }}
{{- end -}}
