package config

import "fmt"

// validate checks the whole file rather than the part some node happens to reference: a
// class or a sensor default nobody uses yet still has to be right, or the typo surfaces
// days later, on a running hub, the moment it is wired to a node.
func validate(f file) error {
	if err := validateLayer(f, "", f.BaseTick, f.Filesystems, f.Sensors); err != nil {
		return err
	}

	for _, name := range classNames(f) {
		if err := validateClass(f, name); err != nil {
			return err
		}
	}

	for _, name := range sorted(f.Nodes) {
		node := f.Nodes[name]
		if err := validateLayer(f, fmt.Sprintf("node %s: ", name), node.BaseTick, node.Filesystems, node.Sensors); err != nil {
			return err
		}
	}
	return nil
}

// validateClass checks a class whether or not a node uses it, down to the sensors its
// profile promises: a typo in a sensor name is what this catches, and it is the likeliest
// typo in the file.
func validateClass(f file, name string) error {
	builtin, custom, _ := classLayers(f, name)
	where := fmt.Sprintf("class %s: ", name)

	if err := validateLayer(f, where, custom.BaseTick, custom.Filesystems, custom.Sensors); err != nil {
		return err
	}
	if _, compiledIn := defaultClasses[name]; !compiledIn && custom.SilenceAfter == "" {
		return fmt.Errorf("%ssilence_after is required of a class the file introduces", where)
	}
	silenceAfter := last(builtin.SilenceAfter, custom.SilenceAfter)
	if silenceAfter != "" {
		if _, err := duration(where+"silence_after", silenceAfter); err != nil {
			return err
		}
	}

	// The node layer is missing here on purpose: a node adds its own sensors and is
	// checked when it is resolved. What the class alone promises has to hold already.
	baseTick, err := duration(where+"base_tick", last(defaultBaseTick, f.BaseTick, custom.BaseTick))
	if err != nil {
		return err
	}
	profile := lastList(builtin.Profile, custom.Profile)
	if _, err := resolveSensors(profile, baseTick,
		defaultSensors, builtin.Sensors, f.Sensors, custom.Sensors); err != nil {
		return fmt.Errorf("%s%w", where, err)
	}
	return nil
}

// classNames is every class the hub knows: the compiled-in ones and the file's own.
func classNames(f file) []string {
	names := map[string]bool{}
	for name := range defaultClasses {
		names[name] = true
	}
	for name := range f.Classes {
		names[name] = true
	}
	return sorted(names)
}

// validateLayer checks what every layer may set: a tick, an allow-list and sensor
// intervals. An absent value is fine; a present one has to be usable at this layer, which
// includes collecting no faster than the tick this layer sees.
func validateLayer(f file, where, baseTick string, filesystems []string, sensors map[string]fileSensor) error {
	tick, err := duration(where+"base_tick", last(defaultBaseTick, f.BaseTick, baseTick))
	if err != nil {
		return err
	}
	if filesystems != nil && len(filesystems) == 0 {
		return fmt.Errorf("%sfilesystems is empty, so no volume would be collected", where)
	}
	for _, name := range sorted(sensors) {
		declared := sensors[name].Interval
		if declared == "" {
			continue
		}
		key := fmt.Sprintf("%ssensor %s: interval", where, name)
		interval, err := duration(key, declared)
		if err != nil {
			return err
		}
		if interval < tick {
			return fmt.Errorf("%s: %v is below the base tick %v", key, interval, tick)
		}
	}
	return nil
}
