package main

import (
	"os"

	"github.com/wow-look-at-my/json-validator/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
