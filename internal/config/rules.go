package config

import (
	"fmt"

	"github.com/pravbeseda/monitor/internal/evaluate"
)

// roleBackup is the only role a volume can declare: it sits nearly full by design, so it
// keeps absolute headroom and drops percentages (ADR 0012).
const roleBackup = "backup"

// resolveRules returns the rules a node judges its volumes by: its own, and one set for
// each volume the file names, resolved from the branch that volume's role selects.
func resolveRules(f file, custom fileClass, entry fileNode) (map[string]evaluate.Rule, map[string]map[string]evaluate.Rule, error) {
	layers := []map[string]fileRule{f.Rules, custom.Rules, entry.Rules}
	plain, backup, err := resolveBranches(layers)
	if err != nil {
		return nil, nil, err
	}
	for _, name := range sorted(plain) {
		if err := checkRule("rules."+name, plain[name]); err != nil {
			return nil, nil, err
		}
	}

	volumes := make(map[string]map[string]evaluate.Rule, len(entry.Volumes))
	for _, mount := range sorted(entry.Volumes) {
		declared := entry.Volumes[mount]
		base := plain
		if declared.Role == roleBackup {
			base = backup
		}
		resolved, err := resolveVolume(mount, base, declared.Rules)
		if err != nil {
			return nil, nil, err
		}
		volumes[mount] = resolved
	}
	return plain, volumes, nil
}

// resolveVolume applies what one volume says onto the branch its role selected.
func resolveVolume(mount string, base map[string]evaluate.Rule, declared map[string]fileRule) (map[string]evaluate.Rule, error) {
	out := make(map[string]evaluate.Rule, len(base))
	for _, name := range sorted(base) {
		where := fmt.Sprintf("volume %q: rules.%s", mount, name)
		merged, err := mergeRule(where, base[name], name, []map[string]fileRule{declared}, false)
		if err != nil {
			return nil, err
		}
		if err := checkRule(where, merged); err != nil {
			return nil, err
		}
		out[name] = merged
	}
	return out, nil
}

// resolveBranches applies the rule layers, most specific last, onto the product defaults of
// every rule the hub implements — including the rules no layer mentions, which is how a
// file that says nothing still judges a volume. The two branches are resolved apart: an
// absent ratio under `backup` stays absent instead of being inherited from the rule beside
// it.
func resolveBranches(layers []map[string]fileRule) (plain, backup map[string]evaluate.Rule, err error) {
	plain, backup = map[string]evaluate.Rule{}, map[string]evaluate.Rule{}
	for _, name := range evaluate.Names() {
		definition, _ := evaluate.Lookup(name)
		key := "rules." + name
		if plain[name], err = mergeRule(key, definition.Default, name, layers, false); err != nil {
			return nil, nil, err
		}
		if backup[name], err = mergeRule(key+".backup", definition.Backup, name, layers, true); err != nil {
			return nil, nil, err
		}
	}
	return plain, backup, nil
}

// mergeRule folds the layers field by field and converts once, the way the sensor layers
// are folded: a size a more specific layer replaces is never parsed, so a typo that lost
// cannot decide the message.
func mergeRule(where string, base evaluate.Rule, name string, layers []map[string]fileRule, backup bool) (evaluate.Rule, error) {
	var warning, critical []fileThreshold
	for _, layer := range layers {
		declared, ok := layer[name]
		switch {
		case !ok:
			continue
		case backup && declared.Backup != nil:
			warning = append(warning, declared.Backup.Warning)
			critical = append(critical, declared.Backup.Critical)
		case !backup:
			warning = append(warning, declared.Warning)
			critical = append(critical, declared.Critical)
		}
	}

	out := base
	var err error
	if out.Warning, err = threshold(where+".warning", base.Warning, lastThreshold(warning...)); err != nil {
		return evaluate.Rule{}, err
	}
	if out.Critical, err = threshold(where+".critical", base.Critical, lastThreshold(critical...)); err != nil {
		return evaluate.Rule{}, err
	}
	return out, nil
}

// lastThreshold takes each field from the most specific layer that sets one.
func lastThreshold(layers ...fileThreshold) fileThreshold {
	var out fileThreshold
	for _, layer := range layers {
		if layer.Floor != "" {
			out.Floor = layer.Floor
		}
		if layer.Ratio != nil {
			out.Ratio = layer.Ratio
		}
		if layer.Ceiling != "" {
			out.Ceiling = layer.Ceiling
		}
	}
	return out
}

// threshold turns what the layers settled on into numbers, and refuses half a band: a
// ceiling with no ratio would be read as no band at all and a ratio with no ceiling would
// be ignored, either way in silence.
func threshold(where string, base evaluate.Threshold, declared fileThreshold) (evaluate.Threshold, error) {
	out := base
	if declared.Floor != "" {
		floor, err := sizeValue(where+".floor", declared.Floor)
		if err != nil {
			return evaluate.Threshold{}, err
		}
		out.Floor = floor
	}
	if declared.Ratio != nil {
		if err := checkRatio(where+".ratio", *declared.Ratio); err != nil {
			return evaluate.Threshold{}, err
		}
		out.Ratio = *declared.Ratio
	}
	if declared.Ceiling != "" {
		ceiling, err := sizeValue(where+".ceiling", declared.Ceiling)
		if err != nil {
			return evaluate.Threshold{}, err
		}
		out.Ceiling = ceiling
	}
	if (out.Ratio > 0) != (out.Ceiling > 0) {
		return evaluate.Threshold{}, fmt.Errorf("%s: a band needs both ratio and ceiling, and half of one would be ignored in silence", where)
	}
	return out, nil
}

// checkRule enforces what only a finished rule can be judged on, which is why it runs on
// the resolved rule of a node and of every volume rather than on a layer: a layer above
// may raise the very value that makes the rule consistent.
func checkRule(where string, rule evaluate.Rule) error {
	fields := []struct {
		name              string
		critical, warning float64
	}{
		{"floor", rule.Critical.Floor, rule.Warning.Floor},
		{"ratio", rule.Critical.Ratio, rule.Warning.Ratio},
		{"ceiling", rule.Critical.Ceiling, rule.Warning.Ceiling},
	}
	for _, field := range fields {
		if field.critical > field.warning {
			return fmt.Errorf("%s: the critical %s %g is above the warning %g, so a volume could leave warning while critical holds",
				where, field.name, field.critical, field.warning)
		}
	}
	return nil
}
