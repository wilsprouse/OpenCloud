package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	buildahDefine "github.com/containers/buildah/define"
	"github.com/containers/podman/v5/pkg/bindings"
	"github.com/containers/podman/v5/pkg/bindings/images"
	podmanEntities "github.com/containers/podman/v5/pkg/domain/entities/types"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()
	app.Name = "podman-builder"
	app.Usage = "Build a Dockerfile image and store it in the local Podman image store"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "podman-socket",
			Usage: "Podman API socket",
			Value: "unix:/run/user/" + strconv.Itoa(os.Getuid()) + "/podman/podman.sock",
		},
		cli.StringFlag{
			Name:  "file, f",
			Usage: "Dockerfile path (default: PATH/Dockerfile)",
		},
		cli.StringFlag{
			Name:  "tag, t",
			Usage: "Image name:tag for Podman",
		},
		cli.BoolFlag{
			Name:  "no-cache",
			Usage: "Do not use cache when building",
		},
	}
	app.Action = buildAction

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func buildAction(c *cli.Context) error {
	ctx := context.Background()

	tag := c.String("tag")
	if tag == "" {
		return errors.New("tag is required (image name:tag)")
	}

	buildCtx := c.Args().First()
	if buildCtx == "" {
		return errors.New("build context required (e.g. '.')")
	}

	dockerfilePath := c.String("file")
	if dockerfilePath == "" {
		dockerfilePath = filepath.Join(buildCtx, "Dockerfile")
	}

	conn, err := bindings.NewConnection(ctx, c.String("podman-socket"))
	if err != nil {
		return err
	}

	buildOpts := podmanEntities.BuildOptions{
		BuildOptions: buildahDefine.BuildOptions{
			ContextDirectory: buildCtx,
			Output:           tag,
			NoCache:          c.Bool("no-cache"),
			CommonBuildOpts:  &buildahDefine.CommonBuildOptions{},
			ReportWriter:     io.Discard,
		},
	}

	if _, err := images.Build(conn, []string{dockerfilePath}, buildOpts); err != nil {
		return err
	}

	fmt.Printf("Image %q built and stored in Podman!\n", tag)
	return nil
}
