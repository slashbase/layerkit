package index

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	StateDir  = ".layerkit"
	LayersDir = "layers"
	IndexFile = "index.json"
)

// layerRef points at a shared, content-addressed layer artifact.
type LayerRef struct {
	Name string `json:"name"`
	Hash string `json:"hash"`
	Path string `json:"path"`
}

// indexEntry describes one tool in .layerkit/index.json.
type IndexEntry struct {
	Name   string     `json:"name"`
	Hash   string     `json:"hash"`
	Layers []LayerRef `json:"layers"`
}

// index is the top-level .layerkit/index.json structure.
type Index struct {
	Tools []IndexEntry `json:"tools"`
}

func (idx *Index) Load(cwd string) error {
	path := filepath.Join(cwd, StateDir, IndexFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, idx); err != nil {
		return err
	}
	return nil
}

func (idx *Index) Save(cwd string) error {
	dir := filepath.Join(cwd, StateDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, IndexFile), data, 0644)
}

// upsertToolEntry updates idx in place so toolName's entry points at
// specHash and the given layer artifacts, adding a new entry if toolName
// hasn't been built before.
func (idx *Index) UpsertToolEntry(toolName, specHash string, refs []LayerRef) {
	for i := range idx.Tools {
		if idx.Tools[i].Name == toolName {
			idx.Tools[i].Hash = specHash
			idx.Tools[i].Layers = refs
			return
		}
	}
	idx.Tools = append(idx.Tools, IndexEntry{
		Name:   toolName,
		Hash:   specHash,
		Layers: refs,
	})
}

// LayersFor returns the layer artifacts recorded for toolName.
func (idx *Index) LayersFor(toolName string) ([]LayerRef, error) {
	// TODO: consider using a map for faster lookups if the index grows large.
	// fine for now since the index is expected to be small.
	for _, entry := range idx.Tools {
		if entry.Name == toolName {
			return entry.Layers, nil
		}
	}
	return nil, fmt.Errorf("no build found for tool %q", toolName)
}

// HashContent is the content hash of a spec file's bytes. build records it in
// the index; serve and run compare against it to detect a spec that has been
// edited since it was last built.
func HashContent(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// HashFor returns the spec hash recorded for toolName, and whether the tool has
// been built at all.
func (idx *Index) HashFor(toolName string) (string, bool) {
	for _, entry := range idx.Tools {
		if entry.Name == toolName {
			return entry.Hash, true
		}
	}
	return "", false
}
