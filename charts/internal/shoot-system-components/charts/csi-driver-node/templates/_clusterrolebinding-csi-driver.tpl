{{- define "csi-driver-node.clusterrolebinding.csi-driver-node" -}}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ include "csi-driver-node.extensionsGroup" . }}:{{ include "csi-driver-node.name" . }}:csi-driver-{{ .role }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{ include "csi-driver-node.extensionsGroup" . }}:{{ include "csi-driver-node.name" . }}:csi-driver
subjects:
- kind: ServiceAccount
  name: csi-driver-node-{{ .role }}
  namespace: {{ .Release.Namespace }}
{{- end -}}
