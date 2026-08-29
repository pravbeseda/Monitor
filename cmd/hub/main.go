// Command hub receives measurements, stores them and serves the web page.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/pravbeseda/monitor/internal/config"
	"github.com/pravbeseda/monitor/internal/hub"
	"github.com/pravbeseda/monitor/internal/storage"
	"github.com/pravbeseda/monitor/internal/version"
)

// readHeaderTimeout keeps a stalled client from holding a connection open forever.
const readHeaderTimeout = 10 * time.Second

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "hub: %v\n", err)
		os.Exit(1)
	}
}

// options are the deployment settings the hub needs; only the listen address has a
// product default, because binding to localhost says nothing about an installation.
type options struct {
	config string
	db     string
	listen string
}

func run(args []string, out io.Writer) error {
	opts, err := parseFlags(args)
	if err != nil {
		return err
	}
	cfg, err := config.Load(opts.config)
	if err != nil {
		return err
	}
	store, err := storage.OpenSQLite(opts.db)
	if err != nil {
		return err
	}
	defer func() {
		if err := store.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "hub: %v\n", err)
		}
	}()

	if _, err := fmt.Fprintf(out, "monitor-hub %s listening on %s (nodes: %d)\n",
		version.Current, opts.listen, len(cfg.Nodes())); err != nil {
		return fmt.Errorf("write to stdout: %w", err)
	}

	server := &http.Server{
		Addr:              opts.listen,
		Handler:           hub.Routes(cfg, store, time.Now),
		ReadHeaderTimeout: readHeaderTimeout,
	}
	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("serve on %s: %w", opts.listen, err)
	}
	return nil
}

func parseFlags(args []string) (options, error) {
	flags := flag.NewFlagSet("hub", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var opts options
	flags.StringVar(&opts.config, "config", "", "path to the hub's YAML configuration")
	flags.StringVar(&opts.db, "db", "", "path to the SQLite database")
	flags.StringVar(&opts.listen, "listen", "127.0.0.1:8080", "address to serve on")
	if err := flags.Parse(args); err != nil {
		return options{}, fmt.Errorf("parse flags: %w", err)
	}
	if opts.config == "" {
		return options{}, errors.New("--config is required: the configuration path has no default")
	}
	if opts.db == "" {
		return options{}, errors.New("--db is required: the database path has no default")
	}
	return opts, nil
}
