// Package config reads the hub's YAML file and resolves it into the per-node
// configuration that the ingest response delivers (ADR 0010).
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

// minTokenLength keeps the security of ADR 0007 rule 5 on the token, not on obscurity.
const minTokenLength = 32

// Sensor is one sensor as the agent receives it.
type Sensor struct {
	Enabled  bool
	Interval time.Duration
}

// Agent is the flat configuration an agent applies: the hub resolves the layers, the
// agent merges nothing.
type Agent struct {
	BaseTick    time.Duration
	Filesystems []string
	// SkipMounts are mount-point prefixes the disk sensor leaves alone.
	SkipMounts []string
	Sensors    map[string]Sensor
}

// Node is everything the hub knows about one node.
type Node struct {
	Name         string
	Class        string
	Token        string
	SilenceAfter time.Duration
	Agent        Agent
	// Version identifies Agent, not the node: identical configurations share it.
	Version string
}

// Config is the resolved file: one entry per listed node.
type Config struct {
	nodes map[string]Node
}

// Load reads the file at path, validates it and resolves every listed node. Deployment
// settings have no defaults, so an incomplete file is an error rather than a fallback.
func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}

	var f file
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(f.Nodes) == 0 {
		return nil, fmt.Errorf("%s: nodes is missing or empty, so the hub would serve nobody", path)
	}
	if err := validate(f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	nodes := make(map[string]Node, len(f.Nodes))
	owner := make(map[string]string, len(f.Nodes))
	for _, name := range sorted(f.Nodes) {
		entry := f.Nodes[name]
		if err := claimToken(owner, name, entry.TokenEnv); err != nil {
			return nil, err
		}
		node, err := resolve(f, name, entry)
		if err != nil {
			return nil, err
		}
		nodes[name] = node
	}
	return &Config{nodes: nodes}, nil
}

// Node returns the resolved configuration of one node.
func (c *Config) Node(name string) (Node, bool) {
	node, ok := c.nodes[name]
	return node, ok
}

// Nodes returns every configured node, ordered by name.
func (c *Config) Nodes() []Node {
	out := make([]Node, 0, len(c.nodes))
	for _, name := range sorted(c.nodes) {
		out = append(out, c.nodes[name])
	}
	return out
}

// claimToken reads the node's token from its environment variable, refusing a variable
// two nodes share, one that is unset, and one holding a token too short to be a secret.
func claimToken(owner map[string]string, node, env string) error {
	if env == "" {
		return fmt.Errorf("node %s: token_env is required and has no default", node)
	}
	if other, taken := owner[env]; taken {
		return fmt.Errorf("nodes %s and %s share the token variable %s", other, node, env)
	}
	owner[env] = node
	return nil
}

func token(env string) (string, error) {
	value := os.Getenv(env)
	switch {
	case value == "":
		return "", fmt.Errorf("%s is unset: the node's token has no default", env)
	case len(value) < minTokenLength:
		return "", fmt.Errorf("%s holds fewer than %d characters", env, minTokenLength)
	}
	return value, nil
}

func sorted[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
