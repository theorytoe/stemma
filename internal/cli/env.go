package cli

import (
	"context"
	"flag"
	"fmt"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/theorytoe/stemma/internal/extract"
	"github.com/theorytoe/stemma/internal/kb"
)

// envReport is what the environment looks like. Everything in it is optional
// except Go itself, and a missing piece is a fact to report rather than a failure.
type envReport struct {
	Go         goInfo      `json:"go"`
	KB         string      `json:"kb,omitempty"`
	Python     toolInfo    `json:"python"`
	Extraction extraction  `json:"extraction"`
	SQLite     bool        `json:"sqlite"`
	Index      *IndexState `json:"index,omitempty"`
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
	Why       string `json:"why,omitempty"`
}

// extraction is what this machine would read documents with: where the script is,
// whether it is the one this binary carries, and what the interpreter can import.
//
// The libraries come from the shim's own probe rather than from this command
// importing them for itself. "Which libraries are importable" is a question only
// the interpreter can answer, and answering it any other way would be a second
// opinion that could disagree with the one that decides whether a fetch works.
type extraction struct {
	// Ready is whether an extraction could run at all.
	Ready bool `json:"ready"`
	// Script is the shim's path, and Condition whether it matches this binary. A
	// copy made for the occasion is named as such, since its path is about to stop
	// existing.
	Script    string `json:"script,omitempty"`
	Condition string `json:"condition,omitempty"`
	Temporary bool   `json:"temporary,omitempty"`
	// Libraries maps a module to its version. A module that is not importable is
	// listed with no version, which is what the probe means by it.
	Libraries map[string]string `json:"libraries,omitempty"`
	// Why is why nothing can be read, in the words the failure itself would use, so
	// the same sentence appears here as at the point of failure.
	Why string `json:"why,omitempty"`
}

// envCommand implements `stemma env`.
//
// Its job is to make a degraded environment diagnosable rather than mysterious:
// Python is a leaf (`P3`, `D6`), so a missing interpreter or a missing library has
// to be explainable without reading a document, and without the reader having to
// guess which of the two is missing.
var envCommand = &command{
	name:    "env",
	summary: "report the optional parts of the environment",
	setup: func(fs *flag.FlagSet, o *options) runFunc {
		o.registerKB(fs)
		return func(c *command, w *output, args []string) int {
			if len(args) > 0 {
				return w.fail(fmt.Errorf("env takes no arguments, got %q", args[0]))
			}

			report := envReport{
				Go: goInfo{Version: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH},
			}

			// A KB is optional here: env reports on the environment first, and adds
			// the shim's own location and the index state when it can find one.
			root := ""
			if found, err := discover(o.kb); err == nil {
				root = found
				report.KB = found
				if k, err := kb.Load(root); err == nil {
					// Reported whether or not there is one: "no index" is the answer a
					// reader of `stemma env` is looking for, and an absent field is not.
					state := readIndexState(k)
					report.Index = &state
				}
			}

			report.Python, report.Extraction = probe(root)
			report.SQLite = sqliteLinked()

			if w.json {
				return w.emit(report)
			}
			fmt.Fprint(w.stdout, report.text())
			return ExitOK
		}
	},
}

// probe answers the two questions env is built around: is there an interpreter,
// and what can it see.
//
// Nothing here is allowed to fail the command. A missing interpreter, a script
// that cannot be placed, and a shim that dies are all answers to the question env
// was asked, and each is reported in the words the failure itself would use.
func probe(root string) (toolInfo, extraction) {
	python, pythonErr := extract.FindPython()
	info := toolInfo{Available: pythonErr == nil, Path: python}
	if pythonErr != nil {
		info.Why = "not found"
	}

	out := extraction{}
	script, scriptErr := extract.ScriptFor(root)
	if scriptErr == nil {
		defer script.Cleanup()
		out.Script = script.Path
		out.Condition = extract.ConditionOf(script.Path).String()
		out.Temporary = script.Temporary
	}

	switch {
	case pythonErr != nil:
		out.Why = reason(pythonErr)
		return info, out
	case scriptErr != nil:
		out.Why = "the extraction script could not be placed: " + scriptErr.Error()
		return info, out
	}

	seen, err := (&extract.Runner{Python: python, Script: script.Path}).Probe(context.Background())
	if err != nil {
		out.Why = err.Error()
		return info, out
	}
	out.Ready = true
	out.Libraries = seen.Libs
	info.Version = seen.Python
	return info, out
}

// reason is an error's message with its class trimmed off. env prints the class as
// its own column, and repeating it in the sentence beside it reads as a stutter.
func reason(err error) string {
	if _, message, found := strings.Cut(err.Error(), ": "); found {
		return message
	}
	return err.Error()
}

func (r envReport) text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-8s%s %s/%s\n", "go", r.Go.Version, r.Go.OS, r.Go.Arch)

	if r.KB == "" {
		fmt.Fprintf(&b, "%-8s%s\n", "kb", "no KB found")
	} else {
		fmt.Fprintf(&b, "%-8s%s\n", "kb", r.KB)
	}

	if r.Python.Available {
		fmt.Fprintf(&b, "%-8s%s %s\n", "python", r.Python.Path, r.Python.Version)
	} else {
		fmt.Fprintf(&b, "%-8s%s\n", "python", "not found")
	}

	switch {
	case r.Extraction.Script == "":
		fmt.Fprintf(&b, "%-8s%s\n", "shim", "not available")
	case r.Extraction.Temporary:
		fmt.Fprintf(&b, "%-8s%s\n", "shim", "a temporary copy, usually because there is no KB")
	default:
		fmt.Fprintf(&b, "%-8s%s %s\n", "shim", r.Extraction.Script, r.Extraction.Condition)
	}

	switch {
	case len(r.Extraction.Libraries) > 0:
		fmt.Fprintf(&b, "%-8s%s\n", "reads", libraries(r.Extraction.Libraries))
	case r.Extraction.Why != "":
		fmt.Fprintf(&b, "%-8s%s\n", "reads", r.Extraction.Why)
	default:
		fmt.Fprintf(&b, "%-8s%s\n", "reads", "nothing")
	}

	if r.SQLite {
		fmt.Fprintf(&b, "%-8s%s\n", "sqlite", "linked in this build")
	} else {
		fmt.Fprintf(&b, "%-8s%s\n", "sqlite", "not linked in this build")
	}

	// The index is only a question once there is a KB for it to be in.
	switch {
	case r.KB == "":
	case r.Index == nil || !r.Index.Present:
		fmt.Fprintf(&b, "%-8s%s\n", "index", "absent")
	case r.Index.Fresh:
		fmt.Fprintf(&b, "%-8s%s\n", "index", "present and fresh")
	default:
		fmt.Fprintf(&b, "%-8s%s\n", "index", "present but stale")
	}
	return b.String()
}

// libraries names the modules the interpreter could import, and says which ones it
// could not, because "bs4 is missing" is the sentence that explains a failed fetch.
func libraries(libs map[string]string) string {
	names := make([]string, 0, len(libs))
	for name := range libs {
		names = append(names, name)
	}
	sort.Strings(names)

	var have, missing []string
	for _, name := range names {
		if version := libs[name]; version != "" {
			have = append(have, name+" "+version)
		} else {
			missing = append(missing, name)
		}
	}

	switch {
	case len(have) == 0 && len(missing) == 0:
		return "nothing"
	case len(missing) == 0:
		return strings.Join(have, ", ")
	case len(have) == 0:
		return "nothing importable (" + strings.Join(missing, ", ") + " missing)"
	default:
		return strings.Join(have, ", ") + " (" + strings.Join(missing, ", ") + " missing)"
	}
}

// sqliteLinked reports whether the pure-Go SQLite driver is part of this build.
// It is a build-time property: the driver is imported by internal/index, so a
// build that somehow dropped it would say so here rather than fail on the first
// query.
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
