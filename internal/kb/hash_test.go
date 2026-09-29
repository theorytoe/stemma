package kb

import (
	"strings"
	"testing"
)

// The short name is the head of the recorded hash rather than a digest of its
// own, so a file named from a key and the hash on the entry that points at it can
// never be computed two ways.
func TestTheShortHashIsTheHeadOfTheHash(t *testing.T) {
	for _, sample := range []string{"smith:2020", "", "https://example.com/a/b"} {
		full := HashOf([]byte(sample))
		short := ShortHashOf([]byte(sample))

		if len(short) != 8 {
			t.Errorf("ShortHashOf(%q) = %q, want eight hex characters", sample, short)
		}
		digest := strings.TrimPrefix(full, "sha256:")
		if want := digest[:len(short)]; short != want {
			t.Errorf("ShortHashOf(%q) = %q, want the head of %q", sample, short, full)
		}
		if strings.ContainsRune(short, ':') {
			t.Errorf("ShortHashOf(%q) = %q, want no algorithm prefix in a filename", sample, short)
		}
	}
}
