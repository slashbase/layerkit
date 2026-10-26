package main

import (
	"github.com/slashbase/layerkit/cmd"
	"github.com/slashbase/layerkit/internal/common/config"
)

func init() {
	config.InitEnv()
}

func main() {
	cmd.Execute()
}
