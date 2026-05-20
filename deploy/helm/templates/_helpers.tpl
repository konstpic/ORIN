{{- define "orin.name" -}}{{- default "orin" .Values.nameOverride | trunc 63 | trimSuffix "-" -}}{{- end -}}

{{- define "orin.fullname" -}}
{{- $name := default "orin" .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}{{ .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else -}}{{ printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}{{- end -}}
{{- end -}}

{{- define "orin.labels" -}}
app.kubernetes.io/name: {{ include "orin.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "orin.postgres.fullname" -}}
{{ printf "%s-postgres" (include "orin.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end -}}
