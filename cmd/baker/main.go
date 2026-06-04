// Command baker hashes a Minecraft client directory into a content-addressed
// store and writes the launcher manifest. The output mirrors the S3 layout, so
// uploading it (mc/aws/rclone) publishes the profile.
//
//	baker -client ./survival -slug survival -out ./dist
//
// Non-derivable manifest fields (runtime, RAM, strictDirs, optional files) come
// from a baker.json spec in the client directory.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lmittmann/tint"
	"github.com/nomfodm/vessel/internal/baker"
)

func main() {
	var (
		clientDir = flag.String("client", "", "path to the client directory to bake (required)")
		slug      = flag.String("slug", "", "profile slug, e.g. survival (required)")
		outDir    = flag.String("out", "./dist", "output directory (S3-layout mirror)")
		specPath  = flag.String("spec", "", "path to baker.json (default: <client>/baker.json)")
		manifest  = flag.String("manifest", "files-v1.json", "manifest filename under profiles/<slug>/")
		verbose   = flag.Bool("v", false, "verbose (per-file logging)")
	)
	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	log := slog.New(tint.NewHandler(os.Stderr, &tint.Options{Level: level}))

	if *clientDir == "" || *slug == "" {
		fmt.Fprintln(os.Stderr, "error: -client and -slug are required")
		flag.Usage()
		os.Exit(2)
	}

	// Default the spec to the client dir; remember its name so the walk skips it.
	specSkip := ""
	if *specPath == "" {
		*specPath = filepath.Join(*clientDir, "baker.json")
		specSkip = "baker.json"
	}

	spec, err := baker.LoadSpec(*specPath)
	if err != nil {
		log.Error("load spec", "err", err)
		os.Exit(1)
	}

	res, err := baker.Bake(log, *clientDir, *slug, *outDir, *manifest, specSkip, spec)
	if err != nil {
		log.Error("bake failed", "err", err)
		os.Exit(1)
	}

	fmt.Printf("\n  profile:   %s\n", *slug)
	fmt.Printf("  files:     %d\n", res.TotalFiles)
	fmt.Printf("  new objs:  %d\n", res.NewObjects)
	fmt.Printf("  size:      %.1f MiB\n", float64(res.TotalBytes)/(1024*1024))
	fmt.Printf("  manifest:  %s\n", filepath.Join(*outDir, filepath.FromSlash(res.ManifestPath)))
	fmt.Printf("\nupload:  mc mirror %s  <alias>/<bucket>\n", *outDir)
}
