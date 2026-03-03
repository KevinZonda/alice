package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"gitee.com/alicespace/alice/internal/automation"
	"gitee.com/alicespace/alice/internal/codearmy"
	"gitee.com/alicespace/alice/internal/mcpbridge"
)

func TestAutomationTaskCreate_PrivateScope(t *testing.T) {
	store := automation.NewStore(filepath.Join(t.TempDir(), "automation_state.json"))
	svc := &service{
		sender:          &senderStub{},
		automationStore: store,
		getenv: func(key string) string {
			switch key {
			case mcpbridge.EnvReceiveIDType:
				return "chat_id"
			case mcpbridge.EnvReceiveID:
				return "oc_p2p"
			case mcpbridge.EnvActorUserID:
				return "ou_actor"
			case mcpbridge.EnvChatType:
				return "p2p"
			default:
				return ""
			}
		},
	}

	okResult, err := svc.handleAutomationTaskCreate(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"every_seconds":    60,
			"text":             "hello",
			"mention_user_ids": []string{"ou_actor"},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected create handler error: %v", err)
	}
	if okResult == nil || okResult.IsError {
		t.Fatalf("expected create success result, got %#v", okResult)
	}

	failedResult, err := svc.handleAutomationTaskCreate(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"every_seconds":    60,
			"text":             "hello",
			"mention_user_ids": []string{"ou_other"},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected create handler error: %v", err)
	}
	if failedResult == nil || !failedResult.IsError {
		t.Fatalf("expected private mention permission error, got %#v", failedResult)
	}
}

func TestAutomationTaskCreate_GroupScopeAllowsMentionOthers(t *testing.T) {
	store := automation.NewStore(filepath.Join(t.TempDir(), "automation_state.json"))
	svc := &service{
		sender:          &senderStub{},
		automationStore: store,
		getenv: func(key string) string {
			switch key {
			case mcpbridge.EnvReceiveIDType:
				return "chat_id"
			case mcpbridge.EnvReceiveID:
				return "oc_group"
			case mcpbridge.EnvActorUserID:
				return "ou_actor"
			case mcpbridge.EnvChatType:
				return "group"
			case mcpbridge.EnvSessionKey:
				return "chat_id:oc_group|thread:omt_alpha"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleAutomationTaskCreate(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"every_seconds":    60,
			"text":             "ping",
			"mention_user_ids": []string{"ou_776ddbea0c07fd88caaf8fce1b413a41"},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected create handler error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected group create success, got %#v", result)
	}

	list, err := store.ListTasks(automation.Scope{Kind: automation.ScopeKindChat, ID: "oc_group"}, "", 10)
	if err != nil {
		t.Fatalf("list tasks failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 created task, got %d", len(list))
	}
	if list[0].Scope.Kind != automation.ScopeKindChat || list[0].Scope.ID != "oc_group" {
		t.Fatalf("unexpected task scope: %+v", list[0].Scope)
	}
}

func TestAutomationTaskCreate_RunLLMByPromptDefaultActionType(t *testing.T) {
	store := automation.NewStore(filepath.Join(t.TempDir(), "automation_state.json"))
	svc := &service{
		sender:          &senderStub{},
		automationStore: store,
		getenv: func(key string) string {
			switch key {
			case mcpbridge.EnvReceiveIDType:
				return "chat_id"
			case mcpbridge.EnvReceiveID:
				return "oc_group"
			case mcpbridge.EnvActorUserID:
				return "ou_actor"
			case mcpbridge.EnvChatType:
				return "group"
			case mcpbridge.EnvSessionKey:
				return "chat_id:oc_group|thread:omt_alpha"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleAutomationTaskCreate(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"every_seconds": 60,
			"prompt":        "请输出当前时间 {{now}}",
			"model":         "gpt-4.1-mini",
			"profile":       "worker-cheap",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected create handler error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected run_llm create success, got %#v", result)
	}

	list, err := store.ListTasks(automation.Scope{Kind: automation.ScopeKindChat, ID: "oc_group"}, "", 10)
	if err != nil {
		t.Fatalf("list tasks failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 created task, got %d", len(list))
	}
	if list[0].Action.Type != automation.ActionTypeRunLLM {
		t.Fatalf("expected run_llm action type, got %s", list[0].Action.Type)
	}
	if list[0].Action.Prompt == "" {
		t.Fatalf("expected run_llm prompt to be stored, got %+v", list[0].Action)
	}
	if list[0].Action.Model != "gpt-4.1-mini" {
		t.Fatalf("expected run_llm model to be stored, got %+v", list[0].Action)
	}
	if list[0].Action.Profile != "worker-cheap" {
		t.Fatalf("expected run_llm profile to be stored, got %+v", list[0].Action)
	}
}

func TestAutomationTaskCreate_RunWorkflow(t *testing.T) {
	store := automation.NewStore(filepath.Join(t.TempDir(), "automation_state.json"))
	svc := &service{
		sender:          &senderStub{},
		automationStore: store,
		getenv: func(key string) string {
			switch key {
			case mcpbridge.EnvReceiveIDType:
				return "chat_id"
			case mcpbridge.EnvReceiveID:
				return "oc_group"
			case mcpbridge.EnvActorUserID:
				return "ou_actor"
			case mcpbridge.EnvChatType:
				return "group"
			case mcpbridge.EnvSessionKey:
				return "chat_id:oc_group|thread:omt_alpha"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleAutomationTaskCreate(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"every_seconds": 60,
			"action_type":   "run_workflow",
			"workflow":      "code_army",
			"state_key":     "project_alpha",
			"prompt":        "推进代码军队流程",
			"model":         "gpt-4.1-mini",
			"profile":       "workflow",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected create handler error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected run_workflow create success, got %#v", result)
	}

	list, err := store.ListTasks(automation.Scope{Kind: automation.ScopeKindChat, ID: "oc_group"}, "", 10)
	if err != nil {
		t.Fatalf("list tasks failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 created task, got %d", len(list))
	}
	if list[0].Action.Type != automation.ActionTypeRunWorkflow {
		t.Fatalf("expected run_workflow action type, got %s", list[0].Action.Type)
	}
	if list[0].Action.Workflow != automation.WorkflowCodeArmy {
		t.Fatalf("expected workflow code_army, got %q", list[0].Action.Workflow)
	}
	if list[0].Action.StateKey != "project_alpha" {
		t.Fatalf("expected workflow state_key to be stored, got %+v", list[0].Action)
	}
	if list[0].Action.SessionKey != "chat_id:oc_group|thread:omt_alpha" {
		t.Fatalf("expected workflow session_key to be stored, got %+v", list[0].Action)
	}
}

func TestAutomationTaskCreate_RunWorkflowNormalizesMessageSessionKey(t *testing.T) {
	store := automation.NewStore(filepath.Join(t.TempDir(), "automation_state.json"))
	svc := &service{
		sender:          &senderStub{},
		automationStore: store,
		getenv: func(key string) string {
			switch key {
			case mcpbridge.EnvReceiveIDType:
				return "chat_id"
			case mcpbridge.EnvReceiveID:
				return "oc_group"
			case mcpbridge.EnvActorUserID:
				return "ou_actor"
			case mcpbridge.EnvChatType:
				return "group"
			case mcpbridge.EnvSessionKey:
				return "chat_id:oc_group|message:om_msg_1"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleAutomationTaskCreate(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"every_seconds": 60,
			"action_type":   "run_workflow",
			"workflow":      "code_army",
			"state_key":     "project_alpha",
			"prompt":        "推进代码军队流程",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected create handler error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected run_workflow create success, got %#v", result)
	}

	list, err := store.ListTasks(automation.Scope{Kind: automation.ScopeKindChat, ID: "oc_group"}, "", 10)
	if err != nil {
		t.Fatalf("list tasks failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 created task, got %d", len(list))
	}
	if list[0].Action.SessionKey != "chat_id:oc_group" {
		t.Fatalf("expected message session to normalize to chat session, got %+v", list[0].Action)
	}
}

func TestAutomationTaskCreate_RunWorkflowByWorkflowFieldDefaultActionType(t *testing.T) {
	store := automation.NewStore(filepath.Join(t.TempDir(), "automation_state.json"))
	svc := &service{
		sender:          &senderStub{},
		automationStore: store,
		getenv: func(key string) string {
			switch key {
			case mcpbridge.EnvReceiveIDType:
				return "chat_id"
			case mcpbridge.EnvReceiveID:
				return "oc_group"
			case mcpbridge.EnvActorUserID:
				return "ou_actor"
			case mcpbridge.EnvChatType:
				return "group"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleAutomationTaskCreate(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"every_seconds": 60,
			"workflow":      "code_army",
			"prompt":        "推进代码军队流程",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected create handler error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected run_workflow create success, got %#v", result)
	}

	list, err := store.ListTasks(automation.Scope{Kind: automation.ScopeKindChat, ID: "oc_group"}, "", 10)
	if err != nil {
		t.Fatalf("list tasks failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 created task, got %d", len(list))
	}
	if list[0].Action.Type != automation.ActionTypeRunWorkflow {
		t.Fatalf("expected run_workflow action type, got %s", list[0].Action.Type)
	}
}

func TestCodeArmyStatusGet_CurrentSession(t *testing.T) {
	stateDir := t.TempDir()
	sessionKey := "chat_id:oc_group|thread:omt_alpha"
	sessionDir := filepath.Join(stateDir, "chat_id_oc_group_thread_omt_alpha")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir failed: %v", err)
	}
	raw, err := json.Marshal(map[string]any{
		"version":       1,
		"workflow":      "code_army",
		"key":           "default",
		"session_key":   sessionKey,
		"task_id":       "task_001",
		"phase":         "reviewer",
		"iteration":     2,
		"objective":     "推进 code army",
		"last_decision": "pass",
		"updated_at":    time.Date(2026, 3, 3, 4, 5, 6, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("marshal state failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "default.json"), raw, 0o644); err != nil {
		t.Fatalf("write state file failed: %v", err)
	}

	svc := &service{
		sender:         &senderStub{},
		codeArmyStatus: codearmy.NewInspector(stateDir),
		getenv: func(key string) string {
			switch key {
			case mcpbridge.EnvReceiveIDType:
				return "chat_id"
			case mcpbridge.EnvReceiveID:
				return "oc_group"
			case mcpbridge.EnvActorUserID:
				return "ou_actor"
			case mcpbridge.EnvChatType:
				return "group"
			case mcpbridge.EnvSessionKey:
				return sessionKey
			default:
				return ""
			}
		},
	}

	result, err := svc.handleCodeArmyStatusGet(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected status handler error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected non-error status result, got %#v", result)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured content map, got %#v", result.StructuredContent)
	}
	if structured["session_key"] != sessionKey {
		t.Fatalf("expected session_key %q, got %#v", sessionKey, structured["session_key"])
	}
	if structured["count"] != 1 {
		t.Fatalf("expected count=1, got %#v", structured["count"])
	}
	states, ok := structured["states"].([]codearmy.StateSnapshot)
	if !ok {
		t.Fatalf("expected states payload, got %#v", structured["states"])
	}
	if len(states) != 1 {
		t.Fatalf("expected 1 state in current session, got %#v", states)
	}
	if states[0].Phase != "reviewer" || states[0].Iteration != 2 {
		t.Fatalf("unexpected state payload: %+v", states[0])
	}
}

func TestCodeArmyStatusGet_MessageSessionUsesConversationKey(t *testing.T) {
	stateDir := t.TempDir()
	sessionKey := "chat_id:oc_group"
	sessionDir := filepath.Join(stateDir, "chat_id_oc_group")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir failed: %v", err)
	}
	raw, err := json.Marshal(map[string]any{
		"version":       1,
		"workflow":      "code_army",
		"key":           "default",
		"session_key":   sessionKey,
		"task_id":       "task_001",
		"phase":         "worker",
		"iteration":     1,
		"objective":     "推进 code army",
		"last_decision": "",
		"updated_at":    time.Date(2026, 3, 3, 4, 5, 6, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("marshal state failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "default.json"), raw, 0o644); err != nil {
		t.Fatalf("write state file failed: %v", err)
	}

	svc := &service{
		sender:         &senderStub{},
		codeArmyStatus: codearmy.NewInspector(stateDir),
		getenv: func(key string) string {
			switch key {
			case mcpbridge.EnvReceiveIDType:
				return "chat_id"
			case mcpbridge.EnvReceiveID:
				return "oc_group"
			case mcpbridge.EnvActorUserID:
				return "ou_actor"
			case mcpbridge.EnvChatType:
				return "group"
			case mcpbridge.EnvSessionKey:
				return "chat_id:oc_group|message:om_msg_2"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleCodeArmyStatusGet(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"state_key": "default",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected status handler error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected non-error status result, got %#v", result)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured content map, got %#v", result.StructuredContent)
	}
	if structured["session_key"] != sessionKey {
		t.Fatalf("expected normalized session_key %q, got %#v", sessionKey, structured["session_key"])
	}
	state, ok := structured["state"].(codearmy.StateSnapshot)
	if !ok {
		t.Fatalf("expected state payload, got %#v", structured["state"])
	}
	if state.Phase != "worker" || state.Iteration != 1 {
		t.Fatalf("unexpected state payload: %+v", state)
	}
}

func TestCodeArmyStatusGet_OtherSessionCannotReadState(t *testing.T) {
	stateDir := t.TempDir()
	ownerSessionKey := "chat_id:oc_group|thread:omt_alpha"
	otherSessionKey := "chat_id:oc_group|thread:omt_beta"
	ownerSessionDir := filepath.Join(stateDir, "chat_id_oc_group_thread_omt_alpha")
	if err := os.MkdirAll(ownerSessionDir, 0o755); err != nil {
		t.Fatalf("mkdir owner session dir failed: %v", err)
	}
	raw, err := json.Marshal(map[string]any{
		"version":       1,
		"workflow":      "code_army",
		"key":           "default",
		"session_key":   ownerSessionKey,
		"task_id":       "task_001",
		"phase":         "worker",
		"iteration":     1,
		"objective":     "推进 code army",
		"last_decision": "",
		"updated_at":    time.Date(2026, 3, 3, 4, 5, 6, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("marshal state failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ownerSessionDir, "default.json"), raw, 0o644); err != nil {
		t.Fatalf("write state file failed: %v", err)
	}

	svc := &service{
		sender:         &senderStub{},
		codeArmyStatus: codearmy.NewInspector(stateDir),
		getenv: func(key string) string {
			switch key {
			case mcpbridge.EnvReceiveIDType:
				return "chat_id"
			case mcpbridge.EnvReceiveID:
				return "oc_group"
			case mcpbridge.EnvActorUserID:
				return "ou_actor"
			case mcpbridge.EnvChatType:
				return "group"
			case mcpbridge.EnvSessionKey:
				return otherSessionKey
			default:
				return ""
			}
		},
	}

	listResult, err := svc.handleCodeArmyStatusGet(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected status list handler error: %v", err)
	}
	if listResult == nil || listResult.IsError {
		t.Fatalf("expected non-error empty list result, got %#v", listResult)
	}
	structured, ok := listResult.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured content map, got %#v", listResult.StructuredContent)
	}
	if structured["session_key"] != otherSessionKey {
		t.Fatalf("expected session_key %q, got %#v", otherSessionKey, structured["session_key"])
	}
	if structured["count"] != 0 {
		t.Fatalf("expected count=0 for other session, got %#v", structured["count"])
	}
	states, ok := structured["states"].([]codearmy.StateSnapshot)
	if !ok {
		t.Fatalf("expected states payload, got %#v", structured["states"])
	}
	if len(states) != 0 {
		t.Fatalf("expected no states for other session, got %#v", states)
	}

	getResult, err := svc.handleCodeArmyStatusGet(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"state_key": "default",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected status get handler error: %v", err)
	}
	if getResult == nil || !getResult.IsError {
		t.Fatalf("expected state not found error for other session, got %#v", getResult)
	}
	if len(getResult.Content) == 0 {
		t.Fatalf("expected error content for other session lookup, got %#v", getResult)
	}
	firstText, ok := getResult.Content[0].(mcp.TextContent)
	if !ok || firstText.Text != "code_army state not found in current conversation" {
		t.Fatalf("unexpected other-session error payload: %#v", getResult.Content)
	}
}

func TestAutomationTaskCreate_MaxRuns(t *testing.T) {
	store := automation.NewStore(filepath.Join(t.TempDir(), "automation_state.json"))
	svc := &service{
		sender:          &senderStub{},
		automationStore: store,
		getenv: func(key string) string {
			switch key {
			case mcpbridge.EnvReceiveIDType:
				return "chat_id"
			case mcpbridge.EnvReceiveID:
				return "oc_group"
			case mcpbridge.EnvActorUserID:
				return "ou_actor"
			case mcpbridge.EnvChatType:
				return "group"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleAutomationTaskCreate(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"every_seconds": 60,
			"text":          "run once",
			"max_runs":      1,
		}},
	})
	if err != nil {
		t.Fatalf("unexpected create handler error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected create success, got %#v", result)
	}

	list, err := store.ListTasks(automation.Scope{Kind: automation.ScopeKindChat, ID: "oc_group"}, "", 10)
	if err != nil {
		t.Fatalf("list tasks failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 created task, got %d", len(list))
	}
	if list[0].MaxRuns != 1 {
		t.Fatalf("expected max_runs=1, got %d", list[0].MaxRuns)
	}
}

func TestAutomationTaskUpdate_PermissionDeniedForCreatorOnly(t *testing.T) {
	store := automation.NewStore(filepath.Join(t.TempDir(), "automation_state.json"))
	created, err := store.CreateTask(automation.Task{
		Title:      "group reminder",
		Scope:      automation.Scope{Kind: automation.ScopeKindChat, ID: "oc_group"},
		Route:      automation.Route{ReceiveIDType: "chat_id", ReceiveID: "oc_group"},
		Creator:    automation.Actor{UserID: "ou_creator"},
		ManageMode: automation.ManageModeCreatorOnly,
		Schedule:   automation.Schedule{Type: automation.ScheduleTypeInterval, EverySeconds: 60},
		Action:     automation.Action{Type: automation.ActionTypeSendText, Text: "hello"},
		Status:     automation.TaskStatusActive,
	})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	svc := &service{
		sender:          &senderStub{},
		automationStore: store,
		getenv: func(key string) string {
			switch key {
			case mcpbridge.EnvReceiveIDType:
				return "chat_id"
			case mcpbridge.EnvReceiveID:
				return "oc_group"
			case mcpbridge.EnvActorUserID:
				return "ou_other"
			case mcpbridge.EnvChatType:
				return "group"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleAutomationTaskUpdate(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"task_id": created.ID,
			"text":    "changed",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected update handler error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("expected permission denied result, got %#v", result)
	}
}

func TestAutomationTaskDelete_ScopeAllAllowsOthers(t *testing.T) {
	store := automation.NewStore(filepath.Join(t.TempDir(), "automation_state.json"))
	created, err := store.CreateTask(automation.Task{
		Title:      "group reminder",
		Scope:      automation.Scope{Kind: automation.ScopeKindChat, ID: "oc_group"},
		Route:      automation.Route{ReceiveIDType: "chat_id", ReceiveID: "oc_group"},
		Creator:    automation.Actor{UserID: "ou_creator"},
		ManageMode: automation.ManageModeScopeAll,
		Schedule:   automation.Schedule{Type: automation.ScheduleTypeInterval, EverySeconds: 60},
		Action:     automation.Action{Type: automation.ActionTypeSendText, Text: "hello"},
		Status:     automation.TaskStatusActive,
	})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	svc := &service{
		sender:          &senderStub{},
		automationStore: store,
		getenv: func(key string) string {
			switch key {
			case mcpbridge.EnvReceiveIDType:
				return "chat_id"
			case mcpbridge.EnvReceiveID:
				return "oc_group"
			case mcpbridge.EnvActorUserID:
				return "ou_other"
			case mcpbridge.EnvChatType:
				return "group"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleAutomationTaskDelete(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{"task_id": created.ID}},
	})
	if err != nil {
		t.Fatalf("unexpected delete handler error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected delete success result, got %#v", result)
	}

	updated, err := store.GetTask(created.ID)
	if err != nil {
		t.Fatalf("get task failed: %v", err)
	}
	if updated.Status != automation.TaskStatusDeleted {
		t.Fatalf("expected deleted status, got %s", updated.Status)
	}
}

func TestAutomationTaskUpdate_CanSwitchToRunLLM(t *testing.T) {
	store := automation.NewStore(filepath.Join(t.TempDir(), "automation_state.json"))
	created, err := store.CreateTask(automation.Task{
		Title:      "group reminder",
		Scope:      automation.Scope{Kind: automation.ScopeKindChat, ID: "oc_group"},
		Route:      automation.Route{ReceiveIDType: "chat_id", ReceiveID: "oc_group"},
		Creator:    automation.Actor{UserID: "ou_creator"},
		ManageMode: automation.ManageModeCreatorOnly,
		Schedule:   automation.Schedule{Type: automation.ScheduleTypeInterval, EverySeconds: 60},
		Action:     automation.Action{Type: automation.ActionTypeSendText, Text: "hello"},
		Status:     automation.TaskStatusActive,
	})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	svc := &service{
		sender:          &senderStub{},
		automationStore: store,
		getenv: func(key string) string {
			switch key {
			case mcpbridge.EnvReceiveIDType:
				return "chat_id"
			case mcpbridge.EnvReceiveID:
				return "oc_group"
			case mcpbridge.EnvActorUserID:
				return "ou_creator"
			case mcpbridge.EnvChatType:
				return "group"
			case mcpbridge.EnvSessionKey:
				return "chat_id:oc_group|thread:omt_alpha"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleAutomationTaskUpdate(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"task_id":     created.ID,
			"action_type": "run_llm",
			"prompt":      "请输出当前时间 {{now}}",
			"model":       "gpt-4.1-mini",
			"profile":     "worker-cheap",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected update handler error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected update success result, got %#v", result)
	}

	updated, err := store.GetTask(created.ID)
	if err != nil {
		t.Fatalf("get task failed: %v", err)
	}
	if updated.Action.Type != automation.ActionTypeRunLLM {
		t.Fatalf("expected run_llm action type, got %s", updated.Action.Type)
	}
	if updated.Action.Prompt == "" {
		t.Fatalf("expected updated run_llm prompt, got %+v", updated.Action)
	}
	if updated.Action.Model != "gpt-4.1-mini" {
		t.Fatalf("expected updated run_llm model, got %+v", updated.Action)
	}
	if updated.Action.Profile != "worker-cheap" {
		t.Fatalf("expected updated run_llm profile, got %+v", updated.Action)
	}
}

func TestAutomationTaskUpdate_CanSwitchToRunWorkflow(t *testing.T) {
	store := automation.NewStore(filepath.Join(t.TempDir(), "automation_state.json"))
	created, err := store.CreateTask(automation.Task{
		Title:      "group reminder",
		Scope:      automation.Scope{Kind: automation.ScopeKindChat, ID: "oc_group"},
		Route:      automation.Route{ReceiveIDType: "chat_id", ReceiveID: "oc_group"},
		Creator:    automation.Actor{UserID: "ou_creator"},
		ManageMode: automation.ManageModeCreatorOnly,
		Schedule:   automation.Schedule{Type: automation.ScheduleTypeInterval, EverySeconds: 60},
		Action:     automation.Action{Type: automation.ActionTypeSendText, Text: "hello"},
		Status:     automation.TaskStatusActive,
	})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	svc := &service{
		sender:          &senderStub{},
		automationStore: store,
		getenv: func(key string) string {
			switch key {
			case mcpbridge.EnvReceiveIDType:
				return "chat_id"
			case mcpbridge.EnvReceiveID:
				return "oc_group"
			case mcpbridge.EnvActorUserID:
				return "ou_creator"
			case mcpbridge.EnvChatType:
				return "group"
			case mcpbridge.EnvSessionKey:
				return "chat_id:oc_group|thread:omt_alpha"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleAutomationTaskUpdate(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"task_id":     created.ID,
			"action_type": "run_workflow",
			"workflow":    "code_army",
			"state_key":   "project_alpha",
			"prompt":      "推进代码军队流程",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected update handler error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected update success result, got %#v", result)
	}

	updated, err := store.GetTask(created.ID)
	if err != nil {
		t.Fatalf("get task failed: %v", err)
	}
	if updated.Action.Type != automation.ActionTypeRunWorkflow {
		t.Fatalf("expected run_workflow action type, got %s", updated.Action.Type)
	}
	if updated.Action.Workflow != automation.WorkflowCodeArmy {
		t.Fatalf("expected updated workflow code_army, got %+v", updated.Action)
	}
	if updated.Action.StateKey != "project_alpha" {
		t.Fatalf("expected updated state_key, got %+v", updated.Action)
	}
	if updated.Action.SessionKey != "chat_id:oc_group|thread:omt_alpha" {
		t.Fatalf("expected updated session_key, got %+v", updated.Action)
	}
}

func TestAutomationTaskUpdate_RunWorkflowNormalizesStoredMessageSessionKey(t *testing.T) {
	store := automation.NewStore(filepath.Join(t.TempDir(), "automation_state.json"))
	created, err := store.CreateTask(automation.Task{
		Title:      "workflow task",
		Scope:      automation.Scope{Kind: automation.ScopeKindChat, ID: "oc_group"},
		Route:      automation.Route{ReceiveIDType: "chat_id", ReceiveID: "oc_group"},
		Creator:    automation.Actor{UserID: "ou_creator"},
		ManageMode: automation.ManageModeCreatorOnly,
		Schedule:   automation.Schedule{Type: automation.ScheduleTypeInterval, EverySeconds: 60},
		Action: automation.Action{
			Type:       automation.ActionTypeRunWorkflow,
			Prompt:     "推进代码军队流程",
			Workflow:   automation.WorkflowCodeArmy,
			StateKey:   "project_alpha",
			SessionKey: "chat_id:oc_group|message:om_msg_1",
		},
		Status: automation.TaskStatusActive,
	})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	svc := &service{
		sender:          &senderStub{},
		automationStore: store,
		getenv: func(key string) string {
			switch key {
			case mcpbridge.EnvReceiveIDType:
				return "chat_id"
			case mcpbridge.EnvReceiveID:
				return "oc_group"
			case mcpbridge.EnvActorUserID:
				return "ou_creator"
			case mcpbridge.EnvChatType:
				return "group"
			case mcpbridge.EnvSessionKey:
				return "chat_id:oc_group|message:om_msg_2"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleAutomationTaskUpdate(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"task_id": created.ID,
			"prompt":  "继续推进代码军队流程",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected update handler error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected update success result, got %#v", result)
	}

	updated, err := store.GetTask(created.ID)
	if err != nil {
		t.Fatalf("get task failed: %v", err)
	}
	if updated.Action.SessionKey != "chat_id:oc_group" {
		t.Fatalf("expected stored message session to normalize, got %+v", updated.Action)
	}
}

func TestAutomationTaskUpdate_MaxRunsReachedCannotEnable(t *testing.T) {
	store := automation.NewStore(filepath.Join(t.TempDir(), "automation_state.json"))
	created, err := store.CreateTask(automation.Task{
		Title:      "single run",
		Scope:      automation.Scope{Kind: automation.ScopeKindChat, ID: "oc_group"},
		Route:      automation.Route{ReceiveIDType: "chat_id", ReceiveID: "oc_group"},
		Creator:    automation.Actor{UserID: "ou_creator"},
		ManageMode: automation.ManageModeCreatorOnly,
		Schedule:   automation.Schedule{Type: automation.ScheduleTypeInterval, EverySeconds: 60},
		Action:     automation.Action{Type: automation.ActionTypeSendText, Text: "hello"},
		Status:     automation.TaskStatusPaused,
		MaxRuns:    1,
		RunCount:   1,
	})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	svc := &service{
		sender:          &senderStub{},
		automationStore: store,
		getenv: func(key string) string {
			switch key {
			case mcpbridge.EnvReceiveIDType:
				return "chat_id"
			case mcpbridge.EnvReceiveID:
				return "oc_group"
			case mcpbridge.EnvActorUserID:
				return "ou_creator"
			case mcpbridge.EnvChatType:
				return "group"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleAutomationTaskUpdate(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"task_id": created.ID,
			"enabled": true,
		}},
	})
	if err != nil {
		t.Fatalf("unexpected update handler error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("expected max_runs reached update error, got %#v", result)
	}
}

func TestAutomationTaskCreate_CronSchedule(t *testing.T) {
	store := automation.NewStore(filepath.Join(t.TempDir(), "automation_state.json"))
	svc := &service{
		sender:          &senderStub{},
		automationStore: store,
		getenv: func(key string) string {
			switch key {
			case mcpbridge.EnvReceiveIDType:
				return "chat_id"
			case mcpbridge.EnvReceiveID:
				return "oc_group"
			case mcpbridge.EnvActorUserID:
				return "ou_actor"
			case mcpbridge.EnvChatType:
				return "group"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleAutomationTaskCreate(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"schedule_type": "cron",
			"cron_expr":     "0 9 * * *",
			"text":          "daily brief",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected create handler error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected cron create success, got %#v", result)
	}

	list, err := store.ListTasks(automation.Scope{Kind: automation.ScopeKindChat, ID: "oc_group"}, "", 10)
	if err != nil {
		t.Fatalf("list tasks failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 created task, got %d", len(list))
	}
	if list[0].Schedule.Type != automation.ScheduleTypeCron {
		t.Fatalf("expected cron schedule type, got %s", list[0].Schedule.Type)
	}
	if list[0].Schedule.CronExpr != "0 9 * * *" {
		t.Fatalf("expected cron_expr to be stored, got %q", list[0].Schedule.CronExpr)
	}
}

func TestAutomationTaskUpdate_CanSwitchIntervalToCron(t *testing.T) {
	store := automation.NewStore(filepath.Join(t.TempDir(), "automation_state.json"))
	created, err := store.CreateTask(automation.Task{
		Title:      "group reminder",
		Scope:      automation.Scope{Kind: automation.ScopeKindChat, ID: "oc_group"},
		Route:      automation.Route{ReceiveIDType: "chat_id", ReceiveID: "oc_group"},
		Creator:    automation.Actor{UserID: "ou_creator"},
		ManageMode: automation.ManageModeCreatorOnly,
		Schedule:   automation.Schedule{Type: automation.ScheduleTypeInterval, EverySeconds: 60},
		Action:     automation.Action{Type: automation.ActionTypeSendText, Text: "hello"},
		Status:     automation.TaskStatusActive,
	})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	svc := &service{
		sender:          &senderStub{},
		automationStore: store,
		getenv: func(key string) string {
			switch key {
			case mcpbridge.EnvReceiveIDType:
				return "chat_id"
			case mcpbridge.EnvReceiveID:
				return "oc_group"
			case mcpbridge.EnvActorUserID:
				return "ou_creator"
			case mcpbridge.EnvChatType:
				return "group"
			default:
				return ""
			}
		},
	}

	result, err := svc.handleAutomationTaskUpdate(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Arguments: map[string]any{
			"task_id":       created.ID,
			"schedule_type": "cron",
			"cron_expr":     "0 9 * * *",
		}},
	})
	if err != nil {
		t.Fatalf("unexpected update handler error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("expected update success result, got %#v", result)
	}

	updated, err := store.GetTask(created.ID)
	if err != nil {
		t.Fatalf("get task failed: %v", err)
	}
	if updated.Schedule.Type != automation.ScheduleTypeCron {
		t.Fatalf("expected cron schedule type, got %s", updated.Schedule.Type)
	}
	if updated.Schedule.CronExpr != "0 9 * * *" {
		t.Fatalf("expected cron_expr to be updated, got %q", updated.Schedule.CronExpr)
	}
	if updated.NextRunAt.IsZero() || !updated.NextRunAt.After(time.Now().UTC().Add(-time.Minute)) {
		t.Fatalf("expected next_run_at to be recalculated, got %s", updated.NextRunAt.Format(time.RFC3339))
	}
}
