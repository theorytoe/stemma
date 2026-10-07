package cli

import (
	"strings"
	"testing"
)

// A noun family's member is named in two words, which the dispatcher accepted
// and help did not. The reference always documented members as if it did, so
// this is the correction rather than a new promise.
func TestHelpNamesAFamilyMember(t *testing.T) {
	code, stdout, stderr := run("help", "cite", "add")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	for _, want := range []string{"usage: stemma cite add", "--doi", "--dry-run"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help cite add is missing %q:\n%s", want, stdout)
		}
	}

	code, stdout, stderr = run("help", "cite", "add", "--json")
	if code != ExitOK {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	var info commandInfo
	decodeData(t, stdout, &info)
	if info.Name != "cite add" {
		t.Errorf("the envelope names %q, want %q", info.Name, "cite add")
	}
}
