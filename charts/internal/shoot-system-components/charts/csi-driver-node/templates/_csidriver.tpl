{{- define "csi-driver-node.csidriver" -}}
apiVersion: storage.k8s.io/v1
kind: CSIDriver
metadata:
  name: {{ include (print "csi-driver-node.provisioner-" .role) . }}
spec:
  attachRequired: true
  podInfoOnMount: false
{{- end -}}
