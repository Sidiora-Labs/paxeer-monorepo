defmodule EthereumJSONRPC.Repair2233445Test.Node do
  @moduledoc false
  @behaviour :cowboy_websocket

  @impl :cowboy_websocket
  def init(request, state) do
    if :cowboy_req.header("upgrade", request) == "websocket" do
      {:cowboy_websocket, request, state}
    else
      {:ok, body, request} = :cowboy_req.read_body(request)
      rpc = Jason.decode!(body)
      response = respond(rpc, state, :http)
      request = :cowboy_req.reply(200, %{"content-type" => "application/json"}, Jason.encode!(response), request)
      {:ok, request, state}
    end
  end

  @impl :cowboy_websocket
  def websocket_handle({:text, text}, state) do
    rpc = Jason.decode!(text)
    response = respond(rpc, state, :websocket)
    if rpc["method"] == "eth_subscribe", do: send(self(), :publish_head)
    {:reply, {:text, Jason.encode!(response)}, state}
  end

  @impl :cowboy_websocket
  def websocket_info(:publish_head, state) do
    notification = %{
      jsonrpc: "2.0",
      method: "eth_subscription",
      params: %{subscription: "0x125", result: state.fixture["new_head"]}
    }

    {:reply, {:text, Jason.encode!(notification)}, state}
  end

  defp respond(batch, state, transport) when is_list(batch) do
    Enum.map(batch, &respond(&1, state, transport))
  end

  defp respond(%{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => params} = rpc, state, transport) do
    send(state.owner, {:node_request, transport, rpc})

    result =
      case {method, params} do
        {"eth_chainId", []} ->
          "0x7d"

        {"eth_getBlockByHash", [hash, true]} ->
          %{"hash" => ^hash} = state.fixture["block"]
          state.fixture["block"]

        {"eth_subscribe", ["newHeads"]} ->
          "0x125"

        {"eth_unsubscribe", ["0x125"]} ->
          true
      end

    %{jsonrpc: "2.0", id: id, result: result}
  end
end

defmodule EthereumJSONRPC.Repair2233445Test do
  use ExUnit.Case, async: false

  alias EthereumJSONRPC.{Blocks, HTTP, PaxeerX, WebSocket}
  alias EthereumJSONRPC.Repair2233445Test.Node
  alias EthereumJSONRPC.WebSocket.WebSocketClient.Options

  @readme Path.expand("../../README.md", __DIR__)
  @backend Path.expand("../../../..", __DIR__)
  @profile Path.join(@backend, "apps/explorer/config/prod/paxeer_x.exs")
  @runtime Path.join(@backend, "config/runtime.exs")
  @fixture Path.expand("../support/fixture/paxeer_x/new_head_full.json", __DIR__)

  setup do
    previous_env =
      System.get_env()
      |> Map.filter(fn {key, _} -> String.starts_with?(key, "ETHEREUM_JSONRPC_") or key == "CHAIN_TYPE" end)

    previous_adapter = Application.fetch_env(:tesla, :adapter)

    on_exit(fn ->
      for {key, _} <- System.get_env(), String.starts_with?(key, "ETHEREUM_JSONRPC_") or key == "CHAIN_TYPE" do
        System.delete_env(key)
      end

      System.put_env(previous_env)

      case previous_adapter do
        {:ok, adapter} -> Application.put_env(:tesla, :adapter, adapter)
        :error -> Application.delete_env(:tesla, :adapter)
      end
    end)

    for {key, _} <- previous_env, do: System.delete_env(key)
    System.put_env("CHAIN_TYPE", "paxeer_x")
    System.put_env("ETHEREUM_JSONRPC_VARIANT", "paxeer_x")
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    {:ok, _} = Application.ensure_all_started(:cowboy)
    %{readme: File.read!(@readme)}
  end

  test "readme-config: documented named arguments match production and import a recorded chain-125 head", %{
    readme: readme
  } do
    {http_url, websocket_url, fixture} = start_node()
    System.put_env("ETHEREUM_JSONRPC_HTTP_URLS", http_url)
    System.put_env("ETHEREUM_JSONRPC_WS_URL", websocket_url)

    {http, subscriptions} = documented_arguments(readme)
    production = production_arguments()
    assert http == Keyword.fetch!(production, :json_rpc_named_arguments)
    assert subscriptions == Keyword.fetch!(production, :subscribe_named_arguments)
    assert http[:variant] == PaxeerX
    assert http[:transport] == HTTP
    assert http[:transport_options][:urls] == [http_url]
    assert http[:transport_options][:trace_urls] == [http_url]
    assert http[:transport_options][:eth_call_urls] == [http_url]
    assert http[:transport_options][:http_options] == [pool: :ethereum_jsonrpc, recv_timeout: 60_000, timeout: 60_000]

    assert {:ok, 125} = EthereumJSONRPC.fetch_chain_id(http)
    assert_request(:http, "eth_chainId", [])

    options =
      WebSocket
      |> struct!(subscriptions[:transport_options])
      |> Map.put(:web_socket_options, %Options{web_socket: __MODULE__.Client})

    start_supervised!({WebSocket.Supervisor, options})
    subscriptions = Keyword.put(subscriptions, :transport_options, options)
    chain_id_request = EthereumJSONRPC.request(%{id: 0, method: "eth_chainId", params: []})
    assert {:ok, chain_id} = WebSocket.json_rpc(chain_id_request, options)
    assert chain_id == "0x7d"
    assert EthereumJSONRPC.quantity_to_integer(chain_id) == 125
    assert_request(:websocket, "eth_chainId", [])

    assert {:ok, subscription} = EthereumJSONRPC.subscribe("newHeads", [], subscriptions)
    assert_request(:websocket, "eth_subscribe", ["newHeads"])
    assert_receive {^subscription, {:ok, head}}, 5_000
    assert head == fixture["new_head"]

    assert {:ok, %Blocks{blocks_params: [block], transactions_params: [], errors: []}} =
             PaxeerX.fetch_block_by_new_head(head, http)

    assert_request(:http, "eth_getBlockByHash", [head["hash"], true])
    assert block.hash == fixture["block"]["hash"]
    assert block.number == EthereumJSONRPC.quantity_to_integer(fixture["block"]["number"])
    assert block.parent_hash == fixture["block"]["parentHash"]
    assert block.receipts_root == fixture["block"]["receiptsRoot"]
    assert :ok = EthereumJSONRPC.unsubscribe(subscription)
    assert_request(:websocket, "eth_unsubscribe", ["0x125"])
  end

  test "readme-config: URL precedence and specialized lists use the active runtime helpers", %{readme: readme} do
    {url, websocket_url, _fixture} = start_node()
    System.put_env("ETHEREUM_JSONRPC_HTTP_URL", "http://unused.invalid/")
    System.put_env("ETHEREUM_JSONRPC_HTTP_URLS", url)
    System.put_env("ETHEREUM_JSONRPC_WS_URL", websocket_url)
    System.put_env("ETHEREUM_JSONRPC_HTTP_TIMEOUT", "9")

    for {type, plural, singular} <- [
          {:trace_urls, "TRACE_URLS", "TRACE_URL"},
          {:eth_call_urls, "ETH_CALL_URLS", "ETH_CALL_URL"},
          {:fallback_urls, "FALLBACK_HTTP_URLS", "FALLBACK_HTTP_URL"},
          {:fallback_trace_urls, "FALLBACK_TRACE_URLS", "FALLBACK_TRACE_URL"},
          {:fallback_eth_call_urls, "FALLBACK_ETH_CALL_URLS", "FALLBACK_ETH_CALL_URL"}
        ] do
      System.put_env("ETHEREUM_JSONRPC_" <> plural, url)
      System.put_env("ETHEREUM_JSONRPC_" <> singular, "http://unused.invalid/")
      {http, _subscriptions} = documented_arguments(readme)
      production = production_arguments()
      assert http == Keyword.fetch!(production, :json_rpc_named_arguments)
      assert http[:transport_options][type] == [url]
      assert http[:transport_options][:http_options][:recv_timeout] == 9_000
      assert http[:transport_options][:http_options][:timeout] == 9_000
      assert {:ok, 125} = EthereumJSONRPC.fetch_chain_id(http)
      assert_request(:http, "eth_chainId", [])
    end
  end

  test "readme-config: documented timeout and guard defaults match executable runtime configuration", %{readme: readme} do
    defaults = [
      {EthereumJSONRPC.Geth, :debug_trace_timeout, "ETHEREUM_JSONRPC_DEBUG_TRACE_TRANSACTION_TIMEOUT", "5s", "5s"},
      {EthereumJSONRPC.RequestCoordinator, :wait_per_timeout, "ETHEREUM_JSONRPC_WAIT_PER_TIMEOUT", 20_000, "20s"},
      {WebSocket.RetryWorker, :retry_interval, "ETHEREUM_JSONRPC_WS_RETRY_INTERVAL", 60_000, "1m"},
      {HTTP.Throttle, :max_inflight, "ETHEREUM_JSONRPC_THROTTLE_MAX_INFLIGHT", 32, "32"},
      {HTTP.Throttle, :rps, "ETHEREUM_JSONRPC_THROTTLE_RPS", 200, "200"},
      {HTTP.Throttle, :trace_max_inflight, "ETHEREUM_JSONRPC_THROTTLE_TRACE_MAX_INFLIGHT", 4, "4"},
      {HTTP.Throttle, :archive_max_inflight, "ETHEREUM_JSONRPC_THROTTLE_ARCHIVE_MAX_INFLIGHT", 8, "8"},
      {HTTP.Throttle, :coalesce?, "ETHEREUM_JSONRPC_THROTTLE_COALESCE_ENABLED", true, "true"}
    ]

    modules = Enum.map(defaults, &elem(&1, 0))
    {:__block__, _, runtime} = @runtime |> File.read!() |> Code.string_to_quoted!()

    configs =
      Enum.filter(runtime, fn
        {:config, _, [:ethereum_jsonrpc, {:__aliases__, _, _} = module, _]} ->
          Macro.expand(module, __ENV__) in modules

        _ ->
          false
      end)

    source = "import Config\n" <> Macro.to_string({:__block__, [], configs})
    config = @runtime |> Config.Reader.eval!(source) |> Keyword.fetch!(:ethereum_jsonrpc)

    for {module, key, env_var, expected, documented} <- defaults do
      assert config[module][key] == expected
      consumer = module |> Module.split() |> Enum.join(".")
      assert readme =~ "| `#{env_var}` | `#{consumer}[:#{key}]` | `#{documented}` |"
    end

    assert readme =~ "60 seconds (60_000 milliseconds)"
    assert readme =~ "chain ID 125 (`0x7d`)"
    assert readme =~ "profile: :geth_compatibility"
    assert readme =~ ":nethermind_compatibility"
    assert readme =~ "profile: :compatibility"
    refute readme =~ ~r/infura|alchemy|quiknode/i

    for [_, endpoint] <- Regex.scan(~r/https:\/\/(api\d+\.mainnet-beta\.paxeer\.network\/)/, readme) do
      assert URI.parse("https://" <> endpoint).path == "/"
    end
  end

  defp documented_arguments(readme) do
    [_, example] = Regex.run(~r/<!-- named-arguments:start -->\s*```elixir\n(.*?)\n```/s, readme)
    {_value, binding} = Code.eval_string(example, [], file: @readme)
    {Keyword.fetch!(binding, :json_rpc_named_arguments), Keyword.fetch!(binding, :subscribe_named_arguments)}
  end

  defp production_arguments do
    File.cd!(@backend, fn -> @profile |> Config.Reader.read!() |> Keyword.fetch!(:explorer) end)
  end

  defp start_node do
    fixture = @fixture |> File.read!() |> Jason.decode!()
    listener = make_ref()
    dispatch = :cowboy_router.compile([{:_, [{"/", Node, %{owner: self(), fixture: fixture}}]}])
    {:ok, _} = :cowboy.start_clear(listener, [ip: {127, 0, 0, 1}, port: 0], %{env: %{dispatch: dispatch}})
    on_exit(fn -> :cowboy.stop_listener(listener) end)
    port = :ranch.get_port(listener)
    {"http://127.0.0.1:#{port}/", "ws://127.0.0.1:#{port}/", fixture}
  end

  defp assert_request(transport, method, params) do
    assert_receive {:node_request, ^transport,
                    %{"jsonrpc" => "2.0", "id" => id, "method" => ^method, "params" => ^params} = rpc},
                   5_000

    assert is_integer(id)
    assert Enum.sort(Map.keys(rpc)) == ~w(id jsonrpc method params)
  end
end
