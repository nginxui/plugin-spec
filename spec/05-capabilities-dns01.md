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
        "links": { "api": "https://mydns.example/docs/api" },
        "propagation_timeout_seconds": 120,
        "polling_interval_seconds": 2,
        "form": {
          "fields": [
            { "key": "MYDNS_API_TOKEN", "label": "API token", "group": "credential", "secret": true },
            { "key": "MYDNS_TTL", "label": "TXT record TTL", "group": "setting", "default": "120", "unit": "seconds" }
          ]
        }
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
| `links.api` | string | no | Vendor API documentation URL. |
| `propagation_timeout_seconds` | integer | no | Overrides the host's default propagation wait for this provider. |
| `polling_interval_seconds` | integer | no | How often the host (or the plugin, for `dns01.check`) re-checks propagation. |
| `form` | object | yes | Every value the provider accepts and how to lay out the credential form: labels, sign-in methods, defaults. See DNS01-14. |

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
| `config` | map\<string, string\> | The values the person filled in, keyed by `form.fields[].key`: the credential and the setting fields merged into one map (DNS01-15). |
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

## Credential form

## DNS01-14

Every provider MUST carry a `form` object. It is the only description of
the values the provider accepts: a host renders the credential form from it
and from nothing else. A provider that takes no values declares an empty
`fields` array.

```json
{
  "form": {
    "fields": [
      { "key": "MYDNS_API_TOKEN", "label": "API token", "help": "Needs the DNS edit permission.", "group": "credential", "secret": true },
      { "key": "MYDNS_API_EMAIL", "label": "Account email", "group": "credential" },
      { "key": "MYDNS_API_KEY", "label": "API key", "group": "credential", "secret": true },
      { "key": "MYDNS_TTL", "label": "TXT record TTL", "group": "setting", "default": "120", "unit": "seconds" }
    ],
    "methods": [
      { "name": "API token", "recommended": true, "fields": ["MYDNS_API_TOKEN"] },
      { "name": "Global API key", "fields": ["MYDNS_API_EMAIL", "MYDNS_API_KEY"] },
      { "name": "Instance role", "fields": [], "values": { "MYDNS_AUTH_MODE": "instance" } }
    ]
  }
}
```

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `fields` | array | yes | The inputs in display order (DNS01-15). |
| `methods` | array | no | The ways to sign in, when there is more than one (DNS01-16). |

## DNS01-15

Each entry of `form.fields`:

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `key` | string | yes | The config key the value is stored under and sent to the plugin under. |
| `label` | string | yes | Short plain English label. |
| `help` | string | no | One plain English sentence shown under the input. |
| `group` | string | yes | `"credential"` for a sign-in value, `"setting"` for a tuning value such as a TTL, a base URL or a timeout. |
| `optional` | boolean | no | The provider works without a value. Absent means `false`. |
| `secret` | boolean | no | The value is a password, token or key and SHOULD be rendered as a password input. |
| `default` | string | no | The value the provider uses when the field is empty. A host SHOULD show it as a placeholder and MUST NOT store it on the person's behalf. |
| `unit` | string | no | `"seconds"` when the value is a number of seconds. Absent means no unit. |
| `link` | string | no | A documentation URL for the field. |

`key` MUST be unique within `form.fields`. A host MUST NOT ask for, store
or send a key that neither `form.fields` nor the `values` of a method
(DNS01-16) lists. A host SHOULD show
`"setting"` fields apart from the credentials, for example collapsed under
an advanced section. Properties that are empty, `false` or absent mean the
same thing, so a plugin SHOULD omit them to keep the manifest small.

`group` decides where a host keeps a value: it stores the values of
`"credential"` fields and of `"setting"` fields in two separate maps of the
saved credential, and it MUST treat the `"credential"` values as secret
(SEC-7). What the plugin receives does not change with the group: every
`dns01` request carries the saved values of both groups merged into the one
`config` map of DNS01-4, keyed by `key`.

## DNS01-16

`form.methods` MUST be absent or have at least two entries. Each entry:

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `name` | string | yes | Plain English name of the way to sign in. |
| `recommended` | boolean | no | The method a host SHOULD preselect. At most one entry MAY set it. |
| `fields` | array\<string\> | yes | The keys of the `"credential"` fields this method uses. Empty when the method needs no input, such as a role the server already has. |
| `values` | map\<string, string\> | no | Fixed config values that select this method on the plugin side, such as an authentication mode. |

Every key in a method's `fields` MUST be the `key` of a `"credential"` entry
of `form.fields`. Two methods MAY list the same key. A `"credential"` field
that no method lists is shared: a host MUST show it with every method. A
host MUST show only the fields of the chosen method (plus the shared ones),
and on save MUST store the values of those fields and clear the credential
values the chosen method does not use.

A key of a method's `values` is usually not a field at all: the person never
edits it. It MAY also be the `key` of a `form.fields` entry when that entry
is a `"credential"` field, at least one method lists it in `fields`, and no
method both lists it in `fields` and sets it in `values`. The field is then
shown only with the methods that list it, and the methods that set it fix
its value instead, such as an algorithm the person picks for one way to sign
in and that another way requires.

On save a host MUST store every entry of the chosen method's `values`, as a
`"credential"` value when the key is a credential field and as a
`"setting"` value otherwise. It MUST remove each key that appears in the
`values` of another method unless the chosen method sets it or lists it in
`fields`. These keys reach the plugin in `config` like any other value
(DNS01-4).

When editing saved values, a host SHOULD preselect the method whose
`values` all match the saved ones and whose fields hold values; otherwise
the recommended method, then the first one.

## DNS01-17

Every `label`, `help` and method `name` in `form`, and the provider `name`,
is an English gettext msgid. A host SHOULD look each one up in the
translations the plugin registers with `registerTranslations`
(`spec/07-webapp.md` WEB-7) and
MUST fall back to the English text when there is no translation.

## DNS01-18

A `form` MUST be well formed:

- every `form.fields` entry has a non-empty `key` no other entry has and a
  non-empty `label`;
- `group` is `"credential"` or `"setting"`, and `unit` is `"seconds"` or
  absent;
- `form.methods` is absent or has at least two entries, method names are
  unique, every key a method's `fields` lists is a `"credential"` field of
  `form.fields`, and at most one method is `recommended`;
- every key of a method's `values` is non-empty; when it is also the `key`
  of a `form.fields` entry, that entry is a `"credential"` field, some
  method lists it in `fields`, and no method both lists it and sets it in
  `values`;
- no two methods have the same `fields` (in any order) and the same
  `values`.

The reference linter reports a form that breaks one of these as an error
under DNS01-18 (`spec/09-conformance.md`). A host SHOULD refuse to install a
plugin whose manifest carries a provider without a `form` or with a form
that breaks one of these.
