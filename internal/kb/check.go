package kb

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// Finding codes for a bibliography check. Two of them are the codes lint
// already uses, because a duplicate key and a cited-but-absent key are the same
// fact whoever reports them; the rest are about a record's own provenance,
// which lint has no reason to look at.
const (
	CodeDuplicateWork    = "duplicate-work"
	CodeMissingRetrieved = "missing-retrieved"
	CodeMissingHash      = "missing-content-hash"
	CodeBadRetrieved     = "malformed-retrieved"
	CodeBadHash          = "malformed-content-hash"
	CodeMissingVendored  = "missing-vendored"
	CodeBadVendored      = "malformed-vendored-hash"
	CodeVendoredDrift    = "vendored-drift"
)

// CheckFindings reports what is wrong with the bibliography itself: a key
// defined twice, two keys that describe one work, a record whose provenance is
// missing or malformed, and a key a page cites that nothing defines.
//
// It is deliberately narrower than Lint. Lint reports everything wrong with the
// KB; this reports what a person fixing the evidence needs, which is why an
// uncited key is not here — that is what `cite list --uncited` is for.
func (k *KB) CheckFindings() []Finding {
	var out []Finding
	if k.Bibliography == nil {
		return out
	}

	for _, key := range k.Bibliography.Duplicates() {
		out = append(out, Finding{
			Severity: Warning,
			Code:     CodeCitationDuplicate,
			Path:     k.Bibliography.PathOf(key),
			Message:  quote(key) + " is defined more than once",
		})
	}

	out = append(out, k.duplicateWorkFindings()...)

	for _, key := range k.Bibliography.SortedKeys() {
		e, ok := k.Bibliography.Entry(key)
		if !ok {
			continue
		}
		path := k.Bibliography.PathOf(key)
		out = append(out, checkProvenance(path, key, e)...)
		out = append(out, vendoredFindings(k.Root, path, key, e)...)
	}

	for _, path := range k.Graph.Paths() {
		for _, c := range k.Graph.Citations(path) {
			if k.Bibliography.Has(c.Key) {
				continue
			}
			out = append(out, Finding{
				Severity: Warning,
				Code:     CodeCitationMissing,
				Path:     path,
				Line:     c.Line,
				Message:  "the bibliography has no entry for " + quote(c.Key),
			})
		}
	}

	sortFindings(out)
	return out
}

// duplicateWorkFindings reports keys that are different but name one work.
//
// The same paper reaches a bibliography under a fresh key whenever a different
// source mints it, so this is the duplicate that a key check cannot see. It is
// reported once per pair, on the key that comes first in sorted order.
//
// Records are indexed by what makes them one work rather than compared two at a
// time. The pairwise form is quadratic in the size of the bibliography, which is
// the size the directory form exists to allow, and it pays that cost even when
// it finds nothing. The index costs what the duplicates cost.
func (k *KB) duplicateWorkFindings() []Finding {
	keys := k.Bibliography.SortedKeys()

	// Identities are keyed by position in keys, which is sorted, so a pair is
	// always reported on the key that comes first.
	index := map[string][]int{}
	for i, key := range keys {
		e, ok := k.Bibliography.Entry(key)
		if !ok {
			continue
		}
		for _, id := range workIdentities(e) {
			index[id] = append(index[id], i)
		}
	}

	seen := map[[2]int]bool{}
	var pairs [][2]int
	for _, at := range index {
		for i := 0; i < len(at); i++ {
			for j := i + 1; j < len(at); j++ {
				p := [2]int{at[i], at[j]}
				if seen[p] {
					continue
				}
				seen[p] = true
				pairs = append(pairs, p)
			}
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i][0] != pairs[j][0] {
			return pairs[i][0] < pairs[j][0]
		}
		return pairs[i][1] < pairs[j][1]
	})

	out := make([]Finding, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, Finding{
			Severity: Warning,
			Code:     CodeDuplicateWork,
			Path:     k.Bibliography.PathOf(keys[p[0]]),
			Message:  fmt.Sprintf("%s and %s describe the same work", quote(keys[p[0]]), quote(keys[p[1]])),
		})
	}
	return out
}

// workIdentities is what makes a record one work with another, as the tokens
// SameWork compares: each identifier it carries, and its title, year and first
// author taken together. Two records share a token exactly when SameWork calls
// them one work.
func workIdentities(e *BibEntry) []string {
	var out []string
	add := func(kind, value string) {
		if value != "" {
			out = append(out, kind+":"+value)
		}
	}
	add("doi", NormalizeDOI(fieldValue(e, "doi")))
	add("eprint", ArXivIdentity(fieldValue(e, "eprint")))
	add("isbn", NormalizeISBN(fieldValue(e, "isbn")))
	add("url", NormalizeURL(fieldValue(e, "url")))

	title := Normalize(fieldValue(e, "title"))
	year := YearOf(fieldValue(e, "year"))
	author := firstSurname(e)
	if title != "" && year != "" && author != "" {
		out = append(out, "work:"+title+"\x1f"+year+"\x1f"+author)
	}
	return out
}

// checkProvenance reports a record that cannot say when it was fetched or what
// was fetched.
//
// A missing date or hash is not a defect in every record: a hand-entered source
// never had one. But it is the one thing that makes drift undetectable, so it is
// reported and a person decides.
//
// A vendored capture is the exception. It is evidence of its own, and it is
// held to account by vendoredFindings against the bytes on disk, so the record
// is not asked for a second hash and a date that a local file can never supply.
// A value that is present but malformed is still a defect and is still reported.
func checkProvenance(path, key string, e *BibEntry) []Finding {
	var out []Finding

	captured := hasCapture(e)

	retrieved, hasRetrieved := e.Value(FieldRetrieved)
	switch {
	case hasRetrieved && !validRetrieved(retrieved):
		out = append(out, Finding{
			Severity: Warning,
			Code:     CodeBadRetrieved,
			Path:     path,
			Field:    FieldRetrieved,
			Message:  quote(key) + " has a retrieval date that is not YYYY-MM-DD",
		})
	case !hasRetrieved && !captured:
		out = append(out, Finding{
			Severity: Warning,
			Code:     CodeMissingRetrieved,
			Path:     path,
			Field:    FieldRetrieved,
			Message:  quote(key) + " has no retrieval date",
		})
	}

	digest, hasHash := e.Value(FieldContentHash)
	switch {
	case hasHash && !validHash(digest):
		out = append(out, Finding{
			Severity: Warning,
			Code:     CodeBadHash,
			Path:     path,
			Field:    FieldContentHash,
			Message:  quote(key) + " has a content hash that is not <algorithm>:<hex>",
		})
	case !hasHash && !captured:
		out = append(out, Finding{
			Severity: Warning,
			Code:     CodeMissingHash,
			Path:     path,
			Field:    FieldContentHash,
			Message:  quote(key) + " has no content hash",
		})
	}

	return out
}

// hasCapture reports whether an entry records a well-formed vendored hash, which
// is what claims that a capture exists beside it. A malformed hash makes no such
// claim, and vendoredFindings reports it on its own.
func hasCapture(e *BibEntry) bool {
	recorded, ok := e.Value(FieldVendored)
	return ok && validHash(recorded)
}

// vendoredFindings checks a capture against the hash its entry recorded.
//
// It reads, and it reports. A capture and a record that disagree are both left
// exactly as they are, because the tool cannot know which of the two someone
// meant, and rewriting either of them would destroy the evidence of what happened.
// The finding names both hashes and leaves the choice to a person.
func vendoredFindings(root, path, key string, e *BibEntry) []Finding {
	recorded, claimed := e.Value(FieldVendored)
	if !claimed {
		return nil
	}

	finding := func(code, message string) []Finding {
		return []Finding{{
			Severity: Warning,
			Code:     code,
			Path:     path,
			Field:    FieldVendored,
			Message:  message,
		}}
	}

	if !validHash(recorded) {
		return finding(CodeBadVendored, quote(key)+" has a vendored hash that is not <algorithm>:<hex>")
	}
	if !strings.HasPrefix(recorded, "sha256:") {
		return finding(CodeBadVendored,
			quote(key)+" records a vendored hash this tool cannot compute, so the capture was not checked")
	}

	// The capture is the original document when one was kept beside the extracted
	// text, and the text itself otherwise; the recorded hash covers whichever it is.
	art, ok := FindOriginal(root, key)
	if !ok {
		return finding(CodeMissingVendored,
			quote(key)+" claims a vendored copy, and "+VendoredName(key)+" is not there")
	}
	have, err := os.ReadFile(art.Path)
	if err != nil {
		return finding(CodeMissingVendored,
			quote(key)+" claims a vendored copy, and "+art.Name+" could not be read: "+err.Error())
	}

	if found := HashOf(have); found != recorded {
		return finding(CodeVendoredDrift,
			quote(key)+" has a vendored copy in "+art.Name+" whose hash is "+found+", not the recorded "+recorded)
	}
	return nil
}

func validRetrieved(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// validHash accepts "<algorithm>:<hex>", the form the format documents. The
// algorithm is a name, not a fixed list, so any lowercase word is allowed; the
// digest is lowercase hexadecimal of even length, because that is what every
// hash function produces.
func validHash(s string) bool {
	i := strings.IndexByte(s, ':')
	if i <= 0 || i == len(s)-1 {
		return false
	}
	for _, r := range s[:i] {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-' {
			return false
		}
	}
	digest := s[i+1:]
	if len(digest)%2 != 0 {
		return false
	}
	for _, r := range digest {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
