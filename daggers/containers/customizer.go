package containers

import (
	"context"
	"fmt"
	"os"
	"strings"

	"dagger.io/dagger"

	"github.com/mesosphere/daggers/daggers"
)

// ContainerCustomizerFn is a function that customizes a container.
type ContainerCustomizerFn func(*daggers.Runtime, *dagger.Container) (*dagger.Container, error)

// AppendToPATH appends the given path to the PATH environment variable.
func AppendToPATH(ctx context.Context, path string) ContainerCustomizerFn {
	return func(_ *daggers.Runtime, c *dagger.Container) (*dagger.Container, error) {
		existingPATH, err := c.EnvVariable(ctx, "PATH")
		if err != nil {
			return nil, err
		}

		return c.WithEnvVariable("PATH", existingPATH+":"+path), nil
	}
}

// WithMountedGoCache mounts a cache volume for the container's GOCACHE and GOMODCACHE environment variables using
// the contents of the go.mod and go.sum files in the given path. If the path is empty, the current working directory
// is used.
func WithMountedGoCache(ctx context.Context, path string) ContainerCustomizerFn {
	return func(runtime *daggers.Runtime, c *dagger.Container) (*dagger.Container, error) {
		var (
			client     = runtime.Client()
			cacheFiles = []string{"go.mod", "go.sum"}
		)

		cacheDir, err := getGoCacheDir(ctx, runtime, path, cacheFiles)
		if err != nil {
			return nil, err
		}

		// Configure go to use the cache volume for the go build cache.
		buildCache, err := NewCacheVolumeWithFileHashKeys(ctx, client, "go-build-", cacheDir, cacheFiles...)
		if err != nil {
			return nil, err
		}

		c, _ = WithMountedCache(buildCache, "/go/.cache/build", "GOCACHE")(runtime, c)

		// Configure go to use the cache volume for the go mod cache.
		modCache, err := NewCacheVolumeWithFileHashKeys(ctx, client, "go-mod-", cacheDir, cacheFiles...)
		if err != nil {
			return nil, err
		}

		c, _ = WithMountedCache(modCache, "/go/.cache/mod", "GOMODCACHE")(runtime, c)

		return c, nil
	}
}

// WithMountedCache mounts the given cache volume at the given path in the container and if envVarName provided, set env
// variable with the cache mount path.
func WithMountedCache(cacheVol *dagger.CacheVolume, path, envVarName string) ContainerCustomizerFn {
	return func(runtime *daggers.Runtime, c *dagger.Container) (*dagger.Container, error) {
		c = c.WithMountedCache(path, cacheVol)

		if envVarName != "" {
			c = c.WithEnvVariable(envVarName, path)
		}

		return c, nil
	}
}

func getGoCacheDir(
	ctx context.Context, runtime *daggers.Runtime, path string, cacheFiles []string,
) (*dagger.Directory, error) {
	// Default to the current working directory if no path is given.
	if path == "" {
		path = "."
	}

	cacheDir := runtime.Client().Directory()

	for _, cacheFile := range cacheFiles {
		file := runtime.Workdir().Directory(path).File(cacheFile)

		if _, err := file.ID(ctx); err == nil {
			cacheDir = cacheDir.WithFile(path, file)
		}
	}

	// List the files in the cache directory and determine if they exist.
	entries, err := cacheDir.Entries(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cache files: %w", err)
	}

	if len(entries) != len(cacheFiles) {
		return nil, fmt.Errorf("missing cache files: %v", cacheFiles)
	}

	return cacheDir, nil
}

// InstallGo installs Go in the container using the given version. If the version is empty, the hardcoded "1.19.3" is
// used.
//
// The container must have the "curl" and "tar" binaries installed in order to install Go.
func InstallGo(ctx context.Context, version string) ContainerCustomizerFn {
	return func(runtime *daggers.Runtime, c *dagger.Container) (*dagger.Container, error) {
		// If no version is given, default to 1.19.3.
		if version == "" {
			version = "1.19.3"
		}

		var (
			url = fmt.Sprintf("https://golang.org/dl/go%s.linux-amd64.tar.gz", version)
			cmd = fmt.Sprintf("curl --location --fail --silent --show-error %s | tar -C /usr/local -xz", url)
		)

		c = c.WithExec([]string{"sh", "-ec", cmd})

		return AppendToPATH(ctx, "/usr/local/go/bin")(runtime, c)
	}
}

// InstallGithubCli installs github cli in the container using the given version and provided extensions. If the version
// is empty, the hardcoded "2.20.2" is used.
//
// Github cli uses GITHUB_TOKEN to authenticate, installation process read GITHUB_TOKEN env variable from host and
// configure it as a secret.
//
// The container must have the "curl" and "tar" binaries installed in order to install Go.
func InstallGithubCli(version string, extensions ...string) ContainerCustomizerFn {
	return func(runtime *daggers.Runtime, c *dagger.Container) (*dagger.Container, error) {
		var err error

		// If no version is given, default to 2.20.2.
		if version == "" {
			version = "2.20.2"
		}

		var (
			ghURLTemplate = "https://github.com/cli/cli/releases/download/v%s/gh_%s_linux_amd64.tar.gz"
			url           = fmt.Sprintf(ghURLTemplate, version, version)
			dest          = "/tmp/gh_linux_amd64.tar.gz"
			extractDir    = "/tmp"
			cliSourcePath = fmt.Sprintf("/tmp/gh_%s_linux_amd64/bin/gh", version)
			cliTargetPath = "/usr/local/bin/gh"
		)

		c, err = ApplyCustomizations(runtime, c, DownloadFile(url, dest))
		if err != nil {
			return nil, err
		}

		token := runtime.Client().Host().EnvVariable("GITHUB_TOKEN").Secret()

		c = c.WithSecretVariable("GITHUB_TOKEN", token).
			WithExec([]string{"tar", "-xf", dest, "-C", extractDir}).
			WithExec([]string{"mv", cliSourcePath, cliTargetPath}).
			WithExec([]string{"rm", "-rf", "/tmp/*"})

		for _, extension := range extensions {
			c = c.WithExec([]string{"gh", "extension", "install", extension})
		}

		return c, nil
	}
}

// DownloadFile downloads the given URL to the given destination file.
func DownloadFile(url, destFile string) ContainerCustomizerFn {
	return func(runtime *daggers.Runtime, c *dagger.Container) (*dagger.Container, error) {
		cmd := fmt.Sprintf("curl --location --fail --silent --show-error %s --output %s", url, destFile)

		return c.WithExec([]string{"sh", "-ec", cmd}), nil
	}
}

// DownloadExecutableFile downloads the given URL to the given destination file and makes it executable.
func DownloadExecutableFile(url, destFile string) ContainerCustomizerFn {
	return func(runtime *daggers.Runtime, c *dagger.Container) (*dagger.Container, error) {
		c, err := DownloadFile(url, destFile)(runtime, c)
		if err != nil {
			return nil, err
		}

		return c.WithExec([]string{"chmod", "755", destFile}), nil
	}
}

// WithEnvVariables sets the given environment variables in the container.
func WithEnvVariables(env map[string]string) ContainerCustomizerFn {
	return func(runtime *daggers.Runtime, c *dagger.Container) (*dagger.Container, error) {
		for k, v := range env {
			c = c.WithEnvVariable(k, v)
		}

		return c, nil
	}
}

// WithHostEnvVariable sets the given environment variable in the container from the host.
func WithHostEnvVariable(ctx context.Context, name string) ContainerCustomizerFn {
	return func(runtime *daggers.Runtime, c *dagger.Container) (*dagger.Container, error) {
		val, err := runtime.Client().Host().EnvVariable(name).Value(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get host env variable %q: %w", name, err)
		}

		return c.WithEnvVariable(name, val), nil
	}
}

// WithHostEnvVariables sets the given environment variables in the container from the host.
func WithHostEnvVariables(ctx context.Context, include ...string) ContainerCustomizerFn {
	return func(runtime *daggers.Runtime, c *dagger.Container) (*dagger.Container, error) {
		var err error

		for _, name := range include {
			c, err = WithHostEnvVariable(ctx, name)(runtime, c)
			if err != nil {
				return nil, err
			}
		}

		return c, nil
	}
}

// WithHostEnvVariablesWithPrefix sets the given environment variables in the container from the host, using the given
// prefix to filter the host environment variables. If a ignore list is given, the variables in the ignore list are
// explicitly ignored to avoid leaking sensitive information and/or to avoid conflicts.
//
// For example, if the prefix is "FOO_" and the ignore list is "FOO_PASSWORD", the environment variable "FOO_PASSWORD"
// from the host will be ignored, but "FOO_USERNAME" will be set in the container.
func WithHostEnvVariablesWithPrefix(ctx context.Context, prefix string, ignore ...string) ContainerCustomizerFn {
	return func(runtime *daggers.Runtime, c *dagger.Container) (*dagger.Container, error) {
		// convert ignore list to a map for faster lookup
		ignoreMap := sliceToKeyMap(ignore)

		var include []string

		for _, name := range os.Environ() {
			// skip if the variable is not prefixed with the given prefix, or it's explicitly ignored
			if !strings.HasPrefix(name, prefix) || ignoreMap[name] {
				continue
			}

			// it seems that, collecting the variables to include in a slice and then calling WithHostEnvVariables
			// is lower cognitive complexity than calling WithHostEnvVariable in a loop, so we do that.
			include = append(include, name)
		}

		return WithHostEnvVariables(ctx, include...)(runtime, c)
	}
}

// sliceToKeyMap returns a map with the given keys and a true value.
func sliceToKeyMap(keys []string) map[string]bool {
	keyMap := make(map[string]bool, len(keys))

	for _, name := range keys {
		keyMap[name] = true
	}

	return keyMap
}

// WithHostEnvSecret sets the given environment variable in the container from the host as a secret.
func WithHostEnvSecret(name string) ContainerCustomizerFn {
	return func(runtime *daggers.Runtime, c *dagger.Container) (*dagger.Container, error) {
		secret := runtime.Client().Host().EnvVariable(name).Secret()

		return c.WithSecretVariable(name, secret), nil
	}
}

// WithHostEnvSecrets sets the given environment variables in the container from the host as secrets.
func WithHostEnvSecrets(include ...string) ContainerCustomizerFn {
	return func(runtime *daggers.Runtime, c *dagger.Container) (*dagger.Container, error) {
		for _, name := range include {
			c = c.WithSecretVariable(name, runtime.Client().Host().EnvVariable(name).Secret())
		}

		return c, nil
	}
}

// WithGitHubEnvs sets GitHub environment variables in the container from the host.
//
// The following environment variables are set:
// - GITHUB_TOKEN as a secret
// - GITHUB_* as regular environment variables except for GITHUB_TOKEN
// - RUNNER_* as regular environment variables.
func WithGitHubEnvs(ctx context.Context) ContainerCustomizerFn {
	return func(runtime *daggers.Runtime, c *dagger.Container) (*dagger.Container, error) {
		// Default environment variables for GitHub runners are documented here:
		// https://docs.github.com/en/actions/learn-github-actions/environment-variables#default-environment-variables

		// load all env variables from the host that start with "GITHUB_" and explicitly ignore GITHUB_TOKEN
		c, err := WithHostEnvVariablesWithPrefix(ctx, "GITHUB_", "GITHUB_TOKEN")(runtime, c)
		if err != nil {
			return nil, err
		}

		c, err = WithHostEnvVariablesWithPrefix(ctx, "RUNNER_")(runtime, c)
		if err != nil {
			return nil, err
		}

		// load GITHUB_TOKEN from the host as a secret
		c, err = WithHostEnvSecret("GITHUB_TOKEN")(runtime, c)
		if err != nil {
			return nil, err
		}

		return c, nil
	}
}

// WithSSHSocket mounts the SSH socket from the host into the container and sets the SSH_AUTH_SOCK environment variable.
func WithSSHSocket(ctx context.Context) ContainerCustomizerFn {
	return func(runtime *daggers.Runtime, c *dagger.Container) (*dagger.Container, error) {
		path, err := runtime.Client().Host().EnvVariable("SSH_AUTH_SOCK").Value(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get host env variable %q: %w", "SSH_AUTH_SOCK", err)
		}

		socket := runtime.Client().Host().UnixSocket(path)

		return c.WithEnvVariable("SSH_AUTH_SOCK", path).WithUnixSocket(path, socket), nil
	}
}

// WithDockerSocket mounts the Docker socket from the host and sets the DOCKER_HOST environment variable in
// the container.
func WithDockerSocket(ctx context.Context) ContainerCustomizerFn {
	return func(runtime *daggers.Runtime, c *dagger.Container) (*dagger.Container, error) {
		path := "/var/run/docker.sock"

		socket := runtime.Client().Host().UnixSocket(path)

		return c.WithEnvVariable("DOCKER_HOST", "unix://"+path).WithUnixSocket(path, socket), nil
	}
}

// WithGithubAuthUsingSSH sets up GitHub authentication in the container using SSH_AUTH_SOCK forwarding.
func WithGithubAuthUsingSSH(ctx context.Context) ContainerCustomizerFn {
	return func(runtime *daggers.Runtime, c *dagger.Container) (*dagger.Container, error) {
		// add github.com to known hosts
		c = c.WithExec(
			[]string{
				"sh", "-c", "mkdir -p /root/.ssh && ssh-keyscan -t rsa github.com >> ~/.ssh/known_hosts",
			},
		)

		// configure git to use ssh
		c = c.WithExec(
			[]string{
				"sh", "-c", "git config --global url.\"git@github.com:\".insteadOf \"https://github.com/\"",
			},
		)

		return c, nil
	}
}

// WithGithubAuthUsingToken sets up GitHub authentication in the container using GITHUB_TOKEN.
func WithGithubAuthUsingToken(_ context.Context) ContainerCustomizerFn {
	return func(runtime *daggers.Runtime, c *dagger.Container) (*dagger.Container, error) {
		// configure git to use ssh
		c = c.WithExec(
			[]string{
				"sh", "-c", "git config --global url.https://$GITHUB_TOKEN@github.com/.insteadOf https://github.com/",
			},
		)

		return c, nil
	}
}
