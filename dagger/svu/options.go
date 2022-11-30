package svu

import "github.com/mesosphere/daggers/daggers"

// Command is represents the svu sub-command.
type Command string

const (
	// CommandNext is the svu next sub-command.
	CommandNext Command = "next"
	// CommandMajor is the svu major sub-command.
	CommandMajor Command = "major"
	// CommandMinor is the svu minor sub-command.
	CommandMinor Command = "minor"
	// CommandPatch is the svu patch sub-command.
	CommandPatch Command = "patch"
	// CommandCurrent is the svu pre sub-command.
	CommandCurrent Command = "current"
)

// TagMode is a custom type representing the possible values for the --tag-mode flag.
type TagMode string

const (
	// TagModeAllBranches is the value for the --tag-mode flag that will use all branches.
	TagModeAllBranches TagMode = "all-branches"
	// TagModeCurrentBranch is the value for the --tag-mode flag that will only use current branch tags.
	TagModeCurrentBranch TagMode = "current-branch"
)

type Config struct {
	Version    string `env:"SVU_VERSION" envDefault:"v1.9.0"`
	Metadata   bool   `env:"SVU_METADATA" envDefault:"true"`
	Prerelease bool   `env:"SVU_PRERELEASE" envDefault:"true"`
	Build      bool   `env:"SVU_BUILD" envDefault:"true"`
	Command    string `env:"SVU_COMMAND" envDefault:"next"`
	Pattern    string `env:"SVU_PATTERN"`
	Prefix     string `env:"SVU_PREFIX"`
	Suffix     string `env:"SVU_SUFFIX"`
	TagMode    string `env:"SVU_TAG_MODE" envDefault:"all-branches"`
}

// SVUVersion specifies the version of svu to use. Defaults to v1.9.0. This should be one of the
// released image tags - see https://github.com/caarlos0/svu/pkgs/container/svu for available
// tags.
//
//nolint:revive // Disable stuttering check.
func SVUVersion(v string) daggers.Option[Config] {
	return func(c Config) Config {
		c.Version = v
		return c
	}
}

// WithMetadata controls whether to include pre-release and build metadata in the version. Defaults to true.
func WithMetadata(b bool) daggers.Option[Config] {
	return func(c Config) Config {
		c.Metadata = b
		return c
	}
}

// WithPreRelease controls whether to include pre-release metadata in the version. Defaults to true.
func WithPreRelease(b bool) daggers.Option[Config] {
	return func(c Config) Config {
		c.Prerelease = b
		return c
	}
}

// WithBuild controls whether to include build metadata in the version. Defaults to true.
func WithBuild(b bool) daggers.Option[Config] {
	return func(c Config) Config {
		c.Build = b
		return c
	}
}

// WithCommand sets the svu sub-command to run. Defaults to "next".
func WithCommand(cmd Command) daggers.Option[Config] {
	return func(c Config) Config {
		c.Command = string(cmd)
		return c
	}
}

// WithPattern sets the pattern to use when searching for tags. Defaults to "*".
func WithPattern(pattern string) daggers.Option[Config] {
	return func(c Config) Config {
		c.Pattern = pattern
		return c
	}
}

// WithPrefix sets the prefix to use when searching for tags. Defaults to "v".
func WithPrefix(prefix string) daggers.Option[Config] {
	return func(c Config) Config {
		c.Prefix = prefix
		return c
	}
}

// WithSuffix sets the suffix to use when searching for tags. Defaults to "".
func WithSuffix(suffix string) daggers.Option[Config] {
	return func(c Config) Config {
		c.Suffix = suffix
		return c
	}
}

// WithTagMode sets the tag mode to use when searching for tags. Defaults to TagModeAllBranches.
func WithTagMode(tagMode TagMode) daggers.Option[Config] {
	return func(c Config) Config {
		c.TagMode = string(tagMode)
		return c
	}
}
