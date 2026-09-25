# RFC 0006: The `cert.deploy` capability

| | |
| --- | --- |
| Status | Accepted, implemented |
| Date | 2026-09-23 |
| Spec changes | `spec/16-capabilities-deploy.md` (DEPLOY-1 through DEPLOY-12), MAN-19, MAN-23, MAN-35, SEC-3 (`cert.deploy` permission), SEC-7, SEC-14, NAME-10, CONF-4, CONF-7, CONF-12, WIRE-6 (`-32002` row), WIRE-9 table, WIRE-11 routing table, LIFE-10 (shutdown list), `proto/nginxui/plugin/v1/deploy.proto`, `ManifestDeploy` in `manifest.proto`, `schema/plugin.schema.json`, vectors 39 through 42 |
| Reference host | nginx-ui `internal/plugin/capability/deploy.go`, `internal/cert/deploy` (registry, certificate loading, runner), `model.CertDeployTarget`, `model.CertDeployment`, migration `20260923000003`, `api/cert_deploy`, `app/src/views/certificate/DeployTargets.vue`, `app/src/views/certificate/components/CertificateDeployTargets.vue`, `app/src/views/system/plugins/permissions.ts` |
| Reference SDK | plugin-sdk-go `deploy.go` (`DeployHandler`, `DeployValidator`) |

## Summary

A plugin may push issued certificates to external targets: a CDN, a cloud
load balancer, another server. It declares kinds of target in a `deploy`
block, each with a code, a name and a configuration form, and requests the
new `cert.deploy` permission. A person creates targets of those kinds and
binds each to one certificate or to all of them; the host calls
`deploy.push` with the certificate, its private key and its chain after
every issuance or renewal of a bound certificate, retries a failed push,
records every outcome, and lets the person push or test a target on demand.
`deploy.validate` optionally checks a configuration before it is stored.

## Motivation

The host writes certificates to local files and synchronizes them to its
cluster nodes. Everything else that terminates TLS (a CDN in front of the
site, a load balancer, a mail server) has to be updated by hand or by a
script after every renewal, which is the most common reason a renewed
certificate never reaches production. Every target has its own API and
often a large SDK, so the core cannot ship them; the issuance flow already
publishes `cert.issued` for plugins, so the core only needs a model of
targets, a trigger and a record of outcomes, with the pushing left to
plugins.

## Design

### Manifest

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
            { "key": "zone_id", "display_name": "Zone ID", "required": true }
          ]
        }
      }
    ]
  }
}
```

The form reuses `ConfigurationSchema` from RFC 0002, and kind codes follow
the same rules as channel codes, in a namespace of their own (NAME-10).

### Methods

| Method | Params | Result |
| --- | --- | --- |
| `deploy.push` | `{ kind, config, certificate: { name, domains, certificate_pem, private_key_pem, chain_pem, not_after }, dry_run }` | `{ message }` |
| `deploy.validate` | `{ kind, config }` | `{}`, or `-32003` with `data.field` |

The leaf and the chain travel apart, since targets disagree on whether they
want the full chain or the leaf alone; concatenating is easy, splitting is
not. `not_after` is an RFC 3339 string. A push must be idempotent, and a dry
run must change nothing while checking what a real push needs.

### Permission

Every other capability receives only what a person typed into its form.
`deploy.push` carries private keys, so the capability requires a permission
of its own, `cert.deploy` ("Receive certificates and their private keys in
order to push them to external targets"). Upgrading to a version that adds
it needs a new approval (SEC-9), and until then the host offers none of the
plugin's kinds and pushes nothing.

### Reference host

* `internal/cert/deploy` holds a target kind registry (`Source`,
  `RegisterSource`, `Kinds`), reads a certificate for a push from the files
  nginx serves (leaf, chain and key, read afresh for every attempt) and runs
  the pushes (`Runner`).
* `internal/plugin/capability.RegisterDeploy` registers the plugin manager as
  a source. A kind is stored as `plugin:<code>`, for the reason RFC 0002
  gives, and the owning plugin is resolved at every call, starting an
  `on_demand` plugin. Only plugins with the `cert.deploy` permission granted
  are asked.
* Two models: `CertDeployTarget { name, kind, config, cert_id, enabled }`,
  where `config` is encrypted at rest and `cert_id` 0 binds every
  certificate, and `CertDeployment { target_id, cert_id, status, message,
  attempts, created_at }` with `status` `ok` or `failed`. Migration
  `20260923000003` creates both tables. The last 20 outcomes of each target
  and certificate pair are kept.
* The runner subscribes to `cert.issued` and `cert.renewed` and pushes to
  every enabled target bound to the certificate. A failed push is retried
  after 30 seconds, 2 minutes and 10 minutes (the schedule is injectable), so
  a run makes at most four attempts and records one outcome with the attempt
  count. A trigger for a pair that is still running makes it run once more
  when it finishes, so a renewal during a retry is never lost.
* `GET/POST/DELETE /api/cert_deploy_targets` (CRUD, changes need an
  interactive administrator with a secure session and are closed in demo
  mode), `GET /api/cert_deploy_targets/kinds`,
  `GET /api/cert_deploy_targets/:id/deployments`,
  `POST /api/cert_deploy_targets/:id/deploy` (every bound certificate, one
  attempt each), `POST /api/cert_deploy_targets/test` (a dry run against a
  certificate), `POST /api/certs/:id/deploy` (every enabled target of the
  certificate) and `GET /api/certs/:id/deploy_targets` (the targets of a
  certificate with their last outcome). Creating or changing a target checks
  the required fields and calls `deploy.validate`; only `-32003` blocks the
  save, reported as error 55204 ("deploy target config field {0} is invalid:
  {1}"). A kind whose plugin is gone fails with 55203 ("deploy target kind
  {0} is not available").
* The certificate page shows a "Deploy targets" section with the bound
  targets, their last outcome and a "Deploy now" button, and a "Deploy
  Targets" page under Certificates manages the targets with a kind selector,
  `PluginConfigForm`, the certificate binding, a dry run and the history.
* `deploy.push` has 5 minutes, `deploy.validate` 30 seconds.

## Compatibility

A new optional capability, block, service and permission; nothing existing
changes meaning (VER-1). The two new tables do not touch existing ones.

## Alternatives considered

* **Letting deploy plugins subscribe to `cert.issued` and read the key
  through `host.credentials.get`.** Every plugin would reimplement binding,
  retrying and recording, and a key would be readable at any time instead of
  only when a person bound the certificate to a target.
* **A generic credential vault kind for target credentials.** Useful later
  for sharing one account between targets; the target form keeps this change
  independent of it.
* **Pushing the full chain as one PEM.** Rejected, see Methods.

## Security considerations

A `cert.deploy` plugin receives private keys. The permission names that
explicitly, the host sends a key only to targets a person bound to the
certificate, and only while the permission is granted. Target values are
credentials; the reference host encrypts them at rest, masks `secret`
fields, leaves them out of the MCP view and never logs them or the keys.
Every push is recorded, so a push nobody expected is visible. Creating or
changing a target, pushing and testing need an interactive administrator
with a secure session and stay unavailable in demo mode.

## Future work

* Binding a target to a set of certificates, or to certificates by domain.
* Removing a certificate from a target when its binding is deleted.
* Target credentials from a shared vault instead of each target's form.
