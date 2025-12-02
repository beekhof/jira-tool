package jira

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateTicketPoints(t *testing.T) {
	// Create a mock Jira server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request method
		if r.Method != "PUT" {
			t.Errorf("expected PUT request, got %s", r.Method)
		}

		// Verify the endpoint
		expectedPath := "/rest/api/2/issue/ENG-123"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}

		// Verify the content type
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Verify Bearer token auth is present
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			t.Error("expected Authorization header, but it was not present")
		}
		expectedAuth := "Bearer test-token"
		if authHeader != expectedAuth {
			t.Errorf("expected Authorization header '%s', got '%s'", expectedAuth, authHeader)
		}

		// Parse and verify the request body
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

		fields, ok := payload["fields"].(map[string]interface{})
		if !ok {
			t.Error("expected 'fields' in payload")
		}

		points, ok := fields["customfield_10016"].(float64)
		if !ok {
			t.Error("expected 'customfield_10016' in fields")
		}

		if int(points) != 5 {
			t.Errorf("expected points 5, got %v", points)
		}

		// Return success response
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	// Create a client pointing to the mock server
	client := &jiraClient{
		baseURL:            server.URL,
		httpClient:         &http.Client{},
		authToken:          "test-token",
		storyPointsFieldID: "customfield_10016", // Set the field ID expected by the test
	}

	// Test the UpdateTicketPoints method
	err := client.UpdateTicketPoints("ENG-123", 5)
	if err != nil {
		t.Errorf("UpdateTicketPoints failed: %v", err)
	}
}

func TestUpdateTicketPoints_NotFound(t *testing.T) {
	// Create a mock server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &jiraClient{
		baseURL:    server.URL,
		httpClient: &http.Client{},
		authToken:  "test-token",
	}

	err := client.UpdateTicketPoints("ENG-999", 5)
	if err == nil {
		t.Error("expected error for 404 response, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestUpdateTicketPoints_Unauthorized(t *testing.T) {
	// Create a mock server that returns 401
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := &jiraClient{
		baseURL:    server.URL,
		httpClient: &http.Client{},
		authToken:  "invalid-token",
	}

	err := client.UpdateTicketPoints("ENG-123", 5)
	if err == nil {
		t.Error("expected error for 401 response, got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("expected 'authentication failed' in error, got: %v", err)
	}
}

func TestCreateTicket(t *testing.T) {
	// Create a mock Jira server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request method
		if r.Method != "POST" {
			t.Errorf("expected POST request, got %s", r.Method)
		}

		// Verify the endpoint
		expectedPath := "/rest/api/2/issue"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}

		// Verify the content type
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Verify Bearer token auth is present
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			t.Error("expected Authorization header, but it was not present")
		}
		expectedAuth := "Bearer test-token"
		if authHeader != expectedAuth {
			t.Errorf("expected Authorization header '%s', got '%s'", expectedAuth, authHeader)
		}

		// Parse and verify the request body
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

		fields, ok := payload["fields"].(map[string]interface{})
		if !ok {
			t.Error("expected 'fields' in payload")
		}

		project, ok := fields["project"].(map[string]interface{})
		if !ok {
			t.Error("expected 'project' in fields")
		}
		if project["key"] != "ENG" {
			t.Errorf("expected project key 'ENG', got '%v'", project["key"])
		}

		summary, ok := fields["summary"].(string)
		if !ok {
			t.Error("expected 'summary' in fields")
		}
		if summary != "Test ticket" {
			t.Errorf("expected summary 'Test ticket', got '%s'", summary)
		}

		issuetype, ok := fields["issuetype"].(map[string]interface{})
		if !ok {
			t.Error("expected 'issuetype' in fields")
		}
		if issuetype["name"] != "Task" {
			t.Errorf("expected issuetype name 'Task', got '%v'", issuetype["name"])
		}

		// Return success response
		response := CreateTicketResponse{
			ID:   "10001",
			Key:  "ENG-123",
			Self: "https://example.atlassian.net/rest/api/2/issue/10001",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create a client pointing to the mock server
	client := &jiraClient{
		baseURL:    server.URL,
		httpClient: &http.Client{},
		authToken:  "test-token",
	}

	// Test the CreateTicket method
	ticketKey, err := client.CreateTicket("ENG", "Task", "Test ticket")
	if err != nil {
		t.Errorf("CreateTicket failed: %v", err)
	}
	if ticketKey != "ENG-123" {
		t.Errorf("expected ticket key 'ENG-123', got '%s'", ticketKey)
	}
}

func TestCreateTicket_Error(t *testing.T) {
	// Create a mock server that returns 400 (bad request)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errorMessages":["Invalid project key"]}`))
	}))
	defer server.Close()

	client := &jiraClient{
		baseURL:    server.URL,
		httpClient: &http.Client{},
		authToken:  "test-token",
	}

	_, err := client.CreateTicket("INVALID", "Task", "Test")
	if err == nil {
		t.Error("expected error for 400 response, got nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected '400' in error, got: %v", err)
	}
}
