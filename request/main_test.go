package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestRoundRobinBalancer(t *testing.T) {
	// Setup
	backends := []*url.URL{
		{
			Scheme: "http",
			Host:   "localhost:81",
		},
		{
			Scheme: "http",
			Host:   "localhost:82",
		},
		{
			Scheme: "http",
			Host:   "localhost:83",
		},
	}

	balancer := NewRoundRobinBalancer(backends)

	// Execute & Verify
	for i, expectedURL := range backends {
		serverURL := balancer.GetNextURL()

		fmt.Println("serverURL", serverURL)
		fmt.Println("expectedURL", expectedURL)

		if serverURL == nil {
			t.Fatalf("Test failed at iteration %d: Received nil URL", i)
		}

		if serverURL.String() != expectedURL.String() {
			t.Errorf("Expected URL %s, got %s", expectedURL, serverURL)
		}

	}
}

func TestRemoveURL(t *testing.T) {
	// Setup
	backends := []*url.URL{
		{
			Scheme: "http",
			Host:   "localhost:81",
		},
		{
			Scheme: "http",
			Host:   "localhost:82",
		},
		{
			Scheme: "http",
			Host:   "localhost:83",
		},
	}

	balancer := NewRoundRobinBalancer(backends)

	// URL to remove
	removeUrlString := "http://localhost:82"
	removeUrl, err := url.Parse(removeUrlString)
	if err != nil {
		t.Fatalf("Failed to parse URL %s: %v", removeUrlString, err)
	}

	// Execute
	balancer.RemoveURL(removeUrl)

	// Verify
	for i := 0; i < len(backends)-1; i++ {
		serverURL := balancer.GetNextURL()
		if serverURL == nil {
			t.Fatalf("Test failed at iteration %d: Received nil URL", i)
		}

		if serverURL.String() == removeUrlString {
			t.Errorf("URL %s was removed but still received in round robin", removeUrlString)
		}
	}
}

func TestCheckAndRestoreUrls(t *testing.T) {
	// Start a test server (simulating a backend server)
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// URL that will be removed and then restored
	testServerURL, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatalf("Failed to parse test server URL: %v", err)
	}

	// Setup
	balancer := NewRoundRobinBalancer([]*url.URL{testServerURL})

	// Initially remove the URL
	balancer.RemoveURL(testServerURL)

	// Check that the URL is removed
	if len(balancer.activeUrls) != 0 {
		t.Errorf("URL was not removed as expected")
	}

	// Simulate the passage of time and server becoming available again
	time.Sleep(1 * time.Second)
	balancer.CheckAndRestoreUrls()

	// Verify that the URL is restored
	if len(balancer.activeUrls) == 0 {
		t.Errorf("URL was not restored as expected")
	}
}

func TestSendRequest(t *testing.T) {
	// Create a mock server that simulates a backend server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("mock response"))
	}))
	defer mockServer.Close()

	mockServerURL, err := url.Parse(mockServer.URL)
	if err != nil {
		t.Fatalf("Failed to parse mock server URL: %v", err)
	}

	// Setup RoundRobinBalancer with the mock server URL
	balancer := NewRoundRobinBalancer([]*url.URL{mockServerURL})

	// Capture the logs to analyze the output
	var capturedLogs string
	log.SetOutput(newMockWriter(&capturedLogs))

	// Call sendRequest function
	sendRequest(balancer)

	// Check if the log contains the expected response
	expectedLogContent := "Response from server: mock response"
	if !contains(capturedLogs, expectedLogContent) {
		t.Errorf("Expected log to contain %q, got %q", expectedLogContent, capturedLogs)
	}
}

// newMockWriter creates an io.Writer that captures the written data in a string
func newMockWriter(capturedLogs *string) *mockWriter {
	return &mockWriter{capturedLogs}
}

type mockWriter struct {
	capturedLogs *string
}

func (m *mockWriter) Write(p []byte) (n int, err error) {
	*m.capturedLogs += string(p)
	return len(p), nil
}

// contains checks if the string contains the specified substring
func contains(str, substr string) bool {
	return strings.Contains(str, substr)
}
