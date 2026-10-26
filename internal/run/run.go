package run

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/slashbase/layerengine"
	"github.com/slashbase/layerengine/validator"
	"github.com/slashbase/layerkit/internal/common/index"
)

// StartLayerkitRun currently does not start an MCP server.
// It loads hello.tool.yaml's structure (name/layers/input/output) via the
// validator, reads the already-built Lua for each layer from .layerkit/,
// and runs the flow once with sample input — to prove the build -> load ->
// execute path works end to end before MCP wiring is added.
func StartLayerkitRun(specFileName string, input map[string]any) {
	engine := layerengine.NewLayerEngine(nil)

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("error: could not determine current directory:", err)
		os.Exit(1)
	}

	specData, err := os.ReadFile(specFileName)
	if err != nil {
		fmt.Println("error reading spec:", err)
		os.Exit(1)
	}

	f, err := validator.Run(specData)
	if err != nil {
		fmt.Println("error parsing spec:", err)
		os.Exit(1)
	}

	var flow layerengine.Flow
	layerengine.MapToStruct(validator.StructToMap(f), &flow)

	refs, err := buildLayersFor(cwd, flow.Name)
	if err != nil {
		fmt.Println("error finding build for tool:", err)
		fmt.Println("hint: run `layerkit build` first")
		os.Exit(1)
	}

	layerPaths := make(map[string]string)
	for _, ref := range refs {
		layerPaths[ref.Name] = ref.Path
	}

	layers := []layerengine.Layer{}
	layerNames := []string{}
	for _, layer := range flow.Layers {
		relPath, ok := layerPaths[layer.Name]
		if !ok {
			fmt.Printf("error: layer %q not found in build index\n", layer.Name)
			os.Exit(1)
		}

		luaPath := filepath.Join(cwd, relPath)
		code, err := os.ReadFile(luaPath)
		if err != nil {
			fmt.Printf("error reading built layer %q: %v\n", layer.Name, err)
			fmt.Println("hint: run `layerkit build` first")
			os.Exit(1)
		}

		layers = append(layers, layerengine.Layer{
			Name:    layer.Name,
			FnProto: nil,
			Input:   layer.Input,
			Output:  layer.Output,
			Code:    string(code),
		})
		layerNames = append(layerNames, layer.Name)
	}

	engine.LoadLayers(layers)
	engine.LoadFlow(flow)

	result, err := engine.RunFlow(flow.Name, input)
	if err != nil {
		fmt.Println("error running flow:", err)
		os.Exit(1)
	}

	fmt.Printf("%v\n", result)
}

// buildLayersFor returns the layer artifacts recorded for toolName.
func buildLayersFor(cwd, toolName string) ([]index.LayerRef, error) {
	path := filepath.Join(cwd, index.StateDir, index.IndexFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", index.IndexFile, err)
	}

	var idx index.Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", index.IndexFile, err)
	}

	for _, entry := range idx.Tools {
		if entry.Name == toolName {
			return entry.Layers, nil
		}
	}
	return nil, fmt.Errorf("no build found for tool %q", toolName)
}
