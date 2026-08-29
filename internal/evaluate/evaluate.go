// Package evaluate turns stored measurements into meaning: every subject gets a level, a
// change of level becomes an event, and events become notifications
// (docs/specs/evaluation.md).
package evaluate

import "sort"

// Definition is what a rule reads and what it defaults to. Naming the sensor is what makes
// staleness computable: the metric ids alone do not identify one.
type Definition struct {
	// Sensor is the sensor whose interval a subject of this rule ages against.
	Sensor string
	// Free and Pct are the metric ids of the absolute and the proportional series a rule
	// reads together: the threshold model of ADR 0012 needs both.
	Free string
	Pct  string
	// Default is the product default of the rule; Backup is the default for a volume
	// declared role: backup, which keeps headroom and drops percentages.
	Default Rule
	Backup  Rule
}

// definitions holds every rule a configuration file may carry. `silence` is not here: it
// has no thresholds and nothing to configure, only a window that belongs to a node class.
var definitions = map[string]Definition{
	"disk": {
		Sensor:  "disk",
		Free:    "disk.free_bytes",
		Pct:     "disk.free_pct",
		Default: defaultDiskRule(),
		Backup:  defaultBackupRule(),
	},
}

// Names lists the rules the hub implements, in order. The hub's configuration resolves a
// rule for each of them, whether or not the file mentions any.
func Names() []string {
	names := make([]string, 0, len(definitions))
	for name := range definitions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Lookup returns the definition of one rule, and reports whether the hub implements it at
// all: a configuration naming a rule nobody reads is a startup error.
func Lookup(name string) (Definition, bool) {
	found, ok := definitions[name]
	return found, ok
}
