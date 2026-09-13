package artifact

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"
)

// DecodeConfigYAML preserves nulls and scalar types until JSON Schema validation.
// Unknown keys are rejected by the same contract used for JSON config fixtures.
func DecodeConfigYAML(b []byte) (SourceConfig, error) {
	var zero SourceConfig
	if len(b) > MaxDocumentBytes {
		return zero, fmt.Errorf("configuration too large")
	}
	d := yaml.NewDecoder(bytes.NewReader(b))
	var doc yaml.Node
	if err := d.Decode(&doc); err != nil {
		return zero, err
	}
	var tail yaml.Node
	if err := d.Decode(&tail); err != io.EOF {
		return zero, fmt.Errorf("expected one YAML document")
	}
	if len(doc.Content) != 1 {
		return zero, fmt.Errorf("empty YAML document")
	}
	value, err := yamlValue(doc.Content[0], 0)
	if err != nil {
		return zero, err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return zero, err
	}
	return DecodeConfigJSON(raw)
}
func yamlValue(n *yaml.Node, depth int) (any, error) {
	if depth > 64 || n.Anchor != "" || n.Kind == yaml.AliasNode {
		return nil, fmt.Errorf("YAML anchors, aliases, or excessive nesting are not supported")
	}
	switch n.Kind {
	case yaml.MappingNode:
		result := map[string]any{}
		for i := 0; i < len(n.Content); i += 2 {
			key := n.Content[i]
			if key.Tag != "!!str" {
				return nil, fmt.Errorf("YAML keys must be strings")
			}
			if _, exists := result[key.Value]; exists {
				return nil, fmt.Errorf("duplicate YAML key %q", key.Value)
			}
			value, err := yamlValue(n.Content[i+1], depth+1)
			if err != nil {
				return nil, err
			}
			result[key.Value] = value
		}
		return result, nil
	case yaml.SequenceNode:
		result := []any{}
		for _, child := range n.Content {
			value, err := yamlValue(child, depth+1)
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
		return result, nil
	case yaml.ScalarNode:
		switch n.Tag {
		case "!!str", "!!timestamp":
			return n.Value, nil
		case "!!null":
			return nil, nil
		case "!!int":
			var value int64
			err := n.Decode(&value)
			return value, err
		case "!!float":
			var value float64
			err := n.Decode(&value)
			return value, err
		case "!!bool":
			var value bool
			err := n.Decode(&value)
			return value, err
		}
	}
	return nil, fmt.Errorf("unsupported YAML value")
}
