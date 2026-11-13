# Bundled plugin artifacts

Bundled runtime plugins live under
[`tools/hyperfide/plugins`](../../../tools/hyperfide/plugins). The workspace package
[`tools/bundled-plugins`](../../../tools/hyperfide/tools/bundled-plugins) collects
their build outputs for the server. The server registry determines each plugin's
identity and automatic installation policy.

Plugin host artifacts are content-addressed. The server records their digest;
the host daemon downloads and verifies the bytes before caching and running
them. Provider bridges use this same artifact path.

The installed inventory is available with:

```sh
hyperfide platform plugin list
```

See [provider distribution](provider-plugin-api.md#6-distribution) and
[forkable plugins](forkable-plugins.md).
