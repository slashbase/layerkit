package config

import (
	"fmt"
	"os"

	"github.com/slashbase/layerengine"
	"github.com/slashbase/layerengine/codegen"
	leconfig "github.com/slashbase/layerengine/config"
)

// Options is the policy layerkit runs flows with: values are converted to the
// types the spec declares, because an MCP client driven by an LLM will stringify
// a number and failing the call teaches the model nothing. Arguments the flow
// does not declare are dropped and reported.
//
// The report goes to stderr, never stdout: on the stdio transport stdout is the
// MCP protocol channel.
func LayerEngineCoerceOptions() layerengine.CoerceOptions {
	return layerengine.CoerceOptions{
		OnUndeclared: func(name string) {
			fmt.Fprintf(os.Stderr, "warning: dropping undeclared argument %q\n", name)
		},
	}
}

func CodeGenFromEnv() (*codegen.CodeGen, codegen.ModelID, error) {

	cfg, err := leconfig.FromEnv()
	if err != nil {
		return nil, "", err
	}

	codegenerater, err := cfg.CodeGen()
	if err != nil {
		return nil, "", err
	}

	return codegenerater, cfg.Model, nil
}
