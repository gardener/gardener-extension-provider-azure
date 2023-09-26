{{- define "csi-driver-node.csidriver" -}}
apiVersion: {{ include "csi-driver-node.storageversion" . }}
kind: CSIDriver
metadata:
  name: {{ include (print "csi-driver-node.provisioner-" .role) . }}
spec:
  attachRequired: true
  podInfoOnMount: false
{{- end -}}
