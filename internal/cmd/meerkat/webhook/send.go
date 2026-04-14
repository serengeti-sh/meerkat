package webhook

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat"
	"github.com/serengeti-sh/meerkat/pkg/api"
)

func NewCmd() *cobra.Command {
	var file string

	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Send a webhook payload",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := meerkat.GetClient(cmd.Context())
			if c == nil {
				return fmt.Errorf("client not initialized")
			}

			if file == "" {
				return fmt.Errorf("-f, --file is required")
			}

			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}

			var payload map[string]any
			if err := json.Unmarshal(data, &payload); err != nil {
				return fmt.Errorf("invalid JSON: %w", err)
			}

			source, _ := payload["source"].(string)
			if source == "" {
				return fmt.Errorf("payload must contain a \"source\" field")
			}

			resp, err := c.ReceiveWebhook(cmd.Context(), &api.ReceiveWebhookReq{
				Source:  optString(source),
				Alert:   optStringField(payload, "alert"),
				Message: optStringField(payload, "message"),
			})
			if err != nil {
				return err
			}
			return meerkat.PrintResult(resp, "json")
		},
	}

	cmd.Flags().StringVarP(&file, "file", "f", "", "Webhook payload JSON file (required)")

	return cmd
}

func optString(s string) api.OptString {
	if s == "" {
		return api.OptString{}
	}
	return api.NewOptString(s)
}

func optStringField(m map[string]any, key string) api.OptString {
	if v, ok := m[key].(string); ok && v != "" {
		return api.NewOptString(v)
	}
	return api.OptString{}
}
