# Paxeer X Network

Paxeer X Network est un réseau BFT pour les agents autonomes, les paiements et l’exécution programmable. LayerX est intégré au nœud Paxeer, `paxd`.

[English](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/README.md) · [Español](es.md) · [日本語](ja.md) · [Русский](ru.md) · [简体中文](zh-cn.md) · [Português](pt-br.md) · [Deutsch](de.md) · Français

*En cas de divergence, le README anglais est la version de référence.*

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/LICENSE)
[![CI](https://github.com/Sidiora-Labs/paxeer-monorepo/actions/workflows/ci.yml/badge.svg)](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/.github/workflows/ci.yml)

## Ce qu’est Paxeer X Network

Paxeer mainnet-beta utilise l’identifiant de chaîne EVM **125** (`0x7d`). Le noyau fait partie du réseau et participe à la proposition de blocs. Les opérations des agents sont des transactions de cette même chaîne, notamment des appels aux précompilés EVM. Un reçu est final dès que son bloc est validé par le consensus BFT. L’acceptation d’un envoi ne prouve pas la réussite de son exécution.

Le noyau natif est intégré à `paxd` par `modules/lxphost/`. Les mouvements monétaires passent par les chemins du registre comptable, de la banque et de l’état EVM de la chaîne. L’exécution native critique pour le consensus utilise un encodage canonique et une arithmétique entière vérifiée ; les suites de rejeu vérifient la reproductibilité.

Les sous-chaînes sont des domaines locataires : des espaces de noms dans la base de données. Elles ne nécessitent ni réseau de validateurs ni infrastructure de nœuds supplémentaires. Ce monorepo contient les composants d’un seul réseau. Les spécifications en vigueur se trouvent dans [`spec/layerx-merge/`](https://github.com/Sidiora-Labs/paxeer-monorepo/tree/main/spec/layerx-merge).

## Essayer la connexion

Le JSON-RPC public est servi en HTTPS à la racine de `api1.mainnet-beta.paxeer.network` à `api14.mainnet-beta.paxeer.network`, ainsi que de `api-mainnet-beta.paxeer.network`. Ces requêtes lisent uniquement l’identifiant de chaîne et la hauteur de bloc. `PAXEER_RPC_URL` est ici une variable shell locale.

```sh
PAXEER_RPC_URL=https://api-mainnet-beta.paxeer.network
curl --fail --silent --show-error --max-time 30 "$PAXEER_RPC_URL" \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
curl --fail --silent --show-error --max-time 30 "$PAXEER_RPC_URL" \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":2,"method":"eth_blockNumber","params":[]}'
```

`eth_chainId` doit renvoyer `"0x7d"`. Mainnet-beta utilise des actifs réels. Pour modifier l’état, l’application signe une transaction pour la chaîne 125 et l’envoie avec `eth_sendRawTransaction`. Elle consulte ensuite `eth_getTransactionReceipt` avec le hash de la transaction. `null` ne confirme pas l’inclusion ; si un reçu existe, vérifier `status`, `blockHash`, `blockNumber` et les logs. Une transaction en échec est également finale après la validation de son bloc.

Programs s’appelle via le précompilé `0x000000000000000000000000000000000000101D` ; son ABI est dans `precompiles/layerxprograms/abi.json`. Voir [Programs](../../programs/index.md). Le [guide de démarrage](../quickstart.md) donne d’autres exemples ; la [présentation du réseau](../../index.md) décrit les points d’accès et la finalité.

Les méthodes hébergées `px_` sont déclarées dans `services/platform/hosted/gateway/openrpc.json`. Cela ne garantit pas leur disponibilité sur chaque nœud public.

## Compiler depuis les sources

Le runtime natif utilise C17, les espaces de travail Rust utilisent le compilateur 1.91.1 fixé dans `rust-toolchain.toml` et le module Go déclare Go 1.25.8. `make paxd-build` compile les bibliothèques natives et le nœud avec sa version de publication. `make build` compile le runtime natif et ses outils auxiliaires.

```sh
make paxd-build
make paxd-version-check
```

Les tests natifs, l’intégration du nœud et les matrices de publication établissent des propriétés différentes du même réseau. Les commandes et prérequis figurent dans [Qualification](../../operators/qualification.md). La compilation d’un composant ne démontre pas le chemin complet d’une transaction.

## Organisation du dépôt

| Chemin | Objet |
| --- | --- |
| `daemon/`, `node/`, `consensus/` | Nœud, traitement des blocs et consensus BFT |
| `src/`, `include/` | Noyau C17 et modules de protocole natifs |
| `modules/lxphost/` | Intégration du noyau dans le nœud |
| `modules/`, `precompiles/` | État de la chaîne et points d’entrée appelables depuis EVM |
| `rpc/` | JSON-RPC du nœud |
| `programs/` | Runtime Programs, sandbox et SDK |
| `services/agent/` | SDK d’agents, démon d’agent et serveur MCP |
| `services/human/`, `apps/web/` | Contrôle humain et application web |
| `services/platform/`, `tools/platform/cli/` | Services hébergés de plateforme et CLI développeur |
| `services/interop/`, `sdk/platform/` | Interopérabilité et SDK clients |
| `spec/` | Spécifications normatives et graphes de tâches |
| `docs/`, `tests/`, `tests/solidity/` | Documentation et suites de qualification |

## Documentation

- [Présentation du réseau](../../index.md)
- [Organisation du monorepo](../monorepo.md)
- [Qualification](../../operators/qualification.md)
- [Spécifications](https://github.com/Sidiora-Labs/paxeer-monorepo/tree/main/spec/layerx-merge)

## SDK et intégrations

| Langage | Chemin |
| --- | --- |
| Rust | `services/agent/crates/layerx-sdk` |
| Python | `services/agent/sdk/python` |
| TypeScript | `services/agent/sdk/typescript` |
| Go | `sdk/platform/go` |
| JVM | `sdk/platform/jvm` |
| .NET | `sdk/platform/dotnet` |
| Swift | `sdk/platform/swift` |

`layerx-agentd` se trouve dans `services/agent/crates/layerx-agentd` et le serveur MCP dans `services/agent/crates/layerx-mcp`. Ce sont des clients du réseau. Le CLI développeur se trouve dans `tools/platform/cli`.

## Contribuer

Lire [`CONTRIBUTING.md`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/CONTRIBUTING.md) avant d'ouvrir une pull request. Les changements de protocole commencent dans `spec/`. Ne pas divulguer une vulnérabilité suspectée dans une issue publique ; suivre [`SECURITY.md`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/SECURITY.md).

## Sécurité

Signaler les vulnérabilités via le reporting privé GitHub, comme décrit dans [`SECURITY.md`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/SECURITY.md).

## Licence

Distribué sous la licence Apache, version 2.0. Voir [`LICENSE`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/LICENSE) et [`NOTICE`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/NOTICE).

Paxeer X Network est développé par Sidiora Labs.
