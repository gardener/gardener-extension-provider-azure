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
      {{- end }}
      {{- end }}
      {{- if eq .role "file" }}
      {{- if .Values.resources.csiDriverFile  }}
        memory: {{ .Values.resources.csiDriverFile.requests.memory }}
      {{- end }}
      {{- end }}
      maxAllowed:
      {{- if eq .role "disk" }}
      {{- if .Values.vpa.resourcePolicy.csiDriverDisk }}
        memory: {{ .Values.vpa.resourcePolicy.csiDriverDisk.maxAllowed.memory }}
        cpu: {{ .Values.vpa.resourcePolicy.csiDriverDisk.maxAllowed.cpu }}
      {{- end }}
      {{- end }}
      {{- if eq .role "file" }}
      {{- if .Values.resources.csiDriverFile  }}
        memory: {{ .Values.vpa.resourcePolicy.csiDriverFile.maxAllowed.memory }}
        cpu: {{ .Values.vpa.resourcePolicy.csiDriverFile.maxAllowed.cpu }}
      {{- end }}
      {{- end }}
      controlledValues: RequestsOnly
    - containerName: csi-node-driver-registrar
      minAllowed:
        memory: {{ .Values.resources.nodeDriverRegistrar.requests.memory }}
      maxAllowed:
        cpu: {{ .Values.vpa.resourcePolicy.nodeDriverRegistrar.maxAllowed.cpu }}
        memory: {{ .Values.vpa.resourcePolicy.nodeDriverRegistrar.maxAllowed.memory }}
      controlledValues: RequestsOnly
    - containerName: csi-liveness-probe
      minAllowed:
        memory: {{ .Values.resources.livenessProbe.requests.memory }}
      maxAllowed:
        cpu: {{ .Values.vpa.resourcePolicy.livenessProbe.maxAllowed.cpu }}
        memory: {{ .Values.vpa.resourcePolicy.livenessProbe.maxAllowed.memory }}
      controlledValues: RequestsOnly
  targetRef:
    apiVersion: apps/v1
    kind: DaemonSet
    name: csi-driver-node-{{ .role }}
  updatePolicy:
    updateMode: "Auto"
{{- end -}}
{{- end -}}
