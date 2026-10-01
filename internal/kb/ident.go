package kb

import (
	"path/filepath"
	"strings"
)

// The identifier rules live here, once.
//
// Two places need them and they must not disagree. SameWork asks whether two
// records describe one work, and resolution asks an external service for a
// record. Two spellings of one DOI that normalise differently make two records
// as far as the bibliography is concerned, which is exactly the duplicate this
// package exists to catch — so there is one rule per identifier and both
// callers read it.

// NormalizeDOI reduces the ways a DOI is written to the DOI itself: lower case,
// no URL wrapper, and no query or fragment, because neither names the work.
func NormalizeDOI(s string) string {
	s = trimAnyPrefixFold(s,
		"https://doi.org/", "http://doi.org/",
		"https://dx.doi.org/", "http://dx.doi.org/",
		"doi:")
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	return strings.ToLower(strings.TrimSpace(s))
}

// NormalizeArXiv reduces the ways an arXiv identifier is written to the form
// the export endpoint expects.
//
// A version is kept: it names the version that was read, and dropping it here
// would silently answer a request for one version with another. ArXivIdentity
// drops it where identity rather than retrieval is the question.
func NormalizeArXiv(s string) string {
	s = trimAnyPrefixFold(s,
		"https://arxiv.org/abs/", "http://arxiv.org/abs/",
		"https://arxiv.org/pdf/", "http://arxiv.org/pdf/",
		"arxiv:")
	s = strings.TrimSuffix(s, ".pdf")
	return strings.ToLower(strings.TrimSpace(s))
}

// ArXivIdentity is an arXiv identifier without its version, because
// "1706.03762" and "1706.03762v2" are one paper.
func ArXivIdentity(s string) string {
	s = NormalizeArXiv(s)
	if i := strings.LastIndexByte(s, 'v'); i > 0 && allDigits(s[i+1:]) {
		s = s[:i]
	}
	return s
}

// NormalizeISBN is an ISBN without the separators people write it with.
func NormalizeISBN(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == 'x' || r == 'X':
			b.WriteRune('x')
		}
	}
	return b.String()
}

// NormalizeURL is a URL without a trailing slash, which is a difference a
// person does not mean to write.
func NormalizeURL(s string) string {
	return strings.TrimRight(strings.TrimSpace(s), "/")
}

// NormalizePath reduces the ways one file on this machine is written to the path
// it names, so "./notes/paper.pdf" and "notes/paper.pdf" are one pointer. It is
// the filesystem's own spelling, because a path names a file and means something
// only on the machine that wrote it.
func NormalizePath(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return filepath.Clean(s)
}

// trimAnyPrefixFold removes the first prefix that matches, ignoring case.
func trimAnyPrefixFold(s string, prefixes ...string) string {
	s = strings.TrimSpace(s)
	for _, p := range prefixes {
		if len(s) >= len(p) && strings.EqualFold(s[:len(p)], p) {
			return strings.TrimSpace(s[len(p):])
		}
	}
	return s
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isDigitByte(s[i]) {
			return false
		}
	}
	return true
}

// YearOf is the first four-digit year in a date, whatever else the date carries.
// A boundary check keeps a longer number from being read as one.
func YearOf(date string) string {
	for i := 0; i+4 <= len(date); i++ {
		if !isFourDigits(date[i : i+4]) {
			continue
		}
		if i > 0 && isDigitByte(date[i-1]) {
			continue
		}
		if i+4 < len(date) && isDigitByte(date[i+4]) {
			continue
		}
		return date[i : i+4]
	}
	return ""
}

func isFourDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isDigitByte(s[i]) {
			return false
		}
	}
	return true
}

func isDigitByte(b byte) bool { return '0' <= b && b <= '9' }
