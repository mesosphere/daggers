package precommit

import (
	_ "embed"

	"context"

	"dagger.io/dagger"
	"github.com/aweris/tools/utils"
)

const (
	configFileName      = ".pre-commit-config.yaml"
	cacheDir            = "/pre-commit-cache"
	precommitHomeEnvVar = "PRE_COMMIT_HOME"
)

func Run(ctx context.Context, client *dagger.Client, workdir *dagger.Directory, options ...Option) error {
	cfg := defaultConfig()
	for _, o := range options {
		cfg = o(cfg)
	}

	srcDirID, err := workdir.ID(ctx)
	if err != nil {
		return err
	}

	configFileHash, err := utils.SHA256SumFile(configFileName)
	if err != nil {
		return err
	}
	cacheKey := "precommit-hooks-" + configFileHash
	cacheID, err := client.CacheVolume(cacheKey).ID(ctx)
	if err != nil {
		return err
	}

	// Create a pre-commit container
	container := client.
		Container().From(cfg.baseImage).
		Exec(dagger.ContainerExecOpts{
			Args: []string{
				"curl",
				"--location", "--fail", "--silent", "--show-error",
				"--output", "/usr/local/bin/pre-commit-2.20.0.pyz",
				"https://github.com/pre-commit/pre-commit/releases/download/v2.20.0/pre-commit-2.20.0.pyz",
			},
		}).
		WithEnvVariable(precommitHomeEnvVar, cacheDir).
		WithMountedCache(cacheID, cacheDir).
		WithMountedDirectory("/src", srcDirID).WithWorkdir("/src").
		Exec(dagger.ContainerExecOpts{
			Args: []string{
				"python", "/usr/local/bin/pre-commit-2.20.0.pyz",
				"run", "--all-files", "--show-diff-on-failure",
			},
		})

	// Run container and get Exit code
	_, err = container.ExitCode(ctx)
	if err != nil {
		return err
	}

	return nil
}
