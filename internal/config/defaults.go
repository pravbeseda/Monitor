package config

// Product defaults: how the tool behaves out of the box. They disclose nothing about any
// installation, so they live in code, and every layer of the file overrides them
// (ADR 0007 rule 1).

const defaultBaseTick = "5m"

var defaultFilesystems = []string{"apfs", "ext4", "xfs", "btrfs", "zfs", "ntfs"}

var defaultSensors = map[string]fileSensor{
	"disk": {Interval: "15m"},
}

var defaultClasses = map[string]fileClass{
	"laptop": {
		Profile:      []string{"disk"},
		SilenceAfter: "48h",
		Sensors:      map[string]fileSensor{"disk": {Interval: "1h"}},
	},
	"server": {
		Profile:      []string{"disk"},
		SilenceAfter: "10m",
	},
}
