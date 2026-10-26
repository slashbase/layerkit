package serve

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/slashbase/layerengine"
	"github.com/slashbase/layerengine/validator"
	"github.com/slashbase/layerkit/internal/common/index"
)

// StartLayerkitServe loads every *.tool.yaml in the current directory,
// loads its built Lua layers from .layerkit/, registers each flow as an
// MCP tool, and serves them over stdio.
//
// NOTE: stdout is the MCP protocol channel on stdio. Never print to stdout
// here — all logging goes to stderr.
func StartLayerkitServe(path string) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var idx index.Index
	err := idx.Load(path)
	if err != nil {
		fatalf("%v\nhint: run `layerkit build` first", err)
	}

	specPaths, err := filepath.Glob(filepath.Join(path, "*.tool.yaml"))
	if err != nil {
		fatalf("globbing specs: %v", err)
	}
	if len(specPaths) == 0 {
		fatalf("no *.tool.yaml files found in %s", path)
	}

	var (
		flows       []layerengine.Flow
		allLayers   []layerengine.Layer
		toolOrigin  = map[string]string{} // tool name -> spec file
		layerOrigin = map[string]string{} // layer name -> spec file
	)
	for _, specPath := range specPaths {
		spec := filepath.Base(specPath)

		flow, layers, err := loadSpec(path, specPath, &idx)
		if err != nil {
			fatalf("%s: %v", spec, err)
		}

		if prev, dup := toolOrigin[flow.Name]; dup {
			fatalf("%s: tool %q already defined in %s", spec, flow.Name, prev)
		}
		toolOrigin[flow.Name] = spec

		// One shared engine means layer names must be globally unique.
		for _, l := range layers {
			if prev, dup := layerOrigin[l.Name]; dup {
				fatalf("%s: layer %q already defined in %s (layer names must be unique across specs)", spec, l.Name, prev)
			}
			layerOrigin[l.Name] = spec
		}

		flows = append(flows, flow)
		allLayers = append(allLayers, layers...)
	}

	r := NewServer()
	r.engine.LoadLayers(allLayers)
	for _, flow := range flows {
		r.engine.LoadFlow(flow)
	}

	for _, flow := range flows {
		r.mcpServer.AddTool(&mcp.Tool{
			Name:        flow.Name,
			Description: flow.Name,
			InputSchema: inputSchema(flow),
		}, r.handler(flow.Name, outputNames(flow.Layers)))
		fmt.Fprintf(os.Stderr, "registered tool %q\n", flow.Name)
	}

	if err := r.RunServer(ctx); err != nil {
		fatalf("server error: %v", err)
	}
}

// loadSpec parses one spec and reads its built layers from disk.
func loadSpec(cwd, specPath string, idx *index.Index) (layerengine.Flow, []layerengine.Layer, error) {
	var flow layerengine.Flow

	specData, err := os.ReadFile(specPath)
	if err != nil {
		return flow, nil, fmt.Errorf("reading spec: %w", err)
	}

	f, err := validator.Run(specData)
	if err != nil {
		return flow, nil, fmt.Errorf("parsing spec: %w", err)
	}

	layerengine.MapToStruct(validator.StructToMap(f), &flow)

	refs, err := idx.LayersFor(flow.Name)
	if err != nil {
		return flow, nil, fmt.Errorf("%w (hint: run `layerkit build`)", err)
	}

	layerPaths := make(map[string]string, len(refs))
	for _, ref := range refs {
		layerPaths[ref.Name] = ref.Path
	}

	layers := make([]layerengine.Layer, 0, len(flow.Layers))
	for _, layer := range flow.Layers {
		relPath, ok := layerPaths[layer.Name]
		if !ok {
			return flow, nil, fmt.Errorf("layer %q not found in build index", layer.Name)
		}

		code, err := os.ReadFile(filepath.Join(cwd, relPath))
		if err != nil {
			return flow, nil, fmt.Errorf("reading built layer %q: %w (hint: run `layerkit build`)", layer.Name, err)
		}

		layers = append(layers, layerengine.Layer{
			Name:    layer.Name,
			FnProto: nil,
			Input:   layer.Input,
			Output:  layer.Output,
			Code:    string(code),
		})
	}

	return flow, layers, nil
}

// inputSchema builds the JSON schema for a flow's input.
// Per-input types aren't in the spec yet, so every input is a string.
// This is the only place that depends on the shape of flow.Input — adjust
// here when typed inputs land.
func inputSchema(flow layerengine.Flow) map[string]any {
	props := map[string]any{}
	required := []string{}
	for _, fi := range flow.Input {
		props[fi.Name] = map[string]any{"type": "string"}
		required = append(required, fi.Name)
	}
	return map[string]any{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
}

func outputNames(layers []layerengine.Layer) []string {
	if len(layers) == 0 {
		return nil
	}
	last := layers[len(layers)-1]
	names := make([]string, len(last.Output))
	for i, o := range last.Output {
		names[i] = o.Name
	}
	return names
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
