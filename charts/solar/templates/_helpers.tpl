{{/*
Expand the name of the chart.
*/}}
{{- define "solar.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "solar.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "solar.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "solar.labels" -}}
helm.sh/chart: {{ include "solar.chart" . }}
{{ include "solar.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "solar.selectorLabels" -}}
app.kubernetes.io/name: {{ include "solar.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Namespace
*/}}
{{- define "solar.namespace" -}}
{{- if .Values.namespaceOverride }}
{{- .Values.namespaceOverride }}
{{- else }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}

{{/*
API Server fullname
*/}}
{{- define "solar.apiserver.fullname" -}}
{{- printf "%s-apiserver" (include "solar.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
API Server component labels
*/}}
{{- define "solar.apiserver.labels" -}}
{{ include "solar.labels" . }}
app.kubernetes.io/component: apiserver
app.kubernetes.io/part-of: solar
{{- end }}

{{/*
API Server selector labels
*/}}
{{- define "solar.apiserver.selectorLabels" -}}
{{ include "solar.selectorLabels" . }}
app.kubernetes.io/component: apiserver
{{- end }}

{{/*
API Server service account name
*/}}
{{- define "solar.apiserver.serviceAccountName" -}}
{{- if .Values.apiserver.serviceAccount.create }}
{{- default (include "solar.apiserver.fullname" .) .Values.apiserver.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.apiserver.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Controller fullname
*/}}
{{- define "solar.controller.fullname" -}}
{{- printf "%s-controller-manager" (include "solar.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Controller component labels
*/}}
{{- define "solar.controller.labels" -}}
{{ include "solar.labels" . }}
app.kubernetes.io/component: controller-manager
app.kubernetes.io/part-of: solar
{{- end }}

{{/*
Controller selector labels
*/}}
{{- define "solar.controller.selectorLabels" -}}
{{ include "solar.selectorLabels" . }}
app.kubernetes.io/component: controller-manager
{{- end }}

{{/*
Controller service account name
*/}}
{{- define "solar.controller.serviceAccountName" -}}
{{- if .Values.controller.serviceAccount.create }}
{{- default (include "solar.controller.fullname" .) .Values.controller.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.controller.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
etcd fullname
*/}}
{{- define "solar.etcd.fullname" -}}
{{- printf "%s-etcd" (include "solar.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
etcd component labels
*/}}
{{- define "solar.etcd.labels" -}}
{{ include "solar.labels" . }}
app.kubernetes.io/component: etcd
app.kubernetes.io/part-of: solar
{{- end }}

{{/*
etcd selector labels
*/}}
{{- define "solar.etcd.selectorLabels" -}}
{{ include "solar.selectorLabels" . }}
app.kubernetes.io/component: etcd
{{- end }}

{{/*
etcd service name
*/}}
{{- define "solar.etcd.serviceName" -}}
{{- include "solar.etcd.fullname" . }}
{{- end }}

{{/*
etcd connection URL
*/}}
{{- define "solar.etcd.connectionUrl" -}}
{{- if .Values.apiserver.args.etcdServers }}
{{- .Values.apiserver.args.etcdServers }}
{{- else }}
{{- printf "http://%s:2379" (include "solar.etcd.serviceName" .) }}
{{- end }}
{{- end }}

{{/*
Image pull secrets
*/}}
{{- define "solar.imagePullSecrets" -}}
{{- $secrets := list }}
{{- if .Values.global.imagePullSecrets }}
{{- $secrets = concat $secrets .Values.global.imagePullSecrets }}
{{- end }}
{{- if .component.imagePullSecrets }}
{{- $secrets = concat $secrets .component.imagePullSecrets }}
{{- end }}
{{- if $secrets }}
imagePullSecrets:
{{- range $secrets }}
  - name: {{ . }}
{{- end }}
{{- end }}
{{- end }}

{{/*
API Server image
*/}}
{{- define "solar.apiserver.image" -}}
{{- $tag := .Values.apiserver.image.tag | default .Chart.AppVersion }}
{{- printf "%s:%s" .Values.apiserver.image.repository $tag }}
{{- end }}

{{/*
Controller image
*/}}
{{- define "solar.controller.image" -}}
{{- $tag := .Values.controller.image.tag | default .Chart.AppVersion }}
{{- printf "%s:%s" .Values.controller.image.repository $tag }}
{{- end }}

{{/*
etcd image
*/}}
{{- define "solar.etcd.image" -}}
{{- printf "%s:%s" .Values.etcd.image.repository .Values.etcd.image.tag }}
{{- end }}

{{/*
cert-manager Issuer name
*/}}
{{- define "solar.certManager.issuerName" -}}
{{- if .Values.certManager.issuer.name }}
{{- .Values.certManager.issuer.name }}
{{- else }}
{{- printf "%s-selfsigned-issuer" (include "solar.apiserver.fullname" .) }}
{{- end }}
{{- end }}

{{/*
cert-manager Certificate name
*/}}
{{- define "solar.certManager.certificateName" -}}
{{- printf "%s-cert" (include "solar.apiserver.fullname" .) }}
{{- end }}

{{/*
cert-manager Certificate secret name
*/}}
{{- define "solar.certManager.certificateSecretName" -}}
{{- printf "%s-cert" (include "solar.apiserver.fullname" .) }}
{{- end }}

{{/*
API Server service name
*/}}
{{- define "solar.apiserver.serviceName" -}}
{{- printf "%s-service" (include "solar.apiserver.fullname" .) }}
{{- end }}

{{/*
Controller metrics service name
*/}}
{{- define "solar.controller.metricsServiceName" -}}
{{- printf "%s-metrics" (include "solar.controller.fullname" .) }}
{{- end }}

{{/*
Common annotations
*/}}
{{- define "solar.annotations" -}}
{{- with .Values.commonAnnotations }}
{{ toYaml . }}
{{- end }}
{{- end }}
