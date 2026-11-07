<h1 align="center">Paxeer X Network — LayerX</h1>

Paxeer X Network 将支付、程序执行和自主智能体操作整合到同一网络。

[English](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/README.md) · [Español](es.md) · [日本語](ja.md) · [Русский](ru.md) · 简体中文 · [Português](pt-br.md) · [Deutsch](de.md) · [Français](fr.md)

## LayerX Network 是什么

LayerX 是集成在 Paxeer X Network 中的内核。`paxd` 节点运行一条 BFT 链，EVM chain ID 为 `125`（`0x7d`）。内核是节点的一部分，参与执行交易和提出区块提案。改变状态的操作是同一网络上的交易；交易所在的区块经共识提交后，其回执即具有最终性。

`modules/lxphost` 将 C17 内核和 Rust 程序运行时嵌入 Go 节点。`node/app.go` 注册原生模块，并验证区块提案中的交易顺序。链的持久化状态记录余额、权限、支付和程序。子链是租户域，即数据库中的命名空间，创建时无需部署一套新网络基础设施。

Sidiora Labs 的本 monorepo 包含节点、内核、合约、服务、应用和 SDK。集成规范见 [`spec/layerx-merge/spec.kvx`](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/spec/layerx-merge/spec.kvx)。

### 模块与预编译合约

EVM 合约通过预编译合约访问原生模块。`precompiles/setup.go` 中的地址表声明了以下地址；启用时间取决于 `v7.0` 升级计划及节点配置的升级高度。下表使用省略前导零的 20 字节 EVM 地址简写。

| 预编译合约 | 地址 |
| --- | --- |
| `layerxledger` | `0x101A` |
| `layerxpay` | `0x101B` |
| `layerxauth` | `0x101C` |
| `layerxprograms` | `0x101D` |
| `layerxchains` | `0x101E` |

目录还包含地址 `0x0901` 至 `0x0908` 的智能体函数。准备交易时，应使用对应预编译合约的 ABI，并核对目标网络的升级配置。程序部署和调用由原生程序模块处理，参见[程序](../../programs/index.md)。

## 连接网络

公共 JSON-RPC 在节点 `https://api1.mainnet-beta.paxeer.network` 至 `https://api14.mainnet-beta.paxeer.network` 的根路径提供服务，也可使用别名 `https://api-mainnet-beta.paxeer.network`。参见[公共端点](../../reference/public-rpc.md)。`node.hyperpaxeer.com` 仅是构建所用的 hpx 库镜像（mirror），不是发送交易的端点。

使用 `curl` 在签名前核对 chain ID，并查询节点区块高度。第一次调用应返回 `result: "0x7d"`，第二次返回十六进制高度。

```sh
RPC_URL=https://api1.mainnet-beta.paxeer.network
curl --fail --silent --show-error --max-time 30 "$RPC_URL" \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
curl --fail --silent --show-error --max-time 30 "$RPC_URL" \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":2,"method":"eth_blockNumber","params":[]}'
```

如需改变状态，请用钱包签署 chain ID 为 `125` 的 EVM 交易，再通过 `eth_sendRawTransaction` 提交已签名的交易字节。返回的哈希用于标识交易，本身不代表区块已经提交。查询已有交易时，将 `TX_HASH` 设为其完整的 32 字节十六进制哈希，然后使用 `jq` 和 `curl`：

```sh
jq -n --arg hash "${TX_HASH:?}" \
  '{jsonrpc:"2.0",id:3,method:"eth_getTransactionReceipt",params:[$hash]}' |
  curl --fail --silent --show-error --max-time 30 "$RPC_URL" \
    --header 'Content-Type: application/json' --data-binary @-
```

`result: null` 表示该节点暂时无法提供此哈希的回执。取得回执后，核对 `transactionHash`、`blockHash`、`blockNumber` 和 `status`：`0x1` 表示执行成功，`0x0` 表示执行失败。最终性来自 BFT 共识提交区块。若提交请求超时，应先查询原交易哈希，再决定是否重试。

请配置多个节点并比较 `eth_blockNumber`，因为节点高度可能不同。公共命名空间为 `debug`、`echo`、`eth`、`net`、`pax`、`px`、`test` 和 `txpool`，具体可用性取决于节点配置。`px` 命名空间由节点根据节点状态应答；`px_register` 和 `px_requestFunds` 属于托管服务，契约位于 `services/platform/hosted/gateway/openrpc.json`；参见[网关 JSON-RPC](../../platform/gateway-rpc.md)。

## 从源码构建

节点使用 Go 1.25.8（`go.mod`），内核使用 C17（`Makefile`），Rust 工作区使用 1.91.1（`rust-toolchain.toml`）。根目录的 Foundry 配置固定使用 Solidity 0.8.27（`foundry.toml`）。安装构建依赖后，在仓库根目录执行：

```sh
make paxd-build
```

此目标构建 `build/liblayerx.a`、release 模式的程序运行时及 `build/paxd`，并写入 `tools/release/version.json` 中的版本。`make build` 构建内核库和工具；节点构建目标是 `make paxd-build`。`make paxeer-build` 调用 `chain.mk`，也会准备原生库。节点目标使用 `GOPROXY=off`，因此 Go 依赖必须已在本地可用。

服务各有构建目标：`make agent-build`、`make human-build`、`make interop-build`、`make platform-build-all` 和 `make programs-build`。请运行所修改组件的检查，参见[验收门禁](../../operators/qualification.md)。

## 仓库布局

| 路径 | 用途 |
| --- | --- |
| `daemon/paxd/`, `node/`, `consensus/` | 节点可执行文件、应用和 BFT 共识 |
| `src/`, `include/`, `modules/lxphost/` | C17 内核及节点内的宿主集成 |
| `modules/`, `precompiles/`, `rpc/` | 原生模块、EVM 访问和 JSON-RPC 服务器 |
| `services/agent/` | SDK、智能体守护进程和 MCP 服务器 |
| `services/human/`, `apps/web/` | 人工控制面和 Web 应用 |
| `services/platform/`, `tools/platform/` | 托管服务、中间件和开发工具 |
| `programs/` | 程序运行时和编译工具 |
| `services/interop/` | 智能体商务和跨网络互操作 |
| `contracts/` | Solidity 合约与接口，包括 bridge 和 launchpad |
| `sdk/`, `apps/` | 客户端 SDK 和应用 |
| `spec/`, `tests/`, `migrations/` | 规范、测试和迁移 |
| `docs/` | Wiki、文档站点和验收说明 |

## 文档

- [站点首页](../../index.md)
- [公共 JSON-RPC 端点](../../reference/public-rpc.md)
- [Monorepo 布局](../monorepo.md)
- [验收门禁](../../operators/qualification.md)
- [规范](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/spec/)

## SDK 与集成

| 语言 | 路径 |
| --- | --- |
| Rust | `services/agent/crates/layerx-sdk` |
| Python | `services/agent/sdk/python` |
| TypeScript | `services/agent/sdk/typescript` |
| Go | `sdk/platform/go` |
| JVM | `sdk/platform/jvm` |
| .NET | `sdk/platform/dotnet` |
| Swift | `sdk/platform/swift` |

`layerx-agentd` 位于 `services/agent/crates/layerx-agentd`，是节点外的智能体服务。MCP 服务器位于 `services/agent/crates/layerx-mcp`。`tools/platform/cli` 中的开发者 CLI 提供 `layerx install mcp` 和 `layerx install a2a`；安装需要配置好 beta 或生产网关环境及相应凭证。

## 贡献

提交 pull request 前请阅读 [`CONTRIBUTING.md`](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/CONTRIBUTING.md)。协议变更从 `spec/` 开始。不要在公开 issue 中披露疑似漏洞；请遵循 [`SECURITY.md`](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/SECURITY.md)。

## 安全

通过 GitHub 私下报告渠道提交漏洞，具体见 [`SECURITY.md`](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/SECURITY.md)。

## 许可证

依 Apache 许可证 2.0 版授权。参见 [`LICENSE`](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/LICENSE) 和 [`NOTICE`](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/NOTICE)。

Paxeer X Network 由 Sidiora Labs 开发。
