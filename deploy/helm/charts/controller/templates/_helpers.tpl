{{- define "controller.fullname" -}}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "controller.labels" -}}
app.kubernetes.io/name: {{ include "controller.fullname" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: controller
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "controller.secretName" -}}
{{- printf "%s-secret" .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "controller.serviceAccountName" -}}
{{- if .Values.global.serviceAccount.name }}{{ .Values.global.serviceAccount.name }}{{ else }}{{ .Release.Name }}{{ end -}}
{{- end -}}

{{- define "controller.reposerverAddr" -}}
{{- printf "%s-reposerver:50051" .Release.Name -}}
{{- end -}}

{{- define "controller.image" -}}
{{- $img := .Values.global.images.controller -}}
{{- $repo := required "global.images.controller.repository is required" $img.repository -}}
{{- $tag := default .Values.global.image.tag $img.tag -}}
{{- printf "%s:%s" $repo $tag -}}
{{- end -}}

{{- define "controller.imagePullPolicy" -}}
{{- default .Values.global.image.pullPolicy .Values.global.images.controller.pullPolicy -}}
{{- end -}}
