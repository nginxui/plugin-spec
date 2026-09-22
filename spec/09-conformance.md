# 09. Conformance

This spec defines three conformance levels. A plugin or a host declares
which level(s) it targets; a level is satisfied only when every requirement
it lists holds, not merely most of them. Levels are additive: `dns01` and
`webapp` both include everything `core` requires.

## CONF-1: level `core`

Every plugin, regardless of capability, MUST satisfy `core`. A host MUST
satisfy `core` to run any plugin at all.

| Section | Requirements |
| --- | --- |
| Manifest | MAN-1 through MAN-14, MAN-18 through MAN-30 (every manifest-level requirement except the `webapp`-specific MAN-15/16/17, which only apply to a plugin that declares `webapp`) |
| Packaging | PKG-1 through PKG-11 |
| Wire protocol | WIRE-1 through WIRE-10; WIRE-11 only for a plugin that lists `grpc` in `transports` |
| Lifecycle | LIFE-1 through LIFE-15 |
| Host API | HOST-1 through HOST-16, limited to the methods the plugin actually calls or subscribes to — a plugin that never calls `host.cron.register` is not tested against HOST-10, but MUST still handle `host.log`/`host.settings.get`/`host.i18n.locale` correctly if it uses them |
| Security | SEC-1 through SEC-12 |
| Versioning | VER-1 through VER-6 |
| Naming | NAME-1 through NAME-6 |

## CONF-2: level `dns01`

A plugin declaring `"dns01"` in `capabilities` MUST additionally satisfy:

| Section | Requirements |
| --- | --- |
| Manifest | MAN-19, MAN-21 (the `dns01` capability declaration rules) |
| Capability | DNS01-1 through DNS01-13 |

`dns01.validate`, `dns01.options` and `dns01.check` are individually
optional (a plugin MAY reply `-32002` to any of them), but `dns01.present`
and `dns01.cleanup` MUST both be implemented for a plugin to claim this
level at all.

## CONF-3: level `webapp`

A plugin declaring a `webapp` block MUST additionally satisfy:

| Section | Requirements |
| --- | --- |
| Manifest | MAN-15, MAN-16, MAN-17 (the `webapp` block rules) |
| Webapp | WEB-1 through WEB-12 |

A plugin whose `webapp` block declares only `pages` (no `bundle_path`) is
exempt from WEB-1 through WEB-8 (the bundle contract) but MUST still satisfy
WEB-10 and WEB-11 (the iframe page contract).

## CONF-4: host conformance

A host implementation claims a level the same way: `core` is mandatory, and
`dns01`/`webapp` apply only if the host intends to run plugins of that kind
at all. A host MAY legitimately support `core` and `dns01` but not `webapp`
(e.g. a headless installation with no browser UI) or vice versa; it MUST NOT
claim a level while silently skipping one of that level's MUST requirements.

## CONF-5: reporting

A conformance report (for a plugin, an SDK, or a host) SHOULD list the
specific requirement ids checked and their pass/fail outcome, not just a
level name, so a reader can see exactly what was and was not verified. The
`vectors/v1/` directory exists to make at least the wire-level requirements
(WIRE-\*, LIFE-\*, DNS01-\*, and the `host.*` methods in HOST-\*)
mechanically checkable without a live host or a live plugin on the other
end.

## CONF-6: partial implementations

A component that implements a strict subset of a level's MUST requirements
is not conformant to that level and MUST NOT claim to be. It MAY describe
itself as "implements a subset of spec 1.0 `core`" or similar, naming which
requirements it does not meet, rather than claiming a level it only mostly
satisfies.
