package meerkat

import (
	"encoding/json"
	"fmt"
	"os"
)

func PrintResult(data any, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
