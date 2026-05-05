package main

import (
	"fmt"
	"log"
	"os"

	"github.com/BthaBro/druid-mcp/internal/config"
	"github.com/BthaBro/druid-mcp/internal/druid"
	"github.com/BthaBro/druid-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Determine which environment to use from DRUID_ENV
	envName := os.Getenv("DRUID_ENV")
	if envName == "" {
		fmt.Fprintln(os.Stderr, "DRUID_ENV environment variable is not set.")
		fmt.Fprintln(os.Stderr, "Set it in your MCP client configuration, e.g.:")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, `  "env": { "DRUID_ENV": "sb5" }`)
		os.Exit(1)
	}

	// Load config from ~/.config/druid-mcp/config.yaml
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Get the specific environment
	env, err := cfg.GetEnvironment(envName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Create Druid client
	client := druid.NewClient(env.URL, env.Auth)

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"druid-mcp",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Register tools
	tools.Register(mcpServer, cfg, client, envName)

	// Log startup info to stderr (stdout is reserved for MCP protocol)
	log.SetOutput(os.Stderr)
	log.Printf("druid-mcp server starting (env=%s, url=%s)", envName, env.URL)

	// Serve on stdio
	if err := server.ServeStdio(mcpServer); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
