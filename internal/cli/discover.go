package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/theorytoe/stemma/internal/kb"
)

// discover finds the KB root.
//
// The order is fixed (D41): an explicit path, then the environment, then
// walking up from the working directory. Walking up prefers a directory holding
// a manifest, and falls back to the nearest one holding pages/, so a KB
// configured by hand beats a directory that merely looks like one however near
// that directory is.
//
// Several candidate roots on the walk up are not an error: the nearest manifest
// wins, and if no manifest exists anywhere above, the nearest pages/ directory
// does. A manifest further up still beats a nearer pages/, because the manifest
// is what marks a KB deliberately rather than coincidentally.
func discover(explicit string) (string, error) {
	if explicit != "" {
		return asDirectory(explicit, "")
	}
	if env := os.Getenv(EnvKB); env != "" {
		return asDirectory(env, EnvKB+" is set to ")
	}

	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	fallback := ""
	for {
		if _, err := os.Stat(filepath.Join(dir, kb.ManifestName)); err == nil {
			return dir, nil
		}
		if fallback == "" {
			if info, err := os.Stat(filepath.Join(dir, kb.PagesDir)); err == nil && info.IsDir() {
				fallback = dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", fmt.Errorf("no KB found: pass --kb, set %s, or run inside one", EnvKB)
}

// asDirectory checks that a named KB root is a directory, and gives the failure
// a message that says where the name came from.
//
// It does not check for a manifest or pages/: that is Load's job, and Load's
// message names what a root is missing, which is more useful than one that only
// says the path is not a KB.
func asDirectory(name, prefix string) (string, error) {
	info, err := os.Stat(name)
	if err != nil {
		return "", fmt.Errorf("%s%s: %w", prefix, name, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s%s is not a directory", prefix, name)
	}
	return name, nil
}
