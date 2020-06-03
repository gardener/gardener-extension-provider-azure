{{- define "csi-driver-controller.deployment" -}}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: csi-driver-controller-{{ .role }}
  namespace: {{ .Release.Namespace }}
  labels:
    app: csi
    role: controller-{{ .role }}
spec:
  replicas: {{ .Values.replicas }}
  revisionHistoryLimit: 0
  selector:
    matchLabels:
      app: csi
      role: controller-{{ .role }}
  strategy:
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 25%
    type: RollingUpdate
  template:
    metadata:
{{- if .Values.podAnnotations }}
      annotations:
{{ toYaml .Values.podAnnotations | indent 8 }}
{{- end }}
      creationTimestamp: null
      labels:
        app: csi
        role: controller-{{ .role }}
        networking.gardener.cloud/to-dns: allowed
        networking.gardener.cloud/to-public-networks: allowed
        networking.gardener.cloud/to-shoot-apiserver: allowed
    spec:
      containers:
      - name: csi-driver
        image: {{ index .Values.images (print "csi-driver-" .role) }}
        imagePullPolicy: IfNotPresent
        args :
        - --endpoint=$(CSI_ENDPOINT)
        {{- if eq .role "file" }}
        - --nodeid=dummy
        - --kubeconfig=/var/lib/csi-driver-controller-file/kubeconfig
        {{- end }}
        - --v=5
        env:
        - name: CSI_ENDPOINT
          value: unix://{{ .Values.socketPath }}/csi.sock
        - name: AZURE_CREDENTIAL_FILE
          value: /etc/kubernetes/cloudprovider/cloudprovider.conf
{{- if .Values.resources.driver }}
        resources:
{{ toYaml .Values.resources.driver | indent 10 }}
{{- end }}
        ports:
        - name: healthz
          containerPort: 9808
          protocol: TCP
        livenessProbe:
          failureThreshold: 5
          httpGet:
            path: /healthz
            port: healthz
            scheme: HTTP
          initialDelaySeconds: 10
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 3
        volumeMounts:
        - name: socket-dir
          mountPath: {{ .Values.socketPath }}
        {{- if eq .role "file" }}
        - name: csi-driver-controller-file
          mountPath: /var/lib/csi-driver-controller-file
        {{- end }}
        - name: cloud-provider-config
          mountPath: /etc/kubernetes/cloudprovider

      - name: csi-provisioner
        image: {{ index .Values.images "csi-provisioner" }}
        imagePullPolicy: IfNotPresent
        args:
        - --csi-address=$(ADDRESS)
        - --enable-leader-election
        - --feature-gates=Topology=true
        - --leader-election-type=leases
        - --leader-election-namespace=kube-system
        - --kubeconfig=/var/lib/csi-provisioner/kubeconfig
        - --timeout=120s
        - --volume-name-prefix=pv-{{ .Release.Namespace }}
        - --v=5
        env:
        - name: ADDRESS
          value: {{ .Values.socketPath }}/csi.sock
{{- if .Values.resources.provisioner }}
        resources:
{{ toYaml .Values.resources.provisioner | indent 10 }}
{{- end }}
        volumeMounts:
        - name: socket-dir
          mountPath: {{ .Values.socketPath }}
        - name: csi-provisioner
          mountPath: /var/lib/csi-provisioner

      - name: csi-attacher
        image: {{ index .Values.images "csi-attacher" }}
        imagePullPolicy: IfNotPresent
        args:
        - --csi-address=$(ADDRESS)
        - --kubeconfig=/var/lib/csi-attacher/kubeconfig
        - --leader-election
        - --leader-election-namespace=kube-system
        - --v=5
        env:
        - name: ADDRESS
          value: {{ .Values.socketPath }}/csi.sock
{{- if .Values.resources.attacher }}
        resources:
{{ toYaml .Values.resources.attacher | indent 10 }}
{{- end }}
        volumeMounts:
        - name: socket-dir
          mountPath: {{ .Values.socketPath }}
        - name: csi-attacher
          mountPath: /var/lib/csi-attacher

      - name: csi-snapshotter
        image: {{ index .Values.images "csi-snapshotter" }}
        imagePullPolicy: IfNotPresent
        args:
        - --csi-address=$(CSI_ENDPOINT)
        - --kubeconfig=/var/lib/csi-snapshotter/kubeconfig
        - --leader-election
        - --leader-election-namespace=kube-system
        - --snapshot-name-prefix={{ .Release.Namespace }}
        env:
        - name: CSI_ENDPOINT
          value: {{ .Values.socketPath }}/csi.sock
{{- if .Values.resources.snapshotter }}
        resources:
{{ toYaml .Values.resources.snapshotter | indent 10 }}
{{- end }}
        volumeMounts:
        - name: socket-dir
          mountPath: {{ .Values.socketPath }}
        - name: csi-snapshotter
          mountPath: /var/lib/csi-snapshotter

      - name: csi-resizer
        image: {{ index .Values.images "csi-resizer" }}
        imagePullPolicy: IfNotPresent
        args:
        - --csi-address=$(ADDRESS)
        - --kubeconfig=/var/lib/csi-resizer/kubeconfig
        - --leader-election=true
        - --leader-election-namespace=kube-system
        - --v=5
        env:
        - name: ADDRESS
          value: {{ .Values.socketPath }}/csi.sock
{{- if .Values.resources.resizer }}
        resources:
{{ toYaml .Values.resources.resizer | indent 10 }}
{{- end }}
        volumeMounts:
        - name: socket-dir
          mountPath: {{ .Values.socketPath }}
        - name: csi-resizer
          mountPath: /var/lib/csi-resizer

      - name: csi-liveness-probe
        image: {{ index .Values.images "csi-liveness-probe" }}
        args:
        - --csi-address=/csi/csi.sock
{{- if .Values.resources.livenessProbe }}
        resources:
{{ toYaml .Values.resources.livenessProbe | indent 10 }}
{{- end }}
        volumeMounts:
        - name: socket-dir
          mountPath: /csi

      volumes:
      - name: socket-dir
        emptyDir: {}
      - name: cloud-provider-config
        secret:
          secretName: cloud-provider-config
      {{- if eq .role "file" }}
      - name: csi-driver-controller-file
        secret:
          secretName: csi-driver-controller-file
      {{- end }}
      - name: csi-provisioner
        secret:
          secretName: csi-provisioner
      - name: csi-attacher
        secret:
          secretName: csi-attacher
      - name: csi-snapshotter
        secret:
          secretName: csi-snapshotter
      - name: csi-resizer
        secret:
          secretName: csi-resizer
{{- end -}}
