// Command hub receives measurements, stores them and serves the web page.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/pravbeseda/Monitor/internal/config"
	"github.com/pravbeseda/Monitor/internal/version"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "hub: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	path, err := configPath(args)
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "monitor-hub %s: %d nodes configured\n", version.Current, len(cfg.Nodes())); err != nil {
		return fmt.Errorf("write to stdout: %w", err)
	}
	return nil
}

// configPath reads --config, which is a deployment setting and therefore has no default.
func configPath(args []string) (string, error) {
	flags := flag.NewFlagSet("hub", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	path := flags.String("config", "", "path to the hub's YAML configuration")
	if err := flags.Parse(args); err != nil {
		return "", fmt.Errorf("parse flags: %w", err)
	}
	if *path == "" {
		return "", errors.New("--config is required: the configuration path has no default")
	}
	return *path, nil
}
