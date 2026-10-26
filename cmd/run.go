package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/slashbase/layerkit/internal/run"
	"github.com/spf13/cobra"
)

var inputsRaw string

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a tool built with layerkit",
	Long:  `Run a tool built with layerkit`,
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := cmd.LocalFlags().GetString("file")
		if err != nil || f == "" {
			cmd.Help()
			return err
		}
		var input map[string]any
		if err := json.Unmarshal([]byte(inputsRaw), &input); err != nil {
			return fmt.Errorf("--input must be JSON object: %w", err)
		}
		run.StartLayerkitRun(f, input)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringP("file", "f", "", "a spec file name to run, e.g. hello.tool.yaml")
	runCmd.Flags().StringVarP(&inputsRaw, "input", "i", "{}", "flow inputs as JSON")
}
