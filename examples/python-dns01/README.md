# MyDNS Challenge (Example)

A reference `dns01` plugin for this spec, written in about 60 lines of
Python 3 standard library and nothing else. It targets a fictional "MyDNS"
HTTP API and exists to demonstrate the minimum viable implementation of:

* the lifecycle methods (`plugin.initialize`, `plugin.configure`,
  `plugin.ping`, `plugin.shutdown`, `plugin.exit`) — see
  [`spec/04-lifecycle.md`](../../spec/04-lifecycle.md);
* the two required `dns01` methods (`dns01.present`, `dns01.cleanup`) — see
  [`spec/05-capabilities-dns01.md`](../../spec/05-capabilities-dns01.md);
* the JSON-RPC 2.0 NDJSON framing and error codes — see
  [`spec/03-wire-protocol.md`](../../spec/03-wire-protocol.md).

It does not actually call any MyDNS API (there is no such vendor); `present`
and `cleanup` only log what they would have done, to stderr. It is meant to
be read end to end and used as a starting point for a real DNS-01 plugin in
any language, or as a conformance fixture for a host implementation.

## What it does not implement

`dns01.validate`, `dns01.options` and `dns01.check` are optional (DNS01-\*)
and this example does not implement them: a host asking for one gets
`-32601` (method not found), which for a JSON-RPC peer that registers no
handler at all is indistinguishable from a deliberate `-32002`
(unsupported) as far as the caller needs to react — a host MUST treat either
the same way, by falling back to its own defaults. A production plugin
SHOULD reply `-32002` explicitly for a capability method it recognizes but
does not implement, so its own logs distinguish "this method does not exist
here" from "the plugin will never do this."

Concurrency is simplified too: this example reads one line, replies, then
reads the next line, which satisfies the wire protocol (WIRE-4 makes
concurrent handling optional, not required) but means a slow `present` call
would delay a `plugin.ping` sent right after it. A real plugin handling
slow vendor APIs SHOULD process requests concurrently (e.g. one thread per
request) so liveness pings are never blocked behind a capability call.

## Running the tests

```bash
python3 test_main.py
```

`test_main.py` spawns `server/main.py` as a subprocess, feeds it NDJSON
requests on stdin exactly as a host would, and asserts the replies for
`plugin.initialize`, `plugin.ping`, an unknown method (expecting `-32601`),
and `plugin.exit`. It needs nothing beyond the Python 3 standard library.

## License

AGPL-3.0. See [LICENSE](LICENSE).
