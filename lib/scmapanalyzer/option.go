package scmapanalyzer

import "strings"

type options struct {
	mapNameHint string
}

// Option configures [Client.Analyze].
type Option func(*options)

// WithMapName sets an optional ladder map name hint. When it matches a
// preloaded or dynamically cached analysis (exact or fuzzy), the replay file
// is not parsed for geometry.
func WithMapName(name string) Option {
	return func(o *options) {
		o.mapNameHint = strings.TrimSpace(name)
	}
}
