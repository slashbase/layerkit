package settings

import (
	"encoding/json"
	"os"

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

func ReadSettingsFromFile() Settings {
	file, err := os.Open(settingsFilePath)
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
