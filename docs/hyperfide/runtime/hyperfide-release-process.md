# Hyperfide application packages

Hyperfide's source workspace is `tools/hyperfide`. It pins Node 22.19.0 in
`.nvmrc` and pnpm 9.15.0 in `package.json`.

The workspace `build` script builds the scripts, web app, server, host daemon,
CLI and launcher packages. The launcher package, `hyperfide-app`, contains the
web assets and server and daemon runtime bundles.

The desktop application lives in
[`apps/desktop`](../../../tools/hyperfide/apps/desktop). It packages an Electron
shell for macOS, Linux and Windows. The mobile client lives in
[`apps/mobile`](../../../tools/hyperfide/apps/mobile) and uses Expo for iOS and
Android.

See [platform support](platform-support.md) for host requirements and
[bundled plugins](official-plugin-release-process.md) for plugin artifacts.
