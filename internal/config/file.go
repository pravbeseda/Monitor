package config

// The YAML file as written, before any layer is applied. Durations stay strings here so
// that a malformed one is reported against its key rather than as a parse failure.

type file struct {
	BaseTick    string                `yaml:"base_tick"`
	Filesystems []string              `yaml:"filesystems"`
	Sensors     map[string]fileSensor `yaml:"sensors"`
	Classes     map[string]fileClass  `yaml:"classes"`
	Nodes       map[string]fileNode   `yaml:"nodes"`
}

type fileSensor struct {
	Enabled  *bool  `yaml:"enabled"`
	Interval string `yaml:"interval"`
}

type fileClass struct {
	Profile      []string              `yaml:"profile"`
	SilenceAfter string                `yaml:"silence_after"`
	BaseTick     string                `yaml:"base_tick"`
	Filesystems  []string              `yaml:"filesystems"`
	Sensors      map[string]fileSensor `yaml:"sensors"`
}

type fileNode struct {
	Class       string                `yaml:"class"`
	TokenEnv    string                `yaml:"token_env"`
	BaseTick    string                `yaml:"base_tick"`
	Filesystems []string              `yaml:"filesystems"`
	Sensors     map[string]fileSensor `yaml:"sensors"`
}
