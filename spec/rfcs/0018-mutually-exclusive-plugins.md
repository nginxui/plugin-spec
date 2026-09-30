# RFC 0018: Mutually exclusive plugins

| | |
| --- | --- |
| Status | Accepted |
| Date | 2026-09-30 |
| Spec changes | MAN-42 (`conflicts`), LIFE-20, the MAN-42 lint row and the lifecycle level in `spec/09-conformance.md`, `Manifest` in `manifest.proto`, `schema/plugin.schema.json` |

## Summary

A manifest may declare `conflicts`, a list of plugin ids that must never be
enabled at the same time as the plugin. A host refuses to run two conflicting
plugins together and lets the caller replace the enabled one.

## Motivation

Two log analytics plugins with different engines both add the same pages and
both index the same logs. Running them together doubles the work and leaves
the person with two versions of every page. Nothing in `requires` can express
that two plugins exclude each other.

## Design

* **Shape.** An optional list of plugin ids. Every entry is a valid id, is not
  the plugin itself, appears once and is not also in `requires` (MAN-42).
* **Symmetric.** Two plugins conflict when either lists the other, so a
  plugin can exclude one that was published without knowing about it.
* **Enable.** A host refuses to enable a plugin while a conflicting plugin is
  enabled and names it. The caller can ask to replace the conflicting
  plugins, which are then disabled like any other disable, dependents
  included (LIFE-20).
* **Install.** A package that is installed and enabled together stays
  disabled when it conflicts with an enabled plugin, unless the caller asks
  to replace. An upgrade keeps its enabled state only without such a
  conflict.
* **Startup.** The first plugin in the start order wins. A later one that
  conflicts with it is disabled and records an error naming the other, so
  the state stays consistent after a restart.
* **Synchronised hosts.** The state of the main host is the truth, so enabling
  on another host replaces conflicting plugins.

## Compatibility

A new optional manifest field (VER-1). Hosts that do not know it ignore it
and can run both plugins.
