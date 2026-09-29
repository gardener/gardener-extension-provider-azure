{{- define "cloud-provider-config-base" -}}
cloud: "{{ .Values.cloud }}"
location: "{{ .Values.region }}"
resourceGroup: "{{ .Values.resourceGroup }}"
routeTableName: "{{ .Values.routeTableName }}"
{{- if hasKey .Values "routeTableResourceGroup" }}
routeTableResourceGroup: "{{ .Values.routeTableResourceGroup }}"
{{- end }}
securityGroupName: "{{ .Values.securityGroupName }}"
{{- if hasKey .Values "securityGroupResourceGroup" }}
securityGroupResourceGroup: "{{ .Values.securityGroupResourceGroup }}"
{{- end }}
subnetName: "{{ .Values.subnetName }}"
vnetName: "{{ .Values.vnetName }}"
{{- if hasKey .Values "vnetResourceGroup" }}
vnetResourceGroup: "{{ .Values.vnetResourceGroup }}"
{{- end }}
loadBalancerSku: "standard"
{{- if .Values.disableOutboundSNAT }}
disableOutboundSNAT: true
{{- end }}
{{- if hasKey .Values "vmType" }}
vmType: "{{ .Values.vmType }}"
{{- end }}
cloudProviderBackoff: true
cloudProviderBackoffRetries: 6
cloudProviderBackoffExponent: 1.5
cloudProviderBackoffDuration: 5
cloudProviderBackoffJitter: 1.0
cloudProviderRateLimit: true
cloudProviderRateLimitQPS: {{ ( max .Values.maxNodes 10 ) }}
cloudProviderRateLimitBucket: {{ ( max .Values.maxNodes 100 ) }}
cloudProviderRateLimitQPSWrite: {{ ( max .Values.maxNodes 10 ) }}
cloudProviderRateLimitBucketWrite: {{ ( max .Values.maxNodes 100 ) }}
{{- end -}}

{{- define "cloud-provider-config" -}}
{{ include "azure-credentials" . }}
{{ include "azure-subscription-info" . }}
{{ include "cloud-provider-config-base" . }}
{{- end -}}

{{- define "cloud-provider-disk-config" -}}
{{ include "cloud-provider-config-base" . }}
{{ include "azure-subscription-info" . }}
useInstanceMetadata: true
{{- end -}}
