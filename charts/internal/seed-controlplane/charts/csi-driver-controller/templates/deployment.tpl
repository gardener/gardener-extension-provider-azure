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
  revisionHistoryLimit: 1
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
        gardener.cloud/role: controlplane
        high-availability-config.resources.gardener.cloud/type: controller
        networking.gardener.cloud/to-dns: allowed
        networking.gardener.cloud/to-public-networks: allowed
        networking.resources.gardener.cloud/to-kube-apiserver-tcp-443: allowed
    spec:
      automountServiceAccountToken: false
      priorityClassName: gardener-system-300
      containers:
      - name: azure-csi-driver
        image: {{ index .Values.images (print "csi-driver-" .role) }}
        imagePullPolicy: IfNotPresent
        args :
        - --endpoint=$(CSI_ENDPOINT)
        {{- if eq .role "disk" }}
        - --kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig
        {{- if hasKey .Values "vmType" }}
        {{- if eq .Values.vmType "vmss" }}
        - --disable-avset-nodes=false
        {{- end }}
        {{- end }}
        {{- end }}
        {{- if eq .role "file" }}
        - --nodeid=dummy
        - --kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig
        {{- end }}
        - --v=5
        env:
        - name: CSI_ENDPOINT
          value: unix://{{ .Values.socketPath }}/csi.sock
        - name: AZURE_CREDENTIAL_FILE
          value: /etc/kubernetes/cloudprovider/cloudprovider.conf
        {{- if eq .role "disk" }}
        {{- if .Values.resources.csiDriverDisk }}
        resources:
{{ toYaml .Values.resources.csiDriverDisk  | indent 10 }}
        {{- end }}
        {{- end }}
        {{- if eq .role "file" }}
        {{- if .Values.resources.csiDriverFile }}
        resources:
{{ toYaml .Values.resources.csiDriverFile | indent 10 }}
        {{- end }}
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
        {{- if eq .role "disk" }}
        - mountPath: /var/run/secrets/gardener.cloud/shoot/generic-kubeconfig
          name: kubeconfig-csi-driver-controller-disk
          readOnly: true
        {{- end }}
        {{- if eq .role "file" }}
        - mountPath: /var/run/secrets/gardener.cloud/shoot/generic-kubeconfig
          name: kubeconfig-csi-driver-controller-file
          readOnly: true
        {{- end }}
        {{- if .Values.useWorkloadIdentity }}
        - name: cloudprovider
          mountPath: /var/run/secrets/gardener.cloud/workload-identity
          readOnly: true
        {{- end }}
        - name: cloud-provider-config
          mountPath: /etc/kubernetes/cloudprovider

      - name: azure-csi-provisioner
        image: {{ index .Values.images "csi-provisioner" }}
        imagePullPolicy: IfNotPresent
        args:
        - --csi-address=$(ADDRESS)
        - --feature-gates=Topology=true
        - --leader-election=true
        - --leader-election-namespace=kube-system
        - --kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig
        - --timeout=120s
        - --volume-name-prefix=pv-{{ .Release.Namespace }}
        - --default-fstype=ext4
        - --extra-create-metadata=true
        {{- if ((.Values.csiProvisioner).featureGates) }}
        - --feature-gates={{ range $feature, $enabled := .Values.csiProvisioner.featureGates }}{{ $feature }}={{ $enabled }},{{ end }}
        {{- end }}
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
        - mountPath: /var/run/secrets/gardener.cloud/shoot/generic-kubeconfig
          name: kubeconfig-csi-provisioner
          readOnly: true

      - name: azure-csi-attacher
        image: {{ index .Values.images "csi-attacher" }}
        imagePullPolicy: IfNotPresent
        args:
        - --csi-address=$(ADDRESS)
        - --kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig
        - --leader-election
        - --leader-election-namespace=kube-system
        - --v=5
        - --timeout=1200s
        - --worker-threads=500
        - --kube-api-qps=50
        - --kube-api-burst=100
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
        - mountPath: /var/run/secrets/gardener.cloud/shoot/generic-kubeconfig
          name: kubeconfig-csi-attacher
          readOnly: true

      - name: azure-csi-snapshotter
        image: {{ index .Values.images "csi-snapshotter" }}
        imagePullPolicy: IfNotPresent
        args:
        - --csi-address=$(CSI_ENDPOINT)
        - --kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig
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
        - mountPath: /var/run/secrets/gardener.cloud/shoot/generic-kubeconfig
          name: kubeconfig-csi-snapshotter
          readOnly: true

      - name: azure-csi-resizer
        image: {{ index .Values.images "csi-resizer" }}
        imagePullPolicy: IfNotPresent
        args:
        - --csi-address=$(ADDRESS)
        - --kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig
        - --leader-election=true
        - --leader-election-namespace=kube-system
        - --v=5
        - --handle-volume-inuse-error=false
        {{- if ((.Values.csiResizer).featureGates) }}
        - --feature-gates={{ range $feature, $enabled := .Values.csiResizer.featureGates }}{{ $feature }}={{ $enabled }},{{ end }}
        {{- end }}
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
        - mountPath: /var/run/secrets/gardener.cloud/shoot/generic-kubeconfig
          name: kubeconfig-csi-resizer
          readOnly: true

      - name: azure-csi-liveness-probe
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
      {{- if .Values.useWorkloadIdentity }}
      - name: cloudprovider
        projected:
          defaultMode: 420
          sources:
          - secret:
              items:
                - key: token
                  path: token
              name: cloudprovider
              optional: false
      {{- end }}
      - name: kubeconfig-csi-driver-controller-{{ .role }}
        projected:
          defaultMode: 420
          sources:
          - secret:
              items:
              - key: kubeconfig
                path: kubeconfig
              name: {{ .Values.global.genericTokenKubeconfigSecretName }}
              optional: false
          - secret:
              items:
              - key: token
                path: token
              name: shoot-access-csi-driver-controller-{{ .role }}
              optional: false
      - name: kubeconfig-csi-attacher
        projected:
          defaultMode: 420
          sources:
          - secret:
              items:
              - key: kubeconfig
                path: kubeconfig
              name: {{ .Values.global.genericTokenKubeconfigSecretName }}
              optional: false
          - secret:
              items:
              - key: token
                path: token
              name: shoot-access-csi-attacher
              optional: false
      - name: kubeconfig-csi-provisioner
        projected:
          defaultMode: 420
          sources:
          - secret:
              items:
              - key: kubeconfig
                path: kubeconfig
              name: {{ .Values.global.genericTokenKubeconfigSecretName }}
              optional: false
          - secret:
              items:
              - key: token
                path: token
              name: shoot-access-csi-provisioner
              optional: false
      - name: kubeconfig-csi-snapshotter
        projected:
          defaultMode: 420
          sources:
          - secret:
              items:
              - key: kubeconfig
                path: kubeconfig
              name: {{ .Values.global.genericTokenKubeconfigSecretName }}
              optional: false
          - secret:
              items:
              - key: token
                path: token
              name: shoot-access-csi-snapshotter
              optional: false
      - name: kubeconfig-csi-resizer
        projected:
          defaultMode: 420
          sources:
          - secret:
              items:
              - key: kubeconfig
                path: kubeconfig
              name: {{ .Values.global.genericTokenKubeconfigSecretName }}
              optional: false
          - secret:
              items:
              - key: token
                path: token
              name: shoot-access-csi-resizer
              optional: false
{{- end -}}
