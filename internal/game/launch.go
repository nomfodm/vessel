package game

import (
	"path/filepath"
	"strconv"
	"strings"
)

// LaunchParams are everything the command builder needs that comes from outside
// the version.json: paths, the auth session and user JVM tuning.
type LaunchParams struct {
	GameDir     string
	AssetsDir   string
	LibsDir     string
	ClientJar   string
	JavaPath    string
	NativesDir  string
	VersionName string

	PlayerName  string
	UUID        string
	AccessToken string
	UserType    string

	MaxRAMMB    int
	ExtraJVMArg []string
}

// BuildCommand turns a parsed Version plus params into argv (java first). Pure
// function — no process is started, so it is fully testable.
func BuildCommand(v *Version, p LaunchParams, env Env) []string {
	classpath := buildClasspath(v, p, env)
	subst := substitutions(v, p, classpath, env)

	argv := []string{p.JavaPath}

	if p.MaxRAMMB > 0 {
		argv = append(argv, "-Xmx"+strconv.Itoa(p.MaxRAMMB)+"M")
	}

	if len(v.Arguments.JVM) > 0 {
		argv = append(argv, expandArgs(v.Arguments.JVM, env, subst)...)
	} else {
		// Legacy versions have no JVM args: provide the essentials ourselves.
		argv = append(argv,
			"-Djava.library.path="+p.NativesDir,
			"-cp", classpath,
		)
	}

	argv = append(argv, p.ExtraJVMArg...)
	argv = append(argv, v.MainClass)

	if len(v.Arguments.Game) > 0 {
		argv = append(argv, expandArgs(v.Arguments.Game, env, subst)...)
	} else if v.MinecraftArguments != "" {
		for _, tok := range strings.Fields(v.MinecraftArguments) {
			argv = append(argv, applySubst(tok, subst))
		}
	}

	return argv
}

func buildClasspath(v *Version, p LaunchParams, env Env) string {
	seen := make(map[string]bool)
	var parts []string
	for _, lib := range v.Libraries {
		if !rulesAllow(lib.Rules, env) {
			continue
		}
		rel := lib.Downloads.Artifact.Path
		if rel == "" {
			rel = mavenPath(lib.Name)
		}
		if rel == "" {
			continue
		}
		abs := filepath.Join(p.LibsDir, filepath.FromSlash(rel))
		if seen[abs] {
			continue
		}
		seen[abs] = true
		parts = append(parts, abs)
	}
	parts = append(parts, p.ClientJar)
	return strings.Join(parts, string(classpathSeparator(env)))
}

func classpathSeparator(env Env) rune {
	if env.OS == "windows" {
		return ';'
	}
	return ':'
}

func substitutions(v *Version, p LaunchParams, classpath string, env Env) map[string]string {
	userType := p.UserType
	if userType == "" {
		userType = "msa"
	}
	return map[string]string{
		"auth_player_name":    p.PlayerName,
		"auth_uuid":           p.UUID,
		"auth_access_token":   p.AccessToken,
		"user_type":           userType,
		"version_name":        p.VersionName,
		"version_type":        v.Type,
		"game_directory":      p.GameDir,
		"assets_root":         p.AssetsDir,
		"assets_index_name":   v.AssetIndex.ID,
		"natives_directory":   p.NativesDir,
		"classpath":           classpath,
		"launcher_name":       "infinity",
		"launcher_version":    "1",
		"library_directory":   p.LibsDir,
		"classpath_separator": string(classpathSeparator(env)),
	}
}

func expandArgs(args []Argument, env Env, subst map[string]string) []string {
	var out []string
	for _, a := range args {
		if !rulesAllow(a.Rules, env) {
			continue
		}
		for _, val := range a.Values {
			out = append(out, applySubst(val, subst))
		}
	}
	return out
}

// applySubst replaces ${key} placeholders. Unknown placeholders are stripped.
func applySubst(s string, subst map[string]string) string {
	if !strings.Contains(s, "${") {
		return s
	}
	for k, v := range subst {
		s = strings.ReplaceAll(s, "${"+k+"}", v)
	}
	// Strip any remaining unknown placeholders so any modloader works without
	// requiring launcher-specific handling for every new variable.
	for strings.Contains(s, "${") {
		start := strings.Index(s, "${")
		end := strings.Index(s[start:], "}")
		if end == -1 {
			break
		}
		s = s[:start] + s[start+end+1:]
	}
	return s
}

// mavenPath turns "group:artifact:version" into "group/path/artifact/version/
// artifact-version.jar" as a fallback when downloads.artifact.path is absent.
func mavenPath(name string) string {
	parts := strings.Split(name, ":")
	if len(parts) < 3 {
		return ""
	}
	group, artifact, version := parts[0], parts[1], parts[2]
	file := artifact + "-" + version
	if len(parts) > 3 {
		file += "-" + parts[3]
	}
	file += ".jar"
	return strings.ReplaceAll(group, ".", "/") + "/" + artifact + "/" + version + "/" + file
}
