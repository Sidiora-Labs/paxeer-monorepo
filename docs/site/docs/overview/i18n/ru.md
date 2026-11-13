# Paxeer X Network

Paxeer X Network объединяет платежи, исполнение программ и операции автономных агентов в одной сети.

[English](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/README.md) · [Español](es.md) · [日本語](ja.md) · Русский · [简体中文](zh-cn.md) · [Português](pt-br.md) · [Deutsch](de.md) · [Français](fr.md)

## Что такое LayerX Network

LayerX — ядро, встроенное в Paxeer X Network. Узел `paxd` исполняет единую BFT-цепочку с EVM chain ID `125` (`0x7d`). Ядро входит в состав узла и участвует в исполнении и формировании предложений блоков. Операции, изменяющие состояние, являются транзакциями этой же сети; квитанция становится окончательной после фиксации её блока консенсусом.

Хост `modules/lxphost` связывает ядро C17 и среду исполнения программ Rust с узлом Go. `node/app.go` регистрирует нативные модули и проверяет порядок транзакций в предложениях блоков. Сохранённое состояние цепочки определяет балансы, полномочия, платежи и программы. Подцепочки — это домены арендаторов: пространства имён в базе данных, которые создаются без инфраструктуры новой сети.

Монорепозиторий Sidiora Labs содержит узел, ядро, контракты, сервисы, приложения и SDK. Спецификация интеграции находится в [`spec/layerx-merge/spec.kvx`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/spec/layerx-merge/spec.kvx).

### Модули и прекомпилированные контракты

EVM-контракты обращаются к нативным модулям через прекомпилированные контракты. Таблица в `precompiles/setup.go` объявляет следующие адреса; их активация зависит от плана `v7.0` и настроенной на узле высоты обновления. Ниже приведены сокращённые 20-байтовые EVM-адреса с опущенными ведущими нулями.

| Прекомпилированный контракт | Адрес |
| --- | --- |
| `layerxledger` | `0x101A` |
| `layerxpay` | `0x101B` |
| `layerxauth` | `0x101C` |
| `layerxprograms` | `0x101D` |
| `layerxchains` | `0x101E` |

Каталог также включает функции агентов по адресам от `0x0901` до `0x0908`. При подготовке транзакций учитывайте ABI контракта и настройки обновления выбранной сети. Развёртывание и вызов программ выполняет нативный модуль программ; см. [Программы](../../programs/index.md).

## Подключение к сети

Получите у оператора корневой HTTPS URL узла и задайте `RPC_URL`. См. [Подключение к узлам](../../reference/public-rpc.md).

С помощью `curl` проверьте chain ID до подписания транзакций и запросите высоту узла. Первый вызов должен вернуть `result: "0x7d"`, второй — высоту в шестнадцатеричном виде.

```sh
: "${RPC_URL:?}"
curl --fail --silent --show-error --max-time 30 "$RPC_URL" \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
curl --fail --silent --show-error --max-time 30 "$RPC_URL" \
  --header 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":2,"method":"eth_blockNumber","params":[]}'
```

Чтобы изменить состояние, подпишите EVM-транзакцию для цепочки `125` своим кошельком и отправьте подписанные байты через `eth_sendRawTransaction`. Полученный хеш идентифицирует транзакцию, но сам по себе не подтверждает фиксацию блока. Для запроса существующей транзакции задайте `TX_HASH` равным её полному 32-байтовому хешу в шестнадцатеричном виде и используйте `jq` и `curl`:

```sh
jq -n --arg hash "${TX_HASH:?}" \
  '{jsonrpc:"2.0",id:3,method:"eth_getTransactionReceipt",params:[$hash]}' |
  curl --fail --silent --show-error --max-time 30 "$RPC_URL" \
    --header 'Content-Type: application/json' --data-binary @-
```

Ответ `result: null` означает, что узел пока не предоставляет квитанцию для этого хеша. В полученной квитанции проверьте `transactionHash`, `blockHash`, `blockNumber` и `status`: `0x1` означает успешное исполнение, `0x0` — неудачное. Окончательность определяется фиксацией блока BFT-консенсусом. При тайм-ауте отправки сначала запросите исходный хеш, прежде чем повторять операцию.

Настройте несколько узлов и сравнивайте `eth_blockNumber`: их высоты могут различаться. Публичные пространства имён — `debug`, `echo`, `eth`, `net`, `pax`, `px`, `test` и `txpool`; доступность зависит от конфигурации узла. Пространство имён `px` отвечает из состояния узла; `px_register` и `px_requestFunds` — операции размещённых сервисов, контракт которых описан в `services/platform/hosted/gateway/openrpc.json`; см. [JSON-RPC шлюза](../../platform/gateway-rpc.md).

## Сборка из исходников

Узел использует Go 1.25.8 (`go.mod`), ядро — C17 (`Makefile`), рабочие пространства Rust — 1.91.1 (`rust-toolchain.toml`). Корневая конфигурация Foundry фиксирует Solidity 0.8.27 (`foundry.toml`). Установив зависимости сборки, выполните в корне репозитория:

```sh
make paxd-build
```

Эта цель собирает `build/liblayerx.a`, среду исполнения программ в режиме release и `build/paxd`, а также задаёт версию из `tools/release/version.json`. `make build` собирает библиотеку и инструменты ядра; цель сборки узла — `make paxd-build`. `make paxeer-build` вызывает `chain.mk`, который также подготавливает нативные библиотеки. Цели узла используют `GOPROXY=off`, поэтому зависимости Go должны быть доступны локально.

Для сервисов предусмотрены отдельные цели: `make agent-build`, `make human-build`, `make interop-build`, `make platform-build-all` и `make programs-build`. Выполняйте проверки изменяемого компонента; см. [Квалификационные проверки](../../operators/qualification.md).

## Структура репозитория

| Путь | Назначение |
| --- | --- |
| `daemon/paxd/`, `node/`, `consensus/` | Исполняемый файл узла, приложение и BFT-консенсус |
| `src/`, `include/`, `modules/lxphost/` | Ядро C17 и его хост внутри узла |
| `modules/`, `precompiles/`, `rpc/` | Нативные модули, доступ из EVM и серверы JSON-RPC |
| `services/agent/` | SDK, демон агента и MCP-сервер |
| `services/human/`, `apps/web/` | Плоскость управления человеком и веб-приложение |
| `services/platform/`, `tools/platform/` | Размещённые сервисы, промежуточное ПО и инструменты разработчика |
| `programs/` | Среда исполнения программ и инструменты компиляции |
| `services/interop/` | Коммерция между агентами и межсетевое взаимодействие |
| `contracts/` | Контракты и интерфейсы Solidity, включая bridge и launchpad |
| `sdk/`, `apps/` | Клиентские SDK и приложения |
| `spec/`, `tests/`, `migrations/` | Спецификации, тесты и миграции |
| `docs/` | Wiki, сайт и документация квалификации |

## Документация

- [Главная страница сайта](../../index.md)
- [Публичные адреса JSON-RPC](../../reference/public-rpc.md)
- [Структура монорепозитория](../monorepo.md)
- [Квалификационные проверки](../../operators/qualification.md)
- [Спецификации](https://github.com/Sidiora-Labs/paxeer-monorepo/tree/main/spec/)

## SDK и интеграции

| Язык | Путь |
| --- | --- |
| Rust | `services/agent/crates/layerx-sdk` |
| Python | `services/agent/sdk/python` |
| TypeScript | `services/agent/sdk/typescript` |
| Go | `sdk/platform/go` |
| JVM | `sdk/platform/jvm` |
| .NET | `sdk/platform/dotnet` |
| Swift | `sdk/platform/swift` |

`layerx-agentd` находится в `services/agent/crates/layerx-agentd` и является сервисом агента вне узла. MCP-сервер расположен в `services/agent/crates/layerx-mcp`. CLI разработчика в `tools/platform/cli` предоставляет команды `layerx install mcp` и `layerx install a2a`; установка требует настроенного окружения шлюза beta или production и соответствующих учётных данных.

## Участие

Прочитайте [`CONTRIBUTING.md`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/CONTRIBUTING.md) перед открытием pull request. Изменения протокола начинаются в `spec/`. Не раскрывайте предполагаемую уязвимость в публичном issue; следуйте [`SECURITY.md`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/SECURITY.md).

## Безопасность

Сообщайте об уязвимостях через приватные отчёты GitHub, как описано в [`SECURITY.md`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/SECURITY.md).

## Лицензия

Лицензировано на условиях Apache License, Version 2.0. См. [`LICENSE`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/LICENSE) и [`NOTICE`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/NOTICE).

Paxeer X Network разрабатывается Sidiora Labs.
