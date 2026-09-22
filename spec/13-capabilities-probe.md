# 13. Capability: `probe`

A plugin declaring `"probe"` in `capabilities` adds ways of checking whether
a target is healthy beyond the host's built-in HTTP and gRPC checks: a TCP
banner, a database login, a vendor status API. The host offers every probe
kind the plugin declares next to its built-in check, and runs the kind a
person picked on the schedule of that target's health check. The one method
here is a host → plugin request.

| Method | Required | Meaning |
| --- | --- | --- |
| `probe.check` | yes | Check one target once. |

A `probe` plugin needs no permission to receive `probe.check`; it SHOULD
request `network` (SEC-3), since it reaches out to the target.

## Manifest block

## PROBE-1

The manifest MUST include a `probe` block with at least one entry in
`probe.kinds` whenever `capabilities` includes `"probe"` (MAN-32):

```json
{
  "probe": {
    "kinds": [
      {
        "code": "tcp-banner",
        "name": "TCP banner",
        "configuration": {
          "fields": [
            { "key": "port", "type": "number", "display_name": "Port", "required": true },
            { "key": "expect", "display_name": "Expected banner prefix", "help_text": "For example SSH-2.0" }
          ]
        }
      }
    ]
  }
}
```

Each entry of `probe.kinds`:

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `code` | string | yes | Identifier of the kind on the wire. See NAME-10. |
| `name` | string | yes | Display name of the kind. |
| `configuration.fields` | object[] | no | The fields of the probe form, as in NOTIFY-4. |

## PROBE-2

`code` MUST match `^[a-z0-9-]{2,32}$` and MUST be unique among the kinds one
manifest declares. See `spec/11-naming.md` NAME-10 for cross-plugin
uniqueness.

## PROBE-3

`name` MUST be a non-empty string. `configuration.fields` follows NOTIFY-4:
the same field shape, the same string encoding of values, the same rules for
`key`, `type`, `display_name`, `required` and `secret`.

## `probe.check`

## PROBE-4

Request:

```json
{
  "jsonrpc": "2.0", "id": 34, "method": "probe.check",
  "params": {
    "kind": "tcp-banner",
    "target": "https://example.com",
    "config": { "port": "22", "expect": "SSH-2.0" },
    "timeout_seconds": 10
  }
}
```

| Field | Type | Meaning |
| --- | --- | --- |
| `kind` | string | The `code` of the probe kind, without any host prefix. |
| `target` | string | What to check. The reference host sends the site URL, or the custom target URL of the health check when one is set. A kind decides which parts it uses (a TCP kind reads the host name and ignores the scheme). |
| `config` | map\<string, string\> | The field values the person filled in, keyed by `key`. |
| `timeout_seconds` | integer | How long the host waits for the answer. Positive. |

A plugin SHOULD finish the check within `timeout_seconds`, and SHOULD report
a target that did not answer in time as `down` rather than letting the call
itself time out. A host MAY give up on the call shortly after
`timeout_seconds`; the reference host allows five more seconds.

## PROBE-5

Reply:

```json
{ "jsonrpc": "2.0", "id": 35, "result": { "status": "down", "latency_ms": 10000, "message": "no banner within 10s" } }
```

| Field | Type | Meaning |
| --- | --- | --- |
| `status` | string | `up`, `down` or `degraded`. |
| `latency_ms` | integer | Time the check took, in milliseconds. Not negative. |
| `message` | string | Human readable detail, shown to the person. Expected when `status` is not `up`. |

`degraded` means the target answers but not as well as it should (slow,
partially failing, a warning from a status API). An unhealthy or unreachable
target is a result, not an error: a plugin MUST report it as `down` with a
`message`. A plugin MUST reply with an error only when the check itself could
not run: `-32003` (`data.field` naming the field at fault) for a
configuration it cannot use, `-32000` for anything else. `message` MUST NOT
contain a credential (SEC-7).

## PROBE-6

A plugin MUST treat `config` values as secret, as NOTIFY-7 describes.

## Host behavior

## PROBE-7

A host MUST offer each kind of every enabled `probe` plugin as a choice next
to its built-in check, MUST keep its built-in check the default, and MUST
keep kind codes apart from the names of its built-in checks; the reference
host stores a plugin kind as `plugin:<code>` in the optional `probe_kind` of
a site's health check, and its `probe_config` next to it. A host MUST render
the probe form from `configuration.fields` as NOTIFY-10 describes. A host
MUST call only the plugin that owns the kind `code` at the time of the check
(NAME-10) and starts an `on_demand` plugin to do so.

## PROBE-8

A host MUST report a failed call (an error reply, a timeout, a plugin that
is disabled or gone) as a failed check that is distinct from `down`, so the
person can tell "the target is down" from "the probe did not run". The
reference host maps `up` to online, `degraded` to online with the message
attached, `down` to offline and a failed call to error, and does not count
`degraded` as a failure for its health check alerts.
