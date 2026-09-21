# 07. Webapp

A plugin may contribute to the host's browser UI: either a JavaScript bundle
that registers routes, slot components and translations, or a set of
zero-build static HTML pages. This is entirely separate from the `server`
process (`spec/04-lifecycle.md`); a plugin MAY declare `webapp` with or
without `server`.

## Bundle contract

## WEB-1

A webapp bundle MUST be built as a single **IIFE** file with no code
splitting. It MUST NOT call `createApp` and MUST NOT bundle its own copy of
Vue, Vue Router, Pinia or the host's UI component library — reactivity only
works within one Vue instance, and the host already provides one.

## WEB-2

A bundle MUST externalize `vue`, `vue-router`, `pinia`, `antdv-next` and
`@antdv-next/icons` and `@vueuse/core` onto the host's shared runtime instead
of bundling them. In an IIFE built with Rollup/Vite, this means passing them
as `external` and mapping each to `window.NginxUI.shared.<key>` via
`output.globals`:

| Package | `window.NginxUI.shared` key |
| --- | --- |
| `vue` | `vue` |
| `vue-router` | `vueRouter` |
| `pinia` | `pinia` |
| `antdv-next` | `antdvNext` |
| `@antdv-next/icons` | `antdvIcons` |
| `@vueuse/core` | `vueuse` |

## WEB-3

Styles MUST be shipped as one separate `style.css`, referenced by
`webapp.style_path` (MAN-15). A bundle MUST write plain CSS (the host's own
utility CSS generator is not part of the shared runtime), MUST use the
host's `--ant-*` custom properties for colors so dark mode follows the host
theme, and MUST scope its selectors — either with a CSS scoping mechanism
(e.g. Vue SFC `scoped` styles) or by prefixing class names with the plugin
id — rather than writing global selectors that could affect the rest of the
host UI.

## WEB-4

`window.NginxUI` MUST be populated by the host before any plugin bundle
executes:

```ts
interface NginxUIGlobal {
  version: string
  shared: {
    vue: unknown; vueRouter: unknown; pinia: unknown
    antdvNext: unknown; antdvIcons: unknown; vueuse: unknown
    gettext: unknown; http: unknown
    /** Resolved versions of the checked shared libraries, keyed by package name. */
    versions: Record<string, string>
  }
  registerPlugin: (id: string, definition: { setup(registry: PluginRegistry): void | Promise<void>, teardown?(): void }) => void
}
```

`shared.versions` covers exactly the packages listed in WEB-2's table
(`vue`, `vue-router`, `pinia`, `antdv-next`, `@vueuse/core`); `shared.gettext`
and `shared.http` are the host's own translation and HTTP client instances,
not their npm packages' generic module exports, and carry no entry in
`versions`.

## WEB-5

A bundle MUST call `window.NginxUI.registerPlugin(id, definition)`
synchronously while its `<script>` executes, with `id` exactly equal to its
own manifest `id`. A host MUST treat a bundle that never calls it (or calls
it with a different id) as failed to load, and MUST NOT let that failure
prevent other plugins' bundles from loading.

## WEB-6: shared runtime compatibility

`webapp.shared` (MAN-17) declares, per shared library, the semver range the
bundle was built against, e.g. `{ "vue": ">=3.5.42 <4", "antdv-next": "~1.5" }`.
Before injecting a bundle, a host MUST check every declared range against
`window.NginxUI.shared.versions` using standard semver range semantics
(`>=`, `>`, `<=`, `<`, `=`, `^`, `~`, space-separated AND, `||` OR;
prerelease/build metadata ignored). A host MUST skip (not crash on) a bundle
that:

* declares a library not present in `versions` at all, or
* declares a range the current version does not satisfy,

and MUST log a warning identifying the plugin and the failing library rather
than failing silently. The rest of the host, and every other plugin's
bundle, MUST keep working.

## Registry API

`setup(registry)` receives:

## WEB-7

| Member | Type | Meaning |
| --- | --- | --- |
| `registerRoute(route, opts?)` | function | Adds a child route under the host's main authenticated layout. `opts.parent` nests it under an existing top level sidebar entry; `opts.order` sorts it within that group. |
| `registerSlot(slot, component, opts?)` | function | Mounts `component` into a host-defined extension point. `opts.order` (ascending) and `opts.when(ctx)` (return `false` to skip) control multiple registrations in the same slot. |
| `registerTranslations(locale, messages)` | function | Merges `{ "<English source string>": "<translation>" }` into the host's translations for `locale`. |
| `registerSettingsPanel(component)` | function | Replaces the schema-driven settings form (MAN-27..30) with a custom component receiving `{ settings, save }`. |
| `http` | object | Axios-like client, `baseURL` = `./api/plugins/{id}/http`, proxied to the plugin's `http` capability (`spec/06-host-api.md` covers `server`/wire methods; this is the browser-side counterpart). Resolves with a full response object. |
| `coreHttp` | object | The host's own REST API client. Usable only when the manifest requests the `core_api` permission (`spec/08-security.md`); resolves with the response body directly. |
| `manifest` | object | This plugin's manifest, as the host parsed it. |
| `host` | object | Read-only `{ theme: 'light'\|'dark', locale, nodeId, username }`. |

## WEB-8

A component mounted into a slot receives the slot's context both spread as
individual props and as a single `context` prop holding the same object, so
either `defineProps<{ options }>()` or `defineProps<{ context }>()` (reading
`context.options`) works. A host MUST wrap each mounted slot component in its
own error boundary: a component that throws MUST degrade to an inline error
message without breaking the rest of the page or any other plugin's
component in the same slot.

## WEB-9

Slots the host defines, and the context object passed to each:

| Slot | Context | Meaning |
| --- | --- | --- |
| `certificate.challenge.form:{method}` | `{ options }` | Configuration UI for one ACME challenge method (`method` = e.g. `dns01`). `options` is the reactive certificate options object; a component reads/writes `options.challenge_config` (an opaque, plugin-owned object later handed unchanged to `dns01.present`'s `options` field, DNS01-12) alongside whatever legacy top-level fields it needs for backward compatibility. |
| `dns.credential.form:{provider_code}` | `{ credential, provider }` | Extra fields on the DNS credential form for one provider. |
| `dns.credential.hint:{provider_code}` | `{ credential, provider }` | Content shown above a provider's credential fields. |
| `certificate.issue.footer` | `{ options }` | Bottom of the certificate issue form, any challenge method. |
| `plugin.settings:{plugin_id}` | `{ settings }` | Inside another plugin's settings drawer (rare; most plugins use `registerSettingsPanel` for their own). |
| `sidebar.footer` | `{}` | Bottom of the sidebar. |

A host MAY add slots beyond this table in a future spec 1.x revision without
a version bump (WIRE-7-style forward compatibility): a bundle MUST treat an
unknown slot name it is not registered for as simply never rendered, and a
host MUST treat a `registerSlot` call for a slot name it does not define as
a harmless no-op rather than an error.

## Zero-build pages

## WEB-10

Instead of, or alongside, a bundle, `webapp.pages` (MAN-16) declares static
HTML pages. A host MUST register one route per page at
`plugins/{id}/pages/{path}` and MUST render it inside a same-origin iframe
pointed at the page's `file`, resolved relative to the plugin package root.

## WEB-11

A page communicates with the host over `postMessage`, targeted at
`location.origin`:

```js
parent.postMessage({ type: 'nginx-ui:token' }, location.origin)
// host replies: { type: 'nginx-ui:token', token: '<current auth token>' }

parent.postMessage({ type: 'nginx-ui:theme' }, location.origin)
// host replies: { type: 'nginx-ui:theme', theme: 'light' | 'dark' }
```

A host MUST answer only messages whose `event.source` is that specific
iframe's `contentWindow`, and MUST push an unsolicited
`{ type: 'nginx-ui:theme', theme }` message to the iframe whenever the host's
own theme changes, without the page having to ask again. A page MUST NOT
assume the token or the theme arrive synchronously or exactly once.

## Development

## WEB-12

A host SHOULD offer a way to load one additional, unpackaged plugin by URL
(pointing at a `plugin.json` served by a local dev server) for iterative
development, resolving `bundle_path`/`style_path`/`pages[].file` relative to
that URL rather than to an installed package directory. This is a
development convenience, not part of the installed-plugin contract, and MUST
NOT be reachable without the same authentication the rest of the host UI
requires.
