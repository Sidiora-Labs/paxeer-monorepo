# Hyperfide workspace

Hyperfide is a programmable workspace for coding agents. The browser, desktop,
mobile client and CLI connect to a server that coordinates threads and state.
Host daemons run providers and provision workspaces on enrolled machines.

Plugins contribute providers, tools, settings and UI surfaces. Provider bridges
translate agent output into a shared thread vocabulary; declarative presentation
keeps timeline items readable across clients. Environment providers supply local
and managed workspace flows.

Operators choose the server, enrolled hosts and credentials. A project source
belongs to its execution host; connecting a client does not expose that client's
filesystem to another machine.

See [system overview](system-overview.md), [platform support](platform-support.md)
and [provider plugin API](provider-plugin-api.md).
