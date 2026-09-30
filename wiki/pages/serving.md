---
title: Serving the site
type: note
tags: [site, cli]
---
`stemma serve` renders the KB and answers it over HTTP, on `127.0.0.1:8080` by
default. It is the local way to read [[The rendered site]] — it runs until it is
stopped and writes nothing to disk.

The server reloads the KB when its content changes rather than on every request.
It compares a hash of every page's content, so touching a file without changing
it costs a check and not a re-render, and an edit shows up on the next request.
A helper injected into each page reloads the tab when the KB has changed; it is
never written to disk, and every page reads without it.

Search is answered on the server by the same ranked search as the `search`
command, so the form works with JavaScript off. The form is added only while a
server runs, because a form that submits to nothing is worse than no form at
all.
