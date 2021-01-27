{{- define "resource-group-reference" -}}
{{- if .Values.create.resourceGroup -}}
azurerm_resource_group.rg.name
{{- else -}}
data.azurerm_resource_group.rg.name
{{- end}}
{{- end -}}

{{- define "natgateway-managed-ip" -}}
# Gardener managed public IP to be attached to the NatGateway.
resource "azurerm_public_ip" "natip" {
  name                = "{{ required "clusterName is required" .Values.clusterName }}-nat-ip"
  location            = "{{ required "azure.region is required" .Values.azure.region }}"
  resource_group_name = {{ template "resource-group-reference" . }}
  allocation_method   = "Static"
  sku                 = "Standard"
  {{ if .Values.natGateway -}}{{ if hasKey .Values.natGateway "zone" -}}
  zones = [{{ .Values.natGateway.zone | quote }}]
  {{- end }}{{- end }}
}
resource "azurerm_nat_gateway_public_ip_association" "natip-association" {
  nat_gateway_id       = azurerm_nat_gateway.nat.id
  public_ip_address_id = azurerm_public_ip.natip.id
}
{{- end -}}

{{- define "natgateway-user-provided-public-ips" -}}
# User provided public IPs to be attached to the NatGateway.
{{ range $index, $ip := .Values.natGateway.ipAddresses -}}
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
