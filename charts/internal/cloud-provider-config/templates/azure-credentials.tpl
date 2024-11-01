{{- define "azure-credentials" -}}
aadClientId: "{{ .Values.aadClientId }}"
{{- if .Values.useWorkloadIdentity }}
aadFederatedTokenFile: "/var/run/secrets/gardener.cloud/workload-identity/token"
useFederatedWorkloadIdentityExtension: true
{{- else }}
aadClientSecret: "{{ .Values.aadClientSecret }}"
{{- end }}
{{- end -}}

{{- define "azure-subscription-info" -}}
tenantId: "{{ .Values.tenantId }}"
subscriptionId: "{{ .Values.subscriptionId }}"
{{- end -}}
