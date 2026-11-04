defmodule EthereumJSONRPC.Repair2233437Test do
  use ExUnit.Case, async: false

  @behaviour :cowboy_handler

  alias EthereumJSONRPC.{Geth, HTTP}
  alias EthereumJSONRPC.Utility.CommonHelper

  @fixture Path.expand("../support/fixture/paxeer_x/call_tracer_block.json", __DIR__)
  @invalid_durations [
    "",
    "5",
    "0",
    "123456789",
    "5ms",
    "5MS",
    "1h5ms",
    "junk5s",
    "5sjunk",
    "junk5sjunk",
    "1h5",
    "1hfoo5m",
    "1h5m!",
    "1.5s",
    "-5s",
    "+5s",
    "5d",
    "5us",
    "s",
    "1hs",
    "1h 5m",
    " 5s",
    "5s ",
    "5s\n",
    "5s\n1m",
    "5s\r\n",
    "5s\0",
    "5ſ"
  ]

  setup do
    previous = Application.get_env(:ethereum_jsonrpc, Geth)

    on_exit(fn ->
      if is_nil(previous) do
        Application.delete_env(:ethereum_jsonrpc, Geth)
      else
        Application.put_env(:ethereum_jsonrpc, Geth, previous)
      end
    end)

    :ok
  end

  test "duration-parser-partial: complete supported components preserve millisecond values" do
    for {duration, milliseconds} <- [
          {"0s", 0},
          {"5s", 5_000},
          {"2m", 120_000},
          {"1h", 3_600_000},
          {"1h5m", 3_900_000},
          {"1h5m7s", 3_907_000},
          {"1H5m7S", 3_907_000},
          {"2M3s", 123_000},
          {"0005S", 5_000},
          {"0h0M0s", 0},
          {"1s2m1s", 122_000},
          {"90s", 90_000}
        ] do
      assert CommonHelper.parse_duration(duration) == milliseconds, inspect(duration)
    end
  end

  test "duration-parser-partial: malformed fragments and bare numbers return a typed error to Geth" do
    arguments = [
      transport: HTTP,
      transport_options: [
        http: HTTP.HTTPoison,
        http_options: [timeout: 11_000, recv_timeout: 13_000, follow_redirect: false],
        urls: ["http://127.0.0.1"]
      ]
    ]

    for duration <- @invalid_durations do
      configure_timeout(duration)
      assert CommonHelper.parse_duration(duration) == {:error, :invalid_format}, inspect(duration)
      assert Geth.correct_timeouts(arguments) == arguments, inspect(duration)
    end
  end

  test "duration-parser-partial: configured composite timeouts drive HTTP trace import with recorded chain data" do
    {:ok, _} = Application.ensure_all_started(:cowboy)
    {:ok, _} = Application.ensure_all_started(:httpoison)
    %{"result" => [%{"txHash" => hash, "result" => trace}]} = @fixture |> File.read!() |> Jason.decode!()
    arguments = trace_endpoint(trace)
    transaction = %{hash_data: hash, block_number: 42, transaction_index: 0}

    for {duration, milliseconds} <- [{"5s", 5_000}, {"1h5m7s", 3_907_000}, {"1H5m7S", 3_907_000}] do
      configure_timeout(duration)
      corrected = Geth.correct_timeouts(arguments)
      assert corrected[:transport_options][:http_options][:timeout] == milliseconds
      assert corrected[:transport_options][:http_options][:recv_timeout] == milliseconds
      assert corrected[:transport_options][:http_options][:follow_redirect] == false

      assert {:ok, [creation, call]} = Geth.fetch_internal_transactions([transaction], arguments)

      assert creation == %{
               block_number: 42,
               transaction_index: 0,
               transaction_hash: hash,
               index: 0,
               trace_address: [],
               type: "create",
               from_address_hash: "0x1111111111111111111111111111111111111111",
               created_contract_address_hash: "0x2222222222222222222222222222222222222222",
               created_contract_code: "0x00",
               gas: 100_000,
               gas_used: 50_000,
               init: "0x60006000f3",
               value: 42
             }

      assert call == %{
               block_number: 42,
               transaction_index: 0,
               transaction_hash: hash,
               index: 1,
               trace_address: [0],
               type: "call",
               call_type: "call",
               from_address_hash: "0x2222222222222222222222222222222222222222",
               to_address_hash: "0x3333333333333333333333333333333333333333",
               gas: 30_000,
               gas_used: 21_000,
               input: "0x12345678",
               output: "0x01",
               value: 7
             }

      assert_receive {:trace_request, request}, 1_000

      assert request == [
               %{
                 "jsonrpc" => "2.0",
                 "id" => 0,
                 "method" => "debug_traceTransaction",
                 "params" => [hash, %{"timeout" => duration, "tracer" => "callTracer"}]
               }
             ]
    end
  end

  @impl :cowboy_handler
  def init(req, {owner, trace} = state) do
    {:ok, body, req} = :cowboy_req.read_body(req)
    requests = Jason.decode!(body)
    send(owner, {:trace_request, requests})

    response =
      Enum.map(requests, fn %{"id" => id} ->
        %{"jsonrpc" => "2.0", "id" => id, "result" => trace}
      end)

    req = :cowboy_req.reply(200, %{"content-type" => "application/json"}, Jason.encode!(response), req)
    {:ok, req, state}
  end

  defp configure_timeout(duration) do
    Application.put_env(:ethereum_jsonrpc, Geth, tracer: "call_tracer", debug_trace_timeout: duration)
  end

  defp trace_endpoint(trace) do
    listener = make_ref()
    dispatch = :cowboy_router.compile([{:_, [{"/", __MODULE__, {self(), trace}}]}])

    {:ok, _} =
      :cowboy.start_clear(listener, %{socket_opts: [ip: {127, 0, 0, 1}, port: 0]}, %{env: %{dispatch: dispatch}})

    on_exit(fn -> :cowboy.stop_listener(listener) end)
    url = "http://127.0.0.1:#{:ranch.get_port(listener)}"

    [
      transport: HTTP,
      transport_options: [
        http: HTTP.HTTPoison,
        http_options: [timeout: 11_000, recv_timeout: 13_000, follow_redirect: false],
        urls: [url],
        trace_urls: [url]
      ]
    ]
  end
end
