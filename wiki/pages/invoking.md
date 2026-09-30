---
title: Invoking the tool
type: concept
aliases:
  - the command line
tags: [cli]
---
A command is `stemma <command> [flags]`. Flags may sit before or after the
positional arguments — `stemma new --draft "A Title"` and `stemma new "A Title"
--draft` are the same run — and `--` ends the flags, so a title that begins with
a dash is still a title. `stemma help` lists the surface and `stemma help
<command>` describes one verb; see [[The command surface]].

**Finding the knowledge base (KB).** A command that works on a KB finds it in
one fixed order: a path given with `--kb`, else the `STEMMA_KB` environment
variable, else a walk up from the working directory. The walk prefers a
directory holding `stemma.toml` and falls back to the nearest one holding
`pages/`, so a KB configured on purpose beats one that merely looks like a KB.
There is no registry and no daemon, and one invocation works on one KB.

**Flags.** Every command accepts `--json`. A command that works on an existing KB
also accepts `--kb` and `--strict`. `--strict` turns warnings into errors, so the
same command is lenient while you work and fails a build when you ask it to.
`STEMMA_OFFLINE` makes a command that can use the network behave as if
`--offline` had been passed.

**Exit codes.** `0` is a clean run, `1` means the run found something to report,
and `2` means the run could not be completed. The last two are deliberately
different: `1` is a fact about the KB and `2` is a fact about the invocation, so
a script treats a broken link and a missing file differently.

**Output.** By default a command writes what a person reads. With `--json` it
writes one envelope and nothing else, on standard output:

```json
{ "command": "list", "ok": true, "data": { "count": 2, "pages": [] } }
```

`command` names the verb that ran, `ok` is true only for a clean run, `data` is
the command's own payload, `findings` carries validation findings, and `error`
carries an operational failure. A `--json` run keeps the JSON stream the whole
story: an error travels in the envelope rather than on standard error.

`stemma env` reports the optional parts of the environment — the interpreter, the
extraction shim, the linked SQLite, and whether an index is present — which is
the first thing to ask when something optional is not working. See
[[Serving the site]] for the one command that runs until it is stopped.
