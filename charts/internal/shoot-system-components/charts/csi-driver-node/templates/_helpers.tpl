{{- define "csi-driver-node.extensionsGroup" -}}
extensions.gardener.cloud
{{- end -}}

{{- define "csi-driver-node.name" -}}
provider-azure
{{- end -}}

{{- define "csi-driver-node.provisioner-disk" -}}
disk.csi.azure.com
{{- end -}}

{{- define "csi-driver-node.provisioner-file" -}}
file.csi.azure.com
{{- end -}}

{{- define "csi-driver-node.storageversion" -}}
{{- if semverCompare "<= 1.18.x" .Values.kubernetesVersion -}}
storage.k8s.io/v1beta1
{{- else -}}
storage.k8s.io/v1
{{- end -}}
{{- end -}}
