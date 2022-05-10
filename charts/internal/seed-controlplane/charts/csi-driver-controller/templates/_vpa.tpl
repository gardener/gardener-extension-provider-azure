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
    - containerName: azure-csi-provisioner
      minAllowed:
        memory: {{ .Values.resources.provisioner.requests.memory }}
      controlledValues: RequestsOnly
    - containerName: azure-csi-attacher
      minAllowed:
        memory: {{ .Values.resources.attacher.requests.memory }}
      controlledValues: RequestsOnly
    - containerName: azure-csi-snapshotter
      minAllowed:
        memory: {{ .Values.resources.snapshotter.requests.memory }}
      controlledValues: RequestsOnly
    - containerName: azure-csi-resizer
      minAllowed:
        memory: {{ .Values.resources.resizer.requests.memory }}
      controlledValues: RequestsOnly
    - containerName: azure-csi-liveness-probe
      minAllowed:
        memory: {{ .Values.resources.livenessProbe.requests.memory }}
      controlledValues: RequestsOnly
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: csi-driver-controller-{{ .role }}
  updatePolicy:
    updateMode: Auto
{{- end -}}
