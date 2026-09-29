package cli

import "github.com/theorytoe/stemma/internal/extract"

// newRunner builds what the commands that read a document read through.
//
// It is a variable so that a test can point extraction at a stand-in script, which
// is how every path through those commands stays testable on a machine with no
// Python at all. Fetching and vendoring both come through here, so that "which
// script ran, and under which interpreter" has one answer.
var newRunner = func(root string) (*extract.Runner, error) {
	// The script is generated, so it is re-established rather than assumed: a
	// cleared .stemma/, an upgrade, and a KB received by clone all have to leave
	// this working.
	if _, err := extract.Materialize(root); err != nil {
		return nil, err
	}
	python, err := extract.FindPython()
	if err != nil {
		return nil, err
	}
	return &extract.Runner{Python: python, Script: extract.Path(root)}, nil
}
