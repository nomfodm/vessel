package game

import (
	"strings"
	"testing"
)

const modernVersion = `{
  "id": "1.20.1",
  "type": "release",
  "mainClass": "net.minecraft.client.main.Main",
  "assetIndex": {"id": "5"},
  "libraries": [
    {"name": "com.example:common:1.0",
     "downloads": {"artifact": {"path": "com/example/common/1.0/common-1.0.jar"}}},
    {"name": "org.lwjgl:lwjgl-windows:3.3",
     "downloads": {"artifact": {"path": "org/lwjgl/lwjgl-windows/3.3/lwjgl-windows-3.3.jar"}},
     "rules": [{"action": "allow", "os": {"name": "windows"}}]}
  ],
  "arguments": {
    "jvm": [
      "-Djava.library.path=${natives_directory}",
      "-cp", "${classpath}",
      {"rules": [{"action": "allow", "os": {"name": "osx"}}], "value": "-XstartOnFirstThread"}
    ],
    "game": [
      "--username", "${auth_player_name}",
      "--uuid", "${auth_uuid}",
      "--accessToken", "${auth_access_token}",
      "--gameDir", "${game_directory}",
      {"rules": [{"action": "allow", "features": {"is_demo_user": true}}], "value": "--demo"}
    ]
  }
}`

func params() LaunchParams {
	return LaunchParams{
		GameDir:     "/data/profiles/survival",
		AssetsDir:   "/data/assets",
		LibsDir:     "/data/libs",
		ClientJar:   "/data/profiles/survival/client.jar",
		JavaPath:    "/data/runtimes/jre-21/bin/java",
		NativesDir:  "/data/profiles/survival/natives",
		VersionName: "1.20.1",
		PlayerName:  "alex",
		UUID:        "uuid-123",
		AccessToken: "game-token-secret",
		MaxRAMMB:    4096,
	}
}

func argvString(argv []string) string { return strings.Join(argv, " ") }

func TestBuildCommandModernWindows(t *testing.T) {
	v, err := ParseVersion([]byte(modernVersion))
	if err != nil {
		t.Fatalf("ParseVersion: %v", err)
	}
	env := Env{OS: "windows", Arch: "x64"}
	argv := BuildCommand(v, params(), env)
	s := argvString(argv)

	if argv[0] != "/data/runtimes/jre-21/bin/java" {
		t.Errorf("java not first: %q", argv[0])
	}
	if !strings.Contains(s, "-Xmx4096M") {
		t.Error("missing -Xmx")
	}
	if !strings.Contains(s, "net.minecraft.client.main.Main") {
		t.Error("missing mainClass")
	}
	// placeholders substituted
	if !strings.Contains(s, "--username alex") || !strings.Contains(s, "--accessToken game-token-secret") {
		t.Errorf("substitution failed: %s", s)
	}
	if strings.Contains(s, "${") {
		t.Errorf("unsubstituted placeholder remains: %s", s)
	}
	// windows classpath sep and windows-only lib included; macOS arg excluded
	if !strings.Contains(s, "lwjgl-windows-3.3.jar") {
		t.Error("windows lib should be in classpath")
	}
	if strings.Contains(s, "-XstartOnFirstThread") {
		t.Error("macOS-only JVM arg leaked into windows command")
	}
	if strings.Contains(s, "--demo") {
		t.Error("demo arg included without feature")
	}
}

func TestBuildCommandClasspathSeparator(t *testing.T) {
	v, _ := ParseVersion([]byte(modernVersion))

	win := argvString(BuildCommand(v, params(), Env{OS: "windows", Arch: "x64"}))
	if !strings.Contains(win, "common-1.0.jar;") && !strings.Contains(win, ";"+params().ClientJar) {
		t.Errorf("windows classpath should use ';': %s", win)
	}

	lin := argvString(BuildCommand(v, params(), Env{OS: "linux", Arch: "x64"}))
	if !strings.Contains(lin, ":"+params().ClientJar) {
		t.Errorf("linux classpath should use ':': %s", lin)
	}
}

func TestBuildCommandMacIncludesStartOnFirstThread(t *testing.T) {
	v, _ := ParseVersion([]byte(modernVersion))
	mac := argvString(BuildCommand(v, params(), Env{OS: "osx", Arch: "arm64"}))
	if !strings.Contains(mac, "-XstartOnFirstThread") {
		t.Error("macOS JVM arg missing on osx")
	}
	if strings.Contains(mac, "lwjgl-windows") {
		t.Error("windows lib leaked into macOS classpath")
	}
}

func TestBuildCommandFeatureGate(t *testing.T) {
	v, _ := ParseVersion([]byte(modernVersion))
	env := Env{OS: "linux", Arch: "x64", Features: map[string]bool{"is_demo_user": true}}
	s := argvString(BuildCommand(v, params(), env))
	if !strings.Contains(s, "--demo") {
		t.Error("--demo should appear when feature enabled")
	}
}

const legacyVersion = `{
  "id": "1.8.9",
  "type": "release",
  "mainClass": "net.minecraft.client.main.Main",
  "assetIndex": {"id": "1.8"},
  "minecraftArguments": "--username ${auth_player_name} --uuid ${auth_uuid} --accessToken ${auth_access_token}",
  "libraries": [
    {"name": "com.example:common:1.0",
     "downloads": {"artifact": {"path": "com/example/common/1.0/common-1.0.jar"}}}
  ]
}`

func TestBuildCommandLegacy(t *testing.T) {
	v, err := ParseVersion([]byte(legacyVersion))
	if err != nil {
		t.Fatalf("ParseVersion: %v", err)
	}
	env := Env{OS: "windows", Arch: "x64"}
	s := argvString(BuildCommand(v, params(), env))

	if !strings.Contains(s, "-Djava.library.path=") || !strings.Contains(s, "-cp") {
		t.Errorf("legacy path should synthesize jvm args: %s", s)
	}
	if !strings.Contains(s, "--username alex") || !strings.Contains(s, "--accessToken game-token-secret") {
		t.Errorf("legacy minecraftArguments not substituted: %s", s)
	}
	if strings.Contains(s, "${") {
		t.Errorf("unsubstituted placeholder: %s", s)
	}
}

func TestRulesAllow(t *testing.T) {
	win := Env{OS: "windows", Arch: "x64"}
	if rulesAllow(nil, win) != true {
		t.Error("no rules should allow")
	}
	deny := []Rule{{Action: "allow", OS: &OSRule{Name: "osx"}}}
	if rulesAllow(deny, win) {
		t.Error("osx-only rule should not allow on windows")
	}
	allow := []Rule{{Action: "allow", OS: &OSRule{Name: "windows"}}}
	if !rulesAllow(allow, win) {
		t.Error("windows rule should allow on windows")
	}
}

func TestMavenPathFallback(t *testing.T) {
	got := mavenPath("org.example:lib:2.1")
	want := "org/example/lib/2.1/lib-2.1.jar"
	if got != want {
		t.Errorf("mavenPath = %q, want %q", got, want)
	}
	if mavenPath("bad") != "" {
		t.Error("malformed name should yield empty")
	}
}
