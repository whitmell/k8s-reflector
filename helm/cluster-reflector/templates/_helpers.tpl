{{/*
Expand the name of the chart.
*/}}
{{- define "cluster-reflector.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "cluster-reflector.fullname" -}}
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
{{- define "cluster-reflector.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "cluster-reflector.labels" -}}
helm.sh/chart: {{ include "cluster-reflector.chart" . }}
{{ include "cluster-reflector.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: cluster-reflector
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "cluster-reflector.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cluster-reflector.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "cluster-reflector.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "cluster-reflector.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the cluster role to use
*/}}
{{- define "cluster-reflector.clusterRoleName" -}}
{{- include "cluster-reflector.fullname" . }}
{{- end }}

{{/*
Create the name of the role to use
*/}}
{{- define "cluster-reflector.roleName" -}}
{{- include "cluster-reflector.fullname" . }}
{{- end }}

{{/*
Create common annotations
*/}}
{{- define "cluster-reflector.annotations" -}}
{{- with .Values.commonAnnotations }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Create the container image reference
*/}}
{{- define "cluster-reflector.image" -}}
{{- printf "%s:%s" .Values.image.repository .Values.image.tag }}
{{- end }}

{{/*
Create container arguments
*/}}
{{- define "cluster-reflector.args" -}}
- --listen=:8080
- --cache-ttl={{ .Values.cache.ttl }}
{{- if .Values.appDiscovery.namespaceSelector }}
- --namespace-selector={{ .Values.appDiscovery.namespaceSelector }}
{{- end }}
- --prefer-crd={{ .Values.appDiscovery.preferCRD }}
- --fallback-workloads={{ .Values.appDiscovery.fallbackWorkloads }}
{{- if .Values.appDiscovery.crdOnly }}
- --crd-only={{ .Values.appDiscovery.crdOnly }}
{{- end }}
- --log-level={{ .Values.logLevel }}
{{- end }}

{{/*
Create pod security context
*/}}
{{- define "cluster-reflector.podSecurityContext" -}}
{{- if .Values.podSecurityContext }}
{{- toYaml .Values.podSecurityContext }}
{{- end }}
{{- end }}

{{/*
Create container security context
*/}}
{{- define "cluster-reflector.securityContext" -}}
{{- if .Values.securityContext }}
{{- toYaml .Values.securityContext }}
{{- end }}
{{- end }}

{{/*
Create resource specifications
*/}}
{{- define "cluster-reflector.resources" -}}
{{- if .Values.resources }}
{{- toYaml .Values.resources }}
{{- end }}
{{- end }}

{{/*
Create node selector
*/}}
{{- define "cluster-reflector.nodeSelector" -}}
{{- if .Values.nodeSelector }}
{{- toYaml .Values.nodeSelector }}
{{- end }}
{{- end }}

{{/*
Create tolerations
*/}}
{{- define "cluster-reflector.tolerations" -}}
{{- if .Values.tolerations }}
{{- toYaml .Values.tolerations }}
{{- end }}
{{- end }}

{{/*
Create affinity
*/}}
{{- define "cluster-reflector.affinity" -}}
{{- if .Values.affinity }}
{{- toYaml .Values.affinity }}
{{- else }}
# Soft pod anti-affinity by default
podAntiAffinity:
  preferredDuringSchedulingIgnoredDuringExecution:
  - weight: 100
    podAffinityTerm:
      labelSelector:
        matchLabels:
          {{- include "cluster-reflector.selectorLabels" . | nindent 10 }}
      topologyKey: kubernetes.io/hostname
{{- end }}
{{- end }}

{{/*
Create topology spread constraints
*/}}
{{- define "cluster-reflector.topologySpreadConstraints" -}}
{{- if .Values.topologySpreadConstraints }}
{{- toYaml .Values.topologySpreadConstraints }}
{{- else }}
# Soft topology spread by default
- maxSkew: 1
  topologyKey: kubernetes.io/hostname
  whenUnsatisfiable: ScheduleAnyway
  labelSelector:
    matchLabels:
      {{- include "cluster-reflector.selectorLabels" . | nindent 6 }}
{{- end }}
{{- end }}
