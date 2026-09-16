{{- define "csi-driver-controller.servicemonitor" -}}
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: shoot-csi-driver-controller-{{ .role }}
  namespace: {{ .Release.Namespace }}
  labels:
    prometheus: shoot
spec:
  selector:
    matchLabels:
      app: csi
      role: controller-{{ .role }}
  endpoints:
  - port: metrics
    relabelings:
    - action: labelmap
      regex: __meta_kubernetes_service_label_(.+)
    metricRelabelings:
    - sourceLabels:
      - __name__
      action: keep
      regex: ^(azure{{ .role }}_csi_driver_operation_duration_seconds_labeled_(bucket|sum|count)|azure{{ .role }}_csi_driver_operations_total)$
{{- end -}}
