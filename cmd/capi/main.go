package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/davidnix/capi/internal/capicmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)

	cmd := capicmd.NewRootCommand()
	cmd.SetArgs(os.Args[1:])
	err := cmd.ExecuteContext(ctx)
	stop()
	if err != nil {
		if _, printErr := fmt.Fprintln(os.Stderr, err); printErr != nil {
			os.Exit(1)
		}
		os.Exit(1)
	}
}
