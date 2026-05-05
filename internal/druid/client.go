package druid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client handles communication with a Druid cluster.
type Client struct {
	baseURL    string
	auth       string
	httpClient *http.Client
}

// NewClient creates a new Druid client for the given environment.
func NewClient(baseURL, auth string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		auth:    auth,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// QueryResult holds the result of a read-only SQL query.
type QueryResult struct {
	Rows  []map[string]any
	Error string
}

// IngestResult holds the result of an INSERT/REPLACE operation.
type IngestResult struct {
	TaskID string
	State  string
	Error  string
}

// TaskStatus holds the status of an ingestion task.
type TaskStatus struct {
	TaskID string
	State  string
	Error  string
}

// Query executes a read-only SQL query against Druid's /druid/v2/sql endpoint.
func (c *Client) Query(query string) (*QueryResult, error) {
	endpoint := c.baseURL + "/druid/v2/sql"

	body, err := json.Marshal(map[string]string{
		"query":        query,
		"resultFormat": "objectLines",
	})
	if err != nil {
		return nil, fmt.Errorf("marshaling query: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.auth)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &QueryResult{
			Error: fmt.Sprintf("Druid query failed (HTTP %d): %s", resp.StatusCode, string(respBody)),
		}, nil
	}

	// Parse objectLines format: one JSON object per line
	rows := make([]map[string]any, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(respBody)), "\n") {
		if line == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("parsing response line: %w", err)
		}
		rows = append(rows, row)
	}

	return &QueryResult{Rows: rows}, nil
}

// Ingest executes an INSERT/REPLACE SQL query via Druid's /druid/v2/sql/task endpoint.
func (c *Client) Ingest(query string) (*IngestResult, error) {
	endpoint := c.baseURL + "/druid/v2/sql/task"

	body, err := json.Marshal(map[string]string{
		"query": query,
	})
	if err != nil {
		return nil, fmt.Errorf("marshaling query: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.auth)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &IngestResult{
			Error: fmt.Sprintf("Druid ingest failed (HTTP %d): %s", resp.StatusCode, string(respBody)),
		}, nil
	}

	var result struct {
		TaskID string `json:"taskId"`
		State  string `json:"state"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing ingest response: %w", err)
	}

	state := result.State
	if state == "" {
		state = "SUBMITTED"
	}

	return &IngestResult{
		TaskID: result.TaskID,
		State:  state,
	}, nil
}

// GetTaskStatus checks the status of an ingestion task.
func (c *Client) GetTaskStatus(taskID string) (*TaskStatus, error) {
	endpoint := fmt.Sprintf("%s/druid/indexer/v1/task/%s/status", c.baseURL, url.PathEscape(taskID))

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", c.auth)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &TaskStatus{
			TaskID: taskID,
			State:  "UNKNOWN",
			Error:  fmt.Sprintf("Failed to get task status (HTTP %d): %s", resp.StatusCode, string(respBody)),
		}, nil
	}

	var result struct {
		Status struct {
			ID       string `json:"id"`
			Status   string `json:"status"`
			ErrorMsg string `json:"errorMsg"`
		} `json:"status"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing status response: %w", err)
	}

	state := result.Status.Status
	if state == "" {
		state = "UNKNOWN"
	}

	return &TaskStatus{
		TaskID: taskID,
		State:  state,
		Error:  result.Status.ErrorMsg,
	}, nil
}

// CancelTask cancels a running ingestion task.
func (c *Client) CancelTask(taskID string) error {
	endpoint := fmt.Sprintf("%s/druid/indexer/v1/task/%s/shutdown", c.baseURL, url.PathEscape(taskID))

	req, err := http.NewRequest(http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", c.auth)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cancel task failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
