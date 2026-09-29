package index

import "math"

// BM25 parameters and field weights.
//
// The weights are what make a page called "Retrieval" outrank one that merely
// mentions the word: a term in the title counts five times one in the body, and
// a tag three times. k1 and b are the usual saturation and length-normalisation
// constants. Nothing here is tuned against a corpus yet; the delegate's
// benchmark is where the numbers would be revisited if they ever needed to be.
const (
	k1 = 1.2
	b  = 0.75

	weightTitle = 5.0
	weightTags  = 3.0
	weightBody  = 1.0
)

// Doc is a page reduced to what ranking needs: its tokens, split by field.
type Doc struct {
	Title []string
	Tags  []string
	Body  []string
}

// Length is the document's weighted length in tokens.
func (d Doc) Length() float64 {
	return WeightedLength(d.Title, d.Tags, d.Body)
}

// WeightedLength is the BM25 document length for tokenized fields. It is
// exported so that the index can store the same number the scorer computes
// rather than deriving a second one that could drift.
func WeightedLength(title, tags, body []string) float64 {
	return weightTitle*float64(len(title)) +
		weightTags*float64(len(tags)) +
		weightBody*float64(len(body))
}

// Stats is the corpus-wide half of a BM25 score: how many documents there are,
// how long they average, and how many contain each query term.
type Stats struct {
	N      int
	AvgLen float64
	DF     map[string]int
}

// Score returns the BM25 score of one document for a query.
//
// The fields combine by weight: a term's frequency is its weighted count across
// title, tags and body, and the document's length is weighted the same way. A
// query term repeated by the caller is counted once, because a word typed twice
// should not rank a document twice as high.
//
// Both tiers call this function, so the same query returns the same scores
// whether or not an index exists. That is the whole reason ranking lives here
// rather than in SQL.
func Score(query []string, d Doc, s Stats) float64 {
	if s.N == 0 {
		return 0
	}
	avg := s.AvgLen
	if avg <= 0 {
		avg = 1
	}

	tf := map[string]float64{}
	for _, t := range d.Title {
		tf[t] += weightTitle
	}
	for _, t := range d.Tags {
		tf[t] += weightTags
	}
	for _, t := range d.Body {
		tf[t] += weightBody
	}

	length := d.Length()
	score := 0.0
	seen := map[string]bool{}
	for _, term := range query {
		if term == "" || seen[term] {
			continue
		}
		seen[term] = true

		f := tf[term]
		if f == 0 {
			continue
		}
		df := s.DF[term]
		idf := math.Log(1 + (float64(s.N)-float64(df)+0.5)/(float64(df)+0.5))
		score += idf * (f * (k1 + 1)) / (f + k1*(1-b+b*length/avg))
	}
	return score
}
