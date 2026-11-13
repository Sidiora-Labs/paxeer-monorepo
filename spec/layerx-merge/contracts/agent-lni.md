# Agent LNI contract

<a id="c-abi"></a>
<a id="message-1"></a>
<a id="message-10"></a>
<a id="message-11"></a>
<a id="message-12"></a>
<a id="message-13"></a>
<a id="message-14"></a>
<a id="message-15"></a>
<a id="message-16"></a>
<a id="message-17"></a>
<a id="message-18"></a>
<a id="message-19"></a>
<a id="message-2"></a>
<a id="message-20"></a>
<a id="message-21"></a>
<a id="message-22"></a>
<a id="message-23"></a>
<a id="message-24"></a>
<a id="message-25"></a>
<a id="message-26"></a>
<a id="message-27"></a>
<a id="message-28"></a>
<a id="message-29"></a>
<a id="message-3"></a>
<a id="message-30"></a>
<a id="message-31"></a>
<a id="message-32"></a>
<a id="message-33"></a>
<a id="message-34"></a>
<a id="message-35"></a>
<a id="message-36"></a>
<a id="message-37"></a>
<a id="message-38"></a>
<a id="message-39"></a>
<a id="message-4"></a>
<a id="message-40"></a>
<a id="message-41"></a>
<a id="message-42"></a>
<a id="message-43"></a>
<a id="message-44"></a>
<a id="message-45"></a>
<a id="message-5"></a>
<a id="message-6"></a>
<a id="message-7"></a>
<a id="message-8"></a>
<a id="message-9"></a>

## Native transport

There is no LNI transport. Agent clients use the node's root JSON-RPC endpoint
for chain reads, signed EIP-1559 transactions and receipt evidence.
[`layerx-client`](../../../services/agent/crates/layerx-client/src/lib.rs) exposes
`sequencer`, `rpc`, `px` and the generic bounded `frame` codec.

The [client contract](clients.md) describes submission and verification.
Local Human peers use the [authenticated peer protocol](agent-human-peer.md).
