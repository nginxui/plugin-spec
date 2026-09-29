# 07. Webapp

A plugin may contribute to the host's browser UI: either a JavaScript bundle
that registers routes, slot components and translations, or a set of
zero-build static HTML pages. This is entirely separate from the `server`
process (`spec/04-lifecycle.md`); a plugin MAY declare `webapp` with or
without `server`.

## Bundle contract

## WEB-1

A webapp bundle MUST be built as an **IIFE** file: the entry named by
`webapp.bundle_path` is a single file with no code splitting, and any further
code the bundle needs later comes as separate IIFE files declared in
`webapp.chunks` (WEB-13), never as ES module chunks or dynamic imports of the
entry. It MUST NOT call `createApp` and MUST NOT bundle its own copy of
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
host UI. A chunk (WEB-13) has no stylesheet reference of its own: styles its
components need either belong to the same `style.css`, or the chunk injects
them itself as a `<style>` element when it runs, under the same scoping rule.

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
    /** Host dialogs a bundle may open, see below. Absent on older hosts. */
    ui?: {
      openDnsCredentialEditor?: () => Promise<DnsCredentialSummary | undefined>
    }
  }
  registerPlugin: (id: string, definition: { setup(registry: PluginRegistry): void | Promise<void>, teardown?(): void }) => void
  /** Called by a chunk file while its script executes, see WEB-13. Absent on hosts without chunk support. */
  registerChunk?: (pluginId: string, name: string, exports: Record<string, unknown>) => void
}
```

`shared.versions` covers exactly the packages listed in WEB-2's table
(`vue`, `vue-router`, `pinia`, `antdv-next`, `@vueuse/core`); `shared.gettext`
and `shared.http` are the host's own translation and HTTP client instances,
not their npm packages' generic module exports, and carry no entry in
`versions`.

`shared.ui` exposes host dialogs so a bundle can reuse the host's own forms
instead of rebuilding them. `openDnsCredentialEditor()` opens the host's DNS
credential editor and resolves with the created credential (`id`, `name`,
`code`, optional `provider` and `provider_code`), or `undefined` when the
person cancels. A bundle MUST feature-detect every `shared.ui` member and
keep working without it.

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
| `registerSlot(slot, component, opts?)` | function | Mounts `component` into a host-defined extension point. `opts.order` (ascending) and `opts.when(ctx)` (return `false` to skip) control multiple registrations in the same slot. `opts.label`, `opts.sortValue` and `opts.filters` are read only by the slots WEB-14 names and ignored by every other slot. |
| `registerTranslations(locale, messages)` | function | Merges `{ "<English source string>": "<translation>" }` into the host's translations for `locale`. |
| `registerSettingsPanel(component)` | function | Replaces the schema-driven settings form (MAN-27..30) with a custom component receiving `{ settings, save }`. |
| `http` | object | Axios-like client, `baseURL` = `./api/plugins/{id}/http`, proxied to the plugin's `http` capability (`spec/06-host-api.md` covers `server`/wire methods; this is the browser-side counterpart). Resolves with a full response object. |
| `loadChunk(name)` | function | Loads the on-demand chunk `name` declared in `webapp.chunks` (WEB-13) and returns a Promise of the exports object the chunk handed to `registerChunk`. |
| `coreHttp` | object | The host's own REST API client. Usable only when the manifest requests the `core_api` permission (`spec/08-security.md`); resolves with the response body directly. |
| `manifest` | object | This plugin's manifest, as the host parsed it. |
| `host` | object | Read-only `{ theme: 'light'\|'dark', locale, nodeId, username }`. |

`registerSlot` options:

| Option | Type | Used by | Meaning |
| --- | --- | --- | --- |
| `order` | number | every slot | Lower values render first. |
| `when` | `(ctx) => boolean` | every slot | Return `false` to skip the registration for that context. |
| `label` | string | `nginx_log.view:{key}`, `nginx_log.list.column:{key}` | Display text, an English source string the host translates with its gettext, the plugin supplying translations through `registerTranslations`. |
| `sortValue` | `(row) => string \| number \| null \| undefined` | `nginx_log.list.column:{key}` | Makes the column sortable by the returned value. |
| `filters` | `{ label: string, value: string, match: (row) => boolean }[]` | `nginx_log.list.column:{key}` | Makes the column filterable, see WEB-14. |

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
| `nginx_log.view:{key}` | `{ path, type }` | An extra view mode of the nginx log page for one log file. `path` is the log file path and `type` is `access` or `error`. `key` names the mode (WEB-14). |
| `nginx_log.list.toolbar` | `{ type }` | The actions area above the nginx log list. `type` is `access` or `error`, the list being shown. |
| `nginx_log.list.column:{key}` | `{ row }` | One extra column of the nginx log list. `row` is the list row of the log file (WEB-14). |
| `nginx_log.list.row.actions` | `{ row }` | Per-row actions of the nginx log list. |
| `site.log.actions` | `{ accessLogPath, errorLogPath, siteName }` | Log related actions of one site, in the site editor and in the site list. A path is an empty string when the site has no such log. |

A host MAY add slots beyond this table in a future spec 1.x revision without
a version bump (WIRE-7-style forward compatibility): a bundle MUST treat an
unknown slot name it is not registered for as simply never rendered, and a
host MUST treat a `registerSlot` call for a slot name it does not define as
a harmless no-op rather than an error.

## WEB-13

`webapp.chunks` (MAN-41) lets a bundle keep its entry small and load heavy
code only when a page needs it. Each chunk is its own IIFE file, built like
the entry (WEB-1 through WEB-3): it externalizes the shared runtime and does
not bundle a second copy of it. A chunk hands its exports to the host with
one synchronous call while its `<script>` executes:

```js
window.NginxUI.registerChunk('com.example.plugin', 'dashboard', { Dashboard, formatBytes })
```

`pluginId` MUST equal the plugin's manifest `id`, `name` MUST be the chunk's
key in `webapp.chunks`, and `exports` MUST be an object. The entry asks for a
chunk with `registry.loadChunk(name)`, which returns a Promise that resolves
with that exports object:

```ts
const { Dashboard } = await registry.loadChunk('dashboard') as { Dashboard: Component }
```

A host:

* MUST load only chunks the calling plugin itself declared in
  `webapp.chunks`, only from that plugin's own package, resolved from the
  declared path, and MUST reject `loadChunk` (the Promise rejects with an
  error) for a name that is not declared, without making any request;
* MUST load a chunk at most once per page: concurrent and later calls for the
  same name share one Promise, and every one of them resolves with the same
  exports object. A chunk that failed to load MAY be tried again by a later
  call;
* MUST NOT load a chunk before `loadChunk` asks for it, and MUST NOT run two
  chunk or entry scripts of one page at the same time, because they hand
  their result over on the same global, which the host reads right after each
  script finished;
* MUST reject `loadChunk` when the file fails to load or the script did not
  call `registerChunk` with the plugin's own id and the requested name, and
  MUST NOT let that failure affect the entry, other chunks or other plugins;
* SHOULD apply the version of the plugin to the file address the way it does
  for the entry, so that an upgrade is not served from cache.

A host without chunk support has no `window.NginxUI.registerChunk` and no
`registry.loadChunk`; a bundle that needs chunks feature-detects
`registry.loadChunk`.

A chunk runs in the same page and against the same shared runtime as the
entry. A route or slot component MAY therefore be an async component that
resolves through `registry.loadChunk(name)`, so the chunk is fetched when the
route or slot is first rendered.

## WEB-14

The slots of the nginx log page and of the site log actions behave as follows.

`nginx_log.view:{key}`: the log page offers a mode switch made of the built-in
raw view and one entry per registration of this slot, whose text is the
registration's `opts.label` (the key when there is none). The selected mode
is kept in the `view` query parameter of the page: the raw view is the
absence of the parameter or `view=raw`, and a registered mode is
`view={key}`. A link with a `view` value nobody registered shows the raw
view. Only the component of the selected mode is mounted, with the context
`{ path, type }` of the file being viewed. `opts.when(ctx)` decides whether a
mode is offered for that file.

`nginx_log.list.column:{key}`: every registration adds one column after the
host's own columns, ordered by `opts.order`. The column title is `opts.label`.
The component renders the cell of the row it receives. The host loads the
whole list at once, so it sorts and filters plugin columns itself, in the
browser:

* with `opts.sortValue(row)`, the column header sorts the list by the returned
  value (numbers by magnitude, strings by locale order, `null` and
  `undefined` last in either direction). Without it the column cannot be
  sorted;
* with `opts.filters`, the header offers one choice per entry, showing its
  `label`, and keeps the rows for which `match(row)` of a selected entry
  returns true. Several selected entries of one column combine with OR, columns
  combine with AND. `value` is the stable identifier of the choice. Without
  `opts.filters` the column cannot be filtered.

`row` is the object the host shows in that list row. It always carries `path`
(the log file path), `type` (`access` or `error`), `name` and `config_file`;
a host MAY add fields, and a plugin MUST ignore the ones it does not know.

`nginx_log.list.toolbar`, `nginx_log.list.row.actions` and `site.log.actions`
render every registration in `opts.order`, the components being free to
render nothing. For `site.log.actions` the paths are those of the log
directives of the site's own configuration.

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
