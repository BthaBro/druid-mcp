package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/BthaBro/druid-mcp/internal/config"
	"github.com/BthaBro/druid-mcp/internal/druid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Register adds all druid tools to the MCP server.
func Register(s *server.MCPServer, cfg *config.Config, client *druid.Client, envName string) {
	registerListEnvironments(s, cfg)
	registerQuery(s, client, envName)
	registerIngest(s, client, envName)
	registerTaskStatus(s, client, envName)
	registerCancelTask(s, client, envName)
}

func registerListEnvironments(s *server.MCPServer, cfg *config.Config) {
	tool := mcp.NewTool("druid_list_environments",
		mcp.WithDescription("List all configured Druid environments and their connection status"),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		envs := cfg.ListEnvironments()
		var lines []string
		for _, env := range envs {
			authStatus := "configured"
			if !env.AuthConfigured {
				authStatus = "NOT SET"
			}
			lines = append(lines, fmt.Sprintf("%s: %s [auth: %s]", env.Name, env.URL, authStatus))
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: strings.Join(lines, "\n")},
			},
		}, nil
	})
}

func registerQuery(s *server.MCPServer, client *druid.Client, envName string) {
	tool := mcp.NewTool("druid_query",
		mcp.WithDescription(fmt.Sprintf("Execute a read-only SELECT query against the %s Druid cluster", envName)),
		mcp.WithString("query",
			mcp.Description("SQL SELECT query to execute"),
			mcp.Required(),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := req.GetString("query", "")
		if query == "" {
			return mcp.NewToolResultError("query parameter is required"), nil
		}

		if err := druid.ValidateReadOnlyQuery(query); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, err := client.Query(query)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Query execution error: %v", err)), nil
		}

		if result.Error != "" {
			return mcp.NewToolResultError(result.Error), nil
		}

		rowCount := len(result.Rows)
		jsonData, err := json.MarshalIndent(result.Rows, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error formatting results: %v", err)), nil
		}

		output := fmt.Sprintf("Results: %d row(s)\n\n%s", rowCount, string(jsonData))
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: output},
			},
		}, nil
	})
}

func registerIngest(s *server.MCPServer, client *druid.Client, envName string) {
	tool := mcp.NewTool("druid_ingest",
		mcp.WithDescription(fmt.Sprintf("Execute an INSERT INTO or REPLACE INTO query against the %s Druid cluster. Requires confirmation.", envName)),
		mcp.WithString("query",
			mcp.Description("SQL INSERT INTO or REPLACE INTO query to execute"),
			mcp.Required(),
		),
		mcp.WithBoolean("confirmed",
			mcp.Description("Set to true to confirm and execute the query. First call without this to preview."),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := req.GetString("query", "")
		if query == "" {
			return mcp.NewToolResultError("query parameter is required"), nil
		}

		confirmed := req.GetBool("confirmed", false)

		if err := druid.ValidateIngestQuery(query); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// If not confirmed, show preview
		if !confirmed {
			msg := fmt.Sprintf(
				"⚠️  WRITE OPERATION PENDING CONFIRMATION\n\n"+
					"Environment: %s\n"+
					"Query:\n%s\n\n"+
					"To execute this query, call druid_ingest again with the same query and set \"confirmed\" to true.",
				envName, query,
			)
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: msg},
				},
			}, nil
		}

		// Execute the ingest
		result, err := client.Ingest(query)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Ingest execution error: %v", err)), nil
		}

		if result.Error != "" {
			return mcp.NewToolResultError(result.Error), nil
		}

		output := fmt.Sprintf(
			"Ingest task submitted successfully.\nTask ID: %s\nState: %s\n\n"+
				"To check task status, use druid_task_status with taskId: %q",
			result.TaskID, result.State, result.TaskID,
		)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: output},
			},
		}, nil
	})
}

func registerTaskStatus(s *server.MCPServer, client *druid.Client, envName string) {
	tool := mcp.NewTool("druid_task_status",
		mcp.WithDescription("Check the status of a Druid ingestion task"),
		mcp.WithString("task_id",
			mcp.Description("The task ID returned by druid_ingest"),
			mcp.Required(),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID := req.GetString("task_id", "")
		if taskID == "" {
			return mcp.NewToolResultError("task_id parameter is required"), nil
		}

		status, err := client.GetTaskStatus(taskID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error getting task status: %v", err)), nil
		}

		lines := []string{
			fmt.Sprintf("Task: %s", status.TaskID),
			fmt.Sprintf("State: %s", status.State),
		}
		if status.Error != "" {
			lines = append(lines, fmt.Sprintf("Error: %s", status.Error))
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: strings.Join(lines, "\n")},
			},
		}, nil
	})
}

func registerCancelTask(s *server.MCPServer, client *druid.Client, envName string) {
	tool := mcp.NewTool("druid_cancel_task",
		mcp.WithDescription("Cancel a running Druid ingestion task"),
		mcp.WithString("task_id",
			mcp.Description("The task ID to cancel"),
			mcp.Required(),
		),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID := req.GetString("task_id", "")
		if taskID == "" {
			return mcp.NewToolResultError("task_id parameter is required"), nil
		}

		if err := client.CancelTask(taskID); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error cancelling task: %v", err)), nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("Task %s cancelled successfully.", taskID)},
			},
		}, nil
	})
}
