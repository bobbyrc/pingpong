package alerter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAppriseSendSuccess(t *testing.T) {
	var received appriseRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/notify" {
			t.Fatalf("expected /notify path, got %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewAppriseClient(server.URL, "discord://webhook/token")
	err := client.Send("Test Title", "Test Body")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if received.Title != "Test Title" {
		t.Fatalf("expected title 'Test Title', got %s", received.Title)
	}
	if received.Body != "Test Body" {
		t.Fatalf("expected body 'Test Body', got %s", received.Body)
	}
	if received.URLs != "discord://webhook/token" {
		t.Fatalf("expected urls to match, got %s", received.URLs)
	}
}

func TestAppriseSendFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewAppriseClient(server.URL, "discord://webhook/token")
	err := client.Send("Title", "Body")
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestAppriseSendConnectionError(t *testing.T) {
	client := NewAppriseClient("http://127.0.0.1:1", "discord://webhook/token")
	err := client.Send("Title", "Body")
	if err == nil {
		t.Fatal("expected error on connection failure")
	}
}
