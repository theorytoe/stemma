package kb

import (
	"reflect"
	"testing"
)

func TestManifestDefaults(t *testing.T) {
	m := DefaultManifest("my-kb")
	if m.Title != "my-kb" {
		t.Errorf("Title = %q, want the root's name", m.Title)
	}
	if m.DefaultType != DefaultType {
		t.Errorf("DefaultType = %q, want %q", m.DefaultType, DefaultType)
	}
	if m.CitationStyle != DefaultCitationStyle {
		t.Errorf("CitationStyle = %q, want %q", m.CitationStyle, DefaultCitationStyle)
	}
	if len(m.Ignore) != 0 || len(m.Types) != 0 {
		t.Errorf("a default manifest should list nothing: %+v", m)
	}
	if m.Export.DefaultDepth != DefaultExportDepth {
		t.Errorf("Export.DefaultDepth = %d, want %d", m.Export.DefaultDepth, DefaultExportDepth)
	}
}

// Every absent key falls back to its default, including the ones in the
// [export] table, which is the only place an absent key could hide.
func TestParseManifestAppliesDefaultsForKeyItOmits(t *testing.T) {
	m, err := ParseManifest("my-kb", []byte("title = \"X\"\n[export]\n"))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if m.Description != "" {
		t.Errorf("Description = %q, want empty", m.Description)
	}
	if m.Export.DefaultDepth != DefaultExportDepth {
		t.Errorf("Export.DefaultDepth = %d, want the default %d", m.Export.DefaultDepth, DefaultExportDepth)
	}
}

func TestParseManifestReadsDescriptionAndExportDepth(t *testing.T) {
	raw := []byte("title = \"X\"\ndescription = \"about things\"\n[export]\ndefault_depth = 3\n")
	m, err := ParseManifest("my-kb", raw)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if m.Description != "about things" {
		t.Errorf("Description = %q", m.Description)
	}
	if m.Export.DefaultDepth != 3 {
		t.Errorf("Export.DefaultDepth = %d, want 3", m.Export.DefaultDepth)
	}
	if len(m.Unknown) != 0 {
		t.Errorf("Unknown = %q, want none", m.Unknown)
	}
}

// A key under [export] that this version does not read is as unknown as a
// top-level one, and is reported by its full path.
func TestParseManifestRecordsUnknownNestedKeys(t *testing.T) {
	m, err := ParseManifest("my-kb", []byte("[export]\ndefault_depth = 1\nfuture = true\n"))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if !reflect.DeepEqual(m.Unknown, []string{"export.future"}) {
		t.Errorf("Unknown = %q", m.Unknown)
	}
}

func TestParseManifestOverridesDefaults(t *testing.T) {
	raw := []byte("title = \"Stemma\"\ndefault_type = \"concept\"\ntypes = [\"person\", \"project\"]\ncitation_style = \"numeric\"\nignore = [\"pages/attic/**\"]\n")
	m, err := ParseManifest("my-kb", raw)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}

	if m.Title != "Stemma" {
		t.Errorf("Title = %q", m.Title)
	}
	if m.DefaultType != "concept" {
		t.Errorf("DefaultType = %q", m.DefaultType)
	}
	if m.CitationStyle != "numeric" {
		t.Errorf("CitationStyle = %q", m.CitationStyle)
	}
	if !reflect.DeepEqual(m.Types, []string{"person", "project"}) {
		t.Errorf("Types = %q", m.Types)
	}
	if len(m.Unknown) != 0 {
		t.Errorf("Unknown = %q, want none", m.Unknown)
	}
	if got := string(m.Bytes()); got != string(raw) {
		t.Errorf("Bytes = %q, want the file as read", got)
	}
}

// A key this version does not understand is not an error. It is recorded so a
// writer knows to keep it, and the manifest is otherwise read normally.
func TestParseManifestKeepsUnknownKeys(t *testing.T) {
	raw := []byte("title = \"X\"\nfuture_key = \"kept\"\nanother = 3\n")
	m, err := ParseManifest("my-kb", raw)
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	if !reflect.DeepEqual(m.Unknown, []string{"another", "future_key"}) {
		t.Errorf("Unknown = %q", m.Unknown)
	}
	if m.Title != "X" {
		t.Errorf("Title = %q", m.Title)
	}
	if got := string(m.Bytes()); got != string(raw) {
		t.Errorf("Bytes = %q, want the file unchanged", got)
	}
}

func TestParseManifestRefusesBrokenToml(t *testing.T) {
	if _, err := ParseManifest("my-kb", []byte("title = \n")); err == nil {
		t.Error("ParseManifest accepted TOML that does not parse")
	}
}

func TestManifestVocabulary(t *testing.T) {
	m, err := ParseManifest("my-kb", []byte("types = [\"person\"]\n"))
	if err != nil {
		t.Fatalf("ParseManifest: %v", err)
	}
	v := m.Vocabulary()
	if !v.Has("person") {
		t.Error("the manifest's types were not added")
	}
	if !v.Has(TypeTopic) {
		t.Error("the built-in types were dropped")
	}
	if v.Assignable(TypeSource) {
		t.Error("a reserved type became assignable")
	}
}

func TestIgnores(t *testing.T) {
	for _, tc := range []struct {
		name    string
		pattern string
		path    string
		ignored bool
	}{
		{"everything under a directory", "pages/attic/**", "pages/attic/a.md", true},
		{"deeper under a directory", "pages/attic/**", "pages/attic/a/b.md", true},
		{"the directory itself", "pages/attic/**", "pages/attic", true},
		{"a directory alone covers only itself", "pages/attic", "pages/attic/a.md", false},
		{"the directory itself, named alone", "pages/attic", "pages/attic", true},
		{"a wildcard within a segment", "pages/*.md", "pages/a.md", true},
		{"a wildcard does not cross a segment", "pages/*.md", "pages/sub/a.md", false},
		{"everything anywhere", "**/*.md", "pages/a.md", true},
		{"a wildcard segment in the middle", "pages/**/draft.md", "pages/a/b/draft.md", true},
		{"a wildcard segment matching nothing", "pages/**/draft.md", "pages/draft.md", true},
		{"an unrelated prefix", "pages/nope/**", "pages/attic/a.md", false},
		{"a different extension", "pages/*.md", "pages/a.txt", false},
		{"nothing matches an empty list", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := Manifest{Ignore: []string{tc.pattern}}
			if got := m.Ignores(tc.path); got != tc.ignored {
				t.Errorf("Ignores(%q) with pattern %q = %v, want %v",
					tc.path, tc.pattern, got, tc.ignored)
			}
		})
	}
}

func TestIgnoresTriesEveryPattern(t *testing.T) {
	m := Manifest{Ignore: []string{"pages/nope/**", "pages/attic/**"}}
	if !m.Ignores("pages/attic/a.md") {
		t.Error("the second pattern was not tried")
	}
}
