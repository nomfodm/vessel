package game

import (
	"encoding/json"
	"os"
	"runtime"
)

// Env is the runtime environment used to evaluate version.json rules. Injected
// so command building is testable on any host.
type Env struct {
	OS       string // windows | osx | linux
	Arch     string // x86 | x64 | arm64
	Features map[string]bool
}

func CurrentEnv() Env {
	return Env{OS: osName(runtime.GOOS), Arch: archName(runtime.GOARCH)}
}

func osName(goos string) string {
	switch goos {
	case "windows":
		return "windows"
	case "darwin":
		return "osx"
	default:
		return "linux"
	}
}

func archName(goarch string) string {
	switch goarch {
	case "amd64":
		return "x64"
	case "386":
		return "x86"
	case "arm64":
		return "arm64"
	default:
		return goarch
	}
}

// Version is the Mojang launch recipe (baked flat by the baker — no inheritsFrom).
type Version struct {
	ID                 string    `json:"id"`
	MainClass          string    `json:"mainClass"`
	MinecraftArguments string    `json:"minecraftArguments"` // legacy single-string form
	Arguments          Arguments `json:"arguments"`          // modern form
	Libraries          []Library `json:"libraries"`
	AssetIndex         struct {
		ID string `json:"id"`
	} `json:"assetIndex"`
	Assets string `json:"assets"`
	Type   string `json:"type"`
}

type Arguments struct {
	Game []Argument `json:"game"`
	JVM  []Argument `json:"jvm"`
}

type Library struct {
	Name      string `json:"name"`
	Downloads struct {
		Artifact struct {
			Path string `json:"path"`
		} `json:"artifact"`
	} `json:"downloads"`
	Rules []Rule `json:"rules"`
}

type Rule struct {
	Action   string          `json:"action"`
	OS       *OSRule         `json:"os"`
	Features map[string]bool `json:"features"`
}

type OSRule struct {
	Name string `json:"name"`
	Arch string `json:"arch"`
}

// Argument is one entry of arguments.game/jvm: either a plain string or an object
// with rules and a string-or-array value.
type Argument struct {
	Rules  []Rule
	Values []string
}

func (a *Argument) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		a.Values = []string{s}
		return nil
	}

	var obj struct {
		Rules []Rule          `json:"rules"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	a.Rules = obj.Rules

	var single string
	if err := json.Unmarshal(obj.Value, &single); err == nil {
		a.Values = []string{single}
		return nil
	}
	var multi []string
	if err := json.Unmarshal(obj.Value, &multi); err != nil {
		return err
	}
	a.Values = multi
	return nil
}

func ParseVersion(b []byte) (*Version, error) {
	var v Version
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func ParseVersionFile(path string) (*Version, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseVersion(b)
}

func (r Rule) matches(env Env) bool {
	if r.OS != nil {
		if r.OS.Name != "" && r.OS.Name != env.OS {
			return false
		}
		if r.OS.Arch != "" && r.OS.Arch != env.Arch {
			return false
		}
	}
	for feat, want := range r.Features {
		if env.Features[feat] != want {
			return false
		}
	}
	return true
}

// rulesAllow implements Mojang semantics: no rules => allowed; otherwise the last
// matching rule's action wins, default disallow.
func rulesAllow(rules []Rule, env Env) bool {
	if len(rules) == 0 {
		return true
	}
	allowed := false
	for _, r := range rules {
		if r.matches(env) {
			allowed = r.Action == "allow"
		}
	}
	return allowed
}
