# Tools ops contract

Build, deployment and operational utilities are implemented in the linked scripts, Makefiles and tool modules. Each command uses the flags and configuration declared by its own parser.

The tables identify code declarations and configuration names. Activation, authentication and transport registration are defined by the linked implementations.

<a id="bringup"></a>

<a id="ci-scripts"></a>

<a id="ci-workflows"></a>

<a id="container-images"></a>

<a id="foundry-profile"></a>

<a id="makefile-graph"></a>

## paxeer-x-gates

| Kind | Declaration | Source |
|---|---|---|
| `config_key` | `SERVICE_CATALOGUE = 'spec/layerx-merge/spec.kvx'` | [tools/paxeer-x/candidate.py](../../../tools/paxeer-x/candidate.py) |
| source | — | [tools/paxeer-x/gates/registry.kvx](../../../tools/paxeer-x/gates/registry.kvx) |

<a id="qualification-harness"></a>

<a id="replay-gates"></a>

## wave-gates

| Kind | Declaration | Source |
|---|---|---|
| `cli_command` | `scripts/gates/wave-5.sh [--list]` | [scripts/gates/wave-5.sh](../../../scripts/gates/wave-5.sh) |
| `cli_command` | `scripts/gates/wave-programme.sh [--list]` | [scripts/gates/wave-programme.sh](../../../scripts/gates/wave-programme.sh) |
| source | — | [scripts/gates/run_gate.py](../../../scripts/gates/run_gate.py) |
| source | — | [scripts/gates/wave-0.sh](../../../scripts/gates/wave-0.sh) |
| source | — | [scripts/gates/wave-1.sh](../../../scripts/gates/wave-1.sh) |
| source | — | [scripts/gates/wave-2.sh](../../../scripts/gates/wave-2.sh) |
| source | — | [scripts/gates/wave-3.sh](../../../scripts/gates/wave-3.sh) |
| source | — | [scripts/gates/wave-4.sh](../../../scripts/gates/wave-4.sh) |
| source | — | [scripts/gates/wave-6.sh](../../../scripts/gates/wave-6.sh) |

<a id="wiring-integration"></a>

See the implementation sources above.
