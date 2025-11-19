package main

import (
	"os"

	"github.com/beekhof/jira-tool/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
