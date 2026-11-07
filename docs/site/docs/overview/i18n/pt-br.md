<h1 align="center">Paxeer X Network — LayerX</h1>

Paxeer X Network reúne pagamentos, execução de programas e operações de agentes autônomos em uma única rede.

[English](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/README.md) · [Español](es.md) · [日本語](ja.md) · [Русский](ru.md) · [简体中文](zh-cn.md) · Português · [Deutsch](de.md) · [Français](fr.md)

## O que é o LayerX Network

LayerX é o kernel integrado à Paxeer X Network. O nó `paxd` executa uma única cadeia BFT, com EVM chain ID `125` (`0x7d`). O kernel faz parte do nó e participa da execução e da proposta de blocos. As operações que alteram o estado são transações dessa mesma rede; o recibo é final quando seu bloco é confirmado pelo consenso.

O host `modules/lxphost` integra o kernel C17 e o runtime de programas Rust ao nó Go. `node/app.go` registra os módulos nativos e verifica a ordem das transações nas propostas. O estado persistido da cadeia sustenta os saldos, as permissões, os pagamentos e os programas. As subcadeias são domínios de locatários: namespaces no banco de dados, criados sem infraestrutura de uma nova rede.

Este monorepo da Sidiora Labs contém o nó, o kernel, os contratos, os serviços, os aplicativos e os SDKs. A referência da integração é [`spec/layerx-merge/spec.kvx`](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/spec/layerx-merge/spec.kvx).

### Módulos e precompiles

Os contratos EVM acessam os módulos nativos por precompiles. A tabela em `precompiles/setup.go` declara os seguintes endereços, cuja ativação depende do plano `v7.0` e da altura de upgrade configurada no nó. Os valores abaixo são endereços EVM de 20 bytes, abreviados com zeros à esquerda omitidos.

| Precompile | Endereço |
| --- | --- |
| `layerxledger` | `0x101A` |
| `layerxpay` | `0x101B` |
| `layerxauth` | `0x101C` |
| `layerxprograms` | `0x101D` |
| `layerxchains` | `0x101E` |

O catálogo também inclui funções de agente em `0x0901` a `0x0908`. Use o ABI do precompile e a configuração de upgrade da rede escolhida ao preparar transações. A implantação e a chamada de programas usam o módulo nativo de programas; consulte [Programas](../../programs/index.md).

## Experimente a rede

O JSON-RPC público é servido na raiz de `https://api1.mainnet-beta.paxeer.network` até `https://api14.mainnet-beta.paxeer.network` e pelo alias `https://api-mainnet-beta.paxeer.network`. Consulte os [endpoints públicos](../../reference/public-rpc.md). `node.hyperpaxeer.com` é somente o mirror de bibliotecas hpx usado no build; não é um endpoint de transações.

Com `curl`, confirme o chain ID antes de assinar e consulte a altura do nó. A primeira chamada deve retornar `result: "0x7d"`; a segunda retorna a altura em hexadecimal.

```sh
RPC_URL=https://api1.mainnet-beta.paxeer.network
curl --fail --silent --show-error --max-time 30 "$RPC_URL" \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
curl --fail --silent --show-error --max-time 30 "$RPC_URL" \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":2,"method":"eth_blockNumber","params":[]}'
```

Para alterar o estado, assine uma transação EVM para a cadeia `125` com sua carteira e envie os bytes assinados por `eth_sendRawTransaction`. O hash retornado identifica a transação; ele não comprova a confirmação do bloco. Para consultar uma transação existente, defina `TX_HASH` com seu hash hexadecimal completo de 32 bytes e use `jq` e `curl`:

```sh
jq -n --arg hash "${TX_HASH:?}" \
  '{jsonrpc:"2.0",id:3,method:"eth_getTransactionReceipt",params:[$hash]}' |
  curl --fail --silent --show-error --max-time 30 "$RPC_URL" \
    --header 'Content-Type: application/json' --data-binary @-
```

Um `result: null` indica que o nó ainda não oferece um recibo para esse hash. Quando o recibo estiver disponível, confira `transactionHash`, `blockHash`, `blockNumber` e `status`: `0x1` indica execução bem-sucedida e `0x0` indica falha. A finalização decorre da confirmação do bloco pelo consenso BFT. Se o envio expirar, consulte o hash original antes de tentar novamente.

Mantenha mais de um nó configurado e compare `eth_blockNumber`, pois os nós podem estar em alturas diferentes. Os namespaces públicos são `debug`, `echo`, `eth`, `net`, `pax`, `px`, `test` e `txpool`, conforme a configuração do nó. O namespace `px` responde a partir do estado do nó; `px_register` e `px_requestFunds` são operações dos serviços hospedados, cujo contrato é `services/platform/hosted/gateway/openrpc.json`; veja [Gateway JSON-RPC](../../platform/gateway-rpc.md).

## Compilar a partir do código-fonte

O nó usa Go 1.25.8 (`go.mod`), o kernel usa C17 (`Makefile`) e os workspaces Rust usam 1.91.1 (`rust-toolchain.toml`). A configuração Foundry da raiz fixa Solidity 0.8.27 (`foundry.toml`). Para compilar o nó integrado, execute na raiz do repositório, com as dependências de build instaladas:

```sh
make paxd-build
```

Esse alvo compila `build/liblayerx.a`, o runtime de programas em release e `build/paxd`, e aplica a versão de `tools/release/version.json`. `make build` compila a biblioteca e as ferramentas do kernel; o alvo do nó é `make paxd-build`. `make paxeer-build` delega ao `chain.mk`, que também prepara as bibliotecas nativas. Os alvos do nó usam `GOPROXY=off`, portanto as dependências Go precisam estar disponíveis localmente.

Os serviços têm alvos próprios: `make agent-build`, `make human-build`, `make interop-build`, `make platform-build-all` e `make programs-build`. Execute as verificações do componente que você alterar; veja os [gates de qualificação](../../operators/qualification.md).

## Estrutura do repositório

| Caminho | Função |
| --- | --- |
| `daemon/paxd/`, `node/`, `consensus/` | Executável do nó, aplicação e consenso BFT |
| `src/`, `include/`, `modules/lxphost/` | Kernel C17 e host integrado ao nó |
| `modules/`, `precompiles/`, `rpc/` | Módulos nativos, acesso EVM e servidores JSON-RPC |
| `services/agent/` | SDKs, daemon de agente e servidor MCP |
| `services/human/`, `apps/web/` | Plano de controle humano e aplicativo web |
| `services/platform/`, `tools/platform/` | Serviços hospedados, middleware e ferramentas de desenvolvedor |
| `programs/` | Runtime de programas e ferramentas de compilação |
| `services/interop/` | Comércio entre agentes e interoperabilidade |
| `contracts/` | Contratos e interfaces Solidity; inclui bridge e launchpad |
| `sdk/`, `apps/` | SDKs de clientes e aplicativos |
| `spec/`, `tests/`, `migrations/` | Especificações, testes e migrações |
| `docs/` | Wiki, site e documentação de qualificação |

## Documentação

- [Índice do site](../../index.md)
- [Endpoints JSON-RPC públicos](../../reference/public-rpc.md)
- [Layout do monorepo](../monorepo.md)
- [Gates de qualificação](../../operators/qualification.md)
- [Especificações](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/spec/)

## SDKs e integrações

| Linguagem | Caminho |
| --- | --- |
| Rust | `services/agent/crates/layerx-sdk` |
| Python | `services/agent/sdk/python` |
| TypeScript | `services/agent/sdk/typescript` |
| Go | `sdk/platform/go` |
| JVM | `sdk/platform/jvm` |
| .NET | `sdk/platform/dotnet` |
| Swift | `sdk/platform/swift` |

`layerx-agentd` fica em `services/agent/crates/layerx-agentd`; ele é um serviço de agente fora do nó. O servidor MCP fica em `services/agent/crates/layerx-mcp`. O CLI de desenvolvedor em `tools/platform/cli` oferece `layerx install mcp` e `layerx install a2a`; a instalação exige um ambiente de gateway beta ou de produção configurado e credenciais apropriadas.

## Contribuindo

Leia [`CONTRIBUTING.md`](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/CONTRIBUTING.md) antes de abrir um pull request. Mudanças no protocolo começam em `spec/`. Não divulgue uma vulnerabilidade suspeita em uma issue pública; siga [`SECURITY.md`](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/SECURITY.md).

## Segurança

Reporte vulnerabilidades pelo relatório privado do GitHub, conforme descrito em [`SECURITY.md`](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/SECURITY.md).

## Licença

Licenciado sob a Licença Apache, Versão 2.0. Consulte [`LICENSE`](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/LICENSE) e [`NOTICE`](https://github.com/Sidiora-Labs/Paxeer-X-Network/blob/main/NOTICE).

Paxeer X Network é desenvolvido pela Sidiora Labs.
