package actions

import (
	"testing"
)

func TestSearchRelevantResults(t *testing.T) {
	r := NewRegistry()
	idx := NewIndex(r.List())

	tests := []struct {
		name      string
		query     string
		wantNames []string // expected action names in results (order-independent)
	}{
		{
			name:      "send message finds send_message and send_channel_message",
			query:     "send message",
			wantNames: []string{"send_message", "send_channel_message"},
		},
		{
			name:      "inbox finds read_inbox",
			query:     "inbox",
			wantNames: []string{"read_inbox"},
		},
		{
			name:      "channel finds channel actions",
			query:     "channel",
			wantNames: []string{"create_channel", "join_channel", "leave_channel", "list_channels"},
		},
		{
			name:      "task finds swarm actions",
			query:     "task",
			wantNames: []string{"post_task", "bid_task", "complete_task", "list_tasks"},
		},
		{
			name:      "attachment finds attachment actions",
			query:     "attachment",
			wantNames: []string{"upload_attachment", "download_attachment"},
		},
		{
			name:      "bid finds bid_task",
			query:     "bid",
			wantNames: []string{"bid_task", "accept_bid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := idx.Search(tt.query, 0)
			resultNames := make(map[string]bool, len(results))
			for _, r := range results {
				resultNames[r.Action.Name] = true
			}
			for _, want := range tt.wantNames {
				if !resultNames[want] {
					t.Errorf("query %q: expected %q in results, got %v", tt.query, want, nameList(results))
				}
			}
		})
	}
}

func TestSearchExactNameRanksHigher(t *testing.T) {
	r := NewRegistry()
	idx := NewIndex(r.List())

	// "send_message" as a query should rank send_message above send_channel_message
	results := idx.Search("send_message", 0)
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	// The top result should be send_message (exact name match).
	if results[0].Action.Name != "send_message" {
		t.Errorf("expected send_message as top result, got %q", results[0].Action.Name)
	}

	// Verify score ordering.
	if results[0].Score <= results[1].Score {
		t.Errorf("expected top result score (%f) > second result score (%f)",
			results[0].Score, results[1].Score)
	}
}

func TestSearchEmptyQueryReturnsAll(t *testing.T) {
	r := NewRegistry()
	idx := NewIndex(r.List())

	results := idx.Search("", 0)
	if len(results) != 23 {
		t.Errorf("empty query: expected 23 results, got %d", len(results))
	}

	// All scores should be 0 for empty query.
	for _, res := range results {
		if res.Score != 0 {
			t.Errorf("empty query: expected score 0 for %q, got %f", res.Action.Name, res.Score)
		}
	}
}

func TestSearchLimitRespected(t *testing.T) {
	r := NewRegistry()
	idx := NewIndex(r.List())

	tests := []struct {
		query string
		limit int
	}{
		{"message", 1},
		{"channel", 3},
		{"", 5},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results := idx.Search(tt.query, tt.limit)
			if len(results) > tt.limit {
				t.Errorf("query %q limit %d: got %d results", tt.query, tt.limit, len(results))
			}
		})
	}
}

func TestSearchNoResults(t *testing.T) {
	r := NewRegistry()
	idx := NewIndex(r.List())

	results := idx.Search("xyzzyplugh", 10)
	if len(results) != 0 {
		t.Errorf("nonsense query: expected 0 results, got %d", len(results))
	}
}

func TestSearchScoresPositive(t *testing.T) {
	r := NewRegistry()
	idx := NewIndex(r.List())

	results := idx.Search("send message agent", 10)
	for _, res := range results {
		if res.Score <= 0 {
			t.Errorf("non-empty query matched %q with non-positive score %f",
				res.Action.Name, res.Score)
		}
	}
}

func TestSearchDescending(t *testing.T) {
	r := NewRegistry()
	idx := NewIndex(r.List())

	results := idx.Search("message", 0)
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted descending: score[%d]=%f > score[%d]=%f",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"send_message", []string{"send", "message"}},
		{"Hello, World!", []string{"hello", "world"}},
		{"JSON metadata object (optional)", []string{"json", "metadata", "object", "optional"}},
		{"", nil},
		{"  spaces  ", []string{"space"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := tokenize(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("tokenize(%q): got %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("tokenize(%q)[%d]: got %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestNewIndexEmpty(t *testing.T) {
	idx := NewIndex(nil)
	results := idx.Search("anything", 10)
	if len(results) != 0 {
		t.Errorf("empty index: expected 0 results, got %d", len(results))
	}

	results = idx.Search("", 10)
	if len(results) != 0 {
		t.Errorf("empty index empty query: expected 0 results, got %d", len(results))
	}
}

// nameList is a test helper that extracts action names from search results.
func nameList(results []SearchResult) []string {
	names := make([]string, len(results))
	for i, r := range results {
		names[i] = r.Action.Name
	}
	return names
}
