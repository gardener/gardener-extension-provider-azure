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
      # process_start_time_seconds is always emitted while the driver is up, so a healthy-but-idle
      # driver (no volume operations yet) is not flagged by the shoot Prometheus scrape:empty healthcheck.
      # The operation_* metrics are created lazily on the first CSI operation.
      regex: ^(azure{{ .role }}_csi_driver_operation_duration_seconds_labeled_(bucket|sum|count)|azure{{ .role }}_csi_driver_operations_total|process_start_time_seconds)$
{{- end -}}
