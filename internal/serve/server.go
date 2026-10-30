package serve

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/slashbase/layerengine"
	"github.com/slashbase/layerkit/internal/common/config"
	"github.com/slashbase/layerkit/internal/common/settings"
)

type serverType string

const (
	stdio serverType = "stdio"
	http  serverType = "http"
)

// server wraps the single shared engine that all flows are loaded into.
type server struct {
	engine    *layerengine.LayerEngine
	settings  settings.Settings
	mcpServer *mcp.Server
}

// NewServer creates a new server instance with a fresh LayerEngine and MCP server.
func NewServer() *server {
	s := settings.ReadSettingsFromFile()
	return &server{
		engine:    layerengine.NewLayerEngine(nil),
		settings:  s,
		mcpServer: mcp.NewServer(&s.McpServer, nil),
	}
}

// RunServer runs the server, serving over the configured transport (stdio or http).
func (r *server) RunServer(ctx context.Context) error {
	switch serverType(r.settings.Type) {
	case stdio:
		if err := r.mcpServer.Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
			return err
		}
		return nil
	case http:
		return fmt.Errorf("HTTP server not yet implemented")
	}
	return fmt.Errorf("unsupported server type: %s", r.settings.Type)
}

// handler returns the MCP tool handler that runs the named flow.
func (r *server) handler(flowName string, outputNames []string) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := map[string]any{}
		if len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return toolError(fmt.Sprintf("invalid arguments: %v", err)), nil
			}
		}

		result, err := r.engine.RunFlowWithOptions(flowName, args, config.LayerEngineCoerceOptions())
		if err != nil {
			// Flow failures are tool-level errors, not protocol errors.
			return toolError(err.Error()), nil
		}

		named := nameOutputs(result, outputNames)
		raw, err := json.Marshal(named)
		if err != nil {
			return toolErrorf("flow %q produced a non-serialisable result: %v", flowName, err), nil
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}},
		}, nil
	}
}

func toolError(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}
}

func toolErrorf(format string, a ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, a...)}},
		IsError: true,
	}
}

// nameOutputs pairs RunFlow's positional result with the flow's declared
// final-layer output names. If the arity doesn't match, it returns the raw
// slice rather than risk mislabelling a value.
func nameOutputs(result any, names []string) any {
	values, ok := result.([]any)
	if !ok || len(values) != len(names) {
		return result
	}
	named := make(map[string]any, len(names))
	for i, name := range names {
		named[name] = values[i]
	}
	return named
}
