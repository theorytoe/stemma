package cli

import (
	"strings"
	"testing"
)

// envReportOf runs env and decodes what it said, so a test can ask about the
// environment rather than about the text.
func envReportOf(t *testing.T, args ...string) envReport {
	t.Helper()
	code, stdout, stderr := run(append([]string{"env", "--json"}, args...)...)
	if code != ExitOK {
		t.Fatalf("env exited %d: %s", code, stderr)
	}
	var report envReport
	decodeData(t, stdout, &report)
	return report
}

func TestEnvNamesTheKBItIsTalkingAbout(t *testing.T) {
	root := fetchKB(t)

	report := envReportOf(t, "--kb", root)
	if report.KB != root {
		t.Errorf("KB = %q, want %q", report.KB, root)
	}
	if report.Go.Version == "" {
		t.Error("no Go version")
	}
	// Nothing has built an index, so a KB with none has to say so rather than
	// inferring one from the generated directory.
	if report.Index == nil || report.Index.Present {
		t.Errorf("Index = %+v, want it reported as absent", report.Index)
	}

	// And the same in words: "present but stale" on a KB that has just been created
	// is how this line read before the field was checked.
	code, stdout, stderr := run("env", "--kb", root)
	if code != ExitOK {
		t.Fatalf("env exited %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "index   absent") {
		t.Errorf("stdout = %q, want the index reported as absent", stdout)
	}
}

// env must reach a verdict on every machine, including one with no interpreter at
// all: that is the machine it exists to explain.
func TestEnvReachesAVerdictEitherWay(t *testing.T) {
	root := fetchKB(t)

	report := envReportOf(t, "--kb", root)
	switch {
	case report.Extraction.Ready:
		if len(report.Extraction.Libraries) == 0 {
			t.Error("extraction is ready but no libraries were reported")
		}
		if report.Python.Version == "" || report.Python.Path == "" {
			t.Errorf("Python = %+v, want the interpreter that was used", report.Python)
		}
	case report.Extraction.Why == "":
		t.Error("extraction is not ready and env gave no reason")
	}
}

func TestEnvSaysWhichInterpreterIsMissing(t *testing.T) {
	root := fetchKB(t)
	t.Setenv("PATH", "")

	code, stdout, stderr := run("env", "--kb", root)
	if code != ExitOK {
		t.Fatalf("env exited %d on a machine with no interpreter: %s", code, stderr)
	}
	if !strings.Contains(stdout, "python  not found") {
		t.Errorf("stdout = %q, want the interpreter reported as missing", stdout)
	}
	// The reason has to be the extraction's own sentence, not a second opinion.
	if !strings.Contains(stdout, "activate the environment that has one") {
		t.Errorf("stdout = %q, want it to say how to fix it", stdout)
	}

	report := envReportOf(t, "--kb", root)
	if report.Python.Available {
		t.Error("Python is reported as available with nothing on PATH")
	}
	if report.Extraction.Ready {
		t.Error("extraction is reported as ready with no interpreter")
	}
	if !strings.Contains(report.Extraction.Why, "python3") {
		t.Errorf("Why = %q, want it to name what is missing", report.Extraction.Why)
	}
}

func TestEnvWorksWithNoKBAtAll(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	code, stdout, stderr := run("env")
	if code != ExitOK {
		t.Fatalf("env exited %d outside a KB: %s", code, stderr)
	}
	if !strings.Contains(stdout, "no KB found") {
		t.Errorf("stdout = %q, want it to say there is no KB", stdout)
	}
	// It still reports on the machine, which is the point of asking outside a KB.
	if !strings.Contains(stdout, "reads") {
		t.Errorf("stdout = %q, want the reading situation reported", stdout)
	}
	// And it does not leave a script anywhere it should not: a temporary copy is
	// named as one rather than shown as a path that is about to vanish.
	if strings.Contains(stdout, "stemma-shim-") {
		t.Errorf("stdout = %q, want the temporary copy described rather than named", stdout)
	}
}

// Every command that reads a document has to fail with a reason a person can act
// on, and env has to be the place that confirms it.
func TestFetchWithoutAnInterpreterSaysWhatToDo(t *testing.T) {
	root := fetchKB(t)
	source := document(t, "note.txt", "whatever")
	t.Setenv("PATH", "")

	code, _, stderr := run("fetch", "--kb", root, source)
	if code != ExitError {
		t.Fatalf("fetch exited %d, want %d", code, ExitError)
	}
	for _, want := range []string{"missing_python", "PATH", "stemma env"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want %q in it", stderr, want)
		}
	}
}
