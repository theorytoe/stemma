package index

import (
	"reflect"
	"testing"
	"unicode"
)

func TestTokenize(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want []string
	}{
		{"words", "Hello, World!", []string{"hello", "world"}},
		{"camel case stays one token", "getUserName", []string{"getusername"}},
		{"underscores separate", "get_user_name", []string{"get", "user", "name"}},
		{"hyphens separate", "machine-learning", []string{"machine", "learning"}},
		{"accents fold", "café naïve", []string{"cafe", "naive"}},
		{"uppercase accents fold", "CAFÉ", []string{"cafe"}},
		{"digits are tokens", "RFC 2119", []string{"rfc", "2119"}},
		{"non-latin survives", "привет мир", []string{"привет", "мир"}},
		{"emoji separate", "a 👍 b", []string{"a", "b"}},
		{"empty", "", nil},
		{"punctuation only", "— ...", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Tokenize(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Tokenize(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// The table is read in pairs, so an odd length would silently drop the last
// entry, and a non-letter would mean the fold produces something that is not a
// word.
func TestFoldPairsAreWellFormed(t *testing.T) {
	rs := []rune(foldPairs)
	if len(rs)%2 != 0 {
		t.Fatalf("foldPairs has %d runes, want an even number", len(rs))
	}
	for i := 0; i < len(rs); i += 2 {
		if !unicode.IsLetter(rs[i]) || !unicode.IsLetter(rs[i+1]) {
			t.Errorf("pair %q is not two letters", string(rs[i:i+2]))
		}
	}
}
