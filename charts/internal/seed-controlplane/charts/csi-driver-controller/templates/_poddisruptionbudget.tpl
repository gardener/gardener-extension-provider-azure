{{- define "csi-driver-controller.poddisruptionbudget" -}}
---
{{- if semverCompare ">= 1.21-0" .Capabilities.KubeVersion.GitVersion }}
apiVersion: policy/v1
{{- else }}
apiVersion: policy/v1beta1
{{- end }}
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
{{- end -}}
