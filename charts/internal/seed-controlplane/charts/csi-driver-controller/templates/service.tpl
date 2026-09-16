{{- define "csi-driver-controller.service" -}}
---
apiVersion: v1
kind: Service
metadata:
  name: csi-driver-controller-{{ .role }}
  namespace: {{ .Release.Namespace }}
  labels:
    app: csi
    role: controller-{{ .role }}
  annotations:
    networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports: '[{"port":{{ .Values.metricsPort }},"protocol":"TCP"}]'
spec:
  type: ClusterIP
  clusterIP: None
  ports:
  - name: metrics
    port: {{ .Values.metricsPort }}
    protocol: TCP
  selector:
    app: csi
    role: controller-{{ .role }}
{{- end -}}
