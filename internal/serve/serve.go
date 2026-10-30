package serve

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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

	r := NewServer(path)

	// A failed load leaves the engine without those layers or flows, and the
	// tool would still be registered below — failing at call time, in front of
	// an MCP client, with a message that points at the flow rather than at the
	// layer that never compiled. Refuse to start instead.
	if err := r.engine.LoadLayers(allLayers); err != nil {
		fatalf("loading layers: %v", err)
	}
	for _, flow := range flows {
		if err := r.engine.LoadFlow(flow); err != nil {
			fatalf("loading flow: %v", err)
		}
	}

	for _, flow := range flows {
		r.mcpServer.AddTool(&mcp.Tool{
			Name:        flow.Name,
			Description: toolDescription(flow),
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

	// A spec edited since it was built still loads its old artifacts: the layer
	// hash covers a layer's fields and its resolved contract, but a spec can
	// change in ways that leave every layer hash alone. Say so rather than
	// serve yesterday's Lua quietly.
	if builtHash, ok := idx.HashFor(flow.Name); ok {
		if index.HashContent(specData) != builtHash {
			fmt.Fprintf(os.Stderr, "warning: %s changed since the last build; run `layerkit build`\n", filepath.Base(specPath))
		}
	}

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

// toolDescription is what an MCP client shows the model when it chooses a
// tool, so it uses the spec's own description. The name is only a fallback
// for specs that omit one, since the validator does not require it.
func toolDescription(flow layerengine.Flow) string {
	if d := strings.TrimSpace(flow.Description); d != "" {
		return d
	}
	return flow.Name
}

// inputSchema builds the JSON schema for a flow's input from the spec's own
// per-input type, description and optional flag. Types are passed through as
// written — the validator does not constrain them to a fixed set, so any JSON
// Schema type an author writes is honoured, and a missing type falls back to
// "string". Inputs marked `optional: true` are left out of `required` so an
// MCP client may omit them; RunFlow enforces the same rule at runtime.
func inputSchema(flow layerengine.Flow) map[string]any {
	props := make(map[string]any, len(flow.Input))
	var required []string
	for _, fi := range flow.Input {
		t := strings.TrimSpace(fi.Type)
		if t == "" {
			t = "string"
		}
		prop := map[string]any{"type": t}
		if d := strings.TrimSpace(fi.Description); d != "" {
			prop["description"] = d
		}
		props[fi.Name] = prop
		if !fi.Optional {
			required = append(required, fi.Name)
		}
	}
	schema := map[string]any{
		"type":       "object",
		"properties": props,
	}
	// Omit `required` entirely when nothing is mandatory: an empty array is
	// valid JSON Schema but reads as noise in a client's tool list.
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
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
