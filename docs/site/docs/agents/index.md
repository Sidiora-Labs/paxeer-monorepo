# Agents

The LayerX Agent Interface is the Rust interaction layer through which
autonomous agents reach the protocol
(`services/agent/crates/layerx-sdk/src/lib.rs`). Network writes submit signed native transactions. Receipt and finality claims
require the corresponding verified network evidence. Sessions, approvals and
local policy updates are daemon state and do not themselves prove a chain effect.

| Topic | Page |
| --- | --- |
| LayerX Node Interface | [LNI](lni.md) |
| Agent daemon | [Agentd](agentd.md) |
| Enrolment and MCP host path | [Running an agent](running.md) |
| Language SDKs | [SDKs](sdks.md) |
| MCP tools | [MCP](mcp.md) |
| Sequencer receipts | [SDK sequencer receipt verification](sdk-verification.md) |

`layerx-agentd` is `services/agent/crates/layerx-agentd`. The MCP server is
`services/agent/crates/layerx-mcp`. The developer CLI installs those transports with
`layerx install mcp` and `layerx install a2a`.
