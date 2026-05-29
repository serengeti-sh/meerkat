package tool

import (
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// baseTool holds the common metadata and schema validation for all tools.
type baseTool struct {
	name        string
	description string
	params      json.RawMessage
	schema      *jsonschema.Schema
}

func newBaseTool(name, description, schemaFile string) (baseTool, error) {
	if name == "" {
		return baseTool{}, fmt.Errorf("tool: name is required")
	}
	if description == "" {
		return baseTool{}, fmt.Errorf("tool %q: description is required", name)
	}
	if schemaFile == "" {
		return baseTool{}, fmt.Errorf("tool %q: param_schema_file is required", name)
	}

	schema, params, err := compileSchema(schemaFile)
	if err != nil {
		return baseTool{}, fmt.Errorf("tool %q: %w", name, err)
	}

	return baseTool{
		name:        name,
		description: description,
		params:      params,
		schema:      schema,
	}, nil
}

func (b baseTool) Name() string               { return b.name }
func (b baseTool) Description() string        { return b.description }
func (b baseTool) Parameters() json.RawMessage { return b.params }

func (b baseTool) validateArgs(args json.RawMessage) error {
	if b.schema == nil {
		return nil
	}
	var v any
	if err := json.Unmarshal(args, &v); err != nil {
		return fmt.Errorf("invalid JSON arguments: %w", err)
	}
	if err := b.schema.Validate(v); err != nil {
		return fmt.Errorf("invalid parameters: %w", err)
	}
	return nil
}
