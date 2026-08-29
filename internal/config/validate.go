package config

import "fmt"

// validate checks the whole file rather than the part some node happens to reference: a
// class or a sensor default nobody uses yet still has to be right, or the typo surfaces
// days later, on a running hub, the moment it is wired to a node.
func validate(f file) error {
	if err := validateLayer("", f.BaseTick, f.Filesystems, f.Sensors); err != nil {
		return err
	}

	for _, name := range sorted(f.Classes) {
		class := f.Classes[name]
		where := fmt.Sprintf("class %s: ", name)
		if err := validateLayer(where, class.BaseTick, class.Filesystems, class.Sensors); err != nil {
			return err
		}
		if _, compiledIn := defaultClasses[name]; !compiledIn && class.SilenceAfter == "" {
			return fmt.Errorf("%ssilence_after is required of a class the file introduces", where)
		}
		if class.SilenceAfter != "" {
			if _, err := duration(where+"silence_after", class.SilenceAfter); err != nil {
				return err
			}
		}
	}

	for _, name := range sorted(f.Nodes) {
		node := f.Nodes[name]
		if err := validateLayer(fmt.Sprintf("node %s: ", name), node.BaseTick, node.Filesystems, node.Sensors); err != nil {
			return err
		}
	}
	return nil
}

// validateLayer checks what every layer may set: a tick, an allow-list and sensor
// intervals. An absent value is fine; a present one has to be usable.
func validateLayer(where, baseTick string, filesystems []string, sensors map[string]fileSensor) error {
	if baseTick != "" {
		if _, err := duration(where+"base_tick", baseTick); err != nil {
			return err
		}
	}
	if filesystems != nil && len(filesystems) == 0 {
		return fmt.Errorf("%sfilesystems is empty, so no volume would be collected", where)
	}
	for _, name := range sorted(sensors) {
		if interval := sensors[name].Interval; interval != "" {
			if _, err := duration(fmt.Sprintf("%ssensor %s: interval", where, name), interval); err != nil {
				return err
			}
		}
	}
	return nil
}
