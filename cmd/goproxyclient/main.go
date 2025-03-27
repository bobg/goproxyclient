package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bobg/errors"
	"github.com/bobg/subcmd/v2"
	"golang.org/x/mod/semver"

	"github.com/bobg/goproxyclient"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	goproxy := os.Getenv("GOPROXY")
	if goproxy == "" {
		goproxy = "https://proxy.golang.org"
	}

	flag.StringVar(&goproxy, "proxy", goproxy, "Go module proxy URL")
	flag.Parse()

	cl := goproxyclient.New(goproxy, nil)

	return subcmd.Run(context.Background(), maincmd{cl: cl}, flag.Args())
}

type maincmd struct {
	cl goproxyclient.Client
}

func (c maincmd) Subcmds() subcmd.Map {
	return subcmd.Commands(
		"info", c.info, "get module info", nil,
		"latest", c.latest, "get the latest module version", nil,
		"list", c.list, "list module versions", nil,
		"mod", c.mod, "get the go.mod file for a module", nil,
		"zip", c.zip, "get the zip file for a module", nil,
	)
}

func (c maincmd) info(ctx context.Context, args []string) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	for _, arg := range args {
		parts := strings.Split(arg, "@")
		if len(parts) != 2 {
			return fmt.Errorf("argument %s is not in MODULE@VERSION form", arg)
		}
		_, _, m, err := c.cl.Info(ctx, parts[0], parts[1])
		if err != nil {
			return errors.Wrapf(err, "getting info for %s", arg)
		}
		if err := enc.Encode(m); err != nil {
			return errors.Wrapf(err, "encoding info for %s", arg)
		}
	}

	return nil
}

func (c maincmd) latest(ctx context.Context, args []string) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	for _, arg := range args {
		_, _, m, err := c.cl.Latest(ctx, arg)
		if err != nil {
			return errors.Wrapf(err, "getting latest info for %s", arg)
		}
		if err := enc.Encode(m); err != nil {
			return errors.Wrapf(err, "encoding latest info for %s", arg)
		}
	}

	return nil
}

func (c maincmd) list(ctx context.Context, args []string) error {
	for _, arg := range args {
		versions, err := c.cl.List(ctx, arg)
		if err != nil {
			return errors.Wrapf(err, "getting versions for %s", arg)
		}
		semver.Sort(versions)

		if len(args) > 1 {
			fmt.Printf("%s:\n", arg)
		}

		for _, v := range versions {
			if len(args) > 1 {
				fmt.Print("  ")
			}
			fmt.Println(v)
		}
	}

	return nil
}

func (c maincmd) mod(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("exactly one argument is required")
	}
	parts := strings.Split(args[0], "@")
	if len(parts) != 2 {
		return fmt.Errorf("argument %s is not in MODULE@VERSION form", args[0])
	}
	mod, err := c.cl.Mod(ctx, parts[0], parts[1])
	if err != nil {
		return errors.Wrapf(err, "getting mod file for %s", args[0])
	}
	defer mod.Close()

	_, err = io.Copy(os.Stdout, mod)
	return errors.Wrapf(err, "writing mod file for %s", args[0])
}

func (c maincmd) zip(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("exactly one argument is required")
	}
	parts := strings.Split(args[0], "@")
	if len(parts) != 2 {
		return fmt.Errorf("argument %s is not in MODULE@VERSION form", args[0])
	}
	z, err := c.cl.Zip(ctx, parts[0], parts[1])
	if err != nil {
		return errors.Wrapf(err, "getting zip file for %s", args[0])
	}
	defer z.Close()

	_, err = io.Copy(os.Stdout, z)
	return errors.Wrapf(err, "writing zip file for %s", args[0])
}
