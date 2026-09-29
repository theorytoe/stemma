//go:build stemma_deps

// Package deps pins the dependency allowlist.
//
// The allowlist is fixed by the top-level planfile: yaml.v3 for frontmatter,
// BurntSushi/toml for the manifest, blackfriday/v2 for markdown rendering, and
// modernc.org/sqlite for the Tier-1 index.
//
// yaml.v3 and toml are imported where they are used, and modernc.org/sqlite is
// now imported by internal/index, so those three would stay in go.mod without
// this file. blackfriday/v2 is the one still not imported anywhere: this file is
// what keeps it, and that entry is the whole reason it exists.
//
// It lives behind a build tag for one reason: `go mod tidy` acts as if every
// build tag is enabled, so blackfriday stays a direct requirement in go.mod and
// cannot be quietly dropped by a tidy between now and the task that needs it.
//
// Adding a fifth entry here means changing the decided allowlist. Do not do that
// without raising it first.
package deps

import (
	_ "github.com/BurntSushi/toml"
	_ "github.com/russross/blackfriday/v2"
	_ "gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"
)
