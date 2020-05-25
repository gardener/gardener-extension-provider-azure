{{- define "cloud-provider-config" -}}
cloud: AZUREPUBLICCLOUD
location: "{{ .Values.region }}"
resourceGroup: "{{ .Values.resourceGroup }}"
routeTableName: "{{ .Values.routeTableName }}"
securityGroupName: "{{ .Values.securityGroupName }}"
subnetName: "{{ .Values.subnetName }}"
vnetName: "{{ .Values.vnetName }}"
{{- if hasKey .Values "vnetResourceGroup" }}
vnetResourceGroup: "{{ .Values.vnetResourceGroup }}"
{{- end }}
{{- if hasKey .Values "availabilitySetName" }}
primaryAvailabilitySetName: "{{ .Values.availabilitySetName }}"
loadBalancerSku: "basic"
{{- else }}
loadBalancerSku: "standard"
{{- end }}
cloudProviderBackoff: true
cloudProviderBackoffRetries: 6
cloudProviderBackoffExponent: 1.5
cloudProviderBackoffDuration: 5
cloudProviderBackoffJitter: 1.0
cloudProviderRateLimit: true
cloudProviderRateLimitQPS: {{ ( max .Values.maxNodes 10 ) }}
cloudProviderRateLimitBucket: 100
cloudProviderRateLimitQPSWrite: {{ ( max .Values.maxNodes 10 ) }}
cloudProviderRateLimitBucketWrite: 100
{{- if and (semverCompare ">= 1.14" .Values.kubernetesVersion) (semverCompare "< 1.18" .Values.kubernetesVersion) }}
cloudProviderBackoffMode: v2
{{- end }}
{{- end -}}
