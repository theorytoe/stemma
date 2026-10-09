// Package mcp exposes a Stemma knowledge base to agents as native tools on
// the Model Context Protocol. The surface is a curated subset of the CLI
// (D58): the read core an agent needs to answer from the KB, plus the four
// mutations that change it — new, promote, cite add, and lint. Every other
// verb stays on the command line, and Omissions says why, one command at a
// time.
//
// The contract with the CLI is structural (D57): a tool's result is the same
// envelope a --json run writes, carrying the same payload struct, and the
// outcomes mean what the exit codes mean. ok with no findings is a clean run;
// ok false with findings is a validation result, the twin of exit 1; ok false
// with error is an operational failure, the twin of exit 2. The envelope is
// composed here so that the result a harness sees and the output a script
// pipes are one shape learned once.
package mcp

import (
	"github.com/theorytoe/stemma/internal/cli"
)

// Tool is one entry of the curated surface. Name is how a harness invokes it;
// Command is the CLI verb whose --json contract the tool carries, checked
// against the command table by the tests so the two cannot drift apart.
//
// Input is the JSON Schema of the tool's arguments. It is written by hand:
// argument names and their limits are a design surface, not a mechanical
// projection of the command's flags. Payload is a zero value of the cli type
// the result's data holds; the shape walked off it is what a consumer reads.
type Tool struct {
	Name        string
	Command     string
	Description string
	Input       map[string]any
	Payload     any
}

// Omission names a CLI verb that stays off the MCP surface, and says why.
// D58 decided the surface is a subset, not a mirror; a subset whose gaps are
// unstated is just a smaller mirror. The reasons are rendered into the tool
// reference, so an agent that looks for a verb that is not there can read the
// decision instead of guessing at it.
type Omission struct {
	Command string
	Reason  string
}

// Tools is the curated surface, in the order a harness lists them: the read
// core first, then the mutations, then the guard. Ten tools, no more (D58).
var Tools = []Tool{
	{
		Name:    "status",
		Command: "status",
		Description: "Summarise a knowledge base's health: page counts by type, orphaned " +
			"pages, names more than one page claims, sources nothing cites, drafts waiting " +
			"in the inbox, and whether the search index is present and fresh. Read-only, " +
			"and correct on a KB with no index at all. Call it first to see what the KB " +
			"holds and what needs attention.",
		Input:   obj(nil),
		Payload: cli.StatusReport{},
	},
	{
		Name:    "list",
		Command: "list",
		Description: "List pages with their title, type, status, and tags. The filters " +
			"combine: each narrows the result, and a page must satisfy all of them. Use " +
			"it to enumerate a KB or one slice of it; use search to rank pages by " +
			"relevance instead.",
		Input: obj(map[string]any{
			"type":   str("only pages of this type"),
			"status": enum("only pages with this status", "active", "archived"),
			"dir":    str("only pages under this KB-relative directory; \"papers\" and \"pages/papers\" mean the same"),
			"tags":   strArray("only pages carrying every one of these tags"),
		}),
		Payload: cli.ListReport{},
	},
	{
		Name:    "search",
		Command: "search",
		Description: "Search the pages, ranked by relevance, each hit carrying a snippet " +
			"around the match. Answers from the Tier-1 index when one is fresh and from " +
			"the KB itself otherwise, reporting which tier ran. Inbox drafts are excluded " +
			"unless include_inbox asks for them.",
		Input: obj(map[string]any{
			"query":         str("the text to search for; empty lists everything the filters allow"),
			"type":          str("only pages of this type"),
			"status":        enum("only pages with this status", "active", "archived"),
			"dir":           str("only pages under this KB-relative directory"),
			"tags":          strArray("only pages carrying every one of these tags"),
			"include_inbox": boolean("also search inbox drafts"),
			"limit":         integer("return at most this many results; omit for no limit", 0),
		}),
		Payload: cli.SearchReport{},
	},
	{
		Name:    "show",
		Command: "show",
		Description: "Show one page with its links and citations resolved: title, type, " +
			"status, aliases, tags, the body, every link with the page it resolves to or " +
			"the candidates it is ambiguous between, and every citation key with whether " +
			"the bibliography defines it. Accepts a title, a path, or the name a link " +
			"uses. The CLI's --path and --raw conveniences are not tools; the report " +
			"already carries the body.",
		Input: obj(map[string]any{
			"page": str("the page's title, its path, or the name a link uses"),
		}, "page"),
		Payload: cli.ShowReport{},
	},
	{
		Name:    "graph",
		Command: "graph",
		Description: "Walk the link graph from one page: the pages reached, each with " +
			"its distance in hops. in follows backlinks, out follows forward links, and " +
			"with neither given the walk follows both. Depth 0 walks the whole reachable " +
			"set. The CLI's orphan and dead-end listings are not tools: status reports " +
			"orphans, and a walk shows the dead ends.",
		Input: obj(map[string]any{
			"page":  str("the page to walk from, by title or path"),
			"in":    boolean("follow backlinks"),
			"out":   boolean("follow forward links"),
			"depth": integer("how many hops to walk; 0 means the whole reachable set; default 1", 0),
		}, "page"),
		Payload: cli.GraphReport{},
	},
	{
		Name:    "new",
		Command: "new",
		Description: "Create a page, or a draft in the inbox when draft is set. The title " +
			"is the page's identity, so a name another active page already answers to is " +
			"refused. An unknown type is a finding rather than an error — a warning by " +
			"default, a failure under strict — and the findings travel in the result " +
			"either way.",
		Input: obj(map[string]any{
			"title":  str("the page's title, which is how links name it"),
			"type":   str("the page's type; the KB's default when omitted"),
			"draft":  boolean("create a draft in the inbox instead of a page"),
			"strict": boolean("treat validation warnings as errors, which leaves the page unwritten"),
		}, "title"),
		Payload: cli.NewReport{},
	},
	{
		Name:    "promote",
		Command: "promote",
		Description: "Move a draft from the inbox into the pages, where links to its " +
			"title begin to resolve; the result reports how many already did. The draft " +
			"is held to the page format first, and findings — an unknown type, a missing " +
			"field — fail the promotion. A draft may also be promoted under a type given " +
			"here, which overrides its own.",
		Input: obj(map[string]any{
			"draft": str("the draft's title or path in the inbox"),
			"type":  str("the type to give the page; the draft's own when omitted"),
		}, "draft"),
		Payload: cli.PromoteReport{},
	},
	{
		Name:    "cite_add",
		Command: "cite add",
		Description: "Record a source in the bibliography. Give identifier — a DOI, " +
			"arXiv ID, ISBN, or URL — and it is resolved into a record; give descriptive " +
			"fields alone and a record is entered by hand; give both and the fields " +
			"correct what the resolver said. A work already present is updated rather " +
			"than duplicated, unless force says otherwise. With dry_run nothing is " +
			"written and the entry that would have been written comes back in the " +
			"result. A local file is recorded with path, never fetched. Exactly one of " +
			"identifier, or one of the identifier fields doi, arxiv, isbn, url, path, " +
			"must be given.",
		Input: obj(map[string]any{
			"identifier": str("a DOI, arXiv ID, ISBN, or URL to resolve"),
			"type":       str("the entry type; inferred when omitted"),
			"key":        str("the citation key; generated when omitted"),
			"title":      str("the title, for a hand-entered record or a correction"),
			"authors":    strArray("authors, for a hand-entered record or a correction"),
			"year":       str("the year"),
			"container":  str("the journal, or the book a chapter is in"),
			"publisher":  str("the publisher"),
			"volume":     str("the volume"),
			"issue":      str("the issue or number"),
			"pages":      str("the page range"),
			"edition":    str("the edition"),
			"doi":        str("a DOI, for a record entered by hand"),
			"arxiv":      str("an arXiv identifier, for a record entered by hand"),
			"isbn":       str("an ISBN, for a record entered by hand"),
			"url":        str("a URL, for a record entered by hand"),
			"path":       str("a path to a local file, for a record entered by hand"),
			"offline":    boolean("resolve without the network"),
			"dry_run":    boolean("report what would be written without writing it"),
			"force":      boolean("append a second copy even when the work is already there"),
		}),
		Payload: cli.CiteAddReport{},
	},
	{
		Name:    "cite_show",
		Command: "cite show",
		Description: "Show one bibliography entry as it is written: the record's own " +
			"bytes, each field verbatim beside its decoded value, the file that defines " +
			"it, and the pages that cite it. The four things to check before relying on " +
			"a record — or citing it again.",
		Input: obj(map[string]any{
			"key": str("the entry's citation key"),
		}, "key"),
		Payload: cli.CiteShowReport{},
	},
	{
		Name:    "lint",
		Command: "lint",
		Description: "Report everything wrong with the KB under the format's rules: " +
			"unresolved and ambiguous links, dangling citations, unknown types, " +
			"structural faults. Every finding names its file, line, and field, and the " +
			"summary counts warnings and errors. strict applies the format's strict " +
			"rules, where a warning is an error.",
		Input: obj(map[string]any{
			"strict": boolean("use the strict rules, where warnings are errors"),
		}),
		Payload: cli.LintSummary{},
	},
}

// Omissions is the complement of Tools on the CLI surface: every verb the
// command table defines that no tool mirrors, with the reason it stays where a
// person runs it. The tests hold both lists against the command table, so a
// verb added to the CLI lands here — named and reasoned — or on the surface.
var Omissions = []Omission{
	{"init", "creates a KB root; a person founds one deliberately, and the tools answer about a KB that exists rather than mint new ones."},
	{"move", "moves a page and rewrites the links that name it; a rearrangement is maintenance a person reviews, not a conversational step."},
	{"rename", "retitles a page and rewrites every link to it; the blast radius is cross-page, so it stays where the diff can be read before it lands."},
	{"archive", "takes a page off the graph for a stated reason; a curation decision of that weight belongs to a person."},
	{"index", "builds the Tier-1 cache, an optimisation; every tool answers from Tier 0 without it, so an agent never needs to run it."},
	{"serve", "runs the local site for people; the protocol carries no daemon and no port (D41)."},
	{"build", "writes the static site another person reads; publishing is the publishing workflow's act, not a tool call's."},
	{"export json", "dumps the whole KB as one JSON document; an agent reads pages through the read tools, and bulk transfer is a publishing act."},
	{"export page", "extracts a page and its closure as a KB of its own; a scoped copy is a publishing act, and the read tools answer without one."},
	{"fetch", "fetches and reduces a remote document for a source; it is the research workflow's step, writing captures rather than answers."},
	{"cite list", "lists records with filters; status already reports the uncited, and cite_show reads one record in full."},
	{"cite cited-by", "lists the pages citing a key; cite_show's result includes cited_by already."},
	{"cite vendor", "vendors a capture under sources/; like fetch it is research-workflow file handling."},
	{"cite export", "writes the bibliography out in another format; a format conversion is publishing, not reading."},
	{"cite check", "lints bibliography-internal rules; the lint tool is this surface's guard, and its findings cover what links and citations depend on."},
	{"env", "describes the installation for a person diagnosing setup; a harness knows how it launched the server."},
	{"help", "the CLI's own help; the tool reference is rendered from the schema set this artifact comes from."},
}

// Envelope returns the schema of the result every tool carries: the same
// envelope a --json run writes (D57), with data described by the given payload
// shape. Findings is the projection of a validation finding; error carries an
// operational failure's message. Which of the three outcomes a result
// represents is read the way the exit codes are read: ok true is clean, ok
// false with findings is exit 1, ok false with error is exit 2.
func Envelope(payload map[string]any) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "the CLI verb whose contract the result carries",
			},
			"ok": map[string]any{
				"type":        "boolean",
				"description": "true only for a clean run",
			},
			"data": payload,
			"findings": func() map[string]any {
				finding := Shape(cli.JSONFinding{})
				finding["description"] = "one finding: what and where, with its severity"
				return map[string]any{
					"type":        "array",
					"items":       finding,
					"description": "validation findings, present when the run found something to report; the twin of exit code 1",
				}
			}(),
			"error": map[string]any{
				"type":        "string",
				"description": "what could not be done and why, when the run failed operationally; the twin of exit code 2",
			},
		},
	}
}

// obj builds an input schema that allows exactly the properties given. The
// surface is closed on purpose: a harness that sends a mistyped or unknown
// argument should be told so, not silently obeyed.
func obj(props map[string]any, required ...string) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	m := map[string]any{
		"type":                 "object",
		"properties":           props,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}

func str(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func boolean(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}

func integer(desc string, min int) map[string]any {
	return map[string]any{"type": "integer", "description": desc, "minimum": min}
}

func enum(desc string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": desc, "enum": values}
}

func strArray(desc string) map[string]any {
	return map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": desc,
	}
}
