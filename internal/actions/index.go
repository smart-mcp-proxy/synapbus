package actions

import (
	"math"
	"sort"
	"strings"
)

// SearchResult pairs an action with a relevance score.
type SearchResult struct {
	Action Action  `json:"action"`
	Score  float64 `json:"score"`
}

// Index provides BM25 search over the action catalog.
type Index struct {
	actions []Action
	// Pre-computed document tokens (name + category + description + param names).
	docs [][]string
	// IDF values per term across all documents.
	idf map[string]float64
	// Average document length.
	avgDL float64
}

// NewIndex builds a BM25 index from the provided actions.
func NewIndex(actions []Action) *Index {
	idx := &Index{
		actions: actions,
		docs:    make([][]string, len(actions)),
		idf:     make(map[string]float64),
	}

	// Tokenize each action into a bag of words.
	df := make(map[string]int) // document frequency per term
	totalLen := 0
	for i, a := range actions {
		tokens := tokenize(a)
		idx.docs[i] = tokens
		totalLen += len(tokens)
		// Count unique terms in this document.
		seen := make(map[string]bool)
		for _, t := range tokens {
			if !seen[t] {
				df[t]++
				seen[t] = true
			}
		}
	}

	n := float64(len(actions))
	if n > 0 {
		idx.avgDL = float64(totalLen) / n
	}

	// Compute IDF for each term.
	for term, freq := range df {
		idx.idf[term] = math.Log(1 + (n-float64(freq)+0.5)/(float64(freq)+0.5))
	}

	return idx
}

// Search returns actions matching the query, sorted by relevance score.
// If query is empty, returns all actions with score 0 (browse mode).
func (idx *Index) Search(query string, limit int) []SearchResult {
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}

	// Browse mode: return all actions.
	if strings.TrimSpace(query) == "" {
		results := make([]SearchResult, len(idx.actions))
		for i, a := range idx.actions {
			results[i] = SearchResult{Action: a, Score: 0}
		}
		if len(results) > limit {
			results = results[:limit]
		}
		return results
	}

	queryTerms := strings.Fields(strings.ToLower(query))

	// BM25 parameters.
	const k1 = 1.2
	const b = 0.75

	type scored struct {
		idx   int
		score float64
	}

	var scored_docs []scored
	for i, docTokens := range idx.docs {
		score := 0.0
		dl := float64(len(docTokens))
		tf := termFrequency(docTokens)

		for _, qt := range queryTerms {
			idfVal := idx.idf[qt]
			freq := float64(tf[qt])
			if freq == 0 {
				continue
			}
			numerator := freq * (k1 + 1)
			denominator := freq + k1*(1-b+b*dl/idx.avgDL)
			score += idfVal * numerator / denominator
		}

		if score > 0 {
			scored_docs = append(scored_docs, scored{idx: i, score: score})
		}
	}

	sort.Slice(scored_docs, func(i, j int) bool {
		return scored_docs[i].score > scored_docs[j].score
	})

	if len(scored_docs) > limit {
		scored_docs = scored_docs[:limit]
	}

	results := make([]SearchResult, len(scored_docs))
	for i, sd := range scored_docs {
		results[i] = SearchResult{
			Action: idx.actions[sd.idx],
			Score:  sd.score,
		}
	}

	return results
}

// tokenize extracts searchable tokens from an action.
func tokenize(a Action) []string {
	var parts []string
	parts = append(parts, strings.Fields(strings.ToLower(a.Name))...)
	parts = append(parts, strings.Fields(strings.ToLower(a.Category))...)
	parts = append(parts, strings.Fields(strings.ToLower(a.Description))...)
	for _, p := range a.Params {
		parts = append(parts, strings.Fields(strings.ToLower(p.Name))...)
		parts = append(parts, strings.Fields(strings.ToLower(p.Description))...)
	}
	// Split compound names (e.g. "read_inbox" -> "read", "inbox").
	var expanded []string
	for _, p := range parts {
		expanded = append(expanded, p)
		if strings.Contains(p, "_") {
			expanded = append(expanded, strings.Split(p, "_")...)
		}
		if strings.Contains(p, "-") {
			expanded = append(expanded, strings.Split(p, "-")...)
		}
	}
	return expanded
}

// termFrequency counts occurrences of each term in a token list.
func termFrequency(tokens []string) map[string]int {
	tf := make(map[string]int)
	for _, t := range tokens {
		tf[t]++
	}
	return tf
}
