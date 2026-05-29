package tool

import (
	"encoding/json"
	"net/url"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Export internal functions for testing only.

func ExportCompileSchema(path string) (*jsonschema.Schema, json.RawMessage, error) {
	return compileSchema(path)
}

func ExportValidateArgs(schema *jsonschema.Schema, args json.RawMessage) error {
	return validateArgs(schema, args)
}

func ExportArgsToQueryParams(schema *jsonschema.Schema, args json.RawMessage, defaults url.Values) (url.Values, error) {
	return argsToQueryParams(schema, args, defaults)
}

func ExportFormatParam(v any) string {
	return formatParam(v)
}

func ExportParseTime(s string) (time.Time, error) {
	return parseTime(s)
}
