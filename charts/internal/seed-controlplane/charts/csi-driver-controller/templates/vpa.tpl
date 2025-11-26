{{- define "csi-driver-controller.vpa" -}}
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: csi-driver-controller-{{ .role }}-vpa
  namespace: {{ .Release.Namespace }}
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: azure-csi-driver
      controlledValues: RequestsOnly
      controlledResources: ["memory"]
    - containerName: azure-csi-provisioner
      controlledValues: RequestsOnly
      controlledResources: ["memory"]
    - containerName: azure-csi-attacher
      controlledValues: RequestsOnly
      controlledResources: ["memory"]
    - containerName: azure-csi-snapshotter
      controlledValues: RequestsOnly
      controlledResources: ["memory"]
    - containerName: azure-csi-resizer
      controlledValues: RequestsOnly
      controlledResources: ["memory"]
    - containerName: azure-csi-liveness-probe
      controlledValues: RequestsOnly
      controlledResources: ["memory"]
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: csi-driver-controller-{{ .role }}
  updatePolicy:
    updateMode: Recreate
{{- end -}}
