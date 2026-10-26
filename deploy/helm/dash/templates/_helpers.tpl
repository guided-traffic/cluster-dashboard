{{/*
The chart's name, 63 characters at most.
*/}}
{{- define "dash.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
The full name the resources are named after: the release name when it holds the chart's name,
else <release>-<chart>; 63 characters at most.
*/}}
{{- define "dash.fullname" -}}
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
The component names: <fullname>-backend, <fullname>-frontend, <fullname>-config.
*/}}
{{- define "dash.backend.fullname" -}}
{{- printf "%s-backend" (include "dash.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "dash.frontend.fullname" -}}
{{- printf "%s-frontend" (include "dash.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "dash.config.fullname" -}}
{{- printf "%s-config" (include "dash.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "dash.icons.fullname" -}}
{{- printf "%s-icons" (include "dash.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
The icon ConfigMaps the backend mounts, one folder each under /etc/dash-icons (docs/adr/0008 D4):
the chart's own <fullname>-icons when icons.configMap.files holds a file, then
icons.existingConfigMaps. A name in both is refused: the folder would be mounted twice.
*/}}
{{- define "dash.iconConfigMaps" -}}
{{- $names := list }}
{{- if .Values.icons.configMap.files }}
{{- $names = append $names (include "dash.icons.fullname" .) }}
{{- end }}
{{- range .Values.icons.existingConfigMaps }}
{{- if has . $names }}
{{- fail (printf "icons.existingConfigMaps names %s, the ConfigMap the chart renders from icons.configMap.files" .) }}
{{- end }}
{{- $names = append $names . }}
{{- end }}
{{- toJson $names }}
{{- end }}

{{/*
The name of the discovery's RBAC objects: <namespace>-<fullname>. Cluster-scoped objects carry the
release namespace, so two releases of one name in two namespaces — two dashboards with their own
instanceName — do not claim the same ClusterRole; the namespaced Roles in the watched namespaces
share the name for the same reason.
*/}}
{{- define "dash.rbac.name" -}}
{{- printf "%s-%s" .Release.Namespace (include "dash.fullname" .) | trunc 253 | trimSuffix "-" }}
{{- end }}

{{/*
The backend's ServiceAccount.
*/}}
{{- define "dash.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "dash.backend.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
The labels every resource carries, without the component.
*/}}
{{- define "dash.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
app.kubernetes.io/name: {{ include "dash.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
The selector of one component. Call with (dict "root" . "component" "backend").
*/}}
{{- define "dash.selectorLabels" -}}
app.kubernetes.io/name: {{ include "dash.name" .root }}
app.kubernetes.io/instance: {{ .root.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{/*
An image reference: the tag, or the chart's appVersion. Call with (dict "root" . "image" .Values.backend.image).
*/}}
{{- define "dash.image" -}}
{{- printf "%s:%s" .image.repository (default .root.Chart.AppVersion .image.tag) }}
{{- end }}

{{/*
The pod's security context (docs/adr/0001 D3): distroless nonroot, by number.
*/}}
{{- define "dash.podSecurityContext" -}}
runAsNonRoot: true
runAsUser: 65532
runAsGroup: 65532
seccompProfile:
  type: RuntimeDefault
{{- end }}

{{/*
The container's security context: nothing written, nothing gained.
*/}}
{{- define "dash.containerSecurityContext" -}}
allowPrivilegeEscalation: false
readOnlyRootFilesystem: true
capabilities:
  drop:
    - ALL
{{- end }}

{{/*
The annotations of one Ingress object: Dash's own opt-out first, so Dash does not list itself,
then ingress.annotations, then the object's own. Call with (dict "root" . "own" .Values.ingress.frontend.annotations).
*/}}
{{- define "dash.ingressAnnotations" -}}
{{- $annotations := dict "dash.gtrfc.com/enable" "false" }}
{{- $annotations = merge (deepCopy (default dict .own)) (deepCopy (default dict .root.Values.ingress.annotations)) $annotations }}
{{- toYaml $annotations }}
{{- end }}
