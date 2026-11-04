<h1 align="center">Paxeer X Network</h1>

Paxeer X Network は、自律エージェント、決済、プログラム実行を支える単一の BFT ネットワークです。LayerX は Paxeer ノード `paxd` に統合されています。

[English](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/README.md) · [Español](es.md) · 日本語 · [Русский](ru.md) · [简体中文](zh-cn.md) · [Português](pt-br.md) · [Deutsch](de.md) · [Français](fr.md)

*内容が異なる場合は、英語版 README を参照版とします。*

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/LICENSE)
[![CI](https://github.com/Sidiora-Labs/Paxeer-X-Network/actions/workflows/ci.yml/badge.svg)](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/.github/workflows/ci.yml)

## Paxeer X Network とは

Paxeer mainnet-beta の EVM チェーン ID は **125**（`0x7d`）です。カーネルはネットワークの一部としてブロックの提案に参加します。エージェントの操作は、EVM プリコンパイルへの呼び出しを含め、このチェーン上のトランザクションとして実行されます。レシートは、そのブロックが BFT コンセンサスによってコミットされた時点で確定します。送信が受理されただけでは、実行の成功を証明できません。

ネイティブカーネルは `modules/lxphost/` を通じて `paxd` に組み込まれています。資金の移動は、チェーンの台帳、bank、EVM 状態の処理経路に反映されます。コンセンサスに関わるネイティブ実行では正規エンコーディングと検査付き整数演算を使い、リプレイの検証で再現性を確認します。

サブチェーンはデータベース内の名前空間であるテナントドメインです。独自のバリデーターネットワークや新しいノード基盤は必要ありません。このモノレポには単一ネットワークの構成要素が収められています。現在の仕様は [`spec/layerx-merge/`](https://github.com/Sidiora-Labs/Paxeer-X-Network/tree/main/spec/layerx-merge) にあります。

## 接続を試す

公開 JSON-RPC は、`api1.mainnet-beta.paxeer.network` から `api14.mainnet-beta.paxeer.network`、および `api-mainnet-beta.paxeer.network` のルートで HTTPS により提供されます。次のクエリはチェーン ID とブロック高を読み取るだけです。`PAXEER_RPC_URL` は、この例で使うローカルのシェル変数です。

```sh
PAXEER_RPC_URL=https://api-mainnet-beta.paxeer.network
curl --fail --silent --show-error --max-time 30 "$PAXEER_RPC_URL" \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
curl --fail --silent --show-error --max-time 30 "$PAXEER_RPC_URL" \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":2,"method":"eth_blockNumber","params":[]}'
```

`eth_chainId` の結果は `"0x7d"` です。mainnet-beta は実資産を扱います。状態を変更するには、アプリケーションがチェーン 125 用のトランザクションに署名し、`eth_sendRawTransaction` で送信します。その後、トランザクションハッシュを指定して `eth_getTransactionReceipt` を照会します。`null` はブロックへの収録を確認するものではありません。レシートが得られたら `status`、`blockHash`、`blockNumber`、ログを確認してください。実行が失敗したトランザクションも、ブロックのコミット後は確定しています。

Programs はプリコンパイル `0x000000000000000000000000000000000000101D` を通じて呼び出します。ABI は `precompiles/layerxprograms/abi.json` にあります。[Programs](../../programs/index.md) を参照してください。[クイックスタート](../quickstart.md) に追加の例があり、接続先とファイナリティは[ネットワーク概要](../../index.md)で説明しています。

ホスト型の `px_` メソッドは `services/platform/hosted/gateway/openrpc.json` に定義されています。すべての公開ノードがこれらのメソッドを提供するという意味ではありません。

## ソースからビルドする

ネイティブランタイムは C17、Rust ワークスペースは `rust-toolchain.toml` で固定されたコンパイラ 1.91.1 を使い、Go モジュールは Go 1.25.8 を指定しています。`make paxd-build` はネイティブライブラリとリリースバージョン付きノードをビルドします。`make build` はネイティブランタイムと補助ツールをビルドします。

```sh
make paxd-build
make paxd-version-check
```

ネイティブテスト、ノード統合テスト、リリース検証は、同じネットワークの異なる性質を検証します。コマンドと前提条件は[適格性検証](../../operators/qualification.md)を参照してください。単一コンポーネントのビルドでは、トランザクションの処理経路全体を検証できません。

## リポジトリ構成

| パス | 用途 |
| --- | --- |
| `daemon/`, `node/`, `consensus/` | ノード、ブロック処理、BFT コンセンサス |
| `src/`, `include/` | C17 カーネルとネイティブプロトコルモジュール |
| `modules/lxphost/` | ノードへのカーネル組み込み |
| `modules/`, `precompiles/` | チェーン状態と EVM から呼び出せる処理 |
| `rpc/` | ノードの JSON-RPC |
| `programs/` | Programs ランタイム、サンドボックス、SDK |
| `services/agent/` | エージェント SDK、エージェントデーモン、MCP サーバー |
| `services/human/`, `apps/web/` | 人間向け制御と Web アプリケーション |
| `services/platform/`, `tools/platform/cli/` | ホスト型プラットフォームサービスと開発者 CLI |
| `services/interop/`, `sdk/platform/` | 相互運用とクライアント SDK |
| `spec/` | 規範的仕様とタスクグラフ |
| `docs/`, `tests/`, `tests/solidity/` | ドキュメントと適格性検証スイート |

## ドキュメント

- [ネットワーク概要](../../index.md)
- [モノレポ構成](../monorepo.md)
- [適格性検証](../../operators/qualification.md)
- [仕様](https://github.com/Sidiora-Labs/Paxeer-X-Network/tree/main/spec/layerx-merge)

## SDK と統合

| 言語 | パス |
| --- | --- |
| Rust | `services/agent/crates/layerx-sdk` |
| Python | `services/agent/sdk/python` |
| TypeScript | `services/agent/sdk/typescript` |
| Go | `sdk/platform/go` |
| JVM | `sdk/platform/jvm` |
| .NET | `sdk/platform/dotnet` |
| Swift | `sdk/platform/swift` |

`layerx-agentd` は `services/agent/crates/layerx-agentd`、MCP サーバーは `services/agent/crates/layerx-mcp` にあります。どちらもネットワークのクライアントです。開発者 CLI は `tools/platform/cli` にあります。

## コントリビューション

プルリクエストを開く前に [`CONTRIBUTING.md`](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/CONTRIBUTING.md) を読んでください。プロトコル変更は `spec/` から始めます。疑わしい脆弱性を公開 Issue に記載しないでください。[`SECURITY.md`](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/SECURITY.md) に従ってください。

## セキュリティ

脆弱性は GitHub の非公開報告機能で報告してください。手順は [`SECURITY.md`](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/SECURITY.md) にあります。

## ライセンス

Apache License, Version 2.0 のもとでライセンスされます。[`LICENSE`](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/LICENSE) および [`NOTICE`](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/NOTICE) を参照してください。

Paxeer X Network は Sidiora Labs が開発しています。
