// Command agent collects measurements on a node and pushes them to the hub.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pravbeseda/Monitor/internal/agent"
	"github.com/pravbeseda/Monitor/internal/sensor"
	"github.com/pravbeseda/Monitor/internal/sensor/disk"
	"github.com/pravbeseda/Monitor/internal/version"
)

// tokenVariable holds the node's token: a secret never lives in a file in the tree.
const tokenVariable = "MONITOR_TOKEN"

// requestTimeout keeps one unanswered request from swallowing a whole tick.
const requestTimeout = 30 * time.Second

func main() {
	if err := start(); err != nil {
		fmt.Fprintf(os.Stderr, "agent: %v\n", err)
		os.Exit(1)
	}
}

// start owns the signal context, so that main holds nothing that os.Exit would skip.
func start() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

// options are everything a node holds locally (ADR 0010): nothing else has a default,
// because everything else comes from the hub.
type options struct {
	hub   string
	node  string
	token string
}

func run(ctx context.Context, args []string, out io.Writer) error {
	opts, err := settings(args)
	if err != nil {
		return err
	}

	// The sensor reads the allow-list the hub last delivered, so it closes over the agent
	// that is built from it.
	var running *agent.Agent
	volumes := disk.New(disk.System(), func() disk.Settings {
		return disk.Settings{Filesystems: running.Filesystems(), SkipMounts: running.SkipMounts()}
	}, time.Now)
	running = agent.New(agent.Options{
		Node:    opts.node,
		Sensors: []sensor.Sensor{volumes},
		Client:  agent.NewHTTPClient(opts.hub, opts.token, requestTimeout),
		Now:     time.Now,
	})

	if _, err := fmt.Fprintf(out, "monitor-agent %s: node %s reporting to %s\n",
		version.Current, opts.node, opts.hub); err != nil {
		return fmt.Errorf("write to stdout: %w", err)
	}
	return running.Run(ctx)
}

func settings(args []string) (options, error) {
	flags := flag.NewFlagSet("agent", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var opts options
	flags.StringVar(&opts.hub, "hub", "", "base URL of the hub")
	flags.StringVar(&opts.node, "node", "", "this node's name, as the hub knows it")
	if err := flags.Parse(args); err != nil {
		return options{}, fmt.Errorf("parse flags: %w", err)
	}
	if opts.hub == "" {
		return options{}, errors.New("--hub is required: the hub URL has no default")
	}
	if opts.node == "" {
		return options{}, errors.New("--node is required: it must match the name the hub's token belongs to")
	}
	if opts.token = os.Getenv(tokenVariable); opts.token == "" {
		return options{}, fmt.Errorf("%s is unset: the node's token has no default", tokenVariable)
	}
	return opts, nil
}
