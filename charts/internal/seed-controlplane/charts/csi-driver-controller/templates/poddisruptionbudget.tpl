{{- define "csi-driver-controller.poddisruptionbudget" -}}
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: csi-driver-controller-{{ .role }}
  namespace: {{ .Release.Namespace }}
  labels:
    app: csi
    role: controller-{{ .role }}
spec:
  maxUnavailable: 1
  selector:
    matchLabels:
      app: csi
      role: controller-{{ .role }}
{{- if semverCompare ">= 1.26-0" .Capabilities.KubeVersion.Version }}
  unhealthyPodEvictionPolicy: AlwaysAllow
{{- end }}
{{- end -}}
