# Windows checks

Run the Node 24 and pnpm 9.15 checks from `tools/hyperfide`; see
[testing](testing.md). The monorepo `.github/workflows/platform.yml` builds the
network platform components.

Windows support requires running the relevant workspace checks on Windows,
including host process handling, filesystem paths and packaged desktop startup.
A Linux test pass does not establish Windows support. See
[platform support](platform-support.md) for capability boundaries.
