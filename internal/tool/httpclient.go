package tool

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"
)

// NewHTTPClient creates an *http.Client with optional CA cert trust.
// If caFile is empty, the system default cert pool is used.
func NewHTTPClient(caFile string) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	if caFile != "" {
		caData, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("reading CA cert %s: %w", caFile, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caData) {
			return nil, fmt.Errorf("no certificates found in %s", caFile)
		}
		transport.TLSClientConfig = &tls.Config{
			RootCAs:    pool,
			MinVersion: tls.VersionTLS12,
		}
	}

	return &http.Client{
		Timeout:   15 * time.Second,
		Transport: transport,
	}, nil
}
