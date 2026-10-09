---
title: The command surface
type: concept
aliases:
  - commands
tags: [cli]
---
The CLI is the universal surface. Everything the tool does is a command, and
every other surface — an agent's tool call, a skill — is a thin layer over the
same core. A command is spelled `stemma <command> [flags]`, and a family such as
`cite` takes a member before its flags: `stemma cite show KEY`.

**Setup**

| Verb   | What it does                           |
| ------ | -------------------------------------- |
| `init` | create a knowledge base (KB) root      |
| `new`  | create a page, or a draft in the inbox |

**Pages**

| Verb      | What it does                                        |
| --------- | --------------------------------------------------- |
| `list`    | list the pages                                      |
| `show`    | show one page with its links and citations resolved |
| `move`    | move a page to another directory                    |
| `rename`  | retitle a page and rewrite every link that named it |
| `archive` | archive a page, recording why                       |
| `promote` | move a draft from the inbox into the pages          |

**Sources**

| Verb    | What it does                                                                     |
| ------- | -------------------------------------------------------------------------------- |
| `cite`  | the bibliography: `add`, `list`, `show`, `cited-by`, `vendor`, `export`, `check` |
| `fetch` | read a document's text into the scratch area                                     |

**Retrieval**

| Verb     | What it does                             |
| -------- | ---------------------------------------- |
| `index`  | build or refresh the Tier-1 search index |
| `search` | search the pages, ranked                 |
| `graph`  | walk the link graph                      |

**Output**

| Verb    | What it does                                           |
| ------- | ------------------------------------------------------ |
| `serve` | serve the KB as a local site; see [[Serving the site]] |

**Checks and help**

| Verb     | What it does                                 |
| -------- | -------------------------------------------- |
| `status` | summarise the health of the KB               |
| `lint`   | report everything wrong with the KB          |
| `env`    | report the optional parts of the environment |
| `help`   | show help for a command                      |

A noun family is not a command on its own: naming `cite` without a member is a
usage error rather than a guess at what you meant. `help` reads the same
definition the dispatcher does, so the surface and its documentation cannot
drift apart, and `help --markdown` writes the whole reference as one document.
`help --man` writes the same surface as manual pages, one per command, which
`make man` collects and `make install-man` puts where `man` looks for them.
See [[Invoking the tool]] for how a command is run and what it returns.
