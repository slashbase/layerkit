package run

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/slashbase/layerengine"
	"github.com/slashbase/layerengine/validator"
	"github.com/slashbase/layerkit/internal/common/config"
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

	var idx index.Index
	if err := idx.Load(cwd); err != nil {
		fmt.Println("error loading index:", err)
		os.Exit(1)
	}
	if builtHash, ok := idx.HashFor(flow.Name); ok && index.HashContent(specData) != builtHash {
		fmt.Fprintf(os.Stderr, "warning: %s changed since the last build; run `layerkit build`\n", specFileName)
	}

	refs, err := idx.LayersFor(flow.Name)
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

	if err := engine.LoadLayers(layers); err != nil {
		fmt.Println("error loading layers:", err)
		os.Exit(1)
	}
	if err := engine.LoadFlow(flow); err != nil {
		fmt.Println("error loading flow:", err)
		os.Exit(1)
	}

	result, err := engine.RunFlowWithOptions(flow.Name, input, config.LayerEngineCoerceOptions())
	if err != nil {
		fmt.Println("error running flow:", err)
		os.Exit(1)
	}

	fmt.Printf("%v\n", result)
}
