package manifest

// RenderContext carries application hints for renderers (Helm release name,
// default namespace for helm template -n, optional values overlay, etc.).
type RenderContext struct {
	AppName        string
	DestNamespace  string
	HelmValuesJSON []byte   // optional; merged as an extra -f layer for Helm charts
	HelmValueFiles []string // optional; paths relative to chart dir passed as -f layers before HelmValuesJSON
}
