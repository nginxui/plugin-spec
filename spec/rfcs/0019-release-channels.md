# RFC 0019: Release channels

| | |
| --- | --- |
| Status | Accepted |
| Date | 2026-09-30 |
| Spec changes | PKG-28, the packaging level in `spec/09-conformance.md`, `channel` on a release in `schema/catalog.schema.json` |

## Summary

Every catalog release belongs to a channel: `stable`, `beta` or `dev`. A host
keeps one followed channel per installed plugin and offers updates from that
channel and the more stable ones. A person can also install any earlier
version on request.

## Motivation

A plugin that is still being tested is published like any other, and the
marketplace gave no sign of it. Picking the highest version also made
`2.0.0-rc.1` the update of a stable `1.4.0`, so a person could end up on
unfinished software without choosing it. A single beta mark does not help the
people who want to test, and nothing let a person go back to a version that
worked for them.

## Design

* **Channels.** Ordered from the most to the least stable: `stable`, `beta`,
  `dev`. The entry `stage` stays a display label and does not set a channel.
* **What channel a release has.** The optional `channel` member of the catalog
  release, else the version: no prerelease part is `stable`, a prerelease whose
  first identifier is `alpha`, `dev`, `nightly`, `snapshot`, `canary` or
  `preview` is `dev`, any other prerelease (`beta`, `rc`, `pre`) is `beta`. The
  member is for a publisher who ships a plain version on a less stable channel.
* **Followed channel.** Each installed plugin follows one channel, `stable`
  by default, which the person can change. A host offers updates, updates
  automatically and resolves dependencies among the releases on that channel
  and the more stable ones. A person on `beta` therefore also receives the
  stable release that ends a beta series, so a publisher maintains one line of
  releases and not one per channel.
* **Ordering.** Releases are ordered by semantic versioning precedence. A
  release sorts after its own prereleases, so `1.1.0` follows `1.1.0-beta.3`,
  and a stable `1.0.1` sorts before `1.1.0-beta.2`, so a beta follower is not
  offered it as an update. Publishers should number prereleases with dot
  separated numeric identifiers (`1.1.0-beta.10`), which compare as numbers;
  `1.1.0-beta10` sorts before `1.1.0-beta9`.
* **Effective channel.** The channel updates come from is the less stable of
  the followed channel and the channel of the installed release. A host
  computes it and does not store it, so the followed channel is only ever what
  the person chose. Installing a beta release does not change the followed
  channel; it raises the effective channel while that release runs, and newer
  betas are offered until a stable release is installed, after which updates
  come from the followed channel again.
* **Nothing installed.** The newest stable release, else the newest beta, else
  the newest dev release. A plugin that so far has only beta releases is
  installed at its newest beta, is offered newer betas and then its first
  stable release, and receives no more betas once that is installed unless the
  person chose `beta`. A person who follows `stable` and installs one beta to
  try it gets the same fallback. A host SHOULD mark a plugin without a stable
  release and say so when it is installed.
* **Earlier versions.** A host installs any release that is not yanked and runs
  on the host when asked for it by version, including an older one. That is a
  downgrade: the host says so and warns that data written by the newer version
  may not be readable. Automatic updates never install an older release, and
  the package checks, trust level included, apply unchanged.
* **Display.** A host SHOULD mark releases and installed plugins that are not
  on `stable`.
* **Synchronised hosts.** Pushing a plugin installs the exact version that
  runs on the main host. The followed channel MAY be handed on too.

## Compatibility

A new optional catalog member (VER-1). Hosts that do not know it ignore it,
infer nothing and keep choosing the highest version. Catalogs that carried
the `beta` member of an earlier draft of this RFC must use `channel`; the
draft was not released.
