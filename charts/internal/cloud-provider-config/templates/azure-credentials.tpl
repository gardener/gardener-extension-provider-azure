{{- define "azure-credentials"}}
aadClientId: "{{ .Values.aadClientId }}"
aadClientSecret: "{{ .Values.aadClientSecret }}"
tenantId: "{{ .Values.tenantId }}"
subscriptionId: "{{ .Values.subscriptionId }}"
{{- end }}
