// The one piece of the tour that is not written in Hive, and the reason it
// is not: it has to be able to give up on a build.
//
// Running an example in the reader's browser means compiling it here first,
// with the same `hive build` a reader would type, for `js/wasm` rather than for
// this machine. `hive.term.exec` would run that command perfectly well, but it
// has no deadline, and `with timeout` abandons the waiting rather than the
// work: a build that never finished would hold the one build slot for good.
// Go's `exec.CommandContext` kills the compiler when the deadline passes,
// which is the whole of what this file is for.
//
// Everything about *what* may be compiled — which imports, how long a
// program, which examples get a Run button at all — is decided in main.hive
// before anything reaches here. This file only builds what it is handed.
package compile

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// One build at a time. A build takes most of a CPU for about a second, and
// the machine serving the tour has one, so a second build alongside the first
// would only make both slower while the pages wait behind them.
var slot = make(chan struct{}, 1)

// What a build came to: the program, or what the compiler had to say about
// it. Exactly one of the two is ever set.
type Outcome struct {
	Wasm     string
	Problems string
}

// Wasm builds one program for the browser. It waits up to `waitMs` for the
// build slot and gives the compiler `buildMs` once it has it; an error means
// the build never got to an answer — the slot stayed busy, the deadline
// passed, or the compiler could not be started — and is not the reader's
// fault, where a program that does not compile is an Outcome like any other.
func Wasm(source string, waitMs int, buildMs int) (Outcome, error) {
	select {
	case slot <- struct{}{}:
		defer func() { <-slot }()
	case <-time.After(time.Duration(waitMs) * time.Millisecond):
		return Outcome{}, errors.New("busy")
	}

	dir, err := os.MkdirTemp("", "hive-tour-")
	if err != nil {
		return Outcome{}, err
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, "main.hive"), []byte(source), 0o600); err != nil {
		return Outcome{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(buildMs)*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "hive", "build", "main.hive", "--target", "js/wasm")
	cmd.Dir = dir
	cmd.Env = buildEnvironment()
	// Killing the compiler leaves the Go toolchain it started holding the
	// output pipe open; without a delay to give up after, reading the output
	// would wait for that to finish too.
	cmd.WaitDelay = 2 * time.Second

	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return Outcome{}, errors.New("timeout")
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return Outcome{Problems: string(out)}, nil
	}
	if err != nil {
		return Outcome{}, err
	}

	wasm, err := os.ReadFile(filepath.Join(dir, "main-js-wasm"))
	if err != nil {
		return Outcome{}, err
	}
	return Outcome{Wasm: string(wasm)}, nil
}

// What the compiler is allowed to see of this server's environment: enough
// to find itself, Go, and Go's caches, and nothing else. Whatever this server
// was started with — a secret handed over with `fly secrets` included — stays
// out of reach of a program that might find a way to read its own build's
// environment back. The last three close the doors a build could otherwise go
// out of: no module downloads, no toolchain downloads, and no git prompts.
func buildEnvironment() []string {
	keep := []string{
		"PATH", "HOME", "USERPROFILE", "LOCALAPPDATA", "APPDATA", "SYSTEMROOT",
		"TMPDIR", "TEMP", "TMP", "GOROOT", "GOPATH", "GOCACHE", "GOMODCACHE",
	}
	var env []string
	for _, pair := range os.Environ() {
		name, _, _ := strings.Cut(pair, "=")
		for _, k := range keep {
			if strings.EqualFold(name, k) {
				env = append(env, pair)
				break
			}
		}
	}
	return append(env, "GOPROXY=off", "GOTOOLCHAIN=local", "GIT_TERMINAL_PROMPT=0")
}
