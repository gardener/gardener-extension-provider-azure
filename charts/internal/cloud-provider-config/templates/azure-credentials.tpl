{{- define "azure-credentials" -}}
aadClientId: "{{ .Values.aadClientId }}"
aadClientSecret: "{{ .Values.aadClientSecret }}"
{{- end -}}

{{- define "azure-subscription-info" -}}
tenantId: "{{ .Values.tenantId }}"
subscriptionId: "{{ .Values.subscriptionId }}"
{{- end -}}
