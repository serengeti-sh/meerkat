package meerkat

import "github.com/serengeti-sh/meerkat/pkg/api"

// OptString returns an api.OptString that is unset if s is empty.
func OptString(s string) api.OptString {
	if s == "" {
		return api.OptString{}
	}
	return api.NewOptString(s)
}
