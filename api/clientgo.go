// client-go
package api

import "fmt"

// AuthProviderConfig holds the configuration for a specified auth provider.
type AuthProviderConfig struct {
	Name string `json:"name"`
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

var _ fmt.Stringer = new(AuthProviderConfig)
var _ fmt.GoStringer = new(AuthProviderConfig)

// GoString implements fmt.GoStringer and sanitizes sensitive fields of
// AuthProviderConfig to prevent accidental leaking via logs.
func (c AuthProviderConfig) GoString() string {
	return c.String()
}

// String implements fmt.Stringer and sanitizes sensitive fields of
// AuthProviderConfig to prevent accidental leaking via logs.
func (c AuthProviderConfig) String() string {
	cfg := "<nil>"
	if c.Config != nil {
		cfg = "--- REDACTED ---"
	}
	return fmt.Sprintf("api.AuthProviderConfig{Name: %q, Config: map[string]string{%s}}", c.Name, cfg)
}

// ExecConfig specifies a command to provide client credentials. The command is exec'd
// and outputs structured stdout holding credentials.
//
// See the client.authentication.k8s.io API group for specifications of the exact input
// and output format
type ExecConfig struct {
	// Command to execute.
	Command string `json:"command"`
	// Arguments to pass to the command when executing it.
	// +optional
	Args []string `json:"args"`
	// Env defines additional environment variables to expose to the process. These
	// are unioned with the host's environment, as well as variables client-go uses
	// to pass argument to the plugin.
	// +optional
	Env []ExecEnvVar `json:"env"`

	// This text is shown to the user when the executable doesn't seem to be
	// present. For example, `brew install foo-cli` might be a good InstallHint for
	// foo-cli on Mac OS systems.
	InstallHint string `json:"installHint,omitempty"`
}

var _ fmt.Stringer = new(ExecConfig)
var _ fmt.GoStringer = new(ExecConfig)

// GoString implements fmt.GoStringer and sanitizes sensitive fields of
// ExecConfig to prevent accidental leaking via logs.
func (c ExecConfig) GoString() string {
	return c.String()
}

// String implements fmt.Stringer and sanitizes sensitive fields of ExecConfig
// to prevent accidental leaking via logs.
func (c ExecConfig) String() string {
	var args []string
	if len(c.Args) > 0 {
		args = []string{"--- REDACTED ---"}
	}
	env := "[]ExecEnvVar(nil)"
	if len(c.Env) > 0 {
		env = "[]ExecEnvVar{--- REDACTED ---}"
	}
	return fmt.Sprintf("api.ExecConfig{Command: %q, Args: %#v, Env: %s}", c.Command, args, env)
}

// ExecEnvVar is used for setting environment variables when executing an exec-based
// credential plugin.
type ExecEnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
