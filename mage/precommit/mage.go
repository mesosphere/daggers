package precommit

import (
	"context"
	"os"

	"dagger.io/dagger"

	precommitDagger "github.com/aweris/tools/dagger/precommit"
)

const (
	baseImageEnvVar = "PRECOMMIT_BASE_IMAGE"
)

// Precommit runs all the precommit checks.
// Configurable via the following environment variables:
//
//	PRECOMMIT_BASE_IMAGE - The base image to run pre-commit in.
func Precommit(ctx context.Context) error {
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		panic(err)
	}
	defer client.Close()

	var opts []precommitDagger.Option

	if baseImage, ok := os.LookupEnv(baseImageEnvVar); ok {
		opts = append(opts, precommitDagger.BaseImage(baseImage))
	}

	return precommitDagger.Run(ctx, client, client.Host().Workdir().Read(), opts...)
}
