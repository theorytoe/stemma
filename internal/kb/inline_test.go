package kb

import (
	"reflect"
	"testing"
)

// TestInlinesSliceTheBody checks the contract a renderer relies on: a span
// taken out of Body() is the construct itself, brackets and all.
func TestInlinesSliceTheBody(t *testing.T) {
	p := mustParse(t, "---\ntitle: S\ntype: note\n---\nSee [[attention]] and [@vaswani2017] here.\n")
	body := p.Body()
	in := p.Inlines()
	if len(in) != 2 {
		t.Fatalf("Inlines = %d, want 2: %+v", len(in), in)
	}

	if got := string(body[in[0].Start:in[0].End]); got != "[[attention]]" {
		t.Errorf("first span = %q, want %q", got, "[[attention]]")
	}
	if in[0].Link == nil || in[0].Link.Target != "attention" {
		t.Errorf("first inline = %+v", in[0])
	}

	if got := string(body[in[1].Start:in[1].End]); got != "[@vaswani2017]" {
		t.Errorf("second span = %q, want %q", got, "[@vaswani2017]")
	}
	if len(in[1].Cites) != 1 || in[1].Cites[0].Key != "vaswani2017" {
		t.Errorf("second inline = %+v", in[1])
	}
}

func TestInlinesSkipCode(t *testing.T) {
	p := mustParse(t, "---\ntitle: S\ntype: note\n---\n`[[a]]` and [[b]]\n\n```\n[[c]]\n[@d]\n```\n")
	body := p.Body()
	var got []string
	for _, in := range p.Inlines() {
		got = append(got, string(body[in.Start:in.End]))
	}
	if want := []string{"[[b]]"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Inlines = %v, want %v", got, want)
	}
}

// TestInlinesKeepALinkWithEmphasisWhole is the case a tree walk would miss:
// blackfriday splits "[[a *b* c]]" into three nodes, but it is one link, and
// the scanner has to see one construct so the renderer can replace it whole.
func TestInlinesKeepALinkWithEmphasisWhole(t *testing.T) {
	p := mustParse(t, "---\ntitle: S\ntype: note\n---\n[[a *b* c]]\n")
	in := p.Inlines()
	if len(in) != 1 || in[0].Link == nil {
		t.Fatalf("Inlines = %+v, want one link", in)
	}
	if got, want := in[0].Link.Target, "a *b* c"; got != want {
		t.Errorf("target = %q, want %q", got, want)
	}
	if got, want := string(p.Body()[in[0].Start:in[0].End]), "[[a *b* c]]"; got != want {
		t.Errorf("span = %q, want %q", got, want)
	}
}

// TestCitationGroupsAreNumberedInOrder checks that a group is one construct and
// that group numbers run across runs, which is what makes a page's citations
// split into the groups they were written in.
func TestCitationGroupsAreNumberedInOrder(t *testing.T) {
	p := mustParse(t, "---\ntitle: S\ntype: note\n---\n[@a; @b] then `code` [@c]\n")
	cites := p.Citations()
	if len(cites) != 3 {
		t.Fatalf("Citations = %d, want 3: %+v", len(cites), cites)
	}
	if cites[0].Group != 0 || cites[0].Position != 0 || cites[1].Group != 0 || cites[1].Position != 1 {
		t.Errorf("first group = %+v and %+v", cites[0], cites[1])
	}
	if cites[2].Group != 1 || cites[2].Position != 0 {
		t.Errorf("second group = %+v", cites[2])
	}

	in := p.Inlines()
	if len(in) != 2 {
		t.Fatalf("Inlines = %d, want 2 groups: %+v", len(in), in)
	}
	if len(in[0].Cites) != 2 || len(in[1].Cites) != 1 {
		t.Errorf("groups hold %d and %d citations, want 2 and 1", len(in[0].Cites), len(in[1].Cites))
	}
}
