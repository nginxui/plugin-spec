# RFC 0017: Recommended memory of a plugin

| | |
| --- | --- |
| Status | Accepted |
| Date | 2026-09-30 |
| Spec changes | MAN-39 (`server.resources.recommended_memory_mb`), LIFE-19, the MAN-39 lint row and the lifecycle level in `spec/09-conformance.md`, `ManifestResources` in `manifest.proto`, `schema/plugin.schema.json` |

## Summary

A manifest may declare `server.resources.recommended_memory_mb`, the memory
the machine should have for the plugin to work well. A host shows it where a
plugin is chosen or installed and warns when it runs with less.

## Motivation

`memory_mb` caps the process, so it says nothing to a person deciding
whether a small server or container can carry a plugin next to nginx-ui.
Plugins that index or cache data need far more than the host alone, and the
first sign of it is an out of memory kill.

## Design

* **Meaning.** Total memory of the machine, or the memory limit of the
  container the host runs in, counting nginx-ui and the plugin together.
* **Advice only.** A host never refuses to install or run a plugin because of
  it. It shows the value in the marketplace and the plugin details, and
  warns before the install and on the installed plugin when its memory is
  lower (LIFE-19).
* **Shape.** An optional integer in MiB, `0` or absent meaning no hint, never
  negative.

## Compatibility

A new optional manifest field (VER-1). Hosts that do not know it ignore it.
