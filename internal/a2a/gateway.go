package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/synapbus/synapbus/internal/agents"
	"github.com/synapbus/synapbus/internal/messaging"
)

// MessagingService defines the messaging operations needed by the A2A gateway.
type MessagingService interface {
	SendMessage(ctx context.Context, from, to, body string, opts messaging.SendOptions) (*messaging.Message, error)
	GetConversation(ctx context.Context, id int64) (*messaging.Conversation, []*messaging.Message, error)
}

// AgentService defines the agent operations needed by the A2A gateway.
type AgentService interface {
	GetAgent(ctx context.Context, name string) (*agents.Agent, error)
}

// Gateway handles inbound A2A JSON-RPC requests.
type Gateway struct {
	taskStore    *A2ATaskStore
	msgService   MessagingService
	agentService AgentService
	logger       *slog.Logger
}

// NewGateway creates a new A2A gateway.
func NewGateway(taskStore *A2ATaskStore, msgService MessagingService, agentService AgentService) *Gateway {
	return &Gateway{
		taskStore:    taskStore,
		msgService:   msgService,
		agentService: agentService,
		logger:       slog.Default().With("component", "a2a-gateway"),
	}
}

// JSON-RPC 2.0 request/response types.

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Standard JSON-RPC 2.0 error codes.
const (
	errCodeParse      = -32700
	errCodeInvalidReq = -32600
	errCodeNoMethod   = -32601
	errCodeInvalidParams = -32602
	errCodeInternal   = -32603
)

// HandleJSONRPC dispatches incoming JSON-RPC 2.0 requests to the appropriate handler.
func (g *Gateway) HandleJSONRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB limit
	if err != nil {
		writeJSONRPC(w, nil, nil, &jsonRPCError{Code: errCodeParse, Message: "failed to read request body"})
		return
	}

	var req jsonRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONRPC(w, nil, nil, &jsonRPCError{Code: errCodeParse, Message: "invalid JSON"})
		return
	}

	if req.JSONRPC != "2.0" {
		writeJSONRPC(w, req.ID, nil, &jsonRPCError{Code: errCodeInvalidReq, Message: "jsonrpc must be \"2.0\""})
		return
	}

	// Extract the calling agent from auth context.
	callerAgent, ok := agents.AgentFromContext(r.Context())
	callerName := ""
	if ok && callerAgent != nil {
		callerName = callerAgent.Name
	}

	switch req.Method {
	case "message.send":
		g.handleMessageSend(w, r.Context(), req, callerName)
	case "tasks.get":
		g.handleTasksGet(w, r.Context(), req)
	case "tasks.cancel":
		g.handleTasksCancel(w, r.Context(), req)
	default:
		writeJSONRPC(w, req.ID, nil, &jsonRPCError{Code: errCodeNoMethod, Message: fmt.Sprintf("unknown method: %s", req.Method)})
	}
}

// message.send params

type messageSendParams struct {
	Message struct {
		Body     string `json:"body"`
		Metadata struct {
			TargetAgent string `json:"target_agent"`
		} `json:"metadata"`
	} `json:"message"`
}

func (g *Gateway) handleMessageSend(w http.ResponseWriter, ctx context.Context, req jsonRPCRequest, callerName string) {
	var params messageSendParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeJSONRPC(w, req.ID, nil, &jsonRPCError{Code: errCodeInvalidParams, Message: "invalid params: " + err.Error()})
		return
	}

	targetAgent := params.Message.Metadata.TargetAgent
	if targetAgent == "" {
		writeJSONRPC(w, req.ID, nil, &jsonRPCError{Code: errCodeInvalidParams, Message: "params.message.metadata.target_agent is required"})
		return
	}

	messageBody := params.Message.Body
	if messageBody == "" {
		writeJSONRPC(w, req.ID, nil, &jsonRPCError{Code: errCodeInvalidParams, Message: "params.message.body is required"})
		return
	}

	// Validate target agent exists.
	_, err := g.agentService.GetAgent(ctx, targetAgent)
	if err != nil {
		writeJSONRPC(w, req.ID, nil, &jsonRPCError{Code: errCodeInvalidParams, Message: fmt.Sprintf("target agent not found: %s", targetAgent)})
		return
	}

	// Create task.
	taskID := uuid.New().String()
	contextID := uuid.New().String()

	// Determine the sender name for the DM. Use the caller's identity if
	// authenticated, otherwise fall back to "a2a-gateway" so SendMessage
	// has a non-empty from field.
	senderName := callerName
	if senderName == "" {
		senderName = "a2a-gateway"
	}

	// Build metadata containing the a2a_task_id.
	metaJSON, _ := json.Marshal(map[string]string{"a2a_task_id": taskID})

	// Send DM to target agent.
	msg, err := g.msgService.SendMessage(ctx, senderName, targetAgent, messageBody, messaging.SendOptions{
		Subject:  fmt.Sprintf("A2A Task %s", taskID),
		Metadata: string(metaJSON),
	})
	if err != nil {
		g.logger.Error("failed to send DM for A2A task", "task_id", taskID, "error", err)
		writeJSONRPC(w, req.ID, nil, &jsonRPCError{Code: errCodeInternal, Message: "failed to deliver message: " + err.Error()})
		return
	}

	convID := msg.ConversationID
	task := &A2ATask{
		ID:             taskID,
		ContextID:      contextID,
		TargetAgent:    targetAgent,
		SourceAgent:    callerName,
		ConversationID: &convID,
		State:          StateSubmitted,
	}

	if err := g.taskStore.CreateTask(ctx, task); err != nil {
		g.logger.Error("failed to create A2A task", "task_id", taskID, "error", err)
		writeJSONRPC(w, req.ID, nil, &jsonRPCError{Code: errCodeInternal, Message: "failed to create task"})
		return
	}

	g.logger.Info("A2A task created",
		"task_id", taskID,
		"target_agent", targetAgent,
		"source_agent", callerName,
		"message_id", msg.ID,
	)

	writeJSONRPC(w, req.ID, task, nil)
}

// tasks.get params

type tasksGetParams struct {
	ID string `json:"id"`
}

func (g *Gateway) handleTasksGet(w http.ResponseWriter, ctx context.Context, req jsonRPCRequest) {
	var params tasksGetParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeJSONRPC(w, req.ID, nil, &jsonRPCError{Code: errCodeInvalidParams, Message: "invalid params: " + err.Error()})
		return
	}

	if params.ID == "" {
		writeJSONRPC(w, req.ID, nil, &jsonRPCError{Code: errCodeInvalidParams, Message: "params.id is required"})
		return
	}

	task, err := g.taskStore.GetTask(ctx, params.ID)
	if err != nil {
		writeJSONRPC(w, req.ID, nil, &jsonRPCError{Code: errCodeInvalidParams, Message: err.Error()})
		return
	}

	// If the task has a conversation and is still SUBMITTED, check whether the
	// target agent has replied, which indicates completion.
	if task.State == StateSubmitted && task.ConversationID != nil {
		_, msgs, err := g.msgService.GetConversation(ctx, *task.ConversationID)
		if err == nil && len(msgs) > 1 {
			// Check if the target agent sent a reply (any message from target after the first).
			for _, m := range msgs[1:] {
				if m.FromAgent == task.TargetAgent {
					task.State = StateCompleted
					_ = g.taskStore.UpdateTaskState(ctx, task.ID, StateCompleted)
					break
				}
			}
		}
	}

	writeJSONRPC(w, req.ID, task, nil)
}

// tasks.cancel params

type tasksCancelParams struct {
	ID string `json:"id"`
}

func (g *Gateway) handleTasksCancel(w http.ResponseWriter, ctx context.Context, req jsonRPCRequest) {
	var params tasksCancelParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeJSONRPC(w, req.ID, nil, &jsonRPCError{Code: errCodeInvalidParams, Message: "invalid params: " + err.Error()})
		return
	}

	if params.ID == "" {
		writeJSONRPC(w, req.ID, nil, &jsonRPCError{Code: errCodeInvalidParams, Message: "params.id is required"})
		return
	}

	task, err := g.taskStore.GetTask(ctx, params.ID)
	if err != nil {
		writeJSONRPC(w, req.ID, nil, &jsonRPCError{Code: errCodeInvalidParams, Message: err.Error()})
		return
	}

	// Cannot cancel a terminal task.
	if task.State == StateCompleted || task.State == StateCanceled {
		writeJSONRPC(w, req.ID, nil, &jsonRPCError{Code: errCodeInvalidParams, Message: fmt.Sprintf("task is already in terminal state: %s", task.State)})
		return
	}

	if err := g.taskStore.UpdateTaskState(ctx, task.ID, StateCanceled); err != nil {
		writeJSONRPC(w, req.ID, nil, &jsonRPCError{Code: errCodeInternal, Message: "failed to cancel task"})
		return
	}

	task.State = StateCanceled
	g.logger.Info("A2A task canceled", "task_id", task.ID)

	writeJSONRPC(w, req.ID, task, nil)
}

// writeJSONRPC writes a JSON-RPC 2.0 response.
func writeJSONRPC(w http.ResponseWriter, id json.RawMessage, result any, rpcErr *jsonRPCError) {
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
		Error:   rpcErr,
	}
	w.Header().Set("Content-Type", "application/json")
	if rpcErr != nil {
		// Use 200 for JSON-RPC errors (per spec), but set result to nil.
		resp.Result = nil
	}
	json.NewEncoder(w).Encode(resp)
}
