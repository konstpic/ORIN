{{- define "apiserver.fullname" -}}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "apiserver.labels" -}}
app.kubernetes.io/name: {{ include "apiserver.fullname" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: apiserver
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "apiserver.secretName" -}}
{{- printf "%s-secret" .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "apiserver.serviceAccountName" -}}
{{- if .Values.global.serviceAccount.name }}{{ .Values.global.serviceAccount.name }}{{ else }}{{ .Release.Name }}{{ end -}}
{{- end -}}

{{- define "apiserver.reposerverAddr" -}}
{{- printf "%s-reposerver:50051" .Release.Name -}}
{{- end -}}

{{- define "apiserver.image" -}}
{{- $img := .Values.global.images.apiserver -}}
{{- $repo := required "global.images.apiserver.repository is required" $img.repository -}}
{{- $tag := default .Values.global.image.tag $img.tag -}}
{{- printf "%s:%s" $repo $tag -}}
{{- end -}}

{{- define "apiserver.imagePullPolicy" -}}
{{- default .Values.global.image.pullPolicy .Values.global.images.apiserver.pullPolicy -}}
{{- end -}}
