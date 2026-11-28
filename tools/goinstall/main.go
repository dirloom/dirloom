// Command goinstall studies how `go install` resolves the Dirloom module
// (anomaly A14 of the v0.4.0-rc1 plan). It has two subcommands.
//
// simulate reproduces the problem offline. It builds a throwaway Git
// repository whose tags have the topology of the real ones: v0.1.0 and v0.1.1
// zippable; v0.2.0 to v0.3.3, v0.4.0-a1 and v0.4.0-a2 holding file names that
// Go module zips refuse; v0.4.0-a3 to v0.4.0-a8 zippable; then, depending on
// the variant, v0.4.0-rc1 with or without a retract directive, and v0.4.0.
// From it, it serves:
//
//   - a module proxy on 127.0.0.1 that answers 404 for the .info, .mod and .zip
//     of an unzippable version, like proxy.golang.org, and whose version list
//     holds either every tag or the zippable tags only (proxy.golang.org has
//     served both);
//   - a checksum database on 127.0.0.1 that has no record for an unzippable
//     version, like sum.golang.org;
//   - the repository itself for the direct mode, through a Git url.insteadOf
//     rewrite of https://github.com/dirloom/dirloom.
//
// Every go command runs with a fresh GOPATH, module cache and GOBIN, GOENV=off,
// GOTOOLCHAIN=local, and HTTP(S)_PROXY pointing at a closed local port, so
// nothing reaches the network.
//
// check runs the same commands against the real module, through the public
// proxy and with GOPROXY=direct, each command with a fresh GOPATH, module cache
// and GOBIN. It is the check of a published tag at gates G4/G5.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
	"golang.org/x/mod/sumdb"
	"golang.org/x/mod/sumdb/dirhash"
	"golang.org/x/mod/sumdb/note"
	modzip "golang.org/x/mod/zip"
)

const (
	modPath = "github.com/dirloom/dirloom"
	pkgPath = modPath + "/cmd/dirloom"
	rc1     = "v0.4.0-rc1"
	// deadProxy is a local port nothing listens on: in the simulation, any
	// attempt to reach the network through HTTP(S)_PROXY fails at once. Go
	// never proxies loopback addresses, so the local module proxy and checksum
	// database stay reachable.
	deadProxy = "http://127.0.0.1:9"
	// commandTimeout bounds each go command, so that a stalled network cannot
	// stall the check.
	commandTimeout = 10 * time.Minute
)

// The two plan files that made seven real tags unzippable: a module zip refuses
// U+2014 and U+2019 in a file name.
var unzippableFiles = []string{
	"plans/Plan v0.2.0 — Confort d’utilisation, complétions et distribution GitOps.md",
	"plans/Plan v0.3.0 — Presentation Richness.md",
}

type release struct {
	version string
	date    string // commit and tag date
	plans   int    // how many of unzippableFiles the tree holds
}

// history is the topology of the real tags on 2026-11-28: the same names,
// the commit dates of the real tags, and the same refused file names
// (`git ls-tree -r <tag>`: one in v0.2.0, two from v0.3.0 to v0.4.0-a2).
var history = []release{
	{"v0.1.0", "2026-08-11T15:13:33Z", 0},
	{"v0.1.1", "2026-08-11T16:58:31Z", 0},
	{"v0.2.0", "2026-09-16T10:09:44Z", 1},
	{"v0.3.0", "2026-09-17T13:45:32Z", 2},
	{"v0.3.1", "2026-09-18T00:01:20Z", 2},
	{"v0.3.2", "2026-09-18T03:22:04Z", 2},
	{"v0.3.3", "2026-09-18T14:55:07Z", 2},
	{"v0.4.0-a1", "2026-09-20T22:24:25Z", 2},
	{"v0.4.0-a2", "2026-09-27T07:55:41Z", 2},
	{"v0.4.0-a3", "2026-09-29T23:10:08Z", 0},
	{"v0.4.0-a4", "2026-09-30T18:52:41Z", 0},
	{"v0.4.0-a5", "2026-10-29T19:31:53Z", 0},
	{"v0.4.0-a6", "2026-10-30T20:47:34Z", 0},
	{"v0.4.0-a7", "2026-11-01T14:01:06Z", 0},
	{"v0.4.0-a8", "2026-11-03T19:48:23Z", 0},
}

const (
	retractUnzippable = `retract (
	v0.4.0-a2 // The module zip cannot be built: file names that Go modules refuse.
	v0.4.0-a1 // The module zip cannot be built: file names that Go modules refuse.
	[v0.3.0, v0.3.3] // The module zip cannot be built: file names that Go modules refuse.
	v0.2.0 // The module zip cannot be built: file names that Go modules refuse.
)
`
	retractEarlier = "retract [v0.1.0, v0.4.0-a8] // Experiment only: every version before v0.4.0-rc1.\n"
)

type variant struct {
	name, title string
	target      string // the version that the named-version commands ask for
	rc1         bool   // v0.4.0-rc1 is tagged
	stable      bool   // v0.4.0 is tagged after v0.4.0-rc1
	retract     string // retract block of the go.mod of v0.4.0-rc1 and v0.4.0
}

var variants = []variant{
	{"today", "Today's tags, v0.4.0-a8 the newest", "v0.4.0-a8", false, false, ""},
	{"rc1", "v0.4.0-rc1 tagged, its go.mod without retract", rc1, true, false, ""},
	{"rc1-retract-unzippable", "v0.4.0-rc1 tagged, its go.mod retracting the seven unzippable versions", rc1, true, false, retractUnzippable},
	{"rc1-retract-earlier", "v0.4.0-rc1 tagged, its go.mod retracting every earlier version", rc1, true, false, retractEarlier},
	{"stable", "v0.4.0 tagged after v0.4.0-rc1, without retract", rc1, true, true, ""},
	{"stable-retract-unzippable", "v0.4.0 tagged after v0.4.0-rc1, both go.mod files retracting the seven unzippable versions", rc1, true, true, retractUnzippable},
}

type mode struct {
	name, title string
}

var simModes = []mode{
	{"proxy-all", "proxy listing every tag, then direct"},
	{"proxy-zippable", "proxy listing the zippable tags only, then direct"},
	{"proxy-all-only", "proxy listing every tag, no direct"},
	{"direct", "`GOPROXY=direct`"},
}

var checkModes = []mode{
	{"public", "`GOPROXY=https://proxy.golang.org,direct`"},
	{"direct", "`GOPROXY=direct`"},
}

type step struct {
	title   string
	args    []string
	install string // the version an install asks for, "" for other commands
}

func steps(target string) []step {
	return []step{
		{"`go list -m -versions -retracted`", []string{"list", "-m", "-versions", "-retracted", modPath}, ""},
		{"`go list -m -versions`", []string{"list", "-m", "-versions", modPath}, ""},
		{"`go list -m …@latest`", []string{"list", "-m", modPath + "@latest"}, ""},
		{"`go mod download -json …@" + target + "`", []string{"mod", "download", "-json", modPath + "@" + target}, ""},
		{"`go install …/cmd/dirloom@" + target + "`", []string{"install", pkgPath + "@" + target}, target},
		{"`go install …/cmd/dirloom@latest`", []string{"install", pkgPath + "@latest"}, "latest"},
	}
}

type outcome struct {
	exit    int
	summary string
	ok      bool // an install that produced a binary of the requested version
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "simulate":
		simulate(os.Args[2:])
	case "check":
		checkPublished(os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  goinstall simulate [-go go] [-out report.md] [-log run.log] [-keep]
  goinstall check -version vX.Y.Z[-pre] [-go go] [-out report.md] [-log run.log] [-keep]`)
	os.Exit(2)
}

type options struct {
	goBin, out, logPath string
	keep                bool
}

func parse(name string, args []string, extra func(*flag.FlagSet)) options {
	var o options
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.StringVar(&o.goBin, "go", "go", "the go command to run; it runs with GOTOOLCHAIN=local")
	fs.StringVar(&o.out, "out", "", "write the Markdown report to this file instead of stdout")
	fs.StringVar(&o.logPath, "log", "", "write every command and its full output to this file")
	fs.BoolVar(&o.keep, "keep", false, "keep the work directory")
	if extra != nil {
		extra(fs)
	}
	check(fs.Parse(args))
	return o
}

type experiment struct {
	opts  options
	work  string
	log   bytes.Buffer
	names map[string]string // local server address: its name in the report
}

func newExperiment(o options) *experiment {
	work, err := os.MkdirTemp("", "dirloom-goinstall-")
	check(err)
	work, err = filepath.EvalSymlinks(work)
	check(err)
	write(filepath.Join(work, "empty-gitconfig"), "")
	running = &experiment{opts: o, work: work, names: map[string]string{}}
	return running
}

func (x *experiment) finish(report string) {
	if !x.opts.keep {
		_ = os.RemoveAll(x.work)
	}
	if x.opts.logPath != "" {
		check(os.WriteFile(x.opts.logPath, x.log.Bytes(), 0o644))
	}
	if x.opts.out == "" {
		fmt.Print(report)
		return
	}
	check(os.WriteFile(x.opts.out, []byte(report), 0o644))
}

func simulate(args []string) {
	x := newExperiment(parse("simulate", args, nil))
	// Keep the host Git environment and configuration away from the
	// repositories and from modzip.CreateFromVCS. An inherited GIT_DIR or
	// GIT_WORK_TREE, as in a Git hook, would point the commits of buildRepo at
	// the host repository.
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(strings.ToUpper(k), "GIT_") {
			check(os.Unsetenv(k))
		}
	}
	check(os.Setenv("GIT_CONFIG_NOSYSTEM", "1"))
	check(os.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(x.work, "empty-gitconfig")))

	var r strings.Builder
	fmt.Fprintf(&r, "# `go install` simulation results\n\n")
	fmt.Fprintf(&r, "Generated by `go run . simulate` (`tools/goinstall`) on %s.\n\n", time.Now().UTC().Format("2006-01-02"))
	x.header(&r)
	fmt.Fprintf(&r, "- Network: none; every command has a fresh GOPATH, module cache and GOBIN\n\n")
	fmt.Fprintf(&r, "A cell gives the exit status, then the versions listed, the version selected, or the module version and the output of the installed binary; for a failure, the last error line. `<work>` is the work directory; `<proxy-all>`, `<proxy-zippable>` and `<sumdb>` are the local servers.\n")

	for _, v := range variants {
		x.logf("\n######## variant %s: %s\n", v.name, v.title)
		dir := filepath.Join(x.work, v.name)
		repo := filepath.Join(dir, "repo")
		tags := buildRepo(repo, v)
		store := buildStore(filepath.Join(dir, "store"), repo, tags)
		proxyAll := serveProxy(writeProxy(filepath.Join(dir, "proxy-all"), store, tags, false))
		proxyZippable := serveProxy(writeProxy(filepath.Join(dir, "proxy-zippable"), store, tags, true))
		vkey, sumdbSrv := serveSumDB(store, v.name)
		x.names = map[string]string{proxyAll.addr: "<proxy-all>", proxyZippable.addr: "<proxy-zippable>", sumdbSrv.addr: "<sumdb>"}
		goproxy := map[string]string{
			"proxy-all":      "http://" + proxyAll.addr + ",direct",
			"proxy-zippable": "http://" + proxyZippable.addr + ",direct",
			"proxy-all-only": "http://" + proxyAll.addr,
			"direct":         "direct",
		}

		fmt.Fprintf(&r, "\n## %s\n\n", v.title)
		fmt.Fprintf(&r, "Tags: %s. Unzippable: %s.\n\n", strings.Join(tagNames(tags), " "), strings.Join(unzippableNames(store, tags), " "))
		results := map[string][]outcome{}
		for _, m := range simModes {
			for i, s := range steps(v.target) {
				run := filepath.Join(dir, "runs", m.name, fmt.Sprint(i))
				env := x.simEnv(run, repo, goproxy[m.name], vkey, sumdbSrv.addr)
				results[m.name] = append(results[m.name], x.goCommand(run, env, s))
			}
		}
		table(&r, simModes, steps(v.target), results)
		for _, srv := range []server{proxyAll, proxyZippable, sumdbSrv} {
			srv.close()
		}
		x.names = map[string]string{}
	}
	x.finish(r.String())
}

func checkPublished(args []string) {
	var version string
	o := parse("check", args, func(fs *flag.FlagSet) {
		fs.StringVar(&version, "version", "", "the published version to check, for example v0.4.0-rc1")
	})
	if !semver.IsValid(version) || semver.Canonical(version) != version {
		fmt.Fprintln(os.Stderr, "goinstall check: -version must be a canonical version such as v0.4.0-rc1")
		os.Exit(2)
	}
	x := newExperiment(o)
	var r strings.Builder
	fmt.Fprintf(&r, "# `go install` check of %s\n\n", version)
	fmt.Fprintf(&r, "Generated by `go run . check -version %s` (`tools/goinstall`) at %s.\n\n", version, time.Now().UTC().Format(time.RFC3339))
	x.header(&r)
	fmt.Fprintf(&r, "- Environment: `GOENV=off`, `GOFLAGS=-modcacherw` only, no `GONOSUMDB`, `GOPRIVATE`, `GONOPROXY` or `GOINSECURE`, `GOSUMDB=sum.golang.org`, the Git global configuration ignored; every command has a fresh GOPATH, module cache and GOBIN\n\n")
	fmt.Fprintf(&r, "A cell gives the exit status, then the versions listed, the version selected, or the module version and the output of `dirloom --version` of the installed binary; for a failure, the last error line. `<work>` is the work directory.\n\n")
	results := map[string][]outcome{}
	goproxy := map[string]string{"public": "https://proxy.golang.org,direct", "direct": "direct"}
	for _, m := range checkModes {
		for i, s := range steps(version) {
			run := filepath.Join(x.work, m.name, fmt.Sprint(i))
			env := x.checkEnv(run, goproxy[m.name])
			results[m.name] = append(results[m.name], x.goCommand(run, env, s))
		}
	}
	table(&r, checkModes, steps(version), results)

	fmt.Fprintf(&r, "\n## Verdict\n\n")
	named := len(steps(version)) - 2
	pass := true
	for _, m := range checkModes {
		o := results[m.name][named]
		state := "**installs**"
		if !o.ok {
			state = "**does not install**"
			pass = false
		}
		fmt.Fprintf(&r, "- %s: `go install …@%s` %s.\n", m.title, version, state)
	}
	for _, m := range checkModes {
		o := results[m.name][named+1]
		fmt.Fprintf(&r, "- %s: `go install …@latest` exits %d: %s.\n", m.title, o.exit, o.summary)
	}
	if pass {
		fmt.Fprintf(&r, "\nThe named version installs in both modes.\n")
	} else {
		fmt.Fprintf(&r, "\nThe named version does not install in every mode: A14 stays open, and the result goes to the CTO.\n")
	}
	fmt.Fprintf(&r, "\n`@latest` does not have to install %s; a failing `@latest` needs an explicit CTO arbitration.\n", version)
	x.finish(r.String())
	if !pass {
		os.Exit(1)
	}
}

func (x *experiment) header(r *strings.Builder) {
	env := cleanEnv(map[string]string{"GOENV": "off", "GOTOOLCHAIN": "local"}, true)
	fmt.Fprintf(r, "- Go: `%s`, run with `GOTOOLCHAIN=local`\n", strings.TrimSpace(x.output(env, x.opts.goBin, "version")))
	fmt.Fprintf(r, "- Git: `%s`\n", strings.TrimSpace(x.output(env, "git", "--version")))
	fmt.Fprintf(r, "- Host: `%s/%s`\n", runtime.GOOS, runtime.GOARCH)
}

func table(r *strings.Builder, modes []mode, ss []step, results map[string][]outcome) {
	fmt.Fprintf(r, "| Command |")
	for _, m := range modes {
		fmt.Fprintf(r, " %s |", m.title)
	}
	fmt.Fprintf(r, "\n| --- |%s\n", strings.Repeat(" --- |", len(modes)))
	for i, s := range ss {
		fmt.Fprintf(r, "| %s |", s.title)
		for _, m := range modes {
			o := results[m.name][i]
			fmt.Fprintf(r, " %d: %s |", o.exit, cell(o.summary))
		}
		fmt.Fprintln(r)
	}
}

func (x *experiment) commonEnv(run, goproxy string) map[string]string {
	gopath := filepath.Join(run, "gopath")
	return map[string]string{
		"GOENV":       "off",
		"GOTOOLCHAIN": "local",
		"GOWORK":      "off",
		"GOFLAGS":     "-modcacherw",
		"GO111MODULE": "on",
		"GOPATH":      gopath,
		"GOMODCACHE":  filepath.Join(gopath, "pkg", "mod"),
		"GOBIN":       filepath.Join(run, "bin"),
		"GOCACHE":     filepath.Join(x.work, "gocache"),
		"GOPROXY":     goproxy,
		// The host Git configuration could rewrite or redirect URLs.
		"GIT_CONFIG_GLOBAL":   filepath.Join(x.work, "empty-gitconfig"),
		"GIT_TERMINAL_PROMPT": "0",
	}
}

func (x *experiment) simEnv(run, repo, goproxy, vkey, sumdbAddr string) []string {
	set := x.commonEnv(run, goproxy)
	set["CGO_ENABLED"] = "0"
	set["GOSUMDB"] = vkey + " http://" + sumdbAddr
	for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"} {
		set[k] = deadProxy
	}
	set["GIT_CONFIG_NOSYSTEM"] = "1"
	// direct: fetch https://github.com/dirloom/dirloom from the local repository.
	set["GIT_CONFIG_COUNT"] = "1"
	set["GIT_CONFIG_KEY_0"] = "url." + fileURL(repo) + ".insteadOf"
	set["GIT_CONFIG_VALUE_0"] = "https://" + modPath
	return cleanEnv(set, false)
}

func (x *experiment) checkEnv(run, goproxy string) []string {
	set := x.commonEnv(run, goproxy)
	set["GOSUMDB"] = "sum.golang.org"
	return cleanEnv(set, true)
}

// goCommand runs one step in env and summarizes its outcome.
func (x *experiment) goCommand(run string, env []string, s step) outcome {
	cwd := filepath.Join(run, "cwd")
	check(os.MkdirAll(cwd, 0o755))
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, x.opts.goBin, s.args...)
	cmd.Dir = cwd
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exit := 0
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			check(err)
		}
		exit = ee.ExitCode()
	}
	x.logf("\n$ GOPROXY=%s go %s\n[exit %d]\n%s%s", x.clean(envValue(env, "GOPROXY")), strings.Join(s.args, " "), exit, x.clean(stdout.String()), x.clean(stderr.String()))

	switch {
	case s.args[0] == "mod":
		var d struct{ Version, Error string }
		if json.Unmarshal(stdout.Bytes(), &d) == nil {
			if d.Error != "" {
				return outcome{exit: exit, summary: x.clean(d.Error)}
			}
			return outcome{exit: exit, summary: "downloaded " + d.Version}
		}
		return outcome{exit: exit, summary: x.lastError(stderr.String())}
	case exit != 0:
		return outcome{exit: exit, summary: x.lastError(stderr.String())}
	case s.install != "":
		return x.installed(envValue(env, "GOBIN"), env, s.install)
	default:
		return outcome{exit: exit, summary: strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(stdout.String()), modPath))}
	}
}

// installed reports the module version recorded in the installed binary and
// what `dirloom --version` prints.
func (x *experiment) installed(gobin string, env []string, want string) outcome {
	bin := filepath.Join(gobin, "dirloom")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, x.opts.goBin, "version", "-m", bin)
	cmd.Env = env
	meta, err := cmd.Output()
	if err != nil {
		return outcome{summary: x.clean("installed, but `go version -m` failed: " + err.Error())}
	}
	modVersion := "?"
	for _, line := range strings.Split(string(meta), "\n") {
		f := strings.Fields(line)
		if len(f) >= 3 && f[0] == "mod" && f[1] == modPath {
			modVersion = f[2]
		}
	}
	printed, err := exec.CommandContext(ctx, bin, "--version").Output()
	if err != nil {
		return outcome{summary: x.clean("installed " + modVersion + ", but `dirloom --version` failed: " + err.Error())}
	}
	text := strings.TrimSpace(string(printed))
	ok := modVersion != "?" && strings.Contains(text, strings.TrimPrefix(modVersion, "v"))
	if want != "latest" && modVersion != want {
		ok = false
	}
	return outcome{summary: fmt.Sprintf("installed %s, prints `%s`", modVersion, text), ok: ok}
}

// lastError returns the last "go: " error line, followed by the lines after
// it, such as the response of a server.
func (x *experiment) lastError(stderr string) string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(stderr), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "go: downloading") || strings.HasPrefix(line, "go: finding") {
			continue
		}
		if strings.HasPrefix(line, "go: ") {
			lines = lines[:0]
		}
		lines = append(lines, line)
	}
	return x.clean(strings.Join(lines, " "))
}

func (x *experiment) clean(s string) string {
	jsonEscaped := strings.ReplaceAll(x.work, `\`, `\\`) // paths in `go mod download -json`
	for _, p := range []string{jsonEscaped, x.work, filepath.ToSlash(x.work), strings.TrimPrefix(fileURL(x.work), "file://")} {
		s = strings.ReplaceAll(s, p, "<work>")
	}
	// Longest first: 127.0.0.1:5000 is a prefix of 127.0.0.1:50001.
	addrs := make([]string, 0, len(x.names))
	for addr := range x.names {
		addrs = append(addrs, addr)
	}
	sort.Slice(addrs, func(i, j int) bool { return len(addrs[i]) > len(addrs[j]) })
	for _, addr := range addrs {
		s = strings.ReplaceAll(s, "http://"+addr, x.names[addr])
		s = strings.ReplaceAll(s, addr, x.names[addr])
	}
	return s
}

func (x *experiment) output(env []string, name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	out, err := cmd.Output()
	check(err)
	return string(out)
}

func (x *experiment) logf(format string, args ...any) {
	fmt.Fprintf(&x.log, format, args...)
}

// buildRepo creates the throwaway repository and returns its tags in order.
func buildRepo(repo string, v variant) []release {
	check(os.MkdirAll(repo, 0o755))
	git(repo, "", "init", "-q", "-b", "main")
	tags := append([]release(nil), history...)
	if v.rc1 {
		tags = append(tags, release{rc1, "2026-11-28T12:00:00Z", 0})
	}
	if v.stable {
		tags = append(tags, release{"v0.4.0", "2026-12-15T12:00:00Z", 0})
	}
	for _, t := range tags {
		gomod := "module " + modPath + "\n\ngo 1.22\n"
		if (t.version == rc1 || t.version == "v0.4.0") && v.retract != "" {
			gomod += "\n" + v.retract
		}
		write(filepath.Join(repo, "go.mod"), gomod)
		write(filepath.Join(repo, "cmd", "dirloom", "main.go"), fmt.Sprintf(
			"package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"dirloom %s\") }\n",
			strings.TrimPrefix(t.version, "v")))
		for i, name := range unzippableFiles {
			path := filepath.Join(repo, filepath.FromSlash(name))
			if i < t.plans {
				write(path, "# plan\n")
			} else {
				_ = os.Remove(path)
			}
		}
		git(repo, t.date, "add", "-A")
		git(repo, t.date, "commit", "-q", "-m", "release "+t.version)
		git(repo, t.date, "tag", "-a", "-m", t.version, t.version)
	}
	return tags
}

// buildStore writes .info, .mod and .zip for every zippable tag, as a module
// proxy holds them.
func buildStore(store, repo string, tags []release) string {
	dir := filepath.Join(store, filepath.FromSlash(modPath), "@v")
	check(os.MkdirAll(dir, 0o755))
	for _, t := range tags {
		var zip bytes.Buffer
		err := modzip.CreateFromVCS(&zip, module.Version{Path: modPath, Version: t.version}, repo, t.version, "")
		// Only the refused file names may make a zip fail: any other failure
		// is a broken environment, not a result.
		if (err != nil) != (t.plans > 0) {
			check(fmt.Errorf("%s: zip error %v, but %d refused file names", t.version, err, t.plans))
		}
		if err != nil {
			continue // like proxy.golang.org: no .info, .mod or .zip
		}
		write(filepath.Join(dir, t.version+".zip"), zip.String())
		write(filepath.Join(dir, t.version+".mod"), git(repo, "", "show", t.version+":go.mod"))
		info, err := json.Marshal(struct{ Version, Time string }{t.version, t.date})
		check(err)
		write(filepath.Join(dir, t.version+".info"), string(info)+"\n")
	}
	return dir
}

// writeProxy copies the store into a proxy whose list holds every tag, or the
// zippable ones only.
func writeProxy(root, store string, tags []release, zippableOnly bool) string {
	dir := filepath.Join(root, filepath.FromSlash(modPath), "@v")
	check(os.MkdirAll(dir, 0o755))
	entries, err := os.ReadDir(store)
	check(err)
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(store, e.Name()))
		check(err)
		write(filepath.Join(dir, e.Name()), string(data))
	}
	var list []string
	for _, t := range tags {
		if zippableOnly && !exists(filepath.Join(store, t.version+".zip")) {
			continue
		}
		list = append(list, t.version)
	}
	write(filepath.Join(dir, "list"), strings.Join(list, "\n")+"\n")
	return root
}

type server struct {
	addr  string
	close func()
}

func serve(h http.Handler) server {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	check(err)
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	return server{addr: ln.Addr().String(), close: func() { _ = srv.Close() }}
}

// serveProxy serves a proxy directory over HTTP. A missing file is 404 Not
// Found, as proxy.golang.org answers for a version it cannot zip, so that the
// go command falls back to the next entry of GOPROXY. http.Dir keeps every
// request inside root.
func serveProxy(root string) server {
	return serve(http.FileServer(http.Dir(root)))
}

// serveSumDB serves a checksum database on 127.0.0.1 that has a record for a
// zippable version of the module only, and answers 404 otherwise, like
// sum.golang.org. It returns the verifier key for GOSUMDB.
func serveSumDB(store, name string) (string, server) {
	skey, vkey, err := note.GenerateKey(rand.Reader, "sumdb."+name+".test")
	check(err)
	gosum := func(path, version string) ([]byte, error) {
		zipFile := filepath.Join(store, version+".zip")
		if path != modPath || !exists(zipFile) {
			// A *fs.PathError, which the server reports as 404 Not Found.
			return nil, &fs.PathError{Op: "not found: no module zip for", Path: path + "@" + version, Err: fs.ErrNotExist}
		}
		zh, err := dirhash.HashZip(zipFile, dirhash.Hash1)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(filepath.Join(store, version+".mod"))
		if err != nil {
			return nil, err
		}
		mh, err := dirhash.Hash1([]string{"go.mod"}, func(string) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		})
		if err != nil {
			return nil, err
		}
		return []byte(fmt.Sprintf("%s %s %s\n%s %s/go.mod %s\n", path, version, zh, path, version, mh)), nil
	}
	return vkey, serve(sumdb.NewServer(sumdb.NewTestServer(skey, gosum)))
}

func git(repo, date string, args ...string) string {
	cmd := exec.Command("git", append([]string{"-c", "core.autocrlf=false", "-c", "core.quotepath=false"}, args...)...)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Dirloom Experiment", "GIT_AUTHOR_EMAIL=experiment@example.invalid",
		"GIT_COMMITTER_NAME=Dirloom Experiment", "GIT_COMMITTER_EMAIL=experiment@example.invalid",
	)
	if date != "" {
		cmd.Env = append(cmd.Env, "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		check(fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out))
	}
	return string(out)
}

// cleanEnv keeps the host environment minus every Go and Git setting, and minus
// the network proxy settings unless keepNetProxy, then adds set.
func cleanEnv(set map[string]string, keepNetProxy bool) []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		u := strings.ToUpper(k)
		if strings.HasPrefix(u, "GO") || strings.HasPrefix(u, "GIT_") || u == "CGO_ENABLED" {
			continue
		}
		if !keepNetProxy && (strings.HasSuffix(u, "_PROXY") || u == "NO_PROXY") {
			continue
		}
		env = append(env, kv)
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		env = append(env, k+"="+set[k])
	}
	return env
}

func envValue(env []string, key string) string {
	value := ""
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			value = v
		}
	}
	return value
}

func tagNames(tags []release) []string {
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.version
	}
	return names
}

func unzippableNames(store string, tags []release) []string {
	var names []string
	for _, t := range tags {
		if !exists(filepath.Join(store, t.version+".zip")) {
			names = append(names, t.version)
		}
	}
	return names
}

func fileURL(path string) string {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return "file://" + p
}

func cell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\n", " ")
	if r := []rune(s); len(r) > 600 {
		s = string(r[:600]) + "…"
	}
	return s
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func write(path, data string) {
	check(os.MkdirAll(filepath.Dir(path), 0o755))
	check(os.WriteFile(path, []byte(data), 0o644))
}

// running is the experiment in progress, whose log check writes on a tool
// error.
var running *experiment

// check stops the tool on its own error, with exit status 3: status 1 is a
// version that does not install, and 2 a usage error. The work directory is
// kept for inspection.
func check(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "goinstall:", err)
	if x := running; x != nil {
		fmt.Fprintln(os.Stderr, "goinstall: work directory kept:", x.work)
		if x.opts.logPath != "" {
			_ = os.WriteFile(x.opts.logPath, x.log.Bytes(), 0o644)
		}
	}
	os.Exit(3)
}
