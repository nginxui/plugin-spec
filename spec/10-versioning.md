# 10. Versioning

Three independent version numbers appear in this system. Confusing them is
the single most common mistake a new plugin author makes.

| Version | Where | What it tracks |
| --- | --- | --- |
| Spec version | This repository's tags/releases (e.g. "spec 1.0") | The wire protocol, manifest schema and behavioral contract documented here. |
| `api_version` | `plugin.json`, `InitializeResult.api_version` | The integer wire-protocol generation a plugin/host speaks. Spec 1.x is `api_version = 1`. |
| Plugin `version` | `plugin.json` | The plugin's own release, semver, unrelated to the protocol it happens to speak. |
| `min_nginx_ui_version` | `plugin.json` | Advisory minimum **host application** release (e.g. `nginx-ui` `2.7.0`), a separate product from the protocol. |

## VER-1

This document describes **spec 1.0**, which defines `api_version = 1`. A
future spec 1.x (1.1, 1.2, ...) MAY add optional fields, new capabilities,
new event types, new `host.*` methods, or new well-known slot names, but
MUST NOT change the meaning of an existing field, remove a field, or change
an existing MUST requirement in a way that breaks a plugin or host
conformant to an earlier 1.x. `api_version` stays `1` across the whole 1.x
series.

## VER-2

A breaking change (removing a field, renaming a JSON key, changing an error
code's meaning, changing a MUST requirement incompatibly) requires a new
major spec version and a new `api_version` value. Spec 2.0 would define
`api_version = 2`, documented in this same repository alongside spec 1.x
until spec 1.x is formally retired (VER-6).

## VER-3

`InitializeResult.api_version` (LIFE-2) MUST equal the `api_version` the
host itself implements, exactly (LIFE-3). Spec 1.x defines no negotiation
between adjacent `api_version` values — a host that implements `api_version
= 2` and receives a plugin reporting `api_version = 1` MUST refuse the
handshake unless that host separately, explicitly, also implements
`api_version = 1` and chooses to run the plugin under it. This spec does not
mandate that a host support more than one `api_version` at a time; it only
requires that if a host does not, it fails the handshake cleanly (LIFE-3)
rather than proceeding with a version mismatch.

## VER-4

A field this spec marks optional MUST be safe to omit from spec 1.0 through
the end of the 1.x series: an implementation MUST NOT require a field that
was optional in an earlier 1.x minor version to be present, purely because a
later 1.x minor version exists. Conversely, a receiver MUST ignore a field
it does not recognize (WIRE-7) so that an older implementation keeps working
against a newer peer that has started sending an additional optional field.

## VER-5

`min_nginx_ui_version` and `api_version` are checked differently. A host
MUST treat `api_version` mismatch as fatal to the handshake (VER-3).
`min_nginx_ui_version` is advisory (MAN-6): a host MAY refuse to enable a
plugin whose declared minimum exceeds its own version, but this is a policy
choice, not a wire-protocol failure, and a host that chooses not to check it
at all is still conformant.

## VER-6

Retiring a spec major version (ceasing to document or support `api_version =
N`) MUST be announced with at least one full spec minor release's advance
notice in this repository's `README.md` and change log, so that plugins and
hosts still on the old version have a documented window to migrate.

## Requirement numbering

## VER-7

Requirement ids (`MAN-7`, `LIFE-3`, ...) are permanent once published: a
later spec revision MUST NOT reuse a retired id for an unrelated
requirement, and MUST NOT renumber an existing requirement to close a gap
left by a removed one. A removed requirement's id is retired, documented as
"Reserved: superseded by `<other-id>`" or "Reserved: removed in spec X.Y",
and never reissued. New requirements always take the next unused number in
their section's series, which is why the ids skip around slightly as this
document keeps evolving.
