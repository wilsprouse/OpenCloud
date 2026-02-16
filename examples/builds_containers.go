package main

import (
        "fmt"
        "os"
        "path/filepath"

        "github.com/moby/buildkit/client"
        "github.com/moby/buildkit/util/appcontext"
        "github.com/moby/buildkit/util/progress/progressui"
        "github.com/pkg/errors"
        "github.com/urfave/cli"
)

func main() {
        app := cli.NewApp()
        app.Name = "buildkit-containerd-builder"
        app.Usage = "Build a Dockerfile image and push directly to local containerd"
        app.Flags = []cli.Flag{
                cli.StringFlag{
                        Name:  "buildkit-addr",
                        Usage: "BuildKit daemon address",
                        Value: "unix:///run/buildkit/buildkitd.sock",
                },
                cli.StringFlag{
                        Name:  "file, f",
                        Usage: "Dockerfile path (default: PATH/Dockerfile)",
                },
                cli.StringFlag{
                        Name:  "tag, t",
                        Usage: "Image name:tag for containerd",
                },
                cli.BoolFlag{
                        Name:  "no-cache",
                        Usage: "Do not use cache when building",
                },
                cli.StringSliceFlag{
                        Name:  "build-arg",
                        Usage: "Set build-time variables",
                },
        }
        app.Action = buildAction

        if err := app.Run(os.Args); err != nil {
                fmt.Fprintf(os.Stderr, "error: %v\n", err)
                os.Exit(1)
        }
}

func buildAction(c *cli.Context) error {
        ctx := appcontext.Context()

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

        // Connect to BuildKit
        bkClient, err := client.New(ctx, c.String("buildkit-addr"))
        if err != nil {
                return err
        }
        defer bkClient.Close()

        // Solve options: push directly to containerd
        solveOpt := &client.SolveOpt{
                LocalDirs: map[string]string{
                        "context":    buildCtx,
                        "dockerfile": filepath.Dir(dockerfilePath),
                },
                Frontend: "dockerfile.v0",
                FrontendAttrs: map[string]string{
                        "filename": filepath.Base(dockerfilePath),
                },
                Exports: []client.ExportEntry{
                        {
                                Type: client.ExporterImage, // Push to containerd
                                Attrs: map[string]string{
                                        "name": tag,
                                        "push": "false", // store locally in containerd
                                },
                        },
                },
        }

        if c.Bool("no-cache") {
                solveOpt.FrontendAttrs["no-cache"] = ""
        }

		// Display progress
        ch := make(chan *client.SolveStatus, 100)
        display, err := progressui.NewDisplay(os.Stderr, progressui.TtyMode)
        if err != nil {
                display, _ = progressui.NewDisplay(os.Stdout, progressui.PlainMode)
        }

        done := make(chan error)
        go func() {
                _, solveErr := bkClient.Solve(ctx, nil, *solveOpt, ch)
                done <- solveErr
        }()

        go func() {
                if _, err := display.UpdateFrom(ctx, ch); err != nil {
                        fmt.Fprintf(os.Stderr, "progress display error: %v\n", err)
                }
        }()

        // Wait for solve to finish
        if err := <-done; err != nil {
                return err
        }

        fmt.Printf("Image %q built and stored in containerd!\n", tag)
        return nil
}
