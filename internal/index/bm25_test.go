package index

import "testing"

func TestScorePrefersTitleAndRepetition(t *testing.T) {
	// A title match and a body match, in a corpus of exactly those two.
	titled := Doc{Title: []string{"retrieval"}}
	bodied := Doc{Body: []string{"retrieval"}}
	stats := Stats{N: 2, AvgLen: (titled.Length() + bodied.Length()) / 2, DF: map[string]int{"retrieval": 2}}
	if Score([]string{"retrieval"}, titled, stats) <= Score([]string{"retrieval"}, bodied, stats) {
		t.Error("a title match should outrank a body match")
	}

	once := Doc{Body: []string{"retrieval"}}
	twice := Doc{Body: []string{"retrieval", "retrieval"}}
	stats = Stats{N: 2, AvgLen: (once.Length() + twice.Length()) / 2, DF: map[string]int{"retrieval": 2}}
	if Score([]string{"retrieval"}, twice, stats) <= Score([]string{"retrieval"}, once, stats) {
		t.Error("a document that says a thing twice should outrank one that says it once")
	}
}

// A word typed twice is one question, and a word no page contains contributes
// nothing; neither should change a score.
func TestScoreIgnoresDuplicatesAndAbsentees(t *testing.T) {
	d := Doc{Body: []string{"alpha"}}
	stats := Stats{N: 1, AvgLen: d.Length(), DF: map[string]int{"alpha": 1}}
	if once, twice := Score([]string{"alpha"}, d, stats), Score([]string{"alpha", "alpha"}, d, stats); once != twice {
		t.Errorf("a repeated term changed the score: %v vs %v", once, twice)
	}
	if got := Score([]string{"missing"}, d, stats); got != 0 {
		t.Errorf("an absent term scored %v, want 0", got)
	}
}

// An empty corpus is not a division by zero.
func TestScoreOnAnEmptyCorpus(t *testing.T) {
	if got := Score([]string{"x"}, Doc{Body: []string{"x"}}, Stats{N: 0}); got != 0 {
		t.Errorf("score = %v, want 0", got)
	}
}
