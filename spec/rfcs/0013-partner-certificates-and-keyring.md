# RFC 0013: Partner certificates and the partner keyring

| | |
| --- | --- |
| Status | Accepted |
| Date | 2026-09-26 |
| Spec changes | The intro of `spec/02-packaging.md`, PKG-19, PKG-21, PKG-22 and PKG-23 (amended), PKG-25 through PKG-27, the intro of Package trust, SEC-18 (rewritten), SEC-24 (amended), SEC-25 through SEC-29, CONF-1, the lint table of `spec/09-conformance.md`, `schema/partners.schema.json`, the layout table of `README.md`, a note in RFC 0012 |
| Supersedes | The partner keys pinned in the host binary of RFC 0012 |
| Reference host | nginx-ui `internal/releasesign` (release keys only, the pinned partner keys removed), `internal/plugin` (certificate check, keyring refresh and cache, lint) |

## Summary

A partner key no longer lives in the host binary. It reaches a host in two
ways, both signed with a release key of the Nginx UI project. A partner
package carries a certificate: `plugin.partner`, the public key of the
partner, and `plugin.partner.minisig`, a release key signature of it whose
trusted comment reads `partner:<name>`, optionally followed by
`;expires:<YYYY-MM-DD>`. The project also
publishes a signed partner keyring, `v1/partners.json` with
`v1/partners.json.minisig`, next to the official catalog; it lists partner
keys and revoked key ids. A package signed by a partner key is `verified`
when a certificate that passes every check, or a keyring entry that has not
expired, vouches for the key and the keyring does not revoke it. A
certificate that fails a check is ignored and never makes a package
invalid. Adding or removing a partner needs no release of the host.

## Motivation

RFC 0012 gave `verified` to the partner keys pinned in the host binary.
That has two costs.

1. **A new partner needs a host release.** Its packages are `verified` only
   on hosts that upgraded to a release that pins its key. A partner that
   joins between two releases waits for the next one, and an operator who
   stays on an older release never sees its packages as `verified`.
2. **Revocation needs a host release too.** RFC 0012 lists this under its
   security considerations and future work. A compromised partner key
   stays `verified` on every host that has not upgraded, which is exactly
   the set of hosts an operator is least likely to watch.

The project owner decided that a new partner must not require a host
release. Partner trust therefore has to come from data the release key
signs, not from code the release ships.

## Design

### Partner certificate

The two files sit at the package root next to the signature files, and
`plugin.sums` lists them like any other file (PKG-19):

```text
com.example.plugin/
├── CHANGELOG.md
├── LICENSE
├── README.md
├── plugin.json
├── plugin.partner
├── plugin.partner.minisig
├── plugin.sums
├── plugin.sums.minisig
└── server/
    └── plugin-linux-amd64
```

`plugin.partner` is the `.pub` file minisign writes for the partner key
(PKG-25), or its key line alone:

```text
untrusted comment: minisign public key 0123456789ABCDEF
RW...
```

The maintainers issue a certificate by signing that file with a release
key, putting the partner name and, optionally but as SEC-29 recommends, the
last valid day into the trusted comment (PKG-26):

```sh
minisign -S -s release.key -m plugin.partner \
  -t "partner:example-corp;expires:2027-09-30"
```

minisign signs the trusted comment together with the signature, so the name
and the date cannot be changed without breaking it. A certificate is a
signed public key and nothing more: no new file format, no new parser and no
key type the host does not already verify.

A host checks a certificate whenever it derives the level of a package
(PKG-22), in this order (PKG-27): `plugin.partner` parses, a release key
verifies `plugin.partner.minisig`, the trusted comment has the fixed form,
the date, when there is one, has not passed, and the keyring does not revoke
the key. A
certificate that passes makes the partner key known for PKG-21, and the
package is `verified` when that key verifies `plugin.sums.minisig` and
`plugin.sums` matches the files.

A certificate that fails is ignored. The package then derives its level from
its other sources, a keyring entry, its catalog entry or the operator's
trusted key list, and is never invalid because of the certificate. A
certificate only adds trust, so its failure costs exactly the trust it
would have added. Making it fatal would turn every expired certificate into
a refused install of a package that is otherwise fine, and would let a
broken certificate hide which of the other sources applies.

A partner puts the two files into the package root before it writes
`plugin.sums` and signs with its own key (PKG-23). One certificate serves
every package the partner signs with that key.

### Partner keyring

The project publishes the keyring next to the official catalog. For the
reference host:

```text
https://plugins.nginxui.com/v1/index.json
https://plugins.nginxui.com/v1/partners.json
https://plugins.nginxui.com/v1/partners.json.minisig
```

```json
{
  "schema_version": 1,
  "updated_at": "2026-09-26T08:00:00Z",
  "partners": [
    {
      "name": "example-corp",
      "public_key": "untrusted comment: minisign public key 0123456789ABCDEF\nRW...",
      "expires": "2027-09-30"
    }
  ],
  "revoked": ["FEDCBA9876543210"]
}
```

`partners.json.minisig` is a detached minisign signature of the exact bytes
of the document by a release key (SEC-25), and
`schema/partners.schema.json` describes the document (SEC-26). `expires` is
optional; the other members are required.

A host fetches the keyring with every catalog refresh and takes a fetched
document only when a release key verifies it, its layout is one the host
knows and its `updated_at` is not earlier than that of the keyring it cached
(SEC-27). It stores the keyring it took on disk with its signature and loads
it when it starts, so a restart or a node without network keeps it. When a
fetch or a check fails it keeps the cached keyring. Only the keyring next to
the official catalog counts: a custom catalog source cannot publish
partners.

A listed key whose entry has not expired makes a package it signs
`verified` without a certificate. This covers packages built before the
partner received its certificate and hosts that fetched the keyring before
they saw the package.

### Trust derivation

| Signer | Level |
| --- | --- |
| A release key of the Nginx UI project, pinned in the host binary | `official` |
| A partner key that a passing certificate in the package, or an unexpired keyring entry, vouches for, and whose key id is not revoked | `verified` |
| The `author_public_key` of the catalog entry the package was downloaded from, or a key on the operator's trusted key list | `community` |
| No signature, or an unknown signer | `unsigned` |

The rows of `official`, `community` and `unsigned` are those of RFC 0012.
Only the `verified` row changes: the release key still decides who is a
partner, but it does so through signed data instead of a list compiled into
the host.

### Revocation and expiry

A key id in `revoked` never gives `verified`, whether a certificate or an
entry of `partners` names it (SEC-28). Revocation withdraws only the partner
level: the key still gives `community` where a catalog entry or the operator
names it, and release keys are outside its reach. A revoked key does not
change the level recorded for an installed plugin (SEC-24), but no update it
signs is `verified` any more, so automatic updates signed with it stop as
downgrades (SEC-22).

Expiry is a date of the UTC calendar, and a certificate or keyring entry is
valid through the whole of that day by the host's own clock (SEC-29). A date
is easy to issue and to read in a trusted comment, and the slack of up to a
day absorbs time zones and small clock errors, which matter more here than
a precise instant.

### Onboarding and offboarding

1. The partner creates a minisign key pair and sends its public key to the
   maintainers.
2. The maintainers sign the public key with a release key, with the partner
   name and, as a rule, an expiry date in the trusted comment, and send back
   `plugin.partner` and `plugin.partner.minisig`.
3. The maintainers add the key to `partners` and publish the keyring with a
   later `updated_at`.
4. The partner ships the certificate in every package it signs, and gets a
   new one before the old one expires, if it carries a date.

To offboard a partner or withdraw a compromised key, the maintainers add its
key id to `revoked`, drop its entry from `partners` and publish the keyring.
Every host that refreshes stops giving the key `verified`, with or without a
certificate. A host that does not refresh keeps honoring the certificate
until its expiry date, or for good when it has none, so the lifetime of a
certificate bounds the damage there. A revoked id stays in the list; a partner that signs again gets a new
key.

### Tooling

`nginx-ui plugin lint` warns when only one of the certificate files is
present, when the certificate does not verify with the release keys pinned
in its binary or breaks the trusted comment form, when it has expired, and
when it names another key than the one that signed `plugin.sums.minisig`
(the lint table of `spec/09-conformance.md`). The linter does not consult
the keyring. A host shows the partner name next to the `verified` level
(SEC-18).

## Compatibility

* The plugin system has not shipped in an nginx-ui release (RFC 0001 and
  RFC 0012 made the same observation), and the reference host's pinned
  partner set was empty, so no package and no host depends on a pinned
  partner key. The change is made within spec 1.0 (VER-1, VER-2).
* A package without the certificate files derives its level exactly as
  before. A host that implements RFC 0012 but not this RFC sees the two
  files as ordinary files listed in `plugin.sums` and derives `community` or
  `unsigned` for a partner package, never a level it should not.
* The catalog does not change. The keyring is a new document next to it,
  and a host that does not know it never fetches it.

## What a partner must adopt

1. Create a minisign key pair and send the public key to the maintainers.
2. Copy `plugin.partner` and `plugin.partner.minisig` unchanged into the
   package root of every package, then write `plugin.sums` and sign it with
   the partner key (PKG-23).
3. Replace the certificate before it expires and sign the packages that
   ship the new one again.
4. Report a lost or compromised key to the maintainers at once, and sign
   with a new key and a new certificate afterwards.

## Alternatives considered

* **Partner keys pinned in the host binary, the design of RFC 0012.** Every
  new partner and every revocation is a host release, and both reach only
  the hosts that upgrade. The host binary becomes the registry of partners.
* **The keyring alone.** Revocation is immediate, but a host has to reach
  the official catalog before it knows any partner. An offline install, a
  node without network, a first start before the first refresh and a host
  whose operator removed the official catalog would never see a partner
  package as `verified`, and the level of a package would depend on whether
  the last fetch succeeded.
* **The certificate alone.** It works everywhere, offline included, but a
  key could only be withdrawn by waiting for its certificates to expire.
  Short lifetimes shrink that window at the price of reissuing certificates
  and rebuilding every partner package often, since the certificate is a
  file `plugin.sums` lists.
* **Both, the chosen design.** The certificate carries partner trust to
  every host, offline ones included, and needs no host release. The keyring
  revokes a key at once wherever it reaches and can vouch for a key without
  a certificate. The expiry of a certificate bounds the damage where the
  keyring does not reach.
* **X.509 or another certificate format.** It needs a new parser, new key
  types and a chain model the project does not need. A minisign signature
  over a minisign public key uses only what every host already verifies,
  and its trusted comment is authenticated.
* **Partner keys in catalog entries**, signed by the project. Trust would
  hang on the catalog again, which RFC 0012 moved away from, and a package
  from an upload, the offline package directory or a cluster push would have
  no entry to take the key from.
* **A keyring next to every catalog source.** A custom source could then
  name partners. Restricting the keyring to the official catalog, and to
  documents a release key signed, keeps `verified` a statement of the
  project.

## Security considerations

* The release key stays the root of trust for both `official` and
  `verified`. A stolen release key could already sign `official` packages,
  so issuing certificates gives it no new power.
* A certificate vouches for one key. Copying it into another package gains
  nothing unless that package is signed with the partner's secret key, and
  replacing or removing it after signing breaks `plugin.sums`.
* Rollback: a host refuses a keyring older than the one it cached, so a
  mirror, a proxy or an attacker on the network cannot bring back a key the
  project revoked. A host that has never fetched a keyring has no reference
  and can be served an older, validly signed one on its first fetch; the
  expiry of certificates bounds that case.
* Freeze: an attacker who blocks the fetch keeps a host on its cached
  keyring. The host reports failed refreshes, a certificate still expires on
  its date, and a keyring entry expires on its `expires`. An entry without
  `expires` stays good on such a host for as long as the cached keyring
  does, so the project sets `expires` on entries it wants bounded.
* Expiry is optional on a certificate as well: `partner:<name>` alone is a
  certificate that never expires and ends only by revocation. It spares a
  partner the rebuild and new signature a renewed certificate needs
  (PKG-23), but revocation reaches only hosts that refresh the keyring, so a
  node that never refreshes would trust a leaked key without a date for
  good. The project therefore SHOULD set a date on every certificate it
  issues (SEC-29); a long lifetime, such as several years, keeps renewals
  rare and still bounds that case.
* Clock: a host clock set back accepts an expired certificate, one set ahead
  refuses a valid one. Either only moves the partner level: the package
  keeps its catalog and operator sources and is never made invalid.
* Revocation matches key ids. A key id collision can only take trust away,
  never grant it, since a host still verifies every signature with the key
  itself.
* Revocation does not reach release keys. Replacing a compromised release
  key still needs a host release.

## Future work

* Distributing the keyring from a cluster controller to nodes that cannot
  reach the official catalog.
* Deriving the level of installed plugins again when their signer is
  revoked, and offering to replace them.
* Maintainer tooling that issues certificates and publishes the keyring
  from CI with the release key.
