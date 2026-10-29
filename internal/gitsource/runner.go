package gitsource

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
)

// Every invocation carries these arguments and environment variables. They
// come after the inherited environment, so they win.
var (
	hardenedArguments   = []string{"-c", "protocol.allow=never"}
	hardenedEnvironment = []string{
		"GIT_NO_LAZY_FETCH=1",
		"GIT_NO_REPLACE_OBJECTS=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
	}
)

// waitDelay is the grace a git process gets to exit after its input is
// closed or its context is cancelled, before it is killed.
var waitDelay = 2 * time.Second

// maxCommandOutput bounds the standard output of a short command.
const maxCommandOutput = 64 << 10

var errUnexpectedOutput = errors.New("unexpected git output")

// Runner runs git for one repository. It never uses a shell.
type Runner struct {
	executable string
	dir        string
}

// NewRunner finds git on PATH, never in the current directory, and checks
// its version once per process and executable. dir is the directory Git
// discovers the repository from; empty means the process working directory.
func NewRunner(ctx context.Context, dir string) (*Runner, error) {
	executable, err := exec.LookPath("git")
	if errors.Is(err, exec.ErrDot) {
		return nil, unavailable("git was found only relative to the current directory, which Dirloom never runs; Dirloom requires git %s or newer on PATH", minimumVersion)
	}
	if err != nil {
		return nil, unavailable("git was not found on PATH; Dirloom requires git %s or newer", minimumVersion)
	}
	runner := &Runner{executable: executable, dir: dir}
	if err := runner.checkVersionOnce(ctx); err != nil {
		return nil, err
	}
	return runner, nil
}

var versionChecks = struct {
	sync.Mutex
	results map[string]error
}{results: make(map[string]error)}

func (r *Runner) checkVersionOnce(ctx context.Context) error {
	versionChecks.Lock()
	defer versionChecks.Unlock()
	if err, ok := versionChecks.results[r.executable]; ok {
		return err
	}
	err := r.checkVersion(ctx)
	if ctx.Err() == nil {
		versionChecks.results[r.executable] = err
	}
	return err
}

func (r *Runner) checkVersion(ctx context.Context) error {
	out, err := r.output(ctx, "version")
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return unavailable("git version failed; Dirloom requires git %s or newer", minimumVersion)
	}
	version, ok := ParseVersion(string(out))
	if !ok {
		return unavailable("the installed git version cannot be parsed; Dirloom requires git %s or newer", minimumVersion)
	}
	if version.Major < minimumMajor || (version.Major == minimumMajor && version.Minor < minimumMinor) {
		return unavailable("git %s is installed; Dirloom requires git %s or newer", version.Text, minimumVersion)
	}
	return nil
}

// Version is a parsed `git version` line.
type Version struct {
	Major int
	Minor int
	// Text is the version token, such as 2.56.0.windows.2.
	Text string
}

// ParseVersion parses the single line printed by `git version`, such as
// "git version 2.56.0.windows.2" or "git version 2.39.5 (Apple Git-154)".
func ParseVersion(output string) (Version, bool) {
	line := strings.TrimSuffix(strings.TrimSuffix(output, "\n"), "\r")
	const prefix = "git version "
	if !strings.HasPrefix(line, prefix) || strings.ContainsAny(line, "\r\n") {
		return Version{}, false
	}
	token := line[len(prefix):]
	if space := strings.IndexByte(token, ' '); space >= 0 {
		token = token[:space]
	}
	if len(token) > 64 {
		return Version{}, false
	}
	for i := 0; i < len(token); i++ {
		if token[i] <= ' ' || token[i] > '~' {
			return Version{}, false
		}
	}
	major, rest, ok := leadingNumber(token)
	if !ok || rest == "" || rest[0] != '.' {
		return Version{}, false
	}
	minor, rest, ok := leadingNumber(rest[1:])
	if !ok || (rest != "" && rest[0] != '.' && rest[0] != '-') {
		return Version{}, false
	}
	return Version{Major: major, Minor: minor, Text: token}, true
}

// leadingNumber reads a decimal of at most 9 digits without a leading zero.
func leadingNumber(text string) (int, string, bool) {
	end := 0
	for end < len(text) && text[end] >= '0' && text[end] <= '9' {
		end++
	}
	if end == 0 || end > 9 || (end > 1 && text[0] == '0') {
		return 0, text, false
	}
	value := 0
	for i := 0; i < end; i++ {
		value = value*10 + int(text[i]-'0')
	}
	return value, text[end:], true
}

// Resolved is the tree a revision names, and the commit it resolved through.
type Resolved struct {
	Tree   string
	Commit string
	Format ObjectFormat
}

// Resolve turns a revision into the tree to observe (Git Tree Source v1,
// Resolution). The revision must already satisfy ValidateRevision.
func (r *Runner) Resolve(ctx context.Context, revision string) (Resolved, error) {
	if err := ValidateRevision(revision); err != nil {
		return Resolved{}, artifact.InternalError("git revision reached resolution unchecked: %v", err)
	}
	format, err := r.objectFormat(ctx)
	if err != nil {
		return Resolved{}, err
	}
	id, err := r.revParse(ctx, revision, format)
	if err != nil {
		return Resolved{}, r.classify(ctx, err, revisionNotFound("the revision does not resolve in this repository"))
	}
	objectType, err := r.objectType(ctx, id)
	if err != nil {
		return Resolved{}, r.classify(ctx, err, revisionNotFound("the revision names object %s, which this repository does not have", id))
	}

	switch objectType {
	case "commit":
		tree, err := r.revParse(ctx, id+"^{tree}", format)
		if err != nil {
			return Resolved{}, r.classify(ctx, err, objectUnavailable("the tree of commit %s cannot be read", id))
		}
		return Resolved{Tree: tree, Commit: id, Format: format}, nil
	case "tag":
		commit, err := r.revParse(ctx, id+"^{commit}", format)
		if err == nil {
			tree, err := r.revParse(ctx, commit+"^{tree}", format)
			if err != nil {
				return Resolved{}, r.classify(ctx, err, objectUnavailable("the tree of commit %s cannot be read", commit))
			}
			return Resolved{Tree: tree, Commit: commit, Format: format}, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Resolved{}, ctxErr
		}
		tree, err := r.revParse(ctx, id+"^{tree}", format)
		if err != nil {
			return Resolved{}, r.classify(ctx, err, revisionNotFound("the revision names tag %s, which points to neither a commit nor a tree", id))
		}
		return Resolved{Tree: tree, Format: format}, nil
	case "tree":
		return Resolved{Tree: id, Format: format}, nil
	default:
		return Resolved{}, revisionNotFound("the revision names a %s, not a commit, tag or tree", objectType)
	}
}

func (r *Runner) objectFormat(ctx context.Context) (ObjectFormat, error) {
	out, err := r.output(ctx, "rev-parse", "--show-object-format")
	if err != nil {
		return ObjectFormat{}, r.classify(ctx, err, repositoryNotFound("not inside a Git repository, or Git refused to open it"))
	}
	format, ok := objectFormatNamed(singleLine(out))
	if !ok {
		return ObjectFormat{}, repositoryNotFound("the repository uses an object format Dirloom does not read")
	}
	return format, nil
}

// revParse resolves one revision expression to a full object id. A revision
// that does not resolve is an *exec.ExitError (`--quiet` prints nothing).
func (r *Runner) revParse(ctx context.Context, revision string, format ObjectFormat) (string, error) {
	out, err := r.output(ctx, "rev-parse", "--verify", "--quiet", revision)
	if err != nil {
		return "", err
	}
	id := singleLine(out)
	if !format.isObjectID(id) {
		return "", errUnexpectedOutput
	}
	return id, nil
}

func (r *Runner) objectType(ctx context.Context, id string) (string, error) {
	out, err := r.output(ctx, "cat-file", "-t", id)
	if err != nil {
		return "", err
	}
	switch objectType := singleLine(out); objectType {
	case "commit", "tag", "tree", "blob":
		return objectType, nil
	default:
		return "", errUnexpectedOutput
	}
}

// classify maps a failed command to its diagnostic. Codes come from exit
// statuses and output shape, never from Git's error text.
func (r *Runner) classify(ctx context.Context, err, semantic error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) || errors.Is(err, errUnexpectedOutput) {
		return semantic
	}
	return unavailable("git could not be run")
}

// output runs one short command and returns its standard output. A non-zero
// exit is an *exec.ExitError. Standard error is discarded.
func (r *Runner) output(ctx context.Context, args ...string) ([]byte, error) {
	// #nosec G204 -- git found on PATH, a fixed hardened argument list, and a revision checked by ValidateRevision.
	cmd := exec.CommandContext(ctx, r.executable, r.arguments(args)...)
	r.configure(cmd)
	cmd.WaitDelay = waitDelay
	stdout := &cappedBuffer{limit: maxCommandOutput}
	cmd.Stdout = stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	if stdout.overflow {
		return nil, errUnexpectedOutput
	}
	return stdout.Bytes(), nil
}

func (r *Runner) arguments(args []string) []string {
	full := make([]string, 0, len(hardenedArguments)+len(args))
	full = append(full, hardenedArguments...)
	return append(full, args...)
}

func (r *Runner) configure(cmd *exec.Cmd) {
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), hardenedEnvironment...)
}

// singleLine returns out without its line terminator.
func singleLine(out []byte) string {
	return strings.TrimSuffix(strings.TrimSuffix(string(out), "\n"), "\r")
}

// cappedBuffer keeps at most limit bytes and records an overflow. It never
// fails a write, so the child process is not killed by a broken pipe.
type cappedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	room := b.limit - b.Len()
	if len(p) > room {
		b.overflow = true
		if room > 0 {
			b.Buffer.Write(p[:room])
		}
		return len(p), nil
	}
	return b.Buffer.Write(p)
}
