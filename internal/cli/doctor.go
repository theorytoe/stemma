package cli

import (
	"context"
	"flag"
	"fmt"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/theorytoe/stemma/internal/kb"
)

// doctorReport is what the environment looks like. Everything in it is
// optional except Go itself, and a missing piece is a fact to report rather
// than a failure.
type doctorReport struct {
	Go     goInfo      `json:"go"`
	Python toolInfo    `json:"python"`
	PDF    toolInfo    `json:"pdf"`
	SQLite bool        `json:"sqlite"`
	Index  *indexState `json:"index,omitempty"`
}

type goInfo struct {
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

// toolInfo describes an external piece the tool may or may not have. Available
// is the whole question; the rest says which one and why.
type toolInfo struct {
	Available bool   `json:"available"`
	Path      string `json:"path,omitempty"`
	Version   string `json:"version,omitempty"`
	Module    string `json:"module,omitempty"`
	Why       string `json:"why,omitempty"`
}

// doctorCommand implements `stemma doctor`.
//
// Its job is to make a degraded environment diagnosable rather than mysterious:
// Python is a leaf (P3, D6), so a missing interpreter or PDF library should be
// explainable without running an extraction. It stays small on purpose; a
// command that grew into a dependency manager would be cut (U4).
var doctorCommand = &command{
	name:    "doctor",
	summary: "report the optional parts of the environment",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) > 0 {
				return w.fail(fmt.Errorf("doctor takes no arguments, got %q", args[0]))
			}

			report := doctorReport{
				Go: goInfo{Version: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH},
			}

			// A KB is optional here: doctor reports on the environment first,
			// and adds the index state when it can find one.
			if root, err := discover(o.kb); err == nil {
				if k, err := kb.Load(root); err == nil {
					if state := readIndexState(k.Root); state.Present {
						report.Index = &state
					}
				}
			}

			report.Python = findPython()
			if !report.Python.Available {
				report.PDF = toolInfo{Why: "no Python interpreter"}
			} else {
				report.PDF = findPDF(report.Python.Path)
			}
			report.SQLite = sqliteLinked()

			if w.json {
				return w.emit(report)
			}
			fmt.Fprint(w.stdout, report.text())
			return ExitOK
		}
	},
}

func (r doctorReport) text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-8s%s %s/%s\n", "go", r.Go.Version, r.Go.OS, r.Go.Arch)

	if r.Python.Available {
		fmt.Fprintf(&b, "%-8s%s %s\n", "python", r.Python.Path, r.Python.Version)
	} else {
		fmt.Fprintf(&b, "%-8s%s\n", "python", "not found")
	}

	switch {
	case r.PDF.Available:
		fmt.Fprintf(&b, "%-8s%s %s\n", "pdf", r.PDF.Module, r.PDF.Version)
	case r.PDF.Why != "":
		fmt.Fprintf(&b, "%-8s%s\n", "pdf", r.PDF.Why)
	default:
		fmt.Fprintf(&b, "%-8s%s\n", "pdf", "no extractor importable")
	}

	if r.SQLite {
		fmt.Fprintf(&b, "%-8s%s\n", "sqlite", "linked in this build")
	} else {
		fmt.Fprintf(&b, "%-8s%s\n", "sqlite", "not linked in this build")
	}

	if r.Index == nil {
		fmt.Fprintf(&b, "%-8s%s\n", "index", "no KB found")
		return b.String()
	}
	switch {
	case r.Index.Fresh:
		fmt.Fprintf(&b, "%-8s%s\n", "index", "present and fresh")
	default:
		fmt.Fprintf(&b, "%-8s%s\n", "index", "present but stale")
	}
	return b.String()
}

// findPython locates the interpreter the extraction shim would use. Python is
// never a dependency of the core; this only reports whether it is there.
func findPython() toolInfo {
	for _, name := range []string{"python3", "python"} {
		p, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		info := toolInfo{Available: true, Path: p}
		if out, err := runTool(p, "--version"); err == nil {
			info.Version = strings.TrimSpace(string(out))
		}
		return info
	}
	return toolInfo{Available: false, Why: "not found"}
}

// findPDF asks Python whether a PDF extractor is importable. The shim belongs
// to a later task; until then the import check is the honest answer to "would
// extraction work here".
func findPDF(python string) toolInfo {
	for _, module := range []string{"pymupdf", "pypdf"} {
		out, err := runTool(python, "-c", "import "+module+", sys; print(getattr("+module+", \"__version__\", \"\"))")
		if err != nil {
			continue
		}
		return toolInfo{Available: true, Module: module, Version: strings.TrimSpace(string(out))}
	}
	return toolInfo{Why: "python3 found, but no known extractor is importable"}
}

// sqliteLinked reports whether the pure-Go SQLite driver is part of this build.
// It is a build-time property: the driver is behind the dependency-pinning tag
// until the index task imports it for real.
func sqliteLinked() bool {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return false
	}
	for _, dep := range info.Deps {
		if dep.Path == "modernc.org/sqlite" {
			return true
		}
	}
	return false
}

// run executes a short diagnostic command with a timeout, so doctor can never
// hang on a misbehaving interpreter.
func runTool(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}
