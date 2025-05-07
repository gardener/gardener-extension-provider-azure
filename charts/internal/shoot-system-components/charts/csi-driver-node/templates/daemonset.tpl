{{- define "csi-driver-node.daemonset.ports.healthz.disk" -}}
29603
{{- end -}}
{{- define "csi-driver-node.daemonset.ports.metrics.disk" -}}
29605
{{- end -}}

{{- define "csi-driver-node.daemonset.ports.healthz.file" -}}
29613
{{- end -}}
{{- define "csi-driver-node.daemonset.ports.metrics.file" -}}
29615
{{- end -}}

{{- define "csi-driver-node.daemonset" -}}
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: csi-driver-node-{{ .role }}
  namespace: {{ .Release.Namespace }}
  labels:
    node.gardener.cloud/critical-component: "true"
    app: csi
    role: driver-{{ .role }}
spec:
  selector:
    matchLabels:
      app: csi
      role: driver-{{ .role }}
  template:
    metadata:
      annotations:
        node.gardener.cloud/wait-for-csi-node-azure: {{ include (print "csi-driver-node.provisioner-" .role) . }}
{{- if .Values.podAnnotations }}
{{ toYaml .Values.podAnnotations | indent 8 }}
{{- end }}
      labels:
        node.gardener.cloud/critical-component: "true"
        app: csi
        role: driver-{{ .role }}
    spec:
      hostNetwork: true
      priorityClassName: system-node-critical
      serviceAccount: csi-driver-node-{{ .role }}
      tolerations:
      - effect: NoSchedule
        operator: Exists
      - key: CriticalAddonsOnly
        operator: Exists
      - effect: NoExecute
        operator: Exists
      securityContext:
        seccompProfile:
          type: RuntimeDefault
      containers:
      - name: csi-driver
        image: {{ index .Values.images (print "csi-driver-" .role) }}
        args:
        - --endpoint=$(CSI_ENDPOINT)
        - --nodeid=$(KUBE_NODE_NAME)
        - --metrics-address=0.0.0.0:{{ include (print "csi-driver-node.daemonset.ports.metrics." .role) . }}
        - --v=5
        env:
        - name: CSI_ENDPOINT
          value: unix://{{ .Values.socketPath }}
        - name: AZURE_CREDENTIAL_FILE
          value: /etc/kubernetes/cloudprovider/cloudprovider.conf
        - name: KUBE_NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
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
        securityContext:
          privileged: true
        ports:
        - containerPort: {{ include (print "csi-driver-node.daemonset.ports.healthz." .role) . }}
          name: healthz
          protocol: TCP
        - containerPort: {{ include (print "csi-driver-node.daemonset.ports.metrics." .role) . }}
          name: metrics
          protocol: TCP
        livenessProbe:
          failureThreshold: 5
          httpGet:
            path: /healthz
            port: healthz
          initialDelaySeconds: 30
          timeoutSeconds: 10
          periodSeconds: 30
        volumeMounts:
        - name: kubelet-dir
          mountPath: /var/lib/kubelet
          mountPropagation: "Bidirectional"
        - name: plugin-dir
          mountPath: /csi
        - name: device-dir
          mountPath: /dev
        - name: cloud-provider-config
          mountPath: /etc/kubernetes/cloudprovider
        {{- if eq .role "disk" }}
        - name: sys-devices-dir
          mountPath: /sys/bus/scsi/devices
        - name: scsi-host-dir
          mountPath: /sys/class/scsi_host/
        {{- end }}

      - name: csi-node-driver-registrar
        image: {{ index .Values.images "csi-node-driver-registrar" }}
        args:
        - --csi-address=$(ADDRESS)
        - --kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)
        - --v=5
        env:
        - name: ADDRESS
          value: {{ .Values.socketPath }}
        - name: DRIVER_REG_SOCK_PATH
          value: /var/lib/kubelet/plugins/{{ include (print "csi-driver-node.provisioner-" .role) . }}/csi.sock
{{- if .Values.resources.nodeDriverRegistrar }}
        resources:
{{ toYaml .Values.resources.nodeDriverRegistrar | indent 10 }}
{{- end }}
        securityContext:
          allowPrivilegeEscalation: false
        volumeMounts:
        - name: plugin-dir
          mountPath: /csi
        - name: registration-dir
          mountPath: /registration

      - name: csi-liveness-probe
        image: {{ index .Values.images "csi-liveness-probe" }}
        args:
        - --csi-address={{ .Values.socketPath }}
        - --probe-timeout=3s
        - --health-port={{ include (print "csi-driver-node.daemonset.ports.healthz." .role) . }}
        - --v=5
{{- if .Values.resources.livenessProbe }}
        resources:
{{ toYaml .Values.resources.livenessProbe | indent 10 }}
{{- end }}
        securityContext:
          allowPrivilegeEscalation: false
        volumeMounts:
        - name: plugin-dir
          mountPath: /csi

      volumes:
      - name: kubelet-dir
        hostPath:
          path: /var/lib/kubelet
          type: DirectoryOrCreate
      - name: plugin-dir
        hostPath:
          path: /var/lib/kubelet/plugins/{{ include (print "csi-driver-node.provisioner-" .role) . }}
          type: DirectoryOrCreate
      - name: registration-dir
        hostPath:
          path: /var/lib/kubelet/plugins_registry/
          type: DirectoryOrCreate
      - name: device-dir
        hostPath:
          path: /dev
          type: Directory
      - name: cloud-provider-config
        configMap:
          name: cloud-provider-disk-config
      {{- if eq .role "disk" }}
      - name: sys-devices-dir
        hostPath:
          path: /sys/bus/scsi/devices
          type: Directory
      - name: scsi-host-dir
        hostPath:
          path: /sys/class/scsi_host/
          type: Directory
      {{- end }}
{{- end -}}
