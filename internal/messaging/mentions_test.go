package messaging

import (
	"reflect"
	"testing"
)

func TestParseMentions(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "no mentions",
			body: "hello world",
			want: nil,
		},
		{
			name: "single mention at start",
			body: "@agent-a check this out",
			want: []string{"agent-a"},
		},
		{
			name: "single mention in middle",
			body: "hey @agent-b can you help?",
			want: []string{"agent-b"},
		},
		{
			name: "single mention at end",
			body: "please review @agent-c",
			want: []string{"agent-c"},
		},
		{
			name: "multiple different mentions",
			body: "@agent-a and @agent-b should coordinate with @agent-c",
			want: []string{"agent-a", "agent-b", "agent-c"},
		},
		{
			name: "duplicate mentions deduplicated",
			body: "@agent-a please tell @agent-a to check",
			want: []string{"agent-a"},
		},
		{
			name: "mention with underscore",
			body: "cc @my_agent_1",
			want: []string{"my_agent_1"},
		},
		{
			name: "mention with hyphen",
			body: "cc @my-agent-1",
			want: []string{"my-agent-1"},
		},
		{
			name: "bare @ at end of string",
			body: "hello @",
			want: nil,
		},
		{
			name: "double @@",
			body: "hello @@agent-a",
			want: nil,
		},
		{
			name: "email address not matched",
			body: "send to user@example.com please",
			want: nil,
		},
		{
			name: "email-like with dot before @",
			body: "contact john.doe@company.org for details",
			want: nil,
		},
		{
			name: "mention after newline",
			body: "line one\n@agent-a check this",
			want: []string{"agent-a"},
		},
		{
			name: "mention after parenthesis",
			body: "see (@agent-a) for details",
			want: []string{"agent-a"},
		},
		{
			name: "mention after comma",
			body: "thanks,@agent-b",
			want: []string{"agent-b"},
		},
		{
			name: "case insensitive dedup",
			body: "@Agent-A and @agent-a",
			want: []string{"agent-a"},
		},
		{
			name: "mention with numbers",
			body: "hey @agent123",
			want: []string{"agent123"},
		},
		{
			name: "empty string",
			body: "",
			want: nil,
		},
		{
			name: "only @",
			body: "@",
			want: nil,
		},
		{
			name: "mention followed by punctuation",
			body: "@agent-a, @agent-b! @agent-c.",
			want: []string{"agent-a", "agent-b", "agent-c"},
		},
		{
			name: "mention at start of line after space",
			body: "  @agent-a",
			want: []string{"agent-a"},
		},
		{
			name: "agent name starting with number",
			body: "@1agent is here",
			want: []string{"1agent"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseMentions(tt.body)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseMentions(%q) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}
