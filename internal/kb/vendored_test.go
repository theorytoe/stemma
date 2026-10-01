package kb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVendoredNameIsAFunctionOfTheKey(t *testing.T) {
	first := VendoredName("bush1945")

	if first != VendoredName("bush1945") {
		t.Error("the same key gave two names")
	}
	if first == VendoredName("cormen2009") {
		t.Error("two keys gave one name")
	}
	if !strings.HasPrefix(first, SourcesDir+"/") || !strings.HasSuffix(first, ".txt") {
		t.Errorf("VendoredName = %q, want it under %s and ending in .txt", first, SourcesDir)
	}
	if !strings.Contains(first, "bush1945") {
		t.Errorf("VendoredName = %q, want the key in it to be readable", first)
	}
}

// Two keys can reduce to one filename — that is the case the bibliography
// directory form was built for too — so the digest is what has to keep their
// captures apart.
func TestVendoredNameSeparatesKeysThatReduceAlike(t *testing.T) {
	one := VendoredName("smith:2020")
	two := VendoredName("smith-2020")

	if one == two {
		t.Fatalf("two keys share the capture %q", one)
	}
}

func TestVendoredPathIsUnderTheKBRoot(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, filepath.FromSlash(VendoredName("bush1945")))

	if got := VendoredPath(root, "bush1945"); got != want {
		t.Errorf("VendoredPath = %q, want %q", got, want)
	}
}

func TestVendoredHashesCoversEveryCapture(t *testing.T) {
	root := t.TempDir()
	// A KB that has vendored nothing is the ordinary case, and it is not an error.
	got, err := VendoredHashes(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("hashes = %v, want none", got)
	}

	name := VendoredName("bush1945")
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	text := "the captured text\n"
	if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err = VendoredHashes(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := HashOf([]byte(text)); got[name] != want {
		t.Errorf("VendoredHashes[%q] = %q, want %q", name, got[name], want)
	}
}

// keyWithCapture builds a KB whose one entry is otherwise well formed, so the
// only findings a test has to think about are the vendored ones. recorded is the
// value written as the vendored hash; text is what is put on disk, and "" leaves
// the capture absent.
func keyWithCapture(t *testing.T, recorded, text string) (*KB, string) {
	t.Helper()
	root := t.TempDir()

	bib, err := ParseBibliography("b.bib", []byte("@article{bush1945,\n"+
		"  title = {As We May Think},\n"+
		"  stemma-retrieved = {2026-09-28},\n"+
		"  stemma-content-hash = {"+HashOf([]byte("the record"))+"},\n"+
		"  stemma-vendored-hash = {"+recorded+"},\n"+
		"}\n"))
	if err != nil {
		t.Fatal(err)
	}

	if text != "" {
		full := VendoredPath(root, "bush1945")
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return &KB{Root: root, Graph: NewGraph(), Bibliography: bib}, root
}

func TestCheckFindsAClaimedCaptureThatIsNotThere(t *testing.T) {
	k, _ := keyWithCapture(t, HashOf([]byte("the text")), "")

	if got := codes(k.CheckFindings()); len(got) != 1 || got[0] != CodeMissingVendored {
		t.Errorf("findings = %v, want one %s", got, CodeMissingVendored)
	}
}

func TestCheckAcceptsACaptureThatMatches(t *testing.T) {
	text := "the captured text\n"
	k, _ := keyWithCapture(t, HashOf([]byte(text)), text)

	if got := codes(k.CheckFindings()); len(got) != 0 {
		t.Errorf("findings = %v, want none", got)
	}
}

func TestCheckReportsACaptureThatHasDrifted(t *testing.T) {
	was := "what was captured"
	k, root := keyWithCapture(t, HashOf([]byte(was)), was)

	// A hand-edit, which is the case the hash exists to notice.
	full := VendoredPath(root, "bush1945")
	if err := os.WriteFile(full, []byte("something else entirely"), 0o644); err != nil {
		t.Fatal(err)
	}

	findings := k.CheckFindings()
	if len(findings) != 1 || findings[0].Code != CodeVendoredDrift {
		t.Fatalf("findings = %v, want one %s", codes(findings), CodeVendoredDrift)
	}
	// The finding names both sides, because the tool does not decide which of them
	// is the one someone meant.
	message := findings[0].Message
	for _, want := range []string{
		HashOf([]byte("something else entirely")),
		HashOf([]byte(was)),
		VendoredName("bush1945"),
	} {
		if !strings.Contains(message, want) {
			t.Errorf("message = %q, want %q in it", message, want)
		}
	}
}

func TestCheckReportsAVendoredHashItCannotRead(t *testing.T) {
	for name, recorded := range map[string]string{
		"not a hash":      "nonsense",
		"not hexadecimal": "sha256:zzzz",
		"unknown method":  "sha512:" + strings.Repeat("ab", 32),
	} {
		t.Run(name, func(t *testing.T) {
			k, _ := keyWithCapture(t, recorded, "the text")
			if got := codes(k.CheckFindings()); len(got) != 1 || got[0] != CodeBadVendored {
				t.Errorf("findings = %v, want one %s", got, CodeBadVendored)
			}
		})
	}
}

func TestCheckSaysNothingAboutAKeyThatClaimsNoCapture(t *testing.T) {
	bib, err := ParseBibliography("b.bib", []byte("@article{bush1945,\n"+
		"  title = {As We May Think},\n"+
		"  stemma-retrieved = {2026-09-28},\n"+
		"  stemma-content-hash = {"+HashOf([]byte("the record"))+"},\n"+
		"}\n"))
	if err != nil {
		t.Fatal(err)
	}
	k := &KB{Root: t.TempDir(), Graph: NewGraph(), Bibliography: bib}

	if got := codes(k.CheckFindings()); len(got) != 0 {
		t.Errorf("findings = %v, want none", got)
	}
}
