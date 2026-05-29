package vectors_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/serengeti-sh/meerkat/internal/vectors"
)

func TestExtractor_Extract(t *testing.T) {
	d := vectors.NewExtractor()

	// First message creates a new template
	tmpl1, isNew1 := d.Extract("connection refused to database")
	assert.True(t, isNew1)
	assert.Equal(t, "connection refused to database", tmpl1)

	// Similar message should match existing template
	tmpl2, isNew2 := d.Extract("connection refused to redis")
	assert.False(t, isNew2)
	assert.Equal(t, "connection refused to <*>", tmpl2)

	// Different message creates new template
	tmpl3, isNew3 := d.Extract("timeout waiting for response")
	assert.True(t, isNew3)
	assert.Equal(t, "timeout waiting for response", tmpl3)

	// Message with parameters should mask them
	tmpl4, isNew4 := d.Extract("user 12345 logged in from 192.168.1.1")
	assert.True(t, isNew4)
	assert.Equal(t, "user <*> logged in from <*>", tmpl4)

	// Similar parameterized message should match
	tmpl5, isNew5 := d.Extract("user 99999 logged in from 10.0.0.1")
	assert.False(t, isNew5)
	assert.Equal(t, "user <*> logged in from <*>", tmpl5)
}

func TestExtractor_Templates(t *testing.T) {
	d := vectors.NewExtractor()

	d.Extract("error: connection failed")
	d.Extract("error: timeout occurred")
	d.Extract("warning: low memory")

	templates := d.Templates()
	assert.Len(t, templates, 3)
	assert.Contains(t, templates, "error connection failed")
	assert.Contains(t, templates, "error timeout occurred")
	assert.Contains(t, templates, "warning low memory")
}

func TestExtractor_Reset(t *testing.T) {
	d := vectors.NewExtractor()

	d.Extract("error: something failed")
	assert.Len(t, d.Templates(), 1)

	d.Reset()
	assert.Len(t, d.Templates(), 0)
}

func TestExtractor_EmptyMessage(t *testing.T) {
	d := vectors.NewExtractor()

	tmpl, isNew := d.Extract("")
	assert.True(t, isNew)
	assert.Equal(t, "", tmpl)
}
