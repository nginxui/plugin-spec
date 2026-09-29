# RFC 0015: Plugin WebSocket URLs, column conditions and the site log context

| | |
| --- | --- |
| Status | Accepted |
| Date | 2026-09-30 |
| Spec changes | `registry.wsUrl` (WEB-7, WEB-15), the context of the `when` option of `nginx_log.list.column:{key}` and the `site.log.actions` context (WEB-9, WEB-14) |

## Summary

A bundle asks the host for the address of a WebSocket to its own backend
instead of building it. A list column decides once per list whether it
exists. The site log actions learn whether a log path is the nginx default
the site falls back to.

## Motivation

A plugin that shows live progress needs a browser WebSocket to its `http`
capability. A browser cannot set headers on a WebSocket, so the host has to
accept credentials in the URL, and the plugin has no legitimate way to learn
them: the registry client adds its headers itself and never hands out the
session. A bundle that reads them back from a request it intercepts depends
on how the host authenticates today.

Log list columns that only suit access logs were hidden by returning `false`
from `when` for every row, which left the header, the sorting and the
filters behind. The decision belongs to the list, so it needs the list
context.

Sites that declare no `access_log` write to the nginx default log. The site
list used to answer "which log belongs to this site" with one request per
row, and a plugin could not tell a site's own log from the shared default.

## Design

* **The host builds the URL.** `registry.wsUrl(path)` returns a complete
  `ws:` or `wss:` address. What it carries (a session token, the selected
  node) stays the host's business and can change without touching plugins.
* **Query credentials for upgrades only.** The route accepts them for
  WebSocket upgrade requests and nowhere else, so ordinary requests keep
  their header based authentication and a token in a URL cannot be replayed
  against the REST surface. The host removes them before it forwards the
  request, so the plugin never logs or stores a session token.
* **The column condition takes the list.** `when` is called once per list
  with `{ type }`. The cell component keeps receiving `{ row }`, and the
  other slots keep their contexts.
* **The site log context carries the fallback.** A path is the site's own
  directive, else the nginx default log, and a flag says which. The host
  computes it while it already scans site configurations, so the list needs
  no extra request. A plugin decides what a shared log means for its action,
  for example to warn that the file holds the traffic of other sites.

## Compatibility

Everything is additive within spec 1.x (VER-1). `wsUrl` is optional and
feature detected, so a bundle on an older host treats live updates as
unavailable. A host that still calls `when` with a row context makes a
condition written for the list see no `type`; a column condition SHOULD
treat a missing `type` as "show", which is what the reference bundle does.
The new site log fields are extra members of an existing context, which
plugins already ignore when unknown, and `accessLogPath` and `errorLogPath`
keep their names. Their value changes for a site without its own directive
from an empty string to the default log, which is why the flags exist: an
action that only wants a site's own log checks `accessLogInherited`.
