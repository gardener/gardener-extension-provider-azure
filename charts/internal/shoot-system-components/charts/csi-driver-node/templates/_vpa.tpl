{{- define "csi-driver-node.vpa" -}}
{{- if .Values.vpaEnabled -}}
apiVersion: "autoscaling.k8s.io/v1beta2"
kind: VerticalPodAutoscaler
metadata:
  name: csi-driver-node-{{ .role }}
  namespace: {{ .Release.Namespace }}
spec:
  resourcePolicy:
    containerPolicies:
    - containerName: '*'
      minAllowed:
        memory: 25Mi
  targetRef:
    apiVersion: apps/v1
    kind: DaemonSet
    name: csi-driver-node-{{ .role }}
  updatePolicy:
    updateMode: "Auto"
{{- end -}}
{{- end -}}
