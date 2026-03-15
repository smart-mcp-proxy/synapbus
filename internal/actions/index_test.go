package actions

import (
	"testing"
)

func TestRegistry_List(t *testing.T) {
	reg := NewRegistry()
	actions := reg.List()
	if len(actions) == 0 {
		t.Fatal("expected actions to be registered")
	}

	// Check that core actions exist
	expectedNames := []string{
		"read_inbox", "claim_messages", "mark_done", "search_messages",
		"discover_agents", "create_channel", "join_channel", "list_channels",
		"send_channel_message", "post_task", "upload_attachment",
	}
	nameSet := make(map[string]bool)
	for _, a := range actions {
		nameSet[a.Name] = true
	}
	for _, name := range expectedNames {
		if !nameSet[name] {
			t.Errorf("expected action %q in registry", name)
		}
	}
}

func TestRegistry_Get(t *testing.T) {
	reg := NewRegistry()

	t.Run("existing action", func(t *testing.T) {
		a := reg.Get("read_inbox")
		if a == nil {
			t.Fatal("expected to find read_inbox")
		}
		if a.Category != "messaging" {
			t.Errorf("category = %q, want messaging", a.Category)
		}
	})

	t.Run("missing action", func(t *testing.T) {
		a := reg.Get("nonexistent")
		if a != nil {
			t.Error("expected nil for nonexistent action")
		}
	})
}

func TestIndex_Search(t *testing.T) {
	reg := NewRegistry()
	idx := NewIndex(reg.List())

	t.Run("messaging query", func(t *testing.T) {
		results := idx.Search("read inbox messages", 5)
		if len(results) == 0 {
			t.Fatal("expected results for 'read inbox messages'")
		}
		// read_inbox should be the top result
		if results[0].Action.Name != "read_inbox" {
			t.Errorf("top result = %q, want read_inbox", results[0].Action.Name)
		}
		if results[0].Score <= 0 {
			t.Error("expected positive relevance score")
		}
	})

	t.Run("channel query", func(t *testing.T) {
		results := idx.Search("create channel", 5)
		if len(results) == 0 {
			t.Fatal("expected results for 'create channel'")
		}
		foundCreateChannel := false
		for _, r := range results {
			if r.Action.Name == "create_channel" {
				foundCreateChannel = true
				break
			}
		}
		if !foundCreateChannel {
			t.Error("expected create_channel in results")
		}
	})

	t.Run("swarm query", func(t *testing.T) {
		results := idx.Search("task auction bid", 5)
		if len(results) == 0 {
			t.Fatal("expected results for 'task auction bid'")
		}
	})

	t.Run("empty query returns all", func(t *testing.T) {
		results := idx.Search("", 20)
		if len(results) == 0 {
			t.Fatal("expected results for empty query")
		}
		// Should return all registered actions (up to limit)
		totalActions := len(reg.List())
		if len(results) > 20 {
			t.Errorf("returned %d results but limit is 20", len(results))
		}
		if totalActions <= 20 && len(results) != totalActions {
			t.Errorf("expected %d results in browse mode, got %d", totalActions, len(results))
		}
	})

	t.Run("limit enforced", func(t *testing.T) {
		results := idx.Search("message", 2)
		if len(results) > 2 {
			t.Errorf("expected at most 2 results, got %d", len(results))
		}
	})

	t.Run("max limit capped at 20", func(t *testing.T) {
		results := idx.Search("", 100)
		if len(results) > 20 {
			t.Errorf("expected at most 20 results, got %d", len(results))
		}
	})
}
