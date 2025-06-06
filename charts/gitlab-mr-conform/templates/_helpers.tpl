{{/*
Expand the name of the chart.
*/}}
{{- define "gitlab-mr-conform.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "gitlab-mr-conform.fullname" -}}
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
{{- define "gitlab-mr-conform.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "gitlab-mr-conform.labels" -}}
helm.sh/chart: {{ include "gitlab-mr-conform.chart" . }}
{{ include "gitlab-mr-conform.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "gitlab-mr-conform.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gitlab-mr-conform.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Secret name
*/}}
{{- define "gitlab-mr-conform.secretName" -}}
{{- if .Values.secret.name }}
{{- .Values.secret.name }}
{{- else }}
{{- include "gitlab-mr-conform.fullname" . }}-secrets
{{- end }}
{{- end }}

{{/*
ConfigMap name
*/}}
{{- define "gitlab-mr-conform.configMapName" -}}
{{- if .Values.config.name }}
{{- .Values.config.name }}
{{- else }}
{{- include "gitlab-mr-conform.fullname" . }}-config
{{- end }}
{{- end }}