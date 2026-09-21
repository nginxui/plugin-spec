# 05. Capability: `dns01`

A plugin declaring `"dns01"` in `capabilities` solves the ACME DNS-01
challenge: publishing a `_acme-challenge` TXT record with a vendor API,
optionally waiting for it to propagate, and cleaning it up afterward. All
five methods here are host → plugin requests.

| Method | Required | Meaning |
| --- | --- | --- |
| `dns01.present` | yes | Publish the challenge TXT record. |
| `dns01.cleanup` | yes | Remove what `present` published. |
| `dns01.validate` | no | Check that credentials look usable, without publishing anything. |
| `dns01.options` | no | Report propagation timing for a provider. |
| `dns01.check` | no | Report whether the record has propagated. |

A plugin MUST implement `present` and `cleanup`; a host MUST NOT call
`validate`, `options` or `check` unless a value for `Provider` in the request
maps to a provider the manifest declares (DNS01-1), and a plugin that does
not implement an optional method it is asked for MUST reply with
`-32002` (Unsupported, WIRE-6).

## Manifest block

## DNS01-1

The manifest MUST include a `dns01` block whenever `capabilities` includes
`"dns01"`:

```json
{
  "dns01": {
    "providers": [
      {
        "name": "MyDNS",
        "code": "mydns",
        "configuration": {
          "credentials": { "MYDNS_API_TOKEN": "API token for the MyDNS account" },
          "additional": { "MYDNS_TTL": "TXT record TTL in seconds (default: 120)" }
        },
        "links": { "api": "https://mydns.example/docs/api" },
        "propagation_timeout_seconds": 120,
        "polling_interval_seconds": 2
      }
    ]
  }
}
```

`dns01.providers` MUST have at least one entry. Each entry:

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `name` | string | yes | Display name of the DNS vendor. |
| `code` | string | yes | Short identifier used on the wire and in the credential form. See `spec/11-naming.md`. |
| `configuration.credentials` | map\<string, string\> | no | Field name to help text, for secret values (API tokens, passwords). |
| `configuration.additional` | map\<string, string\> | no | Field name to help text, for non-secret values (TTL, base URL, timeouts). |
| `links.api` | string | no | Vendor API documentation URL. |
| `links.go_client` | string | no | Upstream client library URL, when the plugin wraps one. |
| `propagation_timeout_seconds` | integer | no | Overrides the host's default propagation wait for this provider. |
| `polling_interval_seconds` | integer | no | How often the host (or the plugin, for `dns01.check`) re-checks propagation. |

## DNS01-2

`code` MUST match `^[a-z0-9-]{2,32}$` and MUST be unique among the providers
one manifest declares. See `spec/11-naming.md` NAME-4 for cross-plugin
uniqueness.

## DNS01-3

`name` MUST be a non-empty string.

## `dns01.present`

## DNS01-4

Request:

```json
{
  "jsonrpc": "2.0", "id": 10, "method": "dns01.present",
  "params": {
    "provider": "mydns",
    "config": { "MYDNS_API_TOKEN": "tok_live_xxx", "MYDNS_TTL": "120" },
    "options": { "credential_id": "7", "disable_cname": false },
    "domain": "example.com",
    "fqdn": "_acme-challenge.example.com.",
    "effective_fqdn": "_acme-challenge.example.com.",
    "value": "gfj9Xq...Rg85nM",
    "token": "evaGxfADs...62jcerQ",
    "key_auth": "evaGxfADs...62jcerQ.9jg46WB3...GKl3Z7",
    "dry_run": false
  }
}
```

| Field | Type | Meaning |
| --- | --- | --- |
| `provider` | string | The `code` of the provider to use, without the plugin id prefix. |
| `config` | map\<string, string\> | The merged `credentials` and `additional` field values the person filled in. |
| `options` | object | Opaque per-certificate options from the challenge form slot (`spec/07-webapp.md`). Absent when the plugin ships no custom form. |
| `domain` | string | The certificate identifier, leading wildcard `*.` removed. |
| `fqdn` | string | The `_acme-challenge` name derived by the ACME client, before CNAME following. |
| `effective_fqdn` | string | The CNAME target when one was followed, otherwise equal to `fqdn`. |
| `value` | string | The TXT record value derived from `key_auth`. |
| `token` | string | The raw ACME challenge token. |
| `key_auth` | string | The ACME key authorization `value` and `fqdn` were derived from. A plugin MAY derive them itself instead of trusting the host's precomputed `value`/`fqdn`. |
| `dry_run` | boolean | When `true`, the plugin MUST validate shapes and MUST NOT contact the vendor API or mutate any DNS record. |

## DNS01-5

On success, a plugin MUST reply with an empty result (`{}`). On failure it
MUST reply with an error: `-32003` (`data.field` naming the credential or
setting at fault) for anything caused by what the person entered, `-32000`
for anything else (a vendor API outage, a network failure).

## DNS01-6

A plugin MUST treat `config` values as secret: they MUST NOT appear in a log
line, an error message, or anywhere sent back to the host. See SEC-7.

## `dns01.cleanup`

## DNS01-7

Request: identical shape to `dns01.present` (same params object). A plugin
MUST attempt to remove the record `present` published for the same
`effective_fqdn`/`value` pair and MUST reply `{}` on success.

## DNS01-8

`dns01.cleanup` MUST be safe to call even when the matching `present` call
never happened or already failed (e.g. the host retries cleanup
unconditionally as part of certificate issuance teardown). A plugin MUST
treat "nothing to clean up" as success, not as an error.

## `dns01.validate`

## DNS01-9

Request:

```json
{ "jsonrpc": "2.0", "id": 11, "method": "dns01.validate", "params": { "provider": "mydns", "config": { "MYDNS_API_TOKEN": "" } } }
```

A plugin implementing `dns01.validate` MUST build the provider client from
`config` and report the first missing or malformed field as `-32003` with
`data.field` set, MUST NOT contact the vendor API to do so, and MUST reply
`{}` when the configuration is well-formed. This lets a host validate
credentials while a person is filling in a form, without issuing anything.

## `dns01.options`

## DNS01-10

Request:

```json
{ "jsonrpc": "2.0", "id": 12, "method": "dns01.options", "params": { "provider": "mydns", "config": { "...": "..." }, "options": { "...": "..." } } }
```

Reply:

```json
{
  "jsonrpc": "2.0", "id": 12,
  "result": { "propagation_timeout_seconds": 120, "polling_interval_seconds": 2, "sequential_interval_seconds": 0 }
}
```

`sequential_interval_seconds`, when non-zero, tells the host this provider's
API cannot solve two challenges for the same account concurrently and that
the host MUST space out calls by at least that many seconds instead of
issuing them in parallel. A host MUST fall back to the manifest's
`propagation_timeout_seconds`/`polling_interval_seconds` for a provider
(DNS01-1), then to its own default, when the plugin does not implement this
method (`-32002`).

## `dns01.check`

## DNS01-11

Request:

```json
{
  "jsonrpc": "2.0", "id": 13, "method": "dns01.check",
  "params": { "provider": "mydns", "config": { "...": "..." }, "options": { "...": "..." }, "domain": "example.com", "fqdn": "_acme-challenge.example.com.", "value": "gfj9Xq...Rg85nM", "key_auth": "evaGxfADs...62jcerQ.9jg46WB3...GKl3Z7" }
}
```

Reply:

```json
{ "jsonrpc": "2.0", "id": 13, "result": { "ready": false, "effective_fqdn": "_acme-challenge.example.com.", "detail": "NXDOMAIN from ns1.example.com" } }
```

| Field | Type | Meaning |
| --- | --- | --- |
| `ready` | boolean | Whether the record is visible and correct wherever the plugin checks. |
| `effective_fqdn` | string | The CNAME target actually queried, when different from `fqdn`. |
| `detail` | string | Human readable reason, mainly useful when `ready` is `false`. |

A host that gets `-32002` for `dns01.check` MUST fall back to its own
built-in propagation check (querying `fqdn`/`effective_fqdn` for the TXT
record directly) rather than treating it as a fatal error.

## Per-certificate options

## DNS01-12

The `options` object in `dns01.present`, `dns01.cleanup`, `dns01.options` and
`dns01.check` is opaque to the host: it is exactly what the certificate's
`challenge_config` field contained at issuance time (`spec/07-webapp.md`
WEB-9). A plugin defining its own challenge form slot component owns this
shape entirely. The official `dns01` plugin uses:

| Key | Type | Meaning |
| --- | --- | --- |
| `credential_id` | string | The selected credential's id, as a string. |
| `disable_cname` | boolean | Do not follow the challenge record's CNAME. |
| `disable_authoritative_ns_propagation` | boolean | Skip the authoritative nameserver check. |
| `disable_recursive_ns_propagation` | boolean | Skip the recursive nameserver check. |

A different `dns01` plugin MAY define a completely different `options`
shape; there is no cross-plugin contract for it beyond "it round-trips
through `challenge_config` unchanged."

## DNS01-13

A plugin MUST tolerate `options` being absent or `{}` (e.g. a certificate
issued before the plugin's own form slot existed) and MUST apply reasonable
defaults in that case rather than failing the call.
