package actions

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// Index is an in-memory BM25 search index over action documentation.
type Index struct {
	actions []Action
	docs    []document   // tokenized documents, one per action
	avgDL   float64      // average document length
	df      map[string]int // document frequency per term
	n       int          // total number of documents
}

// document holds the pre-tokenized content for a single action.
type document struct {
	terms []string          // all tokens in the document
	tf    map[string]int    // term frequency
}

// BM25 parameters
const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// NewIndex builds a BM25 index from the given actions.
func NewIndex(actions []Action) *Index {
	idx := &Index{
		actions: make([]Action, len(actions)),
		docs:    make([]document, len(actions)),
		df:      make(map[string]int),
		n:       len(actions),
	}
	copy(idx.actions, actions)

	totalLen := 0
	for i, a := range actions {
		tokens := tokenizeAction(a)
		tf := make(map[string]int, len(tokens))
		for _, t := range tokens {
			tf[t]++
		}
		idx.docs[i] = document{terms: tokens, tf: tf}
		totalLen += len(tokens)
	}

	if idx.n > 0 {
		idx.avgDL = float64(totalLen) / float64(idx.n)
	}

	// Compute document frequency for each term.
	for _, doc := range idx.docs {
		seen := make(map[string]bool, len(doc.tf))
		for term := range doc.tf {
			if !seen[term] {
				idx.df[term]++
				seen[term] = true
			}
		}
	}

	return idx
}

// Search returns actions ranked by BM25 relevance to the query.
// An empty query returns all actions (useful for browsing).
func (idx *Index) Search(query string, limit int) []SearchResult {
	if limit <= 0 {
		limit = idx.n
	}

	// Empty query: return all actions with score 0.
	queryTerms := tokenizeQuery(query)
	if len(queryTerms) == 0 {
		results := make([]SearchResult, len(idx.actions))
		for i, a := range idx.actions {
			results[i] = SearchResult{Action: a, Score: 0}
		}
		if limit < len(results) {
			return results[:limit]
		}
		return results
	}

	// Score each document.
	type scored struct {
		index int
		score float64
	}
	var candidates []scored

	for i, doc := range idx.docs {
		score := idx.bm25Score(doc, queryTerms)
		if score > 0 {
			candidates = append(candidates, scored{index: i, score: score})
		}
	}

	// Sort by score descending.
	sort.Slice(candidates, func(a, b int) bool {
		return candidates[a].score > candidates[b].score
	})

	if limit < len(candidates) {
		candidates = candidates[:limit]
	}

	results := make([]SearchResult, len(candidates))
	for i, c := range candidates {
		results[i] = SearchResult{
			Action: idx.actions[c.index],
			Score:  c.score,
		}
	}
	return results
}

// bm25Score computes the BM25 score for a document against query terms.
func (idx *Index) bm25Score(doc document, queryTerms []string) float64 {
	dl := float64(len(doc.terms))
	score := 0.0

	for _, qt := range queryTerms {
		tfVal := float64(doc.tf[qt])
		if tfVal == 0 {
			continue
		}

		nq := float64(idx.df[qt])
		// IDF: ln((N - n(q) + 0.5) / (n(q) + 0.5) + 1)
		idf := math.Log((float64(idx.n)-nq+0.5)/(nq+0.5) + 1)

		// BM25 TF component
		num := tfVal * (bm25K1 + 1)
		denom := tfVal + bm25K1*(1-bm25B+bm25B*dl/idx.avgDL)
		score += idf * num / denom
	}

	return score
}

// tokenizeAction builds a searchable token list from an action's fields.
// The action name is included both as a compound token (e.g. "send_message"
// preserved as-is) and as split tokens. This ensures that an exact-name query
// like "send_message" strongly favours the action with that exact name over
// actions that merely contain the same sub-words.
func tokenizeAction(a Action) []string {
	var tokens []string

	// Add the full action name as a compound token (lowercased but not split).
	// Repeat to boost exact-name matches.
	compound := strings.ToLower(a.Name)
	for i := 0; i < 5; i++ {
		tokens = append(tokens, compound)
	}

	// Also add the split tokens from the name.
	for i := 0; i < 3; i++ {
		tokens = append(tokens, tokenize(a.Name)...)
	}

	// Index remaining fields normally.
	var parts []string
	parts = append(parts, a.Category)
	parts = append(parts, a.Description)
	parts = append(parts, a.Returns)

	for _, p := range a.Params {
		parts = append(parts, p.Name)
		parts = append(parts, p.Description)
	}
	for _, ex := range a.Examples {
		parts = append(parts, ex.Description)
	}

	tokens = append(tokens, tokenize(strings.Join(parts, " "))...)
	return tokens
}

// tokenizeQuery produces query tokens. It includes the standard split tokens
// plus any compound tokens (underscore-joined words) found in the original
// query. This allows "send_message" to match the compound name token.
func tokenizeQuery(text string) []string {
	tokens := tokenize(text)

	// Also add compound tokens for underscore-joined words in the query.
	lower := strings.ToLower(text)
	words := strings.Fields(lower)
	for _, w := range words {
		if strings.Contains(w, "_") {
			// The compound token is the full underscore-joined word, lowered.
			// Strip non-alphanumeric/underscore chars from edges.
			cleaned := strings.TrimFunc(w, func(r rune) bool {
				return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
			})
			if cleaned != "" {
				tokens = append(tokens, cleaned)
			}
		}
	}

	return tokens
}

// tokenize splits text into lowercase tokens, splitting on whitespace and
// punctuation, then applies simple stemming to normalize plurals.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	var out []string
	for _, w := range words {
		if len(w) > 0 {
			out = append(out, simpleStem(w))
		}
	}
	return out
}

// simpleStem applies minimal suffix stripping so that plurals and common
// inflections match their base forms (e.g. "channels" -> "channel",
// "messages" -> "message"). This is intentionally simple — no external
// stemming library is needed for ~23 documents.
func simpleStem(w string) string {
	// Order matters: try longer suffixes first.
	if strings.HasSuffix(w, "sses") {
		// "addresses" -> "address"
		return w
	}
	if strings.HasSuffix(w, "ies") && len(w) > 4 {
		return w[:len(w)-3] + "y"
	}
	if strings.HasSuffix(w, "es") && len(w) > 3 {
		base := w[:len(w)-2]
		// Only strip -es after s, x, z, ch, sh (English rule).
		if strings.HasSuffix(base, "s") || strings.HasSuffix(base, "x") ||
			strings.HasSuffix(base, "z") || strings.HasSuffix(base, "ch") ||
			strings.HasSuffix(base, "sh") {
			return base
		}
		// Otherwise strip just the trailing 's'.
		return w[:len(w)-1]
	}
	if strings.HasSuffix(w, "s") && len(w) > 3 && !strings.HasSuffix(w, "ss") {
		return w[:len(w)-1]
	}
	return w
}
