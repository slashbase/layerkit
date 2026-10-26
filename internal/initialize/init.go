package initialize

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed templates/hello.tool.yaml
var helloSpecTemplate embed.FS

//go:embed templates/settings.json
var settingsJsonTemplate embed.FS

//go:embed templates/README.md
var readmeTemplate embed.FS

//go:embed templates/gitignore
var gitignoreTemplate embed.FS

const (
	helloSpecFilename    = "hello.tool.yaml"
	readmeFilename       = "README.md"
	gitignoreFilename    = ".gitignore"
	settingsJsonFilename = "settings.json"
)

// StartLayerkitInit scaffolds a new tool-spec repo in the current directory.
func StartLayerkitInit() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("error: could not determine current directory:", err)
		os.Exit(1)
	}

	if alreadyInitialized(cwd) {
		fmt.Println("error: this directory already looks like a layerkit repo")
		fmt.Println("found an existing *.tool.yaml or .layerkit/ — refusing to overwrite")
		os.Exit(1)
	}

	if err := writeEmbeddedFile(helloSpecTemplate, "templates/hello.tool.yaml", filepath.Join(cwd, helloSpecFilename)); err != nil {
		fmt.Println("error writing", helloSpecFilename+":", err)
		os.Exit(1)
	}

	if err := writeEmbeddedFile(settingsJsonTemplate, "templates/settings.json", filepath.Join(cwd, settingsJsonFilename)); err != nil {
		fmt.Println("error writing", settingsJsonFilename+":", err)
		os.Exit(1)
	}

	if err := writeEmbeddedFile(readmeTemplate, "templates/README.md", filepath.Join(cwd, readmeFilename)); err != nil {
		fmt.Println("error writing", readmeFilename+":", err)
		os.Exit(1)
	}

	if err := writeEmbeddedFile(gitignoreTemplate, "templates/gitignore", filepath.Join(cwd, gitignoreFilename)); err != nil {
		fmt.Println("error writing", gitignoreFilename+":", err)
		os.Exit(1)
	}

	fmt.Println("initialized layerkit repo:")
	fmt.Println("  " + helloSpecFilename)
	fmt.Println("  " + readmeFilename)
	fmt.Println("  " + gitignoreFilename)
	fmt.Println("  " + settingsJsonFilename)
	fmt.Println()
	fmt.Println("next: layerkit build && layerkit serve")
}

// alreadyInitialized reports whether cwd already looks like a layerkit repo.
func alreadyInitialized(cwd string) bool {
	if _, err := os.Stat(filepath.Join(cwd, ".layerkit")); err == nil {
		return true
	}
	matches, _ := filepath.Glob(filepath.Join(cwd, "*.tool.yaml"))
	return len(matches) > 0
}

// writeEmbeddedFile copies an embedded template file to dst.
func writeEmbeddedFile(fsys embed.FS, src, dst string) error {
	data, err := fsys.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
