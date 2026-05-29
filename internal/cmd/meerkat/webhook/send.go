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

			var payload struct {
				Source  string `json:"source"`
				Alert   string `json:"alert"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(data, &payload); err != nil {
				return fmt.Errorf("invalid JSON: %w", err)
			}

			if payload.Source == "" {
				return fmt.Errorf("payload must contain a \"source\" field")
			}

			resp, err := c.ReceiveWebhook(cmd.Context(), &api.ReceiveWebhookReq{
				Source:  meerkat.OptString(payload.Source),
				Alert:   meerkat.OptString(payload.Alert),
				Message: meerkat.OptString(payload.Message),
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
