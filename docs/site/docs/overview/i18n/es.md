<h1 align="center">Paxeer X Network</h1>

Paxeer X Network es una red BFT para agentes autónomos, pagos y ejecución programable. LayerX está integrado en el nodo Paxeer, `paxd`.

[English](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/README.md) · Español · [日本語](ja.md) · [Русский](ru.md) · [简体中文](zh-cn.md) · [Português](pt-br.md) · [Deutsch](de.md) · [Français](fr.md)

*Cuando este archivo y el README en inglés difieren, el README en inglés es la versión de referencia.*

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/LICENSE)
[![CI](https://github.com/Sidiora-Labs/paxeer-monorepo/actions/workflows/ci.yml/badge.svg)](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/.github/workflows/ci.yml)

## Qué es Paxeer X Network

Paxeer mainnet-beta utiliza el ID de cadena EVM **125** (`0x7d`). El kernel forma parte de la red y participa en la propuesta de bloques. Las operaciones de los agentes son transacciones de esta misma cadena, incluidas las llamadas a precompilados EVM. Un recibo es final cuando su bloque queda confirmado por el consenso BFT. La aceptación del envío no demuestra que la ejecución haya tenido éxito.

El kernel nativo se integra en `paxd` mediante `modules/lxphost/`. Los movimientos monetarios se aplican mediante las rutas del libro mayor, del banco y del estado EVM de la cadena. La ejecución nativa crítica para el consenso utiliza codificación canónica y aritmética entera comprobada; las pruebas de replay verifican la reproducibilidad.

Las subcadenas son dominios de inquilino: espacios de nombres en la base de datos. No necesitan una red de validadores ni infraestructura de nodos nueva. Este monorepositorio contiene los componentes de una sola red. Las especificaciones vigentes están en [`spec/layerx-merge/`](https://github.com/Sidiora-Labs/paxeer-monorepo/tree/main/spec/layerx-merge).

## Probar la conexión

El JSON-RPC público se sirve por HTTPS en la raíz de `api1.mainnet-beta.paxeer.network` a `api14.mainnet-beta.paxeer.network`, y de `api-mainnet-beta.paxeer.network`. Estas consultas solo leen el ID de cadena y la altura del bloque. `PAXEER_RPC_URL` es una variable local de shell en este ejemplo.

```sh
PAXEER_RPC_URL=https://api-mainnet-beta.paxeer.network
curl --fail --silent --show-error --max-time 30 "$PAXEER_RPC_URL" \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
curl --fail --silent --show-error --max-time 30 "$PAXEER_RPC_URL" \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":2,"method":"eth_blockNumber","params":[]}'
```

`eth_chainId` debe devolver `"0x7d"`. Mainnet-beta utiliza activos reales. Para modificar el estado, la aplicación firma una transacción para la cadena 125 y la envía con `eth_sendRawTransaction`. Después consulta `eth_getTransactionReceipt` con el hash de la transacción. `null` no confirma la inclusión; si hay recibo, compruebe `status`, `blockHash`, `blockNumber` y los logs. Una transacción fallida también es final una vez confirmado el bloque.

Programs se invoca mediante el precompilado `0x000000000000000000000000000000000000101D`; su ABI está en `precompiles/layerxprograms/abi.json`. Véase [Programs](../../programs/index.md). La [guía de inicio](../quickstart.md) contiene más ejemplos; la [descripción de la red](../../index.md) explica los endpoints y la finalidad.

Las llamadas alojadas `px_` están declaradas en `services/platform/hosted/gateway/openrpc.json`. No se garantiza que cada nodo público exponga esos métodos.

## Compilar desde el código fuente

El runtime nativo usa C17, los espacios de trabajo Rust usan el compilador 1.91.1 fijado en `rust-toolchain.toml` y el módulo Go declara Go 1.25.8. `make paxd-build` compila las bibliotecas nativas y el nodo con la versión de publicación. `make build` compila el runtime nativo y sus herramientas auxiliares.

```sh
make paxd-build
make paxd-version-check
```

Las pruebas nativas, la integración del nodo y las matrices de publicación demuestran propiedades distintas de la misma red. Consulte los comandos y requisitos en [Cualificación](../../operators/qualification.md). Compilar un componente no demuestra la ruta completa de una transacción.

## Estructura del repositorio

| Ruta | Propósito |
| --- | --- |
| `daemon/`, `node/`, `consensus/` | Nodo, procesamiento de bloques y consenso BFT |
| `src/`, `include/` | Kernel C17 y módulos de protocolo nativos |
| `modules/lxphost/` | Integración del kernel en el nodo |
| `modules/`, `precompiles/` | Estado de la cadena y puntos de entrada invocables desde EVM |
| `rpc/` | JSON-RPC del nodo |
| `programs/` | Runtime Programs, sandbox y SDK |
| `services/agent/` | SDKs de agentes, daemon de agente y servidor MCP |
| `services/human/`, `apps/web/` | Control humano y aplicación web |
| `services/platform/`, `tools/platform/cli/` | Servicios alojados de plataforma y CLI de desarrollador |
| `services/interop/`, `sdk/platform/` | Interoperabilidad y SDKs de cliente |
| `spec/` | Especificaciones normativas y grafos de tareas |
| `docs/`, `tests/`, `tests/solidity/` | Documentación y suites de cualificación |

## Documentación

- [Descripción de la red](../../index.md)
- [Estructura del monorepositorio](../monorepo.md)
- [Cualificación](../../operators/qualification.md)
- [Especificaciones](https://github.com/Sidiora-Labs/paxeer-monorepo/tree/main/spec/layerx-merge)

## SDKs e integraciones

| Lenguaje | Ruta |
| --- | --- |
| Rust | `services/agent/crates/layerx-sdk` |
| Python | `services/agent/sdk/python` |
| TypeScript | `services/agent/sdk/typescript` |
| Go | `sdk/platform/go` |
| JVM | `sdk/platform/jvm` |
| .NET | `sdk/platform/dotnet` |
| Swift | `sdk/platform/swift` |

`layerx-agentd` está en `services/agent/crates/layerx-agentd` y el servidor MCP en `services/agent/crates/layerx-mcp`. Ambos son clientes de la red. La CLI de desarrollador está en `tools/platform/cli`.

## Cómo contribuir

Lea [`CONTRIBUTING.md`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/CONTRIBUTING.md) antes de abrir un pull request. Los cambios de protocolo empiezan en `spec/`. No divulgue una vulnerabilidad sospechada en una incidencia pública; siga [`SECURITY.md`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/SECURITY.md).

## Seguridad

Informe de vulnerabilidades mediante el reporte privado de GitHub, como se describe en [`SECURITY.md`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/SECURITY.md).

## Licencia

Licenciado bajo la Licencia Apache, versión 2.0. Véase [`LICENSE`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/LICENSE) y [`NOTICE`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/NOTICE).

Paxeer X Network lo desarrolla Sidiora Labs.
