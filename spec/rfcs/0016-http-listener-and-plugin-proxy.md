# RFC 0016: The plugin HTTP listener and the proxy environment

| | |
| --- | --- |
| Status | Accepted |
| Date | 2026-09-30 |
| Spec changes | LIFE-14, LIFE-17 and LIFE-18 (including `NGINX_UI_PLUGIN_HTTP_SECRET`) in `spec/04-lifecycle.md`, MAN-22, CONF-1 |

## Summary

The lifecycle chapter says where a plugin that serves the `http` capability
listens, and when it has to be ready. A host hands its outbound proxy to the
plugins that may use the network, and to no other.

## Motivation

`http_port` existed in the `plugin.initialize` result, but nothing said what
it was for, and the Unix socket the host proxies to had no written contract at
all: its name, its permissions and the moment it has to accept connections were
known to the reference host and the plugins written against it. Without the
text an SDK cannot offer the capability as a first class feature, and a plugin
author on Windows has no way to learn that the reply has to carry the port.

A plugin process only sees its environment, so it never learned about the
proxy the person configured for the host. Its downloads either failed on a
network that needs a proxy, or the author added a setting that duplicates one
the person already made. On the other hand a plugin without `network` has no
use for the address, which can embed credentials.

## Design

* **One place per platform.** `<data dir>/http.sock` on Unix, a free port on
  `127.0.0.1` reported as `http_port` elsewhere. There is no `http_socket`
  member: the path is fixed, and a data directory whose path does not fit
  `sun_path` fails the handshake with a clear error instead of moving the
  socket where the host does not look.
* **Ready before the reply.** The listener accepts connections before
  `plugin.initialize` is answered, the same rule as gRPC (WIRE-11), so the
  first proxied request never races the start.
* **A per process secret on every request.** Any local process can connect to
  a loopback port, and with the identity headers it could pose as any person
  and read what the plugin serves. The host therefore generates a random
  secret for every process start, passes it in `NGINX_UI_PLUGIN_HTTP_SECRET`
  and sends it in `X-Nginx-UI-Plugin-Secret` on every proxied request, on the
  socket as well, so the plugin has one rule for both. The plugin answers
  `401` to anything without it, compares in constant time, and never logs or
  forwards it. The host strips a client supplied copy first. A plugin that
  finds no secret refuses to listen, since the host always sets it.
* **Proxy by permission.** The host sets `HTTP_PROXY`, `HTTPS_PROXY` and
  `NO_PROXY` in both spellings for a plugin that holds `network`, from its own
  proxy configuration, and for no other plugin, not even by inheritance.
  `NO_PROXY` always lists the loopback addresses.

## Compatibility

Nothing changes on the wire (VER-1): the secret travels in an environment
variable and an HTTP header, and `http_port` already existed. A plugin that
already listens on the socket keeps working with the reference host, which only
adds a header, but it does not satisfy LIFE-18 and stays open to local
processes on Windows until it checks the secret, which the SDK does. A host
without proxy settings changes nothing. A plugin without `network` that relied
on a proxy variable inherited from the host process loses it, which is the
point.
