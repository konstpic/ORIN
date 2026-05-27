package manifest

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// PluginEnvVar is a key/value pair injected into the plugin's process
// environment. Defined here independently of domain to avoid a circular import.
type PluginEnvVar struct {
	Name  string
	Value string
}

// PluginConfig carries the resolved plugin configuration for a single render
// call: the base plugin definition merged with per-application overrides.
//
// This is constructed by reposerver and passed through RenderContext so that
// the manifest package does not import the domain or store packages.
type PluginConfig struct {
	// Command is the executable, e.g. "sh".
	Command string
	// Args are the command-line arguments, e.g. ["-c", "helm template ."].
	Args []string
	// Env is the merged set of env vars (plugin base + app overrides).
	Env []PluginEnvVar
	// AppName is injected as ORIN_APP_NAME.
	AppName string
	// Namespace is injected as ORIN_APP_NAMESPACE.
	Namespace string
}

// PluginRenderer runs an external command in the checked-out directory and
// parses its stdout as a multi-document YAML stream.
//
// The following env vars are always injected in addition to PluginConfig.Env:
//
//	ORIN_APP_NAME        – application name
//	ORIN_APP_NAMESPACE   – destination namespace
//	ORIN_ENV_<NAME>      – per-entry from PluginConfig.Env (upper-cased name)
type PluginRenderer struct {
	Config PluginConfig
}

// Render executes the plugin command with dir as the working directory.
func (pr *PluginRenderer) Render(dir string) ([]*unstructured.Unstructured, error) {
	if pr.Config.Command == "" {
		return nil, fmt.Errorf("plugin: generate.command is empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, pr.Config.Command, pr.Config.Args...)
	cmd.Dir = dir
	cmd.Env = pr.buildEnv()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("plugin %q: %w: %s", pr.Config.Command, err, msg)
		}
		return nil, fmt.Errorf("plugin %q: %w", pr.Config.Command, err)
	}

	return SplitYAMLDocs(out)
}

// buildEnv constructs the os.Environ-style list for the plugin process.
// Injected vars:
//   - ORIN_APP_NAME, ORIN_APP_NAMESPACE
//   - one ORIN_ENV_<UPPER_NAME> per PluginConfig.Env entry
func (pr *PluginRenderer) buildEnv() []string {
	env := []string{
		"ORIN_APP_NAME=" + pr.Config.AppName,
		"ORIN_APP_NAMESPACE=" + pr.Config.Namespace,
	}
	for _, e := range pr.Config.Env {
		env = append(env, "ORIN_ENV_"+strings.ToUpper(e.Name)+"="+e.Value)
	}
	return env
}
