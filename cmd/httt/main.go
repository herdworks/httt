package main

import (
	"context"
	"fmt"
	"os"

	"github.com/herdworks/clic"
	"github.com/herdworks/httt/cli"
)

func main() {
	result := cli.CLI.Run(clic.Execution{
		Context:          context.Background(),
		Stdin:            os.Stdin,
		Stdout:           os.Stdout,
		Stderr:           os.Stderr,
		LookupEnv:        os.LookupEnv,
		WorkingDirectory: ".",
	}, os.Args[1:])
	if result.Error != nil {
		fmt.Fprintln(os.Stderr, result.Error)
	}
	os.Exit(result.ExitCode)
}
