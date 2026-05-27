// Package reporter delivers analysis reports to external channels.
//
// Currently supports webhook/Slack delivery with configurable minimum severity
// filtering. The service formats report data and POSTs it to the configured URL.
package reporter
