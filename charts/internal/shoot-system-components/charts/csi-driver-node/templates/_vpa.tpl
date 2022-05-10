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
      minAllowed:
      {{- if eq .role "disk" }}
      {{- if .Values.resources.csiDriverDisk }}
        memory: {{ .Values.resources.csiDriverDisk.requests.memory }}
        cpu: {{ .Values.resources.csiDriverDisk.requests.cpu }}
      {{- end }}
      {{- end }}
      {{- if eq .role "file" }}
      {{- if .Values.resources.csiDriverFile  }}
        memory: {{ .Values.resources.csiDriverFile.requests.memory }}
        cpu: {{ .Values.resources.csiDriverFile.requests.cpu }}
      {{- end }}
      {{- end }}
      controlledValues: RequestsOnly
    - containerName: csi-node-driver-registrar
      minAllowed:
        cpu: {{ .Values.resources.nodeDriverRegistrar.requests.cpu }}
        memory: {{ .Values.resources.nodeDriverRegistrar.requests.memory }}
      controlledValues: RequestsOnly
    - containerName: csi-liveness-probe
      minAllowed:
        cpu: {{ .Values.resources.livenessProbe.requests.cpu }}
        memory: {{ .Values.resources.livenessProbe.requests.memory }}
      controlledValues: RequestsOnly
  targetRef:
    apiVersion: apps/v1
    kind: DaemonSet
    name: csi-driver-node-{{ .role }}
  updatePolicy:
    updateMode: "Auto"
{{- end -}}
{{- end -}}
