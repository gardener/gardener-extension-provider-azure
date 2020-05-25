{{- define "csi-driver-controller.vpa" -}}
---
apiVersion: autoscaling.k8s.io/v1beta2
kind: VerticalPodAutoscaler
metadata:
  name: csi-driver-controller-{{ .role }}-vpa
  namespace: {{ .Release.Namespace }}
spec:
  resourcePolicy:
    containerPolicies:
      - containerName: '*'
        minAllowed:
          memory: 25Mi
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: csi-driver-controller-{{ .role }}
  updatePolicy:
    updateMode: Auto
{{- end -}}
