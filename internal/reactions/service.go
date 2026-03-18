package reactions

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// Service provides business logic for message reactions.
type Service struct {
	store  Store
	logger *slog.Logger
}

// NewService creates a new reaction service.
func NewService(store Store, logger *slog.Logger) *Service {
	return &Service{
		store:  store,
		logger: logger.With("component", "reactions"),
	}
}

// ToggleResult describes what happened after a toggle operation.
type ToggleResult struct {
	Action   string    `json:"action"` // "added" or "removed"
	Reaction *Reaction `json:"reaction,omitempty"`
}

// Toggle adds a reaction if it doesn't exist, or removes it if it does.
// Returns the action taken and the reaction (if added).
func (s *Service) Toggle(ctx context.Context, messageID int64, agentName, reactionType string, metadata json.RawMessage) (*ToggleResult, error) {
	if !IsValidReaction(reactionType) {
		return nil, ErrInvalidReaction
	}

	// Check if reaction already exists
	exists, err := s.store.Exists(ctx, messageID, agentName, reactionType)
	if err != nil {
		return nil, fmt.Errorf("check existing reaction: %w", err)
	}

	if exists {
		// Toggle off — remove it
		if err := s.store.Delete(ctx, messageID, agentName, reactionType); err != nil {
			return nil, fmt.Errorf("remove reaction: %w", err)
		}
		s.logger.Info("reaction removed",
			"message_id", messageID,
			"agent", agentName,
			"reaction", reactionType,
		)
		return &ToggleResult{Action: "removed"}, nil
	}

	// Check reaction count limit
	count, err := s.store.CountByMessage(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("count reactions: %w", err)
	}
	if count >= MaxReactionsPerMessage {
		return nil, ErrReactionLimit
	}

	// Toggle on — add it
	if metadata == nil {
		metadata = json.RawMessage("{}")
	}

	r := &Reaction{
		MessageID: messageID,
		AgentName: agentName,
		Reaction:  reactionType,
		Metadata:  metadata,
	}

	if err := s.store.Insert(ctx, r); err != nil {
		return nil, fmt.Errorf("add reaction: %w", err)
	}

	s.logger.Info("reaction added",
		"message_id", messageID,
		"agent", agentName,
		"reaction", reactionType,
	)

	return &ToggleResult{Action: "added", Reaction: r}, nil
}

// Remove explicitly removes a reaction.
func (s *Service) Remove(ctx context.Context, messageID int64, agentName, reactionType string) error {
	if !IsValidReaction(reactionType) {
		return ErrInvalidReaction
	}

	if err := s.store.Delete(ctx, messageID, agentName, reactionType); err != nil {
		return fmt.Errorf("remove reaction: %w", err)
	}

	s.logger.Info("reaction removed",
		"message_id", messageID,
		"agent", agentName,
		"reaction", reactionType,
	)
	return nil
}

// GetReactions returns all reactions for a message and the computed workflow state.
func (s *Service) GetReactions(ctx context.Context, messageID int64) ([]*Reaction, string, error) {
	reactions, err := s.store.GetByMessageID(ctx, messageID)
	if err != nil {
		return nil, "", fmt.Errorf("get reactions: %w", err)
	}

	state := ComputeWorkflowState(reactions)
	return reactions, state, nil
}

// GetReactionsByMessageIDs returns reactions grouped by message ID.
func (s *Service) GetReactionsByMessageIDs(ctx context.Context, messageIDs []int64) (map[int64][]*Reaction, error) {
	return s.store.GetByMessageIDs(ctx, messageIDs)
}

// ListByState returns message IDs in a channel that have the given workflow state.
func (s *Service) ListByState(ctx context.Context, channelID int64, state string) ([]int64, error) {
	return s.store.GetMessageIDsByState(ctx, channelID, state)
}
