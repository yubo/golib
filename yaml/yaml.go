package yaml

import "github.com/yubo/golib/yaml/sigs.k8s.io/yaml"

// Marshal marshals the object into JSON then converts JSON to YAML and returns the
// YAML.
func Marshal(o interface{}) ([]byte, error) {
	return yaml.Marshal(o)
}

// JSONToYAML Converts JSON to YAML.
func JSONToYAML(j []byte) ([]byte, error) {
	return yaml.JSONToYAML(j)
}
