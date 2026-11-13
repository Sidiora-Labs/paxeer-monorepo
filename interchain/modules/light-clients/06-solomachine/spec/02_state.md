<!--
order: 2
-->

# State

The solo-machine client stores its latest `ClientState`, including the embedded
`ConsensusState` with the current public key, diversifier and timestamp.
Successful state-proof verification increments its sequence and updates that
embedded timestamp.

A header update returns the replacement client and consensus states; the core
client keeper stores the separate consensus state at the header height. A
governance substitute update copies the substitute's sequence and embedded
consensus state into the subject client state and clears its freeze flag; it
does not add a separate historical consensus-state entry.

See [Concepts](01_concepts.md) and [State Transitions](03_state_transitions.md),
[the client implementation](../types/client_state.go), and
[the core keeper](../../../core/02-client/keeper/client.go).

