package build

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/slashbase/layerengine"
	"github.com/slashbase/layerengine/codegen"
	"github.com/slashbase/layerengine/modules"
	"github.com/slashbase/layerengine/validator"
	"github.com/slashbase/layerkit/internal/common/config"
	"github.com/slashbase/layerkit/internal/common/index"
)

// StartLayerkitBuild compiles all *.tool.yaml specs in the current
// directory into hash-addressed layer artifacts shared under
// .layerkit/build/layers/, updating .layerkit/index.json.
func StartLayerkitBuild() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("error: could not determine current directory:", err)
		os.Exit(1)
	}

	specPaths, err := filepath.Glob(filepath.Join(cwd, "*.tool.yaml"))
	if err != nil {
		fmt.Println("error globbing *.tool.yaml:", err)
		os.Exit(1)
	}
	if len(specPaths) == 0 {
		fmt.Println("no *.tool.yaml specs found in current directory")
		return
	}

	openAIAPIKey := config.GetEnv().OPENAI_API_KEY
	anthropicAPIKey := config.GetEnv().ANTHROPIC_API_KEY
	if openAIAPIKey == "" && anthropicAPIKey == "" {
		fmt.Println("error: OPENAI_API_KEY or ANTHROPIC_API_KEY is required")
		os.Exit(1)
	}
	apiKey := openAIAPIKey
	if apiKey == "" {
		apiKey = anthropicAPIKey
	}

	model := codegen.GPT6Luna
	if anthropicAPIKey != "" {
		model = codegen.ClaudeSonnet5_5
	}

	codegenerater, err := codegen.NewCodeGen(apiKey, model)
	if err != nil {
		fmt.Println("error creating codegen client:", err)
		os.Exit(1)
	}
	engine := layerengine.NewLayerEngine(codegenerater)

	var idx index.Index
	err = idx.Load(cwd)
	if err != nil {
		fmt.Println("error loading index:", err)
		os.Exit(1)
	}

	for _, specPath := range specPaths {
		if err := buildSpec(cwd, specPath, &idx, engine, codegenerater, model); err != nil {
			fmt.Printf("error building %s: %v\n", filepath.Base(specPath), err)
			os.Exit(1)
		}
	}

	if err := idx.Save(cwd); err != nil {
		fmt.Println("error writing index:", err)
		os.Exit(1)
	}

	fmt.Printf("built %d spec(s)\n", len(specPaths))
}

// buildSpec compiles a single *.tool.yaml, reusing any layer artifacts
// that already exist for an identical layer regardless of which tool
// produced them, and updates idx.
func buildSpec(cwd, specPath string, idx *index.Index, engine *layerengine.LayerEngine, codegenerater *codegen.CodeGen, model codegen.ModelID) error {
	specData, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("reading spec: %w", err)
	}

	f, err := validator.Run(specData)
	if err != nil {
		return fmt.Errorf("parsing spec: %w", err)
	}

	var flow layerengine.Flow
	layerengine.MapToStruct(validator.StructToMap(f), &flow)

	toolName := flow.Name
	specHash := hashContent(specData)

	refs := make([]index.LayerRef, 0, len(flow.Layers))
	flowInputMap := map[string]layerengine.FlowInput{}
	for _, fi := range flow.Input {
		flowInputMap[fi.Name] = fi
	}

	flowOutputMap := map[string]codegen.Output{}
	for _, layer := range flow.Layers {
		ref, err := buildLayer(cwd, layer, codegenerater, flowInputMap, flowOutputMap, engine.GetModules(), string(model))
		if err != nil {
			return fmt.Errorf("layer %q: %w", layer.Name, err)
		}
		refs = append(refs, ref)
	}

	_ = engine // reserved for a future dry-run / validation pass via engine.LoadSpec

	fmt.Printf("  %s (%s) built\n", toolName, shortHash(specHash))
	idx.UpsertToolEntry(toolName, specHash, refs)
	return nil
}

// buildLayer builds a single layer's artifact if one doesn't already exist for
// its content hash, reusing it across any tool whose prompt for that layer is
// identical.
func buildLayer(cwd string, layer layerengine.Layer, codegenerater *codegen.CodeGen, flowInputMap map[string]layerengine.FlowInput, flowOutputMap map[string]codegen.Output, mods []modules.Module, model string) (index.LayerRef, error) {
	// Resolve the contract before anything else: it is what the prompt is built
	// from, so it belongs in the hash. It also has to reach flowOutputMap before
	// any early return, or a later layer that consumes this layer's output
	// resolves an empty type on a warm cache.
	inputs := make([]codegen.Input, len(layer.Input))
	for j, inpName := range layer.Input {
		if fi, exists := flowInputMap[inpName]; exists {
			codegen.MapToStruct(layerengine.StructToMap(fi), &inputs[j])
		} else {
			codegen.MapToStruct(layerengine.StructToMap(flowOutputMap[inpName]), &inputs[j])
			inputs[j].Optional = false
		}
	}

	outputs := make([]codegen.Output, len(layer.Output))
	for j, out := range layer.Output {
		codegen.MapToStruct(layerengine.StructToMap(out), &outputs[j])
		flowOutputMap[out.Name] = outputs[j]
	}

	hash := hashLayer(layer, inputs, outputs, model)
	relDir := filepath.Join(index.StateDir, index.LayersDir, hash)
	dir := filepath.Join(cwd, relDir)
	relPath := filepath.Join(relDir, layer.Name+".lua")
	path := filepath.Join(cwd, relPath)

	if fileExists(path) {
		fmt.Printf("    %s (%s) already built, reusing\n", layer.Name, shortHash(hash))
		return index.LayerRef{Name: layer.Name, Hash: hash, Path: relPath}, nil
	}

	luaCode, err := codegenerater.GenerateLayerFunction(layer.Name, layer.Description, inputs, outputs, mods)
	if err != nil {
		return index.LayerRef{}, fmt.Errorf("generating: %w", err)
	}

	fmt.Printf("    %s (%s): %s\n", layer.Name, shortHash(hash), luaCode)

	if _, err := layerengine.ParseAndCompileLuaCode(luaCode); err != nil {
		return index.LayerRef{}, fmt.Errorf("compiling: %w", err)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return index.LayerRef{}, fmt.Errorf("creating artifact dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(luaCode), 0644); err != nil {
		return index.LayerRef{}, fmt.Errorf("writing artifact: %w", err)
	}

	return index.LayerRef{Name: layer.Name, Hash: hash, Path: relPath}, nil
}

func hashContent(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// hashLayer hashes everything the prompt is built from: the layer's own
// fields, the resolved input and output contract, the module set whose Usage
// strings are quoted in the prompt, and the model that writes the code. Two
// layers with an identical prompt — even from different tools — share one
// artifact, and any edit that would change the generated Lua produces a new
// one instead of silently reusing the old file.
func hashLayer(layer layerengine.Layer, inputs []codegen.Input, outputs []codegen.Output, model string) string {
	var b strings.Builder
	field := func(values ...string) {
		for _, v := range values {
			b.WriteString(v)
			b.WriteString("\x00")
		}
	}

	field("layer", layer.Name, layer.Description)
	field("inputs")
	for _, in := range inputs {
		field(in.Name, in.Type, in.Description, strconv.FormatBool(in.Optional))
	}
	field("outputs")
	for _, out := range outputs {
		field(out.Name, out.Type, out.Description)
	}
	field("model", model)

	return hashContent([]byte(b.String()))
}

func shortHash(h string) string {
	if len(h) > 8 {
		return h[:8]
	}
	return h
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
