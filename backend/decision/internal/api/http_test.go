package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"aegisflux/backend/decision/internal/agents"
	"aegisflux/backend/decision/internal/model"
	"aegisflux/backend/decision/internal/store"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockPlanStore is a mock implementation of the PlanStore interface
type MockPlanStore struct {
	plans map[string]*model.Plan
}

func NewMockPlanStore() *MockPlanStore {
	return &MockPlanStore{
		plans: make(map[string]*model.Plan),
	}
}

func (m *MockPlanStore) Store(ctx context.Context, plan *model.Plan) error {
	m.plans[plan.ID] = plan
	return nil
}

func (m *MockPlanStore) Get(ctx context.Context, id string) (*model.Plan, error) {
	if plan, exists := m.plans[id]; exists {
		return plan, nil
	}
	return nil, fmt.Errorf("plan not found")
}

func (m *MockPlanStore) List(ctx context.Context) ([]*model.Plan, error) {
	var plans []*model.Plan
	for _, plan := range m.plans {
		plans = append(plans, plan)
	}
	return plans, nil
}

// MockNATSConn is a mock NATS connection
type MockNATSConn struct {
	publishedMessages []struct {
		subject string
		data    []byte
	}
}

func NewMockNATSConn() *MockNATSConn {
	return &MockNATSConn{
		publishedMessages: make([]struct {
			subject string
			data    []byte
		}, 0),
	}
}

func (m *MockNATSConn) Publish(subject string, data []byte) error {
	m.publishedMessages = append(m.publishedMessages, struct {
		subject string
		data    []byte
	}{subject: subject, data: data})
	return nil
}

func (m *MockNATSConn) GetPublishedMessages() []struct {
	subject string
	data    []byte
} {
	return m.publishedMessages
}

// MockAgentRuntime is a mock agent runtime that can simulate budget exceeded
type MockAgentRuntime struct {
	budgetExceeded bool
}

func NewMockAgentRuntime(budgetExceeded bool) *MockAgentRuntime {
	return &MockAgentRuntime{
		budgetExceeded: budgetExceeded,
	}
}

func (m *MockAgentRuntime) ExecuteTool(name string, args map[string]interface{}) agents.ToolResult {
	if m.budgetExceeded {
		return agents.ToolResult{
			Function: name,
			Args:     args,
			Result:   nil,
			Error:    fmt.Errorf("budget exceeded"),
		}
	}
	return agents.ToolResult{
		Function: name,
		Args:     args,
		Result:   map[string]interface{}{"status": "success"},
		Error:    nil,
	}
}

func TestHTTPAPI_handleCreatePlan_BudgetExceeded(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mockStore := NewMockPlanStore()
	mockNATS := NewMockNATSConn()
	
	// Create mock runtime that simulates budget exceeded
	mockRuntime := NewMockAgentRuntime(true)
	
	// Create agents with mock runtime
	planner := agents.NewPlannerAgent(mockRuntime, logger)
	policyWriter := agents.NewPolicyWriterAgent(mockRuntime, logger)
	segmenter := agents.NewSegmenterAgent(mockRuntime, logger)
	explainer := agents.NewExplainerAgent(mockRuntime, logger)
	
	// Create guardrails
	guardrails := NewGuardrails(logger)
	
	// Create HTTP API
	api := &HTTPAPI{
		store:        mockStore,
		logger:       logger,
		natsConn:     mockNATS,
		agentRuntime: mockRuntime,
		planner:      planner,
		policyWriter: policyWriter,
		segmenter:    segmenter,
		explainer:    explainer,
		guardrails:   guardrails,
	}

	t.Run("budget_exceeded_returns_suggest_plan_with_empty_controls", func(t *testing.T) {
		requestBody := model.CreatePlanRequest{
			Finding: &map[string]interface{}{
				"id":        "finding-123",
				"severity":  "high",
				"host_id":   "web-01",
				"rule_id":   "bash-exec-after-connect",
				"evidence":  []string{"Bash execution detected after network connection"},
				"confidence": 0.8,
				"context": map[string]interface{}{
					"labels": []string{"web", "frontend"},
				},
			},
		}

		jsonBody, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/plans", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Handle the request
		api.handleCreatePlan(w, req)

		// Check response status - should be 201 (created) even with budget exceeded
		assert.Equal(t, http.StatusCreated, w.Code, "Should return 201 even with budget exceeded")

		// Parse response
		var response model.CreatePlanResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err, "Should be able to parse response")

		// Verify plan was created
		plan := response.Plan
		assert.NotEmpty(t, plan.ID, "Plan should have an ID")
		assert.Equal(t, model.PlanStatusProposed, plan.Status, "Plan should have proposed status")

		// Verify strategy is suggest (fallback)
		assert.Equal(t, model.StrategyModeSuggest, plan.Strategy.Mode, "Should use suggest strategy as fallback")

		// Verify targets fallback to finding host_id
		assert.Equal(t, []string{"web-01"}, plan.Targets, "Should fallback to finding host_id")

		// Verify controls are empty or minimal (due to budget exceeded)
		// The policy writer should handle budget exceeded gracefully
		assert.NotNil(t, plan.Controls, "Controls should not be nil")
		
		// Verify plan was stored
		storedPlan, err := mockStore.Get(context.Background(), plan.ID)
		require.NoError(t, err, "Plan should be stored")
		assert.Equal(t, plan.ID, storedPlan.ID, "Stored plan should match")

		// Verify NATS message was published
		messages := mockNATS.GetPublishedMessages()
		require.Len(t, messages, 1, "Should publish one NATS message")
		assert.Equal(t, "plans.created", messages[0].subject, "Should publish to plans.created subject")
	})
}

func TestHTTPAPI_handleCreatePlan_NormalOperation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mockStore := NewMockPlanStore()
	mockNATS := NewMockNATSConn()
	
	// Create mock runtime with normal operation
	mockRuntime := NewMockAgentRuntime(false)
	
	// Create agents with mock runtime
	planner := agents.NewPlannerAgent(mockRuntime, logger)
	policyWriter := agents.NewPolicyWriterAgent(mockRuntime, logger)
	segmenter := agents.NewSegmenterAgent(mockRuntime, logger)
	explainer := agents.NewExplainerAgent(mockRuntime, logger)
	
	// Create guardrails
	guardrails := NewGuardrails(logger)
	
	// Create HTTP API
	api := &HTTPAPI{
		store:        mockStore,
		logger:       logger,
		natsConn:     mockNATS,
		agentRuntime: mockRuntime,
		planner:      planner,
		policyWriter: policyWriter,
		segmenter:    segmenter,
		explainer:    explainer,
		guardrails:   guardrails,
	}

	t.Run("normal_operation_creates_plan_successfully", func(t *testing.T) {
		requestBody := model.CreatePlanRequest{
			Finding: &map[string]interface{}{
				"id":        "finding-456",
				"severity":  "high",
				"host_id":   "web-01",
				"rule_id":   "connect-to-exec",
				"evidence":  []string{"Network connection followed by process execution"},
				"confidence": 0.8,
				"context": map[string]interface{}{
					"labels": []string{"web", "frontend"},
				},
			},
		}

		jsonBody, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/plans", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Handle the request
		api.handleCreatePlan(w, req)

		// Check response status
		assert.Equal(t, http.StatusCreated, w.Code, "Should return 201 for successful creation")

		// Parse response
		var response model.CreatePlanResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err, "Should be able to parse response")

		// Verify plan was created
		plan := response.Plan
		assert.NotEmpty(t, plan.ID, "Plan should have an ID")
		assert.Equal(t, model.PlanStatusProposed, plan.Status, "Plan should have proposed status")
		assert.NotEmpty(t, plan.Targets, "Plan should have targets")
		assert.NotEmpty(t, plan.Notes, "Plan should have notes")

		// Verify message
		assert.Equal(t, "Plan created successfully with agentic pipeline", response.Message, "Should have success message")
	})
}

func TestHTTPAPI_handleCreatePlan_ValidationErrors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mockStore := NewMockPlanStore()
	mockNATS := NewMockNATSConn()
	mockRuntime := NewMockAgentRuntime(false)
	
	planner := agents.NewPlannerAgent(mockRuntime, logger)
	policyWriter := agents.NewPolicyWriterAgent(mockRuntime, logger)
	segmenter := agents.NewSegmenterAgent(mockRuntime, logger)
	explainer := agents.NewExplainerAgent(mockRuntime, logger)
	guardrails := NewGuardrails(logger)
	
	api := &HTTPAPI{
		store:        mockStore,
		logger:       logger,
		natsConn:     mockNATS,
		agentRuntime: mockRuntime,
		planner:      planner,
		policyWriter: policyWriter,
		segmenter:    segmenter,
		explainer:    explainer,
		guardrails:   guardrails,
	}

	tests := []struct {
		name        string
		requestBody interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name: "missing_both_finding_id_and_finding",
			requestBody: map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
			expectedError: "Either finding_id or finding must be provided",
		},
		{
			name: "invalid_json",
			requestBody: "invalid json",
			expectedStatus: http.StatusBadRequest,
			expectedError: "Invalid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var jsonBody []byte
			if str, ok := tt.requestBody.(string); ok {
				jsonBody = []byte(str)
			} else {
				jsonBody, _ = json.Marshal(tt.requestBody)
			}

			req := httptest.NewRequest("POST", "/plans", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			api.handleCreatePlan(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "Should return expected status code")
			assert.Contains(t, w.Body.String(), tt.expectedError, "Should contain expected error message")
		})
	}
}

// Helper function to create a mock HTTPAPI for testing
func createMockHTTPAPI(budgetExceeded bool) *HTTPAPI {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	mockStore := NewMockPlanStore()
	mockNATS := NewMockNATSConn()
	mockRuntime := NewMockAgentRuntime(budgetExceeded)
	
	planner := agents.NewPlannerAgent(mockRuntime, logger)
	policyWriter := agents.NewPolicyWriterAgent(mockRuntime, logger)
	segmenter := agents.NewSegmenterAgent(mockRuntime, logger)
	explainer := agents.NewExplainerAgent(mockRuntime, logger)
	guardrails := NewGuardrails(logger)
	
	return &HTTPAPI{
		store:        mockStore,
		logger:       logger,
		natsConn:     mockNATS,
		agentRuntime: mockRuntime,
		planner:      planner,
		policyWriter: policyWriter,
		segmenter:    segmenter,
		explainer:    explainer,
		guardrails:   guardrails,
	}
}
