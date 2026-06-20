// Command baker hashes a directory into a content-addressed store and writes a
// launcher manifest. The output mirrors the S3 layout, so uploading it
// (mc/aws/rclone) publishes the artifact.
//
//	baker -kind profile -client ./survival -slug survival -out ./dist
//	baker -kind runtime -client ./jre-win-x64 -runtime java17 -platform win-x64 -out ./dist
//
// Non-derivable manifest fields (runtime, RAM, strictDirs, optional files) come
// from a baker.json spec in the client directory (optional for runtimes).
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"

	"github.com/lmittmann/tint"
	"github.com/nomfodm/vessel/internal/baker"
)

func main() {
	var (
		kind      = flag.String("kind", "profile", "what to bake: profile | runtime")
		clientDir = flag.String("client", "", "path to the directory to bake (required)")
		slug      = flag.String("slug", "", "profile slug, e.g. survival (kind=profile)")
		rtName    = flag.String("runtime", "", "runtime name, e.g. java17 (kind=runtime)")
		platform  = flag.String("platform", "", "platform key, e.g. win-x64 (kind=runtime)")
		outDir    = flag.String("out", "./dist", "output directory (S3-layout mirror)")
		specPath  = flag.String("spec", "", "path to baker.json (default: <client>/baker.json)")
		manifest  = flag.String("manifest", "files-v1.json", "manifest filename")
		verbose   = flag.Bool("v", false, "verbose (per-file logging)")
	)
	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	log := slog.New(tint.NewHandler(os.Stderr, &tint.Options{Level: level}))

	if *clientDir == "" {
		fatal(log, "-client is required")
	}

	// Resolve where files and the manifest go, per kind.
	var pathPrefix, manifestRel string
	switch *kind {
	case "profile":
		if *slug == "" {
			fatal(log, "-slug is required for kind=profile")
		}
		pathPrefix = path.Join("profiles", *slug)
		manifestRel = path.Join("profiles", *slug, *manifest)
	case "runtime":
		if *rtName == "" || *platform == "" {
			fatal(log, "-runtime and -platform are required for kind=runtime")
		}
		pathPrefix = path.Join("runtimes", *rtName)
		manifestRel = path.Join("runtimes", *rtName, *platform, *manifest)
	default:
		fatal(log, "unknown -kind %q (want profile|runtime)", *kind)
	}

	// Default the spec to the client dir; a missing default is fine (runtimes and
	// trivial profiles need no spec). Remember its name so the walk skips it.
	specSkip := ""
	if *specPath == "" {
		*specPath = filepath.Join(*clientDir, "baker.json")
		specSkip = "baker.json"
	}
	spec, err := baker.LoadSpec(*specPath)
	if err != nil {
		if os.IsNotExist(err) && specSkip != "" {
			spec = baker.Spec{}
		} else {
			fatal(log, "load spec: %v", err)
		}
	}

	// A runtime's whole tree is strict by default — prune anything stale on a JRE
	// update.
	if *kind == "runtime" && len(spec.StrictDirs) == 0 {
		spec.StrictDirs = []string{"."}
	}

	res, err := baker.Bake(log, *clientDir, pathPrefix, *outDir, manifestRel, specSkip, spec)
	if err != nil {
		fatal(log, "bake failed: %v", err)
	}

	fmt.Printf("\n  kind:      %s\n", *kind)
	fmt.Printf("  prefix:    %s\n", pathPrefix)
	fmt.Printf("  files:     %d\n", res.TotalFiles)
	fmt.Printf("  new objs:  %d\n", res.NewObjects)
	fmt.Printf("  size:      %.1f MiB\n", float64(res.TotalBytes)/(1024*1024))
	fmt.Printf("  manifest:  %s\n", filepath.Join(*outDir, filepath.FromSlash(res.ManifestPath)))
	if *kind == "runtime" {
		fmt.Printf("\nindex.json:  runtimes.%s.%q = %q\n", *rtName, *platform, res.ManifestPath)
	}
	fmt.Printf("\nupload:  mc mirror %s  <alias>/<bucket>\n", *outDir)
}

func fatal(log *slog.Logger, format string, args ...any) {
	log.Error(fmt.Sprintf(format, args...))
	os.Exit(1)
}
