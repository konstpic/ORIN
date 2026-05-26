{{- define "reposerver.fullname" -}}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "reposerver.labels" -}}
app.kubernetes.io/name: {{ include "reposerver.fullname" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: reposerver
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "reposerver.secretName" -}}
{{- printf "%s-secret" .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "reposerver.serviceAccountName" -}}
{{- if .Values.global.serviceAccount.name }}{{ .Values.global.serviceAccount.name }}{{ else }}{{ .Release.Name }}{{ end -}}
{{- end -}}

{{- define "reposerver.image" -}}
{{- $img := .Values.global.images.reposerver -}}
{{- $repo := required "global.images.reposerver.repository is required" $img.repository -}}
{{- $tag := default .Values.global.image.tag $img.tag -}}
{{- printf "%s:%s" $repo $tag -}}
{{- end -}}

{{- define "reposerver.imagePullPolicy" -}}
{{- default .Values.global.image.pullPolicy .Values.global.images.reposerver.pullPolicy -}}
{{- end -}}
