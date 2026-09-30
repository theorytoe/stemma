package cli

import (
	"flag"
	"strings"
	"testing"
)

func TestWantsHelp(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{[]string{"--help"}, true},
		{[]string{"-h"}, true},
		{[]string{"-help"}, true},
		{[]string{"Alpha", "--help"}, true},
		{[]string{"--json"}, false},
		{[]string{"--", "--help"}, false},
	} {
		if got := wantsHelp(tc.args); got != tc.want {
			t.Errorf("wantsHelp(%q) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestWantsJSON(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{[]string{"--json"}, true},
		{[]string{"-json"}, true},
		{[]string{"Alpha", "--json"}, true},
		{[]string{"--strict"}, false},
		{[]string{"--", "--json"}, false},
	} {
		if got := wantsJSON(tc.args); got != tc.want {
			t.Errorf("wantsJSON(%q) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestFind(t *testing.T) {
	if find("lint") == nil {
		t.Error("a command in the table was not found")
	}
	if find("frobnicate") != nil {
		t.Error("an unknown command was found")
	}
}

// The help generator reads a flag's type and default off the flag itself, so
// these have to be right for every kind of value the surface uses.
func TestFlagTypeAndDefault(t *testing.T) {
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	var (
		s    string
		n    int
		b    bool
		list stringList
	)
	fs.StringVar(&s, "title", "", "t")
	fs.IntVar(&n, "depth", 1, "d")
	fs.BoolVar(&b, "draft", false, "b")
	fs.Var(&list, "tag", "g")

	got := map[string]flagDoc{}
	fs.VisitAll(func(f *flag.Flag) {
		got[f.Name] = flagDoc{Type: flagType(f), Default: flagDefault(f)}
	})
	want := map[string]flagDoc{
		"title": {Type: "string", Default: ""},
		"depth": {Type: "int", Default: "1"},
		"draft": {Type: "bool", Default: ""},
		"tag":   {Type: "list", Default: ""},
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s = %+v, want %+v", name, got[name], w)
		}
	}
}

// A noun family has no flags of its own, and its help lists its members.
func TestCommandHelpForAFamily(t *testing.T) {
	family := &command{
		name:    "export",
		summary: "export the KB",
		sub:     []*command{{name: "json", summary: "dump the KB as JSON"}},
	}
	out := commandHelp(family.name, family)
	for _, want := range []string{"usage: stemma export", "commands:", "json"} {
		if !strings.Contains(out, want) {
			t.Errorf("help is missing %q:\n%s", want, out)
		}
	}
}

// A family member's help names the family it belongs to, so the usage line is
// unambiguous about which verb was asked for.
func TestCommandHelpForAFamilyMemberNamesItsFamily(t *testing.T) {
	member := &command{name: "add", summary: "add a source"}
	out := commandHelp("cite add", member)
	if !strings.Contains(out, "usage: stemma cite add") {
		t.Errorf("member help omits the family:\n%s", out)
	}
}

// A "--" ends flag parsing, so anything after it is positional even if it
// looks like a flag.
func TestPermuteStopsAtTheTerminator(t *testing.T) {
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	draft := fs.Bool("draft", false, "")
	got := permute(fs, []string{"Alpha", "--", "--draft"})
	if strings.Join(got, " ") != "-- Alpha --draft" {
		t.Errorf("permute = %q", got)
	}
	if err := fs.Parse(got); err != nil {
		t.Fatal(err)
	}
	if *draft || fs.NArg() != 2 {
		t.Errorf("draft = %v, args = %q", *draft, fs.Args())
	}

	// The terminator survives even with nothing before it, which is what a
	// title like "--draft" needs: flag.Parse must see the "--" and stop, or
	// it reads the title as a flag.
	got = permute(fs, []string{"--", "--draft"})
	if err := fs.Parse(got); err != nil {
		t.Fatal(err)
	}
	if *draft || fs.NArg() != 1 || fs.Args()[0] != "--draft" {
		t.Errorf("draft = %v, args = %q", *draft, fs.Args())
	}
}

func TestUnderDir(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
		bad  bool
	}{
		{"", "", false},
		{".", "", false},
		{"pages", "pages", false},
		{"pages/deep", "pages/deep", false},
		{"deep", "pages/deep", false},
		{"../escape", "", true},
		{"/absolute", "", true},
	} {
		got, err := underDir(tc.in)
		if tc.bad {
			if err == nil {
				t.Errorf("underDir(%q) = %q, want an error", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("underDir(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
}

func TestStringListCollectsEveryUse(t *testing.T) {
	var list stringList
	if list.String() != "" {
		t.Errorf("String on an empty list = %q", list.String())
	}
	list.Set("a")
	list.Set("b")
	if list.String() != "a,b" {
		t.Errorf("String = %q", list.String())
	}
	if got := list.Get().([]string); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("Get = %q", got)
	}
}
