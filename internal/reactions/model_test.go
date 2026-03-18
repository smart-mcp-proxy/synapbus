package reactions

import (
	"testing"
)

func TestIsValidReaction(t *testing.T) {
	tests := []struct {
		name     string
		reaction string
		want     bool
	}{
		{"approve is valid", ReactionApprove, true},
		{"reject is valid", ReactionReject, true},
		{"in_progress is valid", ReactionInProgress, true},
		{"done is valid", ReactionDone, true},
		{"published is valid", ReactionPublished, true},
		{"empty string is invalid", "", false},
		{"thumbs_up is invalid", "thumbs_up", false},
		{"like is invalid", "like", false},
		{"APPROVE uppercase is invalid", "APPROVE", false},
		{"random text is invalid", "foobar", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidReaction(tt.reaction)
			if got != tt.want {
				t.Errorf("IsValidReaction(%q) = %v, want %v", tt.reaction, got, tt.want)
			}
		})
	}
}

func TestComputeWorkflowState(t *testing.T) {
	tests := []struct {
		name      string
		reactions []*Reaction
		want      string
	}{
		{
			name:      "empty reactions returns proposed",
			reactions: []*Reaction{},
			want:      StateProposed,
		},
		{
			name:      "nil reactions returns proposed",
			reactions: nil,
			want:      StateProposed,
		},
		{
			name: "single approve returns approved",
			reactions: []*Reaction{
				{Reaction: ReactionApprove, AgentName: "agent-a"},
			},
			want: StateApproved,
		},
		{
			name: "approve + in_progress returns in_progress (higher priority wins)",
			reactions: []*Reaction{
				{Reaction: ReactionApprove, AgentName: "agent-a"},
				{Reaction: ReactionInProgress, AgentName: "agent-b"},
			},
			want: StateInProgress,
		},
		{
			name: "single reject returns rejected",
			reactions: []*Reaction{
				{Reaction: ReactionReject, AgentName: "agent-a"},
			},
			want: StateRejected,
		},
		{
			name: "single done returns done",
			reactions: []*Reaction{
				{Reaction: ReactionDone, AgentName: "agent-a"},
			},
			want: StateDone,
		},
		{
			name: "single published returns published",
			reactions: []*Reaction{
				{Reaction: ReactionPublished, AgentName: "agent-a"},
			},
			want: StatePublished,
		},
		{
			name: "all five types - published wins",
			reactions: []*Reaction{
				{Reaction: ReactionApprove, AgentName: "agent-a"},
				{Reaction: ReactionInProgress, AgentName: "agent-b"},
				{Reaction: ReactionReject, AgentName: "agent-c"},
				{Reaction: ReactionDone, AgentName: "agent-d"},
				{Reaction: ReactionPublished, AgentName: "agent-e"},
			},
			want: StatePublished,
		},
		{
			name: "published wins over everything",
			reactions: []*Reaction{
				{Reaction: ReactionDone, AgentName: "agent-a"},
				{Reaction: ReactionReject, AgentName: "agent-b"},
				{Reaction: ReactionPublished, AgentName: "agent-c"},
			},
			want: StatePublished,
		},
		{
			name: "reject beats in_progress",
			reactions: []*Reaction{
				{Reaction: ReactionInProgress, AgentName: "agent-a"},
				{Reaction: ReactionReject, AgentName: "agent-b"},
			},
			want: StateRejected,
		},
		{
			name: "done beats reject",
			reactions: []*Reaction{
				{Reaction: ReactionReject, AgentName: "agent-a"},
				{Reaction: ReactionDone, AgentName: "agent-b"},
			},
			want: StateDone,
		},
		{
			name: "in_progress beats approve",
			reactions: []*Reaction{
				{Reaction: ReactionInProgress, AgentName: "agent-a"},
				{Reaction: ReactionApprove, AgentName: "agent-b"},
			},
			want: StateInProgress,
		},
		{
			name: "multiple approves still returns approved",
			reactions: []*Reaction{
				{Reaction: ReactionApprove, AgentName: "agent-a"},
				{Reaction: ReactionApprove, AgentName: "agent-b"},
				{Reaction: ReactionApprove, AgentName: "agent-c"},
			},
			want: StateApproved,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeWorkflowState(tt.reactions)
			if got != tt.want {
				t.Errorf("ComputeWorkflowState() = %q, want %q", got, tt.want)
			}
		})
	}
}
