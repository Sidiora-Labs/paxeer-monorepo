defmodule EthereumJSONRPC.Repair2233434Test do
  @moduledoc """
  Task record 22.33.434: coordinator-rate-limit-key is proved by the tests below,
  using the public block-number fetcher, production coordinator, rolling windows
  and HTTP transport against a loopback peer replaying the committed PaxeerX block.

  Baseline was not run: the ordered task instructions require implementation
  before qualification and verification exclusively through hf-done. No
  pre-change test failure is claimed; the gate records the verified revision.
  Wiring requests: none; the repair and its proof are within the declared paths.

  Qualification note: the first verify exposed the existing two-bucket range
  generation issue in RollingWindow. The fixture uses the declared three-bucket
  timeout and four-bucket request configuration; RollingWindow is unchanged.
  """

  use ExUnit.Case, async: false
  @behaviour :cowboy_handler

  alias EthereumJSONRPC.{HTTP, RequestCoordinator, RollingWindow}

  @timeout_table __MODULE__.TimeoutCounter
  @request_table __MODULE__.RequestCounter
  @fixture Path.expand("../support/fixture/paxeer_x/new_head_full.json", __DIR__)

  setup do
    previous = Application.fetch_env!(:ethereum_jsonrpc, RequestCoordinator)
    on_exit(fn -> Application.put_env(:ethereum_jsonrpc, RequestCoordinator, previous) end)

    timeout_opts = [window_count: 3, duration: 60_000, table: @timeout_table]
    request_opts = [window_count: 4, duration: 60_000, table: @request_table]

    start_supervised!({RollingWindow, [timeout_opts]}, id: @timeout_table)
    window = start_supervised!({RollingWindow, [request_opts]}, id: @request_table)

    Application.put_env(:ethereum_jsonrpc, RequestCoordinator,
      rolling_window_opts: timeout_opts,
      wait_per_timeout: 0,
      max_jitter: 1,
      throttle_rate_limit: 2,
      throttle_rolling_window_opts: request_opts
    )

    {:ok, _} = Application.ensure_all_started(:cowboy)
    {:ok, _} = Application.ensure_all_started(:httpoison)
    %{"block" => block} = @fixture |> File.read!() |> Jason.decode!()
    listener = make_ref()
    dispatch = :cowboy_router.compile([{:_, [{"/", __MODULE__, {self(), block}}]}])

    {:ok, _} =
      :cowboy.start_clear(listener, %{socket_opts: [ip: {127, 0, 0, 1}, port: 0]}, %{env: %{dispatch: dispatch}})

    on_exit(fn -> :cowboy.stop_listener(listener) end)
    url = "http://127.0.0.1:#{:ranch.get_port(listener)}"

    args = [
      transport: HTTP,
      transport_options: [http: HTTP.HTTPoison, http_options: [recv_timeout: 1_000, timeout: 1_000], urls: [url]],
      throttle_timeout: 0
    ]

    %{args: args, height: EthereumJSONRPC.quantity_to_integer(block["number"]), window: window}
  end

  test "coordinator-rate-limit-key enforces the top-level limit before reaching HTTP", %{args: args, height: height} do
    config = Application.fetch_env!(:ethereum_jsonrpc, RequestCoordinator)
    refute Keyword.has_key?(config[:throttle_rolling_window_opts], :throttle_rate_limit)

    for _ <- 1..2 do
      assert EthereumJSONRPC.fetch_block_number_by_tag("latest", args) == {:ok, height}
      assert_node_request()
    end

    assert RollingWindow.count(@request_table, :throttle_requests_count) == 2
    assert EthereumJSONRPC.fetch_block_number_by_tag("latest", args) == {:error, :timeout}
    refute_receive {:node_request, _}
    assert RollingWindow.count(@request_table, :throttle_requests_count) == 2
    assert RollingWindow.count(@timeout_table, :throttleable_error_count_eth_getBlockByNumber) == 0
  end

  test "coordinator-rate-limit-key admits requests again after the real window expires", %{
    args: args,
    height: height,
    window: window
  } do
    put_config(:throttle_rate_limit, 1)
    assert EthereumJSONRPC.fetch_block_number_by_tag("latest", args) == {:ok, height}
    assert_node_request()
    assert EthereumJSONRPC.fetch_block_number_by_tag("latest", args) == {:error, :timeout}
    refute_receive {:node_request, _}

    # Drive the production sweep handler through the configured window's buckets.
    for _ <- 1..3 do
      assert GenServer.call(window, :sweep) == :ok
      assert RollingWindow.count(@request_table, :throttle_requests_count) == 1
      assert EthereumJSONRPC.fetch_block_number_by_tag("latest", args) == {:error, :timeout}
      refute_receive {:node_request, _}
    end

    assert GenServer.call(window, :sweep) == :ok
    assert RollingWindow.count(@request_table, :throttle_requests_count) == 0
    assert EthereumJSONRPC.fetch_block_number_by_tag("latest", args) == {:ok, height}
    assert_node_request()
  end

  test "coordinator-rate-limit-key rejects invalid top-level values before transport", %{args: args} do
    for invalid <- [0, -1, 1.5, "2", true] do
      put_config(:throttle_rate_limit, invalid)

      assert_raise ArgumentError, ~r/:throttle_rate_limit must be a positive integer, nil or false/, fn ->
        EthereumJSONRPC.fetch_block_number_by_tag("latest", args)
      end
    end

    refute_receive {:node_request, _}
    assert RollingWindow.count(@request_table, :throttle_requests_count) == 0
  end

  test "coordinator-rate-limit-key preserves disabled and absent limit configuration", %{args: args, height: height} do
    for disabled <- [nil, false] do
      put_config(:throttle_rate_limit, disabled)

      for _ <- 1..3 do
        assert EthereumJSONRPC.fetch_block_number_by_tag("latest", args) == {:ok, height}
        assert_node_request()
      end
    end

    config = Application.fetch_env!(:ethereum_jsonrpc, RequestCoordinator)

    Application.put_env(
      :ethereum_jsonrpc,
      RequestCoordinator,
      Keyword.drop(config, [:throttle_rate_limit, :throttle_rolling_window_opts])
    )

    assert EthereumJSONRPC.fetch_block_number_by_tag("latest", args) == {:ok, height}
    assert_node_request()
  end

  @impl :cowboy_handler
  def init(req, {owner, block} = state) do
    {:ok, body, req} = :cowboy_req.read_body(req)

    [%{"jsonrpc" => "2.0", "id" => id, "method" => "eth_getBlockByNumber", "params" => ["latest", false]}] =
      request = Jason.decode!(body)

    send(owner, {:node_request, request})
    response = Jason.encode!([%{jsonrpc: "2.0", id: id, result: block}])
    req = :cowboy_req.reply(200, %{"content-type" => "application/json"}, response, req)
    {:ok, req, state}
  end

  defp assert_node_request do
    assert_receive {:node_request,
                    [%{"jsonrpc" => "2.0", "method" => "eth_getBlockByNumber", "params" => ["latest", false]}]},
                   1_000
  end

  defp put_config(key, value) do
    config = Application.fetch_env!(:ethereum_jsonrpc, RequestCoordinator)
    Application.put_env(:ethereum_jsonrpc, RequestCoordinator, Keyword.put(config, key, value))
  end
end

