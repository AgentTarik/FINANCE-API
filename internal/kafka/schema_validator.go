package kafka

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed schemas/v1/*.json
var schemaFS embed.FS

type Validator struct {
	schema *jsonschema.Schema
}

func NewValidator() (*Validator, error) {
	data, err := schemaFS.ReadFile("schemas/v1/transaction_created.v1.json")
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("add resource: %w", err)
	}
	s, err := c.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}
	return &Validator{schema: s}, nil
}

func (v *Validator) Validate(doc any) error {
	// jsonschema espera interface genérica (map[string]any, etc.)
	b, _ := json.Marshal(doc)
	var x any
	_ = json.Unmarshal(b, &x)
	return v.schema.Validate(x)
}
