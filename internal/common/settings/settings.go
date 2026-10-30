package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const settingsFilePath = "settings.json"

type Settings struct {
	Name      string             `json:"name"`
	Version   string             `json:"version"`
	Type      string             `json:"type"`
	McpServer mcp.Implementation `json:"mcpServer"`
}

// DefaultSettings is what a repository with no settings.json gets. The MCP
// implementation is filled in from the same values, because a server that
// announces itself with no name is refused or displayed as nameless by clients.
func DefaultSettings() Settings {
	return Settings{
		Name:      "layerkit",
		Version:   "0.0.0",
		Type:      "stdio",
		McpServer: mcp.Implementation{Name: "layerkit", Version: "0.0.0"},
	}
}

// ReadSettingsFromFile reads settings.json from dir, so a server started with
// --path reads the settings belonging to the layers it serves rather than the
// ones in whatever directory it happened to be started from.
//
// A missing file yields the defaults. A file that exists but cannot be parsed is
// an error: substituting the defaults silently would rename the server and
// change its transport without telling anyone.
func ReadSettingsFromFile(dir string) (Settings, error) {
	path := filepath.Join(dir, settingsFilePath)

	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return DefaultSettings(), nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("opening %s: %w", path, err)
	}
	defer file.Close()

	var loaded Settings
	if err := json.NewDecoder(file).Decode(&loaded); err != nil {
		return Settings{}, fmt.Errorf("parsing %s: %w", path, err)
	}

	return loaded.withDefaults(), nil
}

// withDefaults fills in whatever the file left out. A settings.json written by
// hand, or one predating the mcpServer block, still names the server: the
// implementation inherits the top-level name and version rather than staying
// empty.
func (s Settings) withDefaults() Settings {
	defaults := DefaultSettings()

	if s.Name == "" {
		s.Name = defaults.Name
	}
	if s.Version == "" {
		s.Version = defaults.Version
	}
	if s.Type == "" {
		s.Type = defaults.Type
	}

	if s.McpServer.Name == "" {
		s.McpServer.Name = s.Name
	}
	if s.McpServer.Version == "" {
		s.McpServer.Version = s.Version
	}

	return s
}
