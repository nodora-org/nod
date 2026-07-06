package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	nodBin      string // path to the built binary
	testsDir    string // this package's directory
	projectRoot string // module root (parent of tests/)
	rulesetsDir string // the demo global policy, projectRoot/rulesets
)

func TestMain(m *testing.M) {
	var err error
	testsDir, err = os.Getwd()
	if err != nil {
		panic(err)
	}
	projectRoot = filepath.Dir(testsDir)
	rulesetsDir = filepath.Join(projectRoot, "rulesets")

	tmp, err := os.MkdirTemp("", "nod-e2e")
	if err != nil {
		panic(err)
	}
	nodBin = filepath.Join(tmp, "nod")

	build := exec.Command("go", "build", "-o", nodBin, projectRoot)
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		os.RemoveAll(tmp)
		panic("building nod: " + err.Error())
	}

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

type result struct {
	stdout string
	stderr string
	exit   int
}

func run(t *testing.T, env map[string]string, stdin string, args ...string) result {
	t.Helper()
	iso := t.TempDir() // isolated cwd + project dir so local .nod discovery finds nothing
	cmd := exec.Command(nodBin, args...)
	cmd.Dir = iso
	cmd.Env = append(os.Environ(), "NOD_HOME="+t.TempDir(), "CLAUDE_PROJECT_DIR="+iso)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errb strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errb

	exit := 0
	if err := cmd.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running %v: %v", args, err)
		}
		exit = ee.ExitCode()
	}
	return result{out.String(), errb.String(), exit}
}

var verdictRE = regexp.MustCompile(`VERDICT:\s+(\w+)`)

func verdictOf(t *testing.T, stdout string) string {
	t.Helper()
	m := verdictRE.FindStringSubmatch(stdout)
	if m == nil {
		t.Fatalf("no VERDICT line in output:\n%s", stdout)
	}
	return m[1]
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeRulesets(t *testing.T, files map[string][]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, lines := range files {
		writeFile(t, filepath.Join(dir, name), strings.Join(lines, "\n")+"\n")
	}
	return dir
}

type fixture struct {
	GlobalDir     string              `json:"globalDir"`     // policy dir relative to the module root; else Rulesets
	Rulesets      map[string][]string `json:"rulesets"`      // inline base policy (name -> source lines)
	LocalRulesets map[string][]string `json:"localRulesets"` // optional tighten-only --local layer
	Cases         []caseSpec          `json:"cases"`
}

type caseSpec struct {
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"` // the tool call JSON passed to `nod decide -i`
	Flags []string        `json:"flags"` // extra flags, e.g. ["--default","allow"]
	Want  wantSpec        `json:"want"`
}

type wantSpec struct {
	Verdict        string   `json:"verdict"`        // checked against the VERDICT line, if set
	Exit           *int     `json:"exit"`           // expected exit code (default 0)
	StdoutContains []string `json:"stdoutContains"` // substrings required in stdout
	StderrContains []string `json:"stderrContains"` // substrings required in stderr
}

func TestDecide(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(testsDir, "decide", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		return // no fixtures
	}

	for _, f := range files {
		fx := loadFixture(t, f)

		base := fx.GlobalDir
		if base != "" {
			base = filepath.Join(projectRoot, base)
		} else {
			base = writeRulesets(t, fx.Rulesets)
		}
		var localArgs []string
		if len(fx.LocalRulesets) > 0 {
			localArgs = []string{"--local", writeRulesets(t, fx.LocalRulesets)}
		}

		t.Run(strings.TrimSuffix(filepath.Base(f), ".json"), func(t *testing.T) {
			for _, c := range fx.Cases {
				t.Run(c.Name, func(t *testing.T) {
					body := string(c.Input)
					if body == "" {
						body = "{}"
					}
					in := filepath.Join(t.TempDir(), "input.json")
					writeFile(t, in, body)

					args := append([]string{"decide", "-d", base, "-i", in}, localArgs...)
					args = append(args, c.Flags...)
					checkWant(t, run(t, nil, "", args...), c.Want)
				})
			}
		})
	}
}

func loadFixture(t *testing.T, path string) fixture {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fx fixture
	if err := json.Unmarshal(b, &fx); err != nil {
		t.Fatalf("%s: %v", filepath.Base(path), err)
	}
	return fx
}

func checkWant(t *testing.T, r result, w wantSpec) {
	t.Helper()
	exit := 0
	if w.Exit != nil {
		exit = *w.Exit
	}
	if r.exit != exit {
		t.Fatalf("exit = %d, want %d; stderr: %s", r.exit, exit, r.stderr)
	}
	if w.Verdict != "" {
		if got := verdictOf(t, r.stdout); got != w.Verdict {
			t.Fatalf("verdict = %s, want %s", got, w.Verdict)
		}
	}
	for _, s := range w.StdoutContains {
		if !strings.Contains(r.stdout, s) {
			t.Errorf("stdout missing %q:\n%s", s, r.stdout)
		}
	}
	for _, s := range w.StderrContains {
		if !strings.Contains(r.stderr, s) {
			t.Errorf("stderr missing %q:\n%s", s, r.stderr)
		}
	}
}

func TestCheck(t *testing.T) {
	ok := run(t, nil, "", "check", "-d", rulesetsDir)
	if ok.exit != 0 {
		t.Fatalf("check on good rulesets: exit %d, stderr: %s", ok.exit, ok.stderr)
	}
	if !strings.Contains(ok.stdout, "0 failed") {
		t.Fatalf("expected a clean report, got:\n%s", ok.stdout)
	}

	broken := writeRulesets(t, map[string][]string{
		"00-good.ruleset": {"signal Deny(reason)", "rule R { emit Deny(\"x\") when input.bad == true }"},
		"99-bad.ruleset":  {"not valid nodora"},
	})
	bad := run(t, nil, "", "check", "-d", broken)
	if bad.exit == 0 {
		t.Fatalf("check with a broken ruleset should exit non-zero")
	}
	if !strings.Contains(bad.stdout, "99-bad.ruleset") || !strings.Contains(bad.stdout, "1 failed") {
		t.Fatalf("expected the broken file in the report, got:\n%s", bad.stdout)
	}
}
