package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/theorytoe/stemma/internal/cli"
	"github.com/theorytoe/stemma/internal/index"
	"github.com/theorytoe/stemma/internal/kb"
	"github.com/theorytoe/stemma/internal/source"
)

// ToolHandlers wires the curated registry to the code that runs each tool.
// Every handler leans on the same builders the CLI command runs, so what a
// verb accepts, refuses, and reports is decided once, in internal/cli, and
// both surfaces read it: the tool cannot drift lenient where the command is
// strict, or resolve a page name by other rules.
//
// A handler returns what the envelope carries — the payload, the findings
// about what was asked, or the error when the run could not complete — and
// the server composes the envelope and its exit-code twins (D57).
func ToolHandlers() Handlers {
	return Handlers{
		"status":    statusOf,
		"list":      listOf,
		"search":    searchFor,
		"show":      showOf,
		"graph":     graphOf,
		"new":       createOf,
		"promote":   promoteOf,
		"cite_add":  citeAdd,
		"cite_show": citeShow,
		"lint":      lintOf,
	}
}

// statusOf answers how the KB is doing, in one read-only pass.
func statusOf(ctx context.Context, call Call) (any, []kb.Finding, error) {
	k, err := kb.Load(call.Root)
	if err != nil {
		return nil, nil, err
	}
	report, err := cli.GatherStatus(k)
	return report, nil, err
}

// listOf collects the pages answering the filters.
func listOf(ctx context.Context, call Call) (any, []kb.Finding, error) {
	var args struct {
		Type   string   `json:"type"`
		Status string   `json:"status"`
		Dir    string   `json:"dir"`
		Tags   []string `json:"tags"`
	}
	if err := json.Unmarshal(call.Args, &args); err != nil {
		return nil, nil, err
	}
	under, err := cli.UnderDir(args.Dir)
	if err != nil {
		return nil, nil, err
	}
	k, err := kb.Load(call.Root)
	if err != nil {
		return nil, nil, err
	}
	entries := cli.ListEntries(k, args.Type, args.Status, under, args.Tags)
	return cli.ListReport{Pages: entries, Count: len(entries)}, nil, nil
}

// searchFor ranks the pages against a query, through the same tier-selecting
// source the command uses, so an index that is stale answers from the KB
// exactly as the CLI's would.
func searchFor(ctx context.Context, call Call) (any, []kb.Finding, error) {
	var args struct {
		Query        string   `json:"query"`
		Type         string   `json:"type"`
		Status       string   `json:"status"`
		Dir          string   `json:"dir"`
		Tags         []string `json:"tags"`
		IncludeInbox bool     `json:"include_inbox"`
		Limit        int      `json:"limit"`
	}
	if err := json.Unmarshal(call.Args, &args); err != nil {
		return nil, nil, err
	}
	under, err := cli.UnderDir(args.Dir)
	if err != nil {
		return nil, nil, err
	}
	src, extra, err := cli.SearchSource(call.Root, args.IncludeInbox)
	if err != nil {
		return nil, nil, err
	}
	defer src.Close()
	hits, err := index.Search(src, extra, index.SearchRequest{
		Query:  args.Query,
		Type:   args.Type,
		Status: args.Status,
		Tags:   args.Tags,
		Dir:    under,
		Limit:  args.Limit,
	})
	if err != nil {
		return nil, nil, err
	}
	return cli.SearchReport{
		Query:   args.Query,
		Tier:    src.Tier().String(),
		Count:   len(hits),
		Results: hits,
	}, nil, nil
}

// showOf reads one page with its links and citations resolved.
func showOf(ctx context.Context, call Call) (any, []kb.Finding, error) {
	var args struct {
		Page string `json:"page"`
	}
	if err := json.Unmarshal(call.Args, &args); err != nil {
		return nil, nil, err
	}
	k, err := kb.Load(call.Root)
	if err != nil {
		return nil, nil, err
	}
	p, err := cli.Resolve(k, args.Page)
	if err != nil {
		return nil, nil, err
	}
	page, ok := k.Graph.Page(p)
	if !ok {
		return nil, nil, fmt.Errorf("%s is not a page", p)
	}
	return cli.Describe(k, p, page), nil, nil
}

// graphOf walks the link graph from one page.
func graphOf(ctx context.Context, call Call) (any, []kb.Finding, error) {
	var args struct {
		Page  string `json:"page"`
		In    bool   `json:"in"`
		Out   bool   `json:"out"`
		Depth *int   `json:"depth"`
	}
	if err := json.Unmarshal(call.Args, &args); err != nil {
		return nil, nil, err
	}
	// The CLI's flag defaults to one hop; an omitted argument is the same
	// question, so the default lives here rather than in the schema.
	depth := 1
	if args.Depth != nil {
		depth = *args.Depth
	}

	src, err := index.NewSourceAt(call.Root, nil)
	if err != nil {
		return nil, nil, err
	}
	defer src.Close()

	dirs := make([]index.Direction, 0, 2)
	if args.In {
		dirs = append(dirs, index.In)
	}
	if args.Out {
		dirs = append(dirs, index.Out)
	}
	if len(dirs) == 0 {
		dirs = []index.Direction{index.In, index.Out}
	}

	start, err := index.ResolvePage(src, args.Page)
	if err != nil {
		return nil, nil, err
	}
	neighbors, err := index.Neighbors(src, start, dirs, depth)
	if err != nil {
		return nil, nil, err
	}
	return cli.GraphReport{
		Start:     start,
		Depth:     depth,
		Tier:      src.Tier().String(),
		Neighbors: cli.NonNil(neighbors),
	}, nil, nil
}

// createOf writes a new page, or a draft in the inbox. Findings travel even
// when the page was written, because the envelope's findings are how the KB
// says "this needs attention" — the twin of the CLI exiting 1 with the file
// on disk.
func createOf(ctx context.Context, call Call) (any, []kb.Finding, error) {
	var args struct {
		Title  string `json:"title"`
		Type   string `json:"type"`
		Draft  bool   `json:"draft"`
		Strict bool   `json:"strict"`
	}
	if err := json.Unmarshal(call.Args, &args); err != nil {
		return nil, nil, err
	}
	k, err := kb.Load(call.Root)
	if err != nil {
		return nil, nil, err
	}
	mode := kb.Lenient
	if args.Strict {
		mode = kb.Strict
	}
	return cli.CreatePage(k, args.Title, args.Type, args.Draft, mode)
}

// promoteOf moves a draft into the pages, where its links begin to resolve.
func promoteOf(ctx context.Context, call Call) (any, []kb.Finding, error) {
	var args struct {
		Draft string `json:"draft"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal(call.Args, &args); err != nil {
		return nil, nil, err
	}
	k, err := kb.Load(call.Root)
	if err != nil {
		return nil, nil, err
	}
	return cli.Promote(k, args.Draft, args.Type)
}

// citeAdd records a source: an identifier resolved, or the fields of a record
// entered by hand, with the fields correcting a resolution when both are
// given. The rules — one identifier at most, the duplicate check, the
// dry-run report — are ApplyEntry's and the resolver's, the same code the
// command runs.
func citeAdd(ctx context.Context, call Call) (any, []kb.Finding, error) {
	var args struct {
		Identifier string   `json:"identifier"`
		Type       string   `json:"type"`
		Key        string   `json:"key"`
		Title      string   `json:"title"`
		Authors    []string `json:"authors"`
		Year       string   `json:"year"`
		Container  string   `json:"container"`
		Publisher  string   `json:"publisher"`
		Volume     string   `json:"volume"`
		Issue      string   `json:"issue"`
		Pages      string   `json:"pages"`
		Edition    string   `json:"edition"`
		DOI        string   `json:"doi"`
		Arxiv      string   `json:"arxiv"`
		ISBN       string   `json:"isbn"`
		URL        string   `json:"url"`
		Path       string   `json:"path"`
		Offline    bool     `json:"offline"`
		DryRun     bool     `json:"dry_run"`
		Force      bool     `json:"force"`
	}
	if err := json.Unmarshal(call.Args, &args); err != nil {
		return nil, nil, err
	}
	k, err := kb.Load(call.Root)
	if err != nil {
		return nil, nil, err
	}

	fields := source.Fields{
		Type: args.Type, Key: args.Key, Title: args.Title, Authors: args.Authors,
		Year: args.Year, Container: args.Container, Publisher: args.Publisher,
		Volume: args.Volume, Issue: args.Issue, Pages: args.Pages, Edition: args.Edition,
		DOI: args.DOI, ArXiv: args.Arxiv, ISBN: args.ISBN, URL: args.URL, Path: args.Path,
	}
	// The identifier arguments and the metadata arguments are two different
	// things, and only the metadata can stand beside a resolved identifier:
	// an identifier is what gets resolved, and passing one as a field would
	// be asking for two records at once.
	identifier := args.DOI != "" || args.Arxiv != "" || args.ISBN != "" || args.URL != "" || args.Path != ""
	metadata := args.Title != "" || len(args.Authors) > 0 || args.Year != "" ||
		args.Container != "" || args.Publisher != "" || args.Volume != "" ||
		args.Issue != "" || args.Pages != "" || args.Edition != ""
	handEntered := identifier || metadata

	res := source.New()
	res.Offline = args.Offline

	var entry *kb.BibEntry
	var record []byte
	resolved := false

	switch {
	case args.Identifier != "" && identifier:
		return nil, nil, fmt.Errorf("an identifier carries its own metadata; drop the doi, arxiv, isbn, url and path arguments, " +
			"which are only for a record entered by hand")
	case args.Identifier != "":
		result, err := res.ResolveResult(ctx, args.Identifier)
		if err != nil {
			// The twin of the command's failResolution: a network failure is
			// one the run could not complete, while an identifier that names
			// nothing, or is malformed, is a finding about what was asked
			// for.
			if source.Operational(err) {
				return nil, nil, err
			}
			return nil, []kb.Finding{{
				Severity: kb.Warning,
				Code:     cli.CodeCiteUnresolved,
				Message:  err.Error(),
			}}, nil
		}
		entry, record, resolved = result.Entry, result.Record, true
	default:
		if !handEntered {
			return nil, nil, fmt.Errorf("cite add needs an identifier, or the fields of a record to add by hand")
		}
		var err error
		entry, err = source.Manual(fields)
		if err != nil {
			return nil, nil, err
		}
	}

	// The key and type are the author's choice and not a fact a resolver
	// owns, so they land before the fields do: the type decides which field a
	// container belongs in.
	if args.Key != "" {
		entry.SetKey(args.Key)
	}
	if args.Type != "" {
		entry.SetType(args.Type)
	}
	// A resolver knows a record but not every reader's view of it, and a raw
	// document can carry no title at all. A field given beside a resolved
	// identifier is a correction, so it is written over what the resolver
	// said. A hand-entered record was built from the same fields already,
	// which is why this runs only for a resolution.
	if resolved {
		source.ApplyFields(entry, fields)
	}
	if len(record) > 0 {
		entry.Set(kb.FieldContentHash, "{"+kb.HashOf(record)+"}")
	}

	report, err := cli.ApplyEntry(k, entry, args.Force, args.DryRun)
	return report, nil, err
}

// citeShow reads one bibliography entry as it is written.
func citeShow(ctx context.Context, call Call) (any, []kb.Finding, error) {
	var args struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(call.Args, &args); err != nil {
		return nil, nil, err
	}
	k, err := kb.Load(call.Root)
	if err != nil {
		return nil, nil, err
	}
	e, ok := k.Bibliography.Entry(args.Key)
	if !ok {
		return nil, nil, fmt.Errorf("the bibliography has no entry for %q", args.Key)
	}
	return cli.CiteShowReport{
		Key:     args.Key,
		Type:    e.Type(),
		Path:    k.Bibliography.PathOf(args.Key),
		Entry:   string(e.Bytes()),
		Fields:  cli.CiteFields(e),
		CitedBy: k.Graph.CitedBy(args.Key),
	}, nil, nil
}

// lintOf reports everything wrong with the KB, with the summary and the
// findings travelling together, the way --json prints them.
func lintOf(ctx context.Context, call Call) (any, []kb.Finding, error) {
	var args struct {
		Strict bool `json:"strict"`
	}
	if err := json.Unmarshal(call.Args, &args); err != nil {
		return nil, nil, err
	}
	k, err := kb.Load(call.Root)
	if err != nil {
		return nil, nil, err
	}
	mode := kb.Lenient
	if args.Strict {
		mode = kb.Strict
	}
	findings := k.Lint(mode)
	return cli.SummariseLint(args.Strict, findings), findings, nil
}
