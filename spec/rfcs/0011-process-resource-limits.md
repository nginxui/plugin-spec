# RFC 0011: Resource limits of plugin processes

| | |
| --- | --- |
| Status | Accepted, implemented |
| Date | 2026-09-23 |
| Spec changes | LIFE-16, MAN-39 (`server.resources`), SEC-1, SEC-17, NAME-12, CONF-1, CONF-4, `ManifestResources` in `manifest.proto`, `schema/plugin.schema.json` |
| Reference host | nginx-ui `settings/plugin.go` (`MemoryLimitMB`, `CPUPercent`, `CgroupRoot`), `internal/plugin/resources.go`, `internal/plugin/cgroup.go`, `internal/plugin/cgroup_linux.go`, `internal/plugin/cgroup_other.go`, `app/src/views/system/plugins/PluginDrawer.vue` |

## Summary

A host may confine the memory and CPU time of plugin processes. A manifest
may declare hints, `server.resources: { memory_mb, cpu_percent }`, which
can only lower the limits the host is configured with. The reference host
enforces the limits with cgroup v2 on Linux and runs processes without
limits everywhere else, reporting whether the limits are enforced.

## Motivation

A plugin process runs with the privileges of the host (SEC-1). A leak or a
busy loop in a plugin, or a hostile plugin, can take the memory or the CPU
nginx-ui and nginx need. Confinement does not stop a plugin from reading
what it may read, but it keeps a misbehaving one from taking the node down.

## Design

* **Limits.** The host settings `MemoryLimitMB` and `CPUPercent` apply to
  every plugin, `0` meaning unlimited. A hint of the manifest applies when
  it is smaller, and alone when the setting is unlimited: the plugin author
  knows the budget of the process best, and a smaller limit only protects
  the host.
* **cgroup v2 layout.** `<CgroupRoot>/nginx-ui/plugins/<plugin id>`, with
  `CgroupRoot` defaulting to `/sys/fs/cgroup`. The host enables the `memory`
  and `cpu` controllers in `cgroup.subtree_control` of the root and of the
  two intermediate groups, writes `memory.max` and `memory.swap.max` `0`,
  `cpu.max` as `<cpu_percent * 1000> 100000`, and starts the process
  directly inside the group through `clone3` (`SysProcAttr.UseCgroupFD`),
  so it never runs outside. A kernel without `CLONE_INTO_CGROUP` gets the
  process moved through `cgroup.procs` right after the start. After the
  process exited the host kills what is left in the group and removes it.
  A flat layout under a directory of its own was chosen over a
  `nginx-ui.slice` name so the host does not look like a systemd unit it is
  not.
* **Not enforceable.** No cgroup v2 (`cgroup.controllers` missing), another
  operating system, no write access to the hierarchy (a process without
  privilege, a container without delegation): the host logs once at warning
  level (or at debug level for another operating system and cgroup v1) and
  runs the process without limits.
* **Reporting.** The plugin info carries `resources: { memory_limit_mb,
  cpu_percent, enforced }`, shown in the overview of the plugin drawer.

## Compatibility

A new optional manifest field; hosts that do not confine processes ignore
it (VER-1). An OOM kill is a crash under LIFE-12, which plugins already had
to survive.

## Alternatives considered

* **setrlimit.** `RLIMIT_AS` counts address space, not memory, and breaks
  Go and JVM runtimes that reserve large ranges; there is no CPU rate limit.
* **A limit per plugin in the UI.** More knobs than the problem needs; the
  hint covers the plugin that needs less, the host setting the rest.
* **cgroup v1.** Deprecated, and every distribution this project targets
  runs v2 by default.

## Future work

* Applying changed limits to running processes without a restart.
* An I/O weight.
