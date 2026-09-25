# RFC 0009: The `upstream.discovery` capability

| | |
| --- | --- |
| Status | Accepted, implemented |
| Date | 2026-09-23 |
| Spec changes | `spec/19-capabilities-discovery.md` (DISCOVERY-1 through DISCOVERY-11), MAN-19, MAN-37, SEC-3 (`network` row), SEC-15, NAME-10, CONF-4, CONF-7, CONF-15, WIRE-9 table, WIRE-11 routing table, LIFE-10 (shutdown list), `proto/nginxui/plugin/v1/discovery.proto`, `ManifestDiscovery` in `manifest.proto`, `schema/plugin.schema.json`, vectors 45 and 46 |
| Reference host | nginx-ui `internal/plugin/capability/discovery.go`, `internal/upstream/discovery` (registry, rendering, runner), `internal/config/generated.go`, `model.UpstreamDiscovery`, migration `20260923000005`, `api/upstream_discovery`, `app/src/views/upstream/components/UpstreamDiscoveries.vue` |
| Reference SDK | plugin-sdk-go `discovery.go` (`DiscoveryHandler`) |

## Summary

A plugin may resolve services into the servers that back them. It declares
providers in a `discovery` block, each with a code, a name and a
configuration form, and requests `network`. A person binds an upstream name
to a service of a provider; the host calls `discovery.resolve` on the
interval of the binding, validates every target, writes an `upstream` block
to a file of its own in the nginx configuration directory and reloads nginx
when the servers changed. The person includes the file at the `http` level
and proxies to the upstream by name.

## Motivation

Servers behind nginx come and go: containers are rescheduled, instances
scale, deployments roll. nginx resolves host names only at start (without a
commercial `resolve` parameter), so upstreams are edited by hand or by
scripts. Every registry and orchestrator has its own API, so the core cannot
ship them, while writing an upstream block, scheduling refreshes and
reloading nginx safely is the same for all of them.

## Design

### Manifest

```json
{
  "capabilities": ["upstream.discovery"],
  "permissions": ["network"],
  "discovery": {
    "providers": [
      {
        "code": "registry",
        "name": "Service registry",
        "configuration": { "fields": [{ "key": "address", "display_name": "Registry address", "required": true }] }
      }
    ]
  }
}
```

### Method

| Method | Params | Result |
| --- | --- | --- |
| `discovery.resolve` | `{ provider, config, service }` | `{ targets: [{ address, port, weight, tags }], ttl_seconds }` |

`port` and `weight` are `int32` (WIRE-10); `weight` `0` means 1. The answer
is the complete set of servers, and an error keeps the servers the host has
(DISCOVERY-6). An empty set is a valid answer, but the host cannot write an
upstream without a server, so it keeps the previous file and records the run
as failed. A service the provider does not know is `-32003` naming
`service`, since the person typed it.

### Reference host

* `internal/upstream/discovery` holds the provider registry (`Source`,
  `RegisterSource`, `Providers`), turns an answer into the file (`Render`)
  and runs the refreshes (`Runner`). `internal/plugin/capability.RegisterDiscovery`
  registers the plugin manager as a source; a binding stores its provider as
  `plugin:<code>`.
* `model.UpstreamDiscovery { upstream_name, kind, config, service,
  refresh_seconds, extra_directives, enabled, last_run_at, next_run_at,
  last_status, last_message, target_count }`, with `config` encrypted at
  rest. Migration `20260923000005` creates the table.
* The runner works like the blocklist runner of RFC 0008, checking every
  10 seconds, with a default interval of 60 seconds and a shortest one of
  10, and `discovery.resolve` has 30 seconds. Files are written with the same `GeneratedWriter`: nothing
  happens unless the content changed, and every change is tested before the
  reload and rolled back when the test fails, for example because another
  file defines an upstream with the same name.
* A target is kept when its address parses as an IP address or is a valid
  host name, its port is 1 to 65535 and its weight 0 to 1000; an IPv6
  address is written in brackets, duplicates are removed and the rest
  sorted. The upstream name must match
  `^[A-Za-z0-9_][A-Za-z0-9_.-]{0,63}$` and be unique among the bindings
  (55210, 55211); extra directives, appended inside the block, must not
  contain braces (55212).
* The file is `<nginx conf dir>/upstreams/<upstream_name>.conf`; the page
  shows `include upstreams/<upstream_name>.conf;` for the `http` block with a
  copy button.
* `GET/POST/DELETE /api/upstream_discoveries` (CRUD with the same
  protections as the blocklist sources), `GET /api/upstream_discoveries/kinds`
  and `POST /api/upstream_discoveries/:id/refresh`. A provider whose plugin
  is gone fails with 55207, a missing required field with 55208. Deleting a
  binding removes its file after a configuration test and is refused with
  55213 while nginx still includes it.
* The upstream page gains a "Service Discovery" section listing the bindings
  with their last result and managing them with a provider selector,
  `PluginConfigForm`, the service, the interval, the extra directives, a
  refresh button and the include line.

## Compatibility

A new optional capability, block and service; nothing existing changes
meaning (VER-1). The generated files take effect only where a person
includes them.

## Alternatives considered

* **Rewriting upstream blocks inside the site files.** Every refresh would
  rewrite files people edit by hand.
* **The nginx Plus API or `resolve` parameter.** Not available in the open
  source build most installations run.
* **A DNS based approach only.** Covered by the host names a provider may
  return; registries that expose weights and ports need more than DNS.

## Security considerations

The answer decides where requests are proxied, so the host only writes
parsed addresses, ports and weights, never other text of the answer
(SEC-15), and only into a file a person includes. Extra directives come from
the administrator and cannot leave the block. Every reload is preceded by a
configuration test with rollback. Provider values are credentials:
encrypted at rest, masked in the form and never logged.

## Future work

* Health information from the provider mapped to `down` or `backup`.
* Stream (TCP/UDP) upstreams.
