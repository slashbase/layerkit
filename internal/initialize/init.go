package initialize

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed templates
var templates embed.FS

const (
	helloSpecFilename    = "hello.tool.yaml"
	readmeFilename       = "README.md"
	gitignoreFilename    = ".gitignore"
	settingsJsonFilename = "settings.json"
)

// scaffold is one embedded template and the file it becomes in the target
// directory.
type scaffold struct {
	src  string // path inside the embedded templates directory
	name string // file name written into the target directory
}

// StartLayerkitInit scaffolds a new tool-spec repo in the current directory.
//
// A file that already exists is left alone and reported rather than replaced:
// init writes ordinary project files, and a README.md or .gitignore may already
// hold the user's own content.
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

	scaffolds := []scaffold{
		{src: "templates/hello.tool.yaml", name: helloSpecFilename},
		{src: "templates/settings.json", name: settingsJsonFilename},
		{src: "templates/README.md", name: readmeFilename},
		{src: "templates/gitignore", name: gitignoreFilename},
	}

	var created, skipped []string
	for _, s := range scaffolds {
		dst := filepath.Join(cwd, s.name)

		if _, err := os.Stat(dst); err == nil {
			skipped = append(skipped, s.name)
			continue
		} else if !os.IsNotExist(err) {
			fmt.Println("error checking "+s.name+":", err)
			os.Exit(1)
		}

		if err := writeEmbeddedFile(s.src, dst); err != nil {
			fmt.Println("error writing "+s.name+":", err)
			os.Exit(1)
		}
		created = append(created, s.name)
	}

	if len(created) > 0 {
		fmt.Println("initialized layerkit repo:")
		for _, name := range created {
			fmt.Println("  " + name)
		}
	}
	if len(skipped) > 0 {
		fmt.Println("left existing files untouched:")
		for _, name := range skipped {
			fmt.Println("  " + name)
		}
	}
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
func writeEmbeddedFile(src, dst string) error {
	data, err := templates.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
