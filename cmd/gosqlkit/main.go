package main

import (
	"fmt"
	"os"

	"github.com/webdeveloperben/gosqlkit/internal/cli"
)

func main() {
	code := run(os.Args[1:])
	os.Exit(code)
}

func run(args []string) (code int) {
	defer func() {
		if r := recover(); r != nil {
			if ep, ok := r.(*cli.ExitPanic); ok {
				if ep.Code != 0 {
					code = 2
				} else {
					code = 0
				}
				return
			}
			fmt.Fprintf(os.Stderr, "gosqlkit: panic: %v\n", r)
			code = 1
		}
	}()
	code, _ = cli.Run(args)
	return code
}
