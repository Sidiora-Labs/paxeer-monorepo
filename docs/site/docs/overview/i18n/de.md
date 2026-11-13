# Paxeer X Network

Paxeer X Network ist ein BFT-Netzwerk für autonome Agenten, Zahlungen und programmierbare Ausführung. LayerX ist in den Paxeer-Knoten `paxd` integriert.

[English](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/README.md) · [Español](es.md) · [日本語](ja.md) · [Русский](ru.md) · [简体中文](zh-cn.md) · [Português](pt-br.md) · Deutsch · [Français](fr.md)

*Weicht diese Fassung von der englischen README ab, gilt die englische Fassung als Referenz.*

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/LICENSE)
[![CI](https://github.com/Sidiora-Labs/paxeer-monorepo/actions/workflows/ci.yml/badge.svg)](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/.github/workflows/ci.yml)

## Was Paxeer X Network ist

Paxeer mainnet-beta verwendet die EVM-Chain-ID **125** (`0x7d`). Der Kernel ist Teil des Netzwerks und wirkt an der Blockerstellung mit. Agentenoperationen laufen als Transaktionen derselben Chain, unter anderem über EVM-Precompiles. Eine Quittung ist final, sobald ihr Block durch den BFT-Konsens festgeschrieben ist. Eine erfolgreiche Einreichung allein belegt keine erfolgreiche Ausführung.

Der native Kernel wird über `modules/lxphost/` in `paxd` eingebunden. Geldbewegungen werden über die Ledger-, Bank- und EVM-Zustandspfade der Chain angewendet. Konsenskritische native Ausführung verwendet kanonische Kodierung und geprüfte Ganzzahlarithmetik; Replay-Gates prüfen die Reproduzierbarkeit.

Sub-Chains sind Mandantendomänen: Namensräume in der Datenbank. Sie benötigen kein eigenes Validatornetzwerk und keine neue Knoten-Infrastruktur. Dieses Monorepo enthält die Komponenten eines gemeinsamen Netzwerks. Maßgeblich sind die Spezifikationen unter [`spec/layerx-merge/`](https://github.com/Sidiora-Labs/paxeer-monorepo/tree/main/spec/layerx-merge).

## Schnellstart

Öffentliches JSON-RPC wird über HTTPS am Wurzelpfad von `api1.mainnet-beta.paxeer.network` bis `api14.mainnet-beta.paxeer.network` sowie `api-mainnet-beta.paxeer.network` angeboten. Die folgenden Abfragen lesen nur Chain-ID und Blockhöhe. `PAXEER_RPC_URL` ist hier eine lokale Shell-Variable.

```sh
PAXEER_RPC_URL=https://api-mainnet-beta.paxeer.network
curl --fail --silent --show-error --max-time 30 "$PAXEER_RPC_URL" \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
curl --fail --silent --show-error --max-time 30 "$PAXEER_RPC_URL" \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":2,"method":"eth_blockNumber","params":[]}'
```

`eth_chainId` muss `"0x7d"` liefern. Mainnet-beta verwendet reale Vermögenswerte. Für eine schreibende Operation signiert die Anwendung eine Transaktion für Chain 125 und übermittelt sie mit `eth_sendRawTransaction`. Anschließend fragt sie `eth_getTransactionReceipt` mit dem Transaktionshash ab. `null` bestätigt keine Aufnahme in einen Block; bei einer vorhandenen Quittung sind `status`, `blockHash`, `blockNumber` und die Logs zu prüfen. Auch eine fehlgeschlagene Transaktion ist nach dem Block-Commit final.

Programs wird über das Precompile `0x000000000000000000000000000000000000101D` aufgerufen; die ABI steht in `precompiles/layerxprograms/abi.json`. Siehe [Programs](../../programs/index.md). Weitere Beispiele enthält der [Quickstart](../quickstart.md); für Endpunkte und Finalität gilt die [Netzwerkübersicht](../../index.md).

Die gehosteten `px_`-Methoden sind in `services/platform/hosted/gateway/openrpc.json` beschrieben. Sie sind keine Zusage, dass jeder öffentliche Knoten diese Methoden anbietet.

## Aus dem Quellcode bauen

Die native Runtime verwendet C17, die Rust-Workspaces den in `rust-toolchain.toml` festgelegten Compiler 1.91.1 und das Go-Modul Go 1.25.8. `make paxd-build` baut die nativen Bibliotheken und den Knoten mit der Release-Version. `make build` baut die native Runtime und Hilfswerkzeuge.

```sh
make paxd-build
make paxd-version-check
```

Native Tests, Node-Integration und Release-Matrizen belegen unterschiedliche Eigenschaften desselben Netzwerks. Die passenden Befehle und Voraussetzungen stehen unter [Qualifikation](../../operators/qualification.md). Ein Komponenten-Build belegt nicht den vollständigen Transaktionspfad.

## Repository-Aufbau

| Pfad | Zweck |
| --- | --- |
| `daemon/`, `node/`, `consensus/` | Knoten, Blockverarbeitung und BFT-Konsens |
| `src/`, `include/` | C17-Kernel und native Protokollmodule |
| `modules/lxphost/` | Einbettung des Kernels in den Knoten |
| `modules/`, `precompiles/` | Chain-Zustand und EVM-aufrufbare Einstiegspunkte |
| `rpc/` | JSON-RPC des Knotens |
| `programs/` | Programs-Runtime, Sandbox und SDK |
| `services/agent/` | Agenten-SDKs, Agent-Daemon und MCP-Server |
| `services/human/`, `apps/web/` | Menschliche Steuerung und Webanwendung |
| `services/platform/`, `tools/platform/cli/` | Gehostete Plattformdienste und Entwickler-CLI |
| `services/interop/`, `sdk/platform/` | Interoperabilität und Client-SDKs |
| `spec/` | Normative Spezifikationen und Task-Graphen |
| `docs/`, `tests/`, `tests/solidity/` | Dokumentation und Qualifikationssuiten |

## Dokumentation

- [Netzwerkübersicht](../../index.md)
- [Monorepo-Aufbau](../monorepo.md)
- [Qualifikation](../../operators/qualification.md)
- [Spezifikationen](https://github.com/Sidiora-Labs/paxeer-monorepo/tree/main/spec/layerx-merge)

## SDKs und Integrationen

| Sprache | Pfad |
| --- | --- |
| Rust | `services/agent/crates/layerx-sdk` |
| Python | `services/agent/sdk/python` |
| TypeScript | `services/agent/sdk/typescript` |
| Go | `sdk/platform/go` |
| JVM | `sdk/platform/jvm` |
| .NET | `sdk/platform/dotnet` |
| Swift | `sdk/platform/swift` |

`layerx-agentd` liegt in `services/agent/crates/layerx-agentd`, der MCP-Server in `services/agent/crates/layerx-mcp`. Beide sind Clients des Netzwerks. Die Entwickler-CLI liegt in `tools/platform/cli`.

## Mitwirken

Vor dem Öffnen eines Pull Requests [`CONTRIBUTING.md`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/CONTRIBUTING.md) lesen. Protokolländerungen beginnen in `spec/`. Eine vermutete Schwachstelle nicht in einem öffentlichen Issue offenlegen; [`SECURITY.md`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/SECURITY.md) folgen.

## Sicherheit

Schwachstellen über GitHub Private Reporting melden, wie in [`SECURITY.md`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/SECURITY.md) beschrieben.

## Lizenz

Lizenziert unter der Apache License, Version 2.0. Siehe [`LICENSE`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/LICENSE) und [`NOTICE`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/NOTICE).

Paxeer X Network wird von Sidiora Labs entwickelt.
