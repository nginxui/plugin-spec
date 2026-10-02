# Changelog

Changes to the plugin contract. The developer guide at
[nginxui.com/plugin](https://nginxui.com/plugin/overview) describes the
behavior behind each entry.

## Unreleased

First public version, plugin API version 1 (`api_version = 1`):

- Proto contract for the lifecycle, the host API, events, and the `dns01`,
  `http`, `notify`, `probe`, `mcp`, `storage`, `cert.deploy`,
  `security.blocklist`, `upstream.discovery` and `log.sink` capabilities,
  over JSON-RPC 2.0 on stdio and over gRPC.
- JSON Schemas of `plugin.json`, the marketplace catalog and the partner
  keyring.
- Test vectors of the wire protocol and a Python example plugin.
- A catalog release can carry its `notes` in Markdown next to
  `release_notes_url`.
- The `manifest` of a catalog release is a snapshot of the members a host
  reads before the install. The capability blocks stay in the package.
- A catalog entry can list what the plugin `provides`: the dns01 providers of
  its newest release with the plugin version since which it provides dns01,
  a version of its own on each provider added later, and the version that
  dropped a provider the newest stable release still has.
