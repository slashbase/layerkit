package settings

import (
	"encoding/json"
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

func DefaultSettings() Settings {
	return Settings{
		Name:    "layerkit",
		Version: "0.0.0",
		Type:    "stdio",
	}
}

// ReadSettingsFromFile reads settings.json from dir, so a server started with
// --path reads the settings belonging to the layers it serves rather than the
// ones in whatever directory it happened to be started from.
func ReadSettingsFromFile(dir string) Settings {
	file, err := os.Open(filepath.Join(dir, settingsFilePath))
	if err != nil {
		return DefaultSettings()
	}
	defer file.Close()

	var settings Settings
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&settings); err != nil {
		return DefaultSettings()
	}

	return settings
}
