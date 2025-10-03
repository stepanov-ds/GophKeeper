package main

import (
	"fmt"
	"os"

	"github.com/stepanov-ds/GophKeeper/internal/cli/commands"
)

func main() {
	if err := commands.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

