//go:build stemma_deps

// Package deps pins the dependency allowlist.
//
// The allowlist is fixed by the top-level planfile: yaml.v3 for frontmatter,
// BurntSushi/toml for the manifest, blackfriday/v2 for markdown rendering, and
// modernc.org/sqlite for the Tier-1 index.
//
// yaml.v3 and toml are imported where they are used, so they would stay in go.mod
// without this file. blackfriday/v2 and sqlite are not imported anywhere yet:
// this file is what keeps them, and the entries below are the whole reason it
// exists.
//
// They live behind a build tag for one reason: `go mod tidy` acts as if every
// build tag is enabled, so the four stay direct requirements in go.mod and
// cannot be quietly dropped by a tidy between now and the task that needs them.
// The tag also keeps `go build ./...` from compiling the sqlite tree on every
// run.
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
