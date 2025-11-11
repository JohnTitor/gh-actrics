package githubapi

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/JohnTitor/gh-actrics/internal/cache"
)

type mockRESTClient struct {
	mu        sync.Mutex
	responses map[string]interface{}
	calls     map[string]int
}

func newMockREST(responses map[string]interface{}) *mockRESTClient {
	return &mockRESTClient{
		responses: responses,
		calls:     make(map[string]int),
	}
}

func (m *mockRESTClient) Get(path string, response interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls[path]++
	payload, ok := m.responses[path]
	if !ok {
		return fmt.Errorf("no mock response for %s", path)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, response)
}

func TestListWorkflowRunsHonorsLimit(t *testing.T) {
	responses := map[string]interface{}{
		"repos/org/repo/actions/workflows/1/runs?created=2025-01-01T00%3A00%3A00Z..2025-01-02T00%3A00%3A00Z&page=1&per_page=2": workflowRunsResponse{
			TotalCount: 4,
			WorkflowRuns: []workflowRunJSON{
				{ID: 1, WorkflowID: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				{ID: 2, WorkflowID: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			},
		},
	}

	mock := newMockREST(responses)
	client := &Client{rest: mock}
	filter := WorkflowRunFilter{Created: "2025-01-01T00:00:00Z..2025-01-02T00:00:00Z"}

	runs, err := client.ListWorkflowRuns(nil, "org", "repo", 1, filter, 2)
	if err != nil {
		t.Fatalf("ListWorkflowRuns failed: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}
	mock.mu.Lock()
	defer mock.mu.Unlock()
	if calls := mock.calls["repos/org/repo/actions/workflows/1/runs?created=2025-01-01T00%3A00%3A00Z..2025-01-02T00%3A00%3A00Z&page=1&per_page=2"]; calls != 1 {
		t.Fatalf("expected single API call, got %d", calls)
	}
}

func TestListWorkflowsUsesCache(t *testing.T) {
	path := "repos/org/repo/actions/workflows?per_page=100&page=1"
	responses := map[string]interface{}{
		path: workflowListResponse{
			TotalCount: 1,
			Workflows: []workflowRecord{
				{ID: 1, Name: "build", Path: ".github/workflows/build.yaml", State: "active"},
			},
		},
	}
	mock := newMockREST(responses)

	dir := t.TempDir()
	cacheStore, err := cache.New(dir, time.Minute)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	client := &Client{rest: mock, cache: cacheStore}

	if _, err := client.ListWorkflows(nil, "org", "repo"); err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if _, err := client.ListWorkflows(nil, "org", "repo"); err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if calls := mock.calls[path]; calls != 1 {
		t.Fatalf("expected cached response to prevent duplicate API call, got %d calls", calls)
	}
}
