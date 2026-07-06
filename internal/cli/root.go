package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/alecthomas/kong"
)

type CLI struct {
	Version  VersionCmd  `cmd:"" help:"Print the pgkit version and exit."`
	Generate GenerateCmd `cmd:"" help:"Generate deterministic PostgreSQL SQL from a Go schema package."`
	GlobalFlags
}

type GlobalFlags struct {
	outW io.Writer
	errW io.Writer
	Root string `help:"Project root (default: current directory)." short:"r" default:""`
}

func (g *GlobalFlags) stdout() io.Writer {
	if g.outW != nil {
		return g.outW
	}
	return os.Stdout
}

func (g *GlobalFlags) stderr() io.Writer {
	if g.errW != nil {
		return g.errW
	}
	return os.Stderr
}

type ExitError struct {
	Err  error
	Code int
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

func Exit(code int, err error) error {
	return &ExitError{Code: code, Err: err}
}

type ExitPanic struct {
	Code int
}

func (e *ExitPanic) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

func Run(args []string) (code int, err error) {
	defer func() {
		if r := recover(); r != nil {
			ep, ok := r.(*ExitPanic)
			if !ok {
				panic(r)
			}
			if ep.Code == 0 {
				code = 0
			} else {
				code = 2
			}
			err = nil
		}
	}()
	if len(args) == 0 {
		return runHelp()
	}
	return run(args, nil, nil)
}

func run(args []string, stdout, stderr io.Writer) (int, error) {
	var cli CLI
	cli.outW = stdout
	cli.errW = stderr

	parser, err := kong.New(
		&cli,
		kong.Name("pgkit"),
		kong.Description("Generate deterministic PostgreSQL schema SQL from Go definitions."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:  true,
			Indenter: kong.SpaceIndenter,
			Summary:  true,
		}),
		kong.Exit(func(code int) { panic(&ExitPanic{Code: code}) }),
	)
	if err != nil {
		return 2, fmt.Errorf("init cli parser: %w", err)
	}
	kctx, err := parser.Parse(args)
	if err != nil {
		parser.FatalIfErrorf(err)
		panic(&ExitPanic{Code: 2})
	}
	if err := kctx.Run(&cli.GlobalFlags); err != nil {
		var exitErr *ExitError
		if !errors.As(err, &exitErr) {
			exitErr = &ExitError{Code: 1, Err: err}
		}
		fmt.Fprintf(cli.stderr(), "pgkit: error: %s\n", exitErr.Err)
		return exitErr.Code, exitErr.Err
	}
	return 0, nil
}

func runHelp() (int, error) {
	var cli CLI
	parser, err := kong.New(
		&cli,
		kong.Name("pgkit"),
		kong.Description("Generate deterministic PostgreSQL schema SQL from Go definitions."),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:  true,
			Indenter: kong.SpaceIndenter,
			Summary:  true,
		}),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		return 2, err
	}
	_, _ = parser.Parse([]string{"--help"})
	return 0, nil
}
