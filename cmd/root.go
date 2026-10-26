package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "layerkit",
	Short: "Simple spec-first compiler that turns YAML tool specs into MCP servers. Built on layerengine.",
	Long: `layerkit is a simple spec-first compiler that turns YAML tool specs into
MCP (Model Context Protocol) servers. Built on layerengine.

Author a *.tool.yaml spec describing a flow of layers (name, description,
input, output). layerkit generates the Lua code for each layer using LLMs,
compiles it in a runtime, and exposes the flow as a callable MCP tool.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.layerkit.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
