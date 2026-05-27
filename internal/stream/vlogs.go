package stream

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Connector reads log entries from VictoriaLogs in real-time.
type Connector struct {
	baseURL    string
	httpClient *http.Client
}

// NewConnector creates a stream connector for VictoriaLogs.
func NewConnector(baseURL string, client *http.Client) *Connector {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &Connector{
		baseURL:    baseURL,
		httpClient: client,
	}
}

// Entry represents a single log entry from the stream.
type Entry struct {
	ID         string            `json:"_msg_id"`
	Timestamp  int64             `json:"_time"`
	Service    string            `json:"service"`
	Severity   string            `json:"severity"`
	Body       string            `json:"_msg"`
	Attributes map[string]string `json:"attributes"`
}

// Subscribe opens a persistent connection to VM Logs tail endpoint and
// yields each log entry to the handler. The connection is closed when
// the context is cancelled.
func (c *Connector) Subscribe(ctx context.Context, query string, handler func(Entry)) error {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}
	u.Path = "/select/logsql/tail"

	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connect to stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("stream returned status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry Entry
		if err := json.Unmarshal(line, &entry); err != nil {
			// Skip malformed lines rather than failing the entire stream.
			continue
		}

		handler(entry)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read stream: %w", err)
	}

	return nil
}
