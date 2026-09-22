# 16. Capability: `cert.deploy`

A plugin declaring `"cert.deploy"` in `capabilities` pushes certificates the
host issued or renewed to places that terminate TLS outside the host: a CDN,
a cloud load balancer, a mail server, a NAS. It declares the kinds of target
it can push to; a person configures one or more targets of a kind and binds
them to certificates, and the host calls the plugin with the certificate,
its private key and its chain whenever a bound certificate is issued or
renewed, and whenever the person asks for it. Both methods here are host →
plugin requests.

| Method | Required | Meaning |
| --- | --- | --- |
| `deploy.push` | yes | Push one certificate to one target, or check that it could be pushed. |
| `deploy.validate` | no | Check a target configuration without contacting the target. |

A plugin that does not implement `deploy.validate` MUST reply with `-32002`
(Unsupported, WIRE-6) or `-32601`, and a host MUST treat both as "no
opinion" rather than as a rejected configuration.

`deploy.push` hands the plugin a private key. A `cert.deploy` plugin MUST
therefore request the `cert.deploy` permission, which tells the person
approving it exactly that (SEC-14), and SHOULD request `network` (SEC-3),
since it talks to the target.

## Manifest block

## DEPLOY-1

The manifest MUST include a `deploy` block with at least one entry in
`deploy.targets`, and MUST request the `cert.deploy` permission, whenever
`capabilities` includes `"cert.deploy"` (MAN-35):

```json
{
  "capabilities": ["cert.deploy"],
  "permissions": ["cert.deploy", "network"],
  "deploy": {
    "targets": [
      {
        "code": "mycdn",
        "name": "MyCDN",
        "configuration": {
          "fields": [
            { "key": "api_token", "display_name": "API token", "required": true, "secret": true },
            { "key": "zone_id", "display_name": "Zone ID", "help_text": "Zone the certificate is bound to", "required": true }
          ]
        }
      }
    ]
  }
}
```

Each entry of `deploy.targets` describes a kind of target:

| Field | Type | Required | Meaning |
| --- | --- | --- | --- |
| `code` | string | yes | Identifier of the target kind on the wire, sent as `kind`. See NAME-10. |
| `name` | string | yes | Display name of the target kind. |
| `configuration.fields` | object[] | no | The fields of the target form, as in NOTIFY-4. |

## DEPLOY-2

`code` MUST match `^[a-z0-9-]{2,32}$` and MUST be unique among the target
kinds one manifest declares. See `spec/11-naming.md` NAME-10 for
cross-plugin uniqueness.

## DEPLOY-3

`name` MUST be a non-empty string. `configuration.fields` follows NOTIFY-4:
the same field shape, the same string encoding of values, the same rules for
`key`, `type`, `display_name`, `required` and `secret`.

## `deploy.push`

## DEPLOY-4

Request:

```json
{
  "jsonrpc": "2.0", "id": 47, "method": "deploy.push",
  "params": {
    "kind": "mycdn",
    "config": { "api_token": "tok_live_xxx", "zone_id": "zone_123" },
    "certificate": {
      "name": "example.com",
      "domains": ["example.com", "www.example.com"],
      "certificate_pem": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n",
      "private_key_pem": "-----BEGIN EC PRIVATE KEY-----\n...\n-----END EC PRIVATE KEY-----\n",
      "chain_pem": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----\n",
      "not_after": "2026-12-20T08:15:00Z"
    },
    "dry_run": false
  }
}
```

| Field | Type | Meaning |
| --- | --- | --- |
| `kind` | string | The `code` of the target kind, without any host prefix. |
| `config` | map\<string, string\> | The field values the person filled in, keyed by `key` (NOTIFY-4). |
| `certificate.name` | string | The name of the certificate in the host. |
| `certificate.domains` | string[] | The domains and IP addresses the certificate covers. |
| `certificate.certificate_pem` | string | The leaf certificate, one PEM block. |
| `certificate.private_key_pem` | string | The private key of the leaf, one PEM block in the format the host stores (PKCS #1, SEC 1 or PKCS #8). |
| `certificate.chain_pem` | string | The intermediate certificates as PEM blocks, starting with the issuer of the leaf. Empty when there is none. |
| `certificate.not_after` | string | Expiry of the leaf as an RFC 3339 timestamp. |
| `dry_run` | boolean | Check without changing anything (DEPLOY-6). |

A plugin that needs the full chain in one piece concatenates
`certificate_pem` and `chain_pem`.

## DEPLOY-5

On success a plugin MUST reply with `{ "message": "..." }` once the target
accepted the certificate, with a short human readable summary of what it did
(which resource it created or replaced). `message` MAY be empty and MUST NOT
contain a credential or the private key (SEC-7). On failure it MUST reply
with an error: `-32003` (`data.field` naming the field at fault) for anything
caused by what the person entered (a revoked token, a zone that does not
exist), `-32000` for anything else (a target outage, a network failure). A
push MUST be idempotent: pushing the certificate a target already serves
MUST succeed, since the host retries and a person may push again.

## DEPLOY-6

When `dry_run` is `true` a plugin MUST NOT change anything at the target. It
SHOULD check what a real push needs (the target is reachable, the
credentials are accepted, the resource exists) with read-only calls, MUST
reject a `config` that leaves a required field empty with `-32003`, and
replies with a `message` that says what a real push would do.

## DEPLOY-7

A plugin MUST treat `config` values and the whole `certificate` as secret,
as NOTIFY-7 describes: the private key MUST NOT appear in a log line, an
error message or anything sent back to the host, and a plugin MUST NOT write
it to disk except where the target itself needs a file (a plugin that pushes
over SSH writes it on the target, not in its data directory).

## DEPLOY-8

A plugin MUST NOT keep the private key after it replied, and MUST NOT push
the certificate anywhere the `config` of the call does not name.

## `deploy.validate`

## DEPLOY-9

Request:

```json
{ "jsonrpc": "2.0", "id": 46, "method": "deploy.validate", "params": { "kind": "mycdn", "config": { "api_token": "tok_live_xxx" } } }
```

A plugin implementing `deploy.validate` MUST check `config` against what the
target kind needs, MUST report the first missing or malformed field as
`-32003` with `data.field` set, MUST NOT contact the target to do so, and
MUST reply `{}` when the configuration is well-formed.

## Host behavior

## DEPLOY-10

A host MUST offer each target kind of every enabled `cert.deploy` plugin
whose `cert.deploy` permission is granted (SEC-4, SEC-14), and MUST stop
offering it, and stop calling `deploy.push` for it, once the plugin is
disabled, uninstalled or upgraded to a version whose permissions still need
approval. A host MUST keep kind codes apart from any target kind of its own;
the reference host stores a target's kind as `plugin:<code>`. A host MUST
render the target form from `configuration.fields` as NOTIFY-10 describes,
MUST NOT store a configuration that leaves a `required` field empty, MUST
store the values encrypted at rest, MAY call `deploy.validate` before it
stores a configuration and MUST NOT store it when the plugin answers
`-32003`. A host MUST call only the plugin that owns the kind `code` at the
time of the call (NAME-10) and starts an `on_demand` plugin to do so.

## DEPLOY-11

A host MUST send a certificate only to the targets a person bound to it, and
MUST read the certificate, key and chain afresh for every call, so a retry
never pushes material that was replaced in the meantime. The reference host
binds a target to one certificate or to every certificate, and pushes to
every enabled target bound to a certificate when it publishes `cert.issued`
or `cert.renewed` for it (HOST-16). A push that fails is retried 30 seconds,
2 minutes and 10 minutes later; the schedule is a recommendation. A person
can also push a certificate to its targets, or a target to its
certificates, on demand, which runs one attempt without retries, and can
test a target with `dry_run`.

## DEPLOY-12

A host MUST record the outcome of every push it made on the person's behalf,
succeeded or failed, with the `message` or the error and the number of
attempts, and MUST show it next to the certificate, so a certificate that
silently stopped reaching a target does not go unnoticed. The reference host
waits 5 minutes for `deploy.push` and 30 seconds for `deploy.validate`; these
values are a recommendation.
