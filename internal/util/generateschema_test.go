package util

import (
	"testing"

	"github.com/invopop/jsonschema"
)

// A sample struct to generate a schema for
type Example struct {
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email,omitempty"`
}

func TestGenerateSchema(t *testing.T) {
	schemaAny := GenerateSchema[Example]()

	schema, ok := schemaAny.(*jsonschema.Schema)
	if !ok {
		t.Fatalf("expected *jsonschema.Schema, got %T", schemaAny)
	}

	// Check that the schema has expected properties
	if schema.Type != "object" {
		t.Errorf("expected type to be 'object', got %s", schema.Type)
	}

	props := schema.Properties
	if props == nil {
		t.Fatal("expected schema to have properties, got nil")
	}

	if _, ok := props.Get("name"); !ok {
		t.Error("missing 'name' property in schema")
	}
	if _, ok := props.Get("age"); !ok {
		t.Error("missing 'age' property in schema")
	}
	if _, ok := props.Get("email"); !ok {
		t.Error("missing 'email' property in schema")
	}
}
