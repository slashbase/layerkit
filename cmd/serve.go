package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/slashbase/layerkit/internal/serve"
	"github.com/spf13/cobra"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run an MCP server exposing built flows as tools",
	Long:  `Run an MCP server exposing built flows as tools`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := cmd.Flags().GetString("path")
		if err != nil {
			return fmt.Errorf("failed to read --path flag: %w", err)
		}

		if path != "" {
			absPath, err := validateLayerkitPath(path)
			if err != nil {
				return err
			}
			path = absPath
		}
		if path == "" {
			path, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("could not determine current directory: %v", err)
			}
		}

		serve.StartLayerkitServe(path)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringP("path", "p", "", "path to the layerkit directory")
}

// validateLayerkitPath checks that path exists and is a directory,
// and returns its absolute form.
func validateLayerkitPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("could not resolve path %q: %w", path, err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path %q does not exist", absPath)
		}
		return "", fmt.Errorf("could not access path %q: %w", absPath, err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("path %q is not a directory", absPath)
	}

	return absPath, nil
}
