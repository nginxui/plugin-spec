# RFC 0003: The `probe` capability

| | |
| --- | --- |
| Status | Accepted, implemented |
| Date | 2026-09-23 |
| Spec changes | `spec/13-capabilities-probe.md` (PROBE-1 through PROBE-8), MAN-19, MAN-32, NAME-10, CONF-9, WIRE-9 table, `proto/nginxui/plugin/v1/probe.proto`, `ManifestProbe` in `manifest.proto`, `schema/plugin.schema.json`, vectors 27 through 29 |
| Reference host | nginx-ui `internal/plugin/capability/probe.go`, `internal/sitecheck/probe.go`, `model.SiteConfig` (`probe_kind`, `probe_config`), migration `20260923000001`, `api/sites` (`GET /api/site_navigation/probe_kinds`), `app/src/views/dashboard/components/SiteHealthCheckModal.vue` |
| Reference SDK | plugin-sdk-go `probe.go` (`ProbeHandler`, `ProbeUp`, `ProbeDown`, `ProbeDegraded`) |

## Summary

A plugin may add ways of checking whether a target is healthy. It declares
probe kinds in a `probe` block, each with a code, a name and a configuration
form. The host offers them as check methods of a site's health check next to
its built-in HTTP and gRPC check, and calls `probe.check` on the check's
schedule. The answer is `up`, `down` or `degraded` with a latency and a
message, which the host maps onto the site status and its alerting.

## Motivation

The site health check speaks HTTP and the gRPC health protocol. A site behind
nginx is often something else: a mail relay, a database, an SSH bastion, a
service whose health is only visible through a vendor status API. Growing
every protocol into the core is not sustainable, while the health check
already separates "how to probe" from scheduling, caching, status display and
alerting. A plugin only needs to supply the probing.

## Design

### Manifest

```json
{
  "capabilities": ["probe"],
  "permissions": ["network"],
  "probe": {
    "kinds": [
      {
        "code": "tcp-banner",
        "name": "TCP banner",
        "configuration": {
          "fields": [
            { "key": "port", "type": "number", "display_name": "Port", "required": true },
            { "key": "expect", "display_name": "Expected banner prefix" }
          ]
        }
      }
    ]
  }
}
```

The form reuses `ConfigurationSchema` from RFC 0002, and kind codes follow
the same rules as channel codes (NAME-10).

### Method

| Method | Params | Result |
| --- | --- | --- |
| `probe.check` | `{ kind, target, config, timeout_seconds }` | `{ status, latency_ms, message }` |

`target` is the site URL or the custom target URL of the health check; a
kind uses whatever part of it makes sense. An unreachable target is a `down`
result with a message, never an error: errors are reserved for a check that
could not run, so the host can keep "the target is down" apart from "the
probe is broken" (PROBE-5, PROBE-8).

### Reference host

* `internal/sitecheck` gains a probe kind registry (`ProbeSource`,
  `RegisterProbeSource`, `ProbeKinds`, `RunProbe`). The built-in check is the
  kind `http`, and an empty kind means the same, so every existing site keeps
  its behavior.
* The site health check model gains `probe_kind` and `probe_config`
  (encrypted at rest, since fields may be credentials). Migration
  `20260923000001` adds the two columns; existing rows keep NULL. Both are
  omitted from JSON while empty, so the responses of a site without a probe
  kind are byte for byte what they were.
* `internal/plugin/capability.RegisterProbe` registers the plugin manager as a
  source. A kind is stored as `plugin:<code>`, for the reason RFC 0002 gives.
  The owner is resolved per check and an `on_demand` plugin is started.
  `timeout_seconds` is the site's timeout rounded up to seconds; the host
  waits five more seconds before it gives up.
* `up` maps to online, `degraded` to online with the message and the error
  type `degraded`, `down` to offline with the error type `probe`, and a failed
  call (error reply, timeout, plugin disabled) to the error status. A
  degraded result is not an alert failure.
* `GET /api/site_navigation/probe_kinds` lists the kinds with their forms. The
  health check dialog shows a "Check method" selector only when the list is
  not empty (or the site still uses a kind whose plugin is gone), renders the
  kind's form with `PluginConfigForm`, and hides the HTTP and gRPC specific
  settings while a plugin kind is selected. The test button runs the kind
  once through `POST /api/site_navigation/test_health_check/:id`, which
  accepts `probe_kind` and `probe_config` next to `config`.
* Synchronizing a health check to other nodes carries the kind and its
  values. A node without the plugin reports the site as an error until the
  plugin is installed there.

## Compatibility

Additive on every side. Hosts that predate the capability reject a manifest
declaring it (MAN-19), as for any unknown capability. The two new database
columns are nullable and the JSON fields are omitted while empty.

## Alternatives considered

* **A probe per upstream instead of per site.** The upstream availability
  check has its own TCP prober; extending it can follow once a probe plugin
  exists, it does not change this contract.
* **Letting a probe replace only parts of the HTTP check** (for example the
  response validation). It would couple the contract to the HTTP check's
  options. A kind replaces the whole check instead.
* **Reporting a numeric score instead of three states.** The dashboard and the
  alerting only distinguish healthy, unhealthy and failed; three named states
  are enough and leave no room for per-plugin thresholds.

## Security considerations

A probe plugin runs with the host's privileges and reaches whatever target a
person configures, like the built-in check. Probe values may be credentials;
the reference host encrypts them at rest and masks `secret` fields, and a
plugin must not log them (PROBE-6). Changing a health check keeps requiring a
secure session, and testing one stays unavailable in demo mode.

## Future work

* The same kinds for the upstream availability check.
* A `probe.validate` method, if forms turn out to need checking before save.
