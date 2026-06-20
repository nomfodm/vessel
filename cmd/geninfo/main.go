// geninfo writes build/windows/info.json with the given version injected into
// Windows PE metadata fields. Replaces the PowerShell gen-info.ps1 so the build
// works on any platform that has Go.
//
// Usage: go run ./cmd/geninfo [-version 1.2.3] [-out path/to/info.json]
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
)

func main() {
	version := flag.String("version", "dev", "version string to embed (e.g. 1.2.3)")
	out := flag.String("out", filepath.Join("build", "windows", "info.json"), "output path")
	flag.Parse()

	type fixed struct {
		FileVersion string `json:"file_version"`
	}
	type langInfo struct {
		ProductVersion  string `json:"ProductVersion"`
		CompanyName     string `json:"CompanyName"`
		FileDescription string `json:"FileDescription"`
		LegalCopyright  string `json:"LegalCopyright"`
		ProductName     string `json:"ProductName"`
		Comments        string `json:"Comments"`
	}
	payload := struct {
		Fixed fixed               `json:"fixed"`
		Info  map[string]langInfo `json:"info"`
	}{
		Fixed: fixed{FileVersion: *version},
		Info: map[string]langInfo{
			"0000": {
				ProductVersion:  *version,
				CompanyName:     "Infinity Server",
				FileDescription: "Infinity Launcher",
				LegalCopyright:  "(c) 2026, Infinity Server",
				ProductName:     "Infinity Launcher",
				Comments:        "Play on servers by Infinity",
			},
		},
	}

	data, err := json.MarshalIndent(payload, "", "\t")
	if err != nil {
		log.Fatalf("marshal: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(*out, append(data, '\n'), 0o644); err != nil {
		log.Fatalf("write: %v", err)
	}
}
