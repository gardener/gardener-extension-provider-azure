{{- define "csi-driver-node.vpa" -}}
{{- if .Values.global.vpaEnabled -}}
apiVersion: "autoscaling.k8s.io/v1"
kind: VerticalPodAutoscaler
metadata:
  name: csi-driver-node-{{ .role }}
  namespace: {{ .Release.Namespace }}
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: csi-driver
      controlledValues: RequestsOnly
    - containerName: csi-node-driver-registrar
      controlledValues: RequestsOnly
    - containerName: csi-liveness-probe
      controlledValues: RequestsOnly
  targetRef:
    apiVersion: apps/v1
    kind: DaemonSet
    name: csi-driver-node-{{ .role }}
  updatePolicy:
    updateMode: "Auto"
{{- end -}}
{{- end -}}
