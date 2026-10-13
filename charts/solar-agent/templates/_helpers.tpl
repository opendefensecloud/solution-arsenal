{{/*
Expand the name of the chart.
*/}}
{{- define "solar-agent.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "solar-agent.fullname" -}}
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
{{- define "solar-agent.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "solar-agent.labels" -}}
helm.sh/chart: {{ include "solar-agent.chart" . }}
{{ include "solar-agent.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "solar-agent.selectorLabels" -}}
app.kubernetes.io/name: {{ include "solar-agent.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "solar-agent.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "solar-agent.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Agent image
*/}}
{{- define "solar-agent.image" -}}
{{- $tag := .Values.image.tag | default .Chart.AppVersion }}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}

{{/*
Name of the Secret holding the solar-apiserver kubeconfig: the existing one if
referenced, otherwise the chart-owned Secret rendered from the inline value.
*/}}
{{- define "solar-agent.apiserverKubeconfigSecretName" -}}
{{- if and .Values.apiserverKubeconfig.value .Values.apiserverKubeconfig.existingSecret -}}
{{- fail "apiserverKubeconfig: set either value or existingSecret, not both" -}}
{{- end -}}
{{- if .Values.apiserverKubeconfig.existingSecret -}}
{{- .Values.apiserverKubeconfig.existingSecret -}}
{{- else -}}
{{- include "solar-agent.fullname" . -}}
{{- end -}}
{{- end }}

{{/*
Whether the agent is connected to a remote solar apiserver (a kubeconfig is
configured). When it is not, the agent runs standalone: no kubeconfig is
mounted, no Target is resolved, and the helm test apiserver check is omitted.
*/}}
{{- define "solar-agent.remoteApiServer" -}}
{{- if or .Values.apiserverKubeconfig.value .Values.apiserverKubeconfig.existingSecret }}true{{ end -}}
{{- end }}
