{{- define "common.name" -}}{{- default "orin" .Chart.Name -}}{{- end -}}

{{- define "common.secretName" -}}
{{- printf "%s-secret" .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "common.serviceAccountName" -}}
{{- default .Release.Name (.Values.global.serviceAccount.name | default "") -}}
{{- end -}}

{{- define "common.labels" -}}
app.kubernetes.io/name: {{ .Release.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
