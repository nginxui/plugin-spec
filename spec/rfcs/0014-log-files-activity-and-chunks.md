# RFC 0014: Log files, activity entries and on-demand chunks

| | |
| --- | --- |
| Status | Accepted |
| Date | 2026-09-30 |
| Spec changes | `log.files` permission (SEC-3, SEC-30, MAN-23), `host.logs.list` (HOST-17), `log.paths_changed` (HOST-16, HOST-18), `host.activity.set` (HOST-19), the nginx log and site log slots and the `registerSlot` options `label`, `sortValue`, `filters` (WEB-7, WEB-9, WEB-14), `webapp.chunks` and `registry.loadChunk` (MAN-41, WEB-1, WEB-13), `HostLogsList*`, `HostLogFile` and `HostActivitySet*` in `host.proto`, `ManifestWebapp.chunks` in `manifest.proto`, `schema/plugin.schema.json` |

## Summary

A plugin can read the nginx log files the host knows about, add views,
columns and actions to the log pages, show its background work in the
host's processing indicator, and load heavy parts of its browser code on
demand. Together these let a plugin own log analysis end to end, without
the host linking any of that code.

## Motivation

`log.sink` streams new access log lines, which is right for a plugin that
reacts to traffic. A plugin that builds an index needs the files
themselves, including the rotated history, and needs to learn when the set
of files changes. It also needs places in the log pages to show what it
builds, and a large page with charts must not be downloaded on every visit
to the host.

## Design

* **The files stay with the plugin.** `host.logs.list` returns paths, not
  content. The plugin already runs with the privileges of the host (SEC-1),
  so streaming the bytes through the host would add a copy and no
  protection. The permission is what the person approves and what gates the
  list (SEC-30).
* **Only live paths are listed.** Rotation schemes differ between systems,
  and the plugin has to cope with them anyway when it reads history.
  Listing the live path only keeps the host from guessing the scheme.
* **One event, no payload.** `log.paths_changed` says that the set changed.
  The plugin asks again, so the answer cannot go stale inside the event.
* **Activity entries are English keys.** The host owns the indicator and its
  language. A plugin supplies translations through the bundle it already
  ships (`registerTranslations`), so no new translation channel exists. Entries
  vanish with the process so a crash cannot leave the indicator on.
* **Sorting and filtering happen in the browser.** The log list has no
  pagination, and a plugin cannot influence a server side query, so the
  column options `sortValue` and `filters` give the host what it needs to do
  the same locally.
* **Chunks are more IIFE files.** An ES module graph would need a module
  loader in the host and breaks the shared runtime mapping (WEB-2). Extra
  IIFE files reuse the entry's contract, and a global hand-over, the way the
  entry uses `registerPlugin`, keeps the loader small. The host loads a
  declared name only, so a bundle cannot make the host fetch an arbitrary
  address.

## Compatibility

Everything is additive within spec 1.x (VER-1): a new permission, two methods,
one event type, new slot names (which WEB-9 already allows), new optional
`registerSlot` options and an optional manifest member. A host that lacks
them replies `-32601` for the methods, never delivers the event, ignores
the slots and has no `registerChunk`. WEB-1 is relaxed rather than
changed: a bundle without `webapp.chunks` is a single file as before.
