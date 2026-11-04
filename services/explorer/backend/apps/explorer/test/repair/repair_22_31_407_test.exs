defmodule Explorer.Repair.Repair2231407Test do
  use ExUnit.Case, async: false

  alias EthereumJSONRPC.{Contract, HTTP}
  alias Explorer.Chain.PaxeerX.Capabilities

  @launchpad "0x0000000000000000000000000000000000001017"
  @selector "0x" <> (ExKeccak.hash_256("getPoolCount()") |> binary_part(0, 4) |> Base.encode16(case: :lower))
  @fixtures Path.expand("../explorer/chain/paxeer_x/fixtures/launchpad_rpc", __DIR__)

  defmodule RecordedRPC do
    @behaviour Plug

    def init(options), do: options

    def call(conn, {exchanges, observer}) do
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      request = Jason.decode!(body)

      response =
        if is_list(request) do
          if length(request) == 4, do: send(observer, {:capability_batch, request})
          request |> Enum.map(&reply(&1, exchanges)) |> Enum.reverse()
        else
          reply(request, exchanges)
        end

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.send_resp(200, Jason.encode!(response))
    end

    defp reply(%{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => params}, exchanges) do
      exchanges |> Map.fetch!({method, params}) |> Map.put("id", id)
    end
  end

  test "launchpad_code_probe_live" do
    {arguments, height, response, fixture} = node_case("LIVE")

    assert %{result: result} = response
    assert abi_word?(result)

    assert result ==
             "0x" <> Base.encode16(ABI.TypeEncoder.encode([fixture["pool_count"]], [{:uint, 256}]), case: :lower)

    assert {:ok, "0x"} = rpc("eth_getCode", [@launchpad, height], arguments)
    assert {:ok, %{launchpad: true, checked_at: %DateTime{}}} = Capabilities.probe(arguments)
    assert_probe_requests(fixture)
    assert {:ok, ^height} = rpc("eth_blockNumber", [], arguments)
  end

  test "launchpad_code_probe_refused" do
    {arguments, height, response, fixture} = node_case("REFUSED")

    assert %{error: %{code: code, message: message}} = response
    assert is_integer(code)
    assert is_binary(message)
    assert {:ok, "0x"} = rpc("eth_getCode", [@launchpad, height], arguments)
    assert {:ok, %{launchpad: false, checked_at: %DateTime{}}} = Capabilities.probe(arguments)
    assert_probe_requests(fixture)
    assert {:ok, ^height} = rpc("eth_blockNumber", [], arguments)
  end

  test "launchpad_code_probe_malformed" do
    {arguments, height, response, fixture} = node_case("MALFORMED")

    assert %{result: result} = response
    refute abi_word?(result)
    assert {:ok, "0x"} = rpc("eth_getCode", [@launchpad, height], arguments)

    assert {:error, {:launchpad, {:unexpected_response, %{result: ^result}}}} =
             Capabilities.probe(arguments)

    assert_probe_requests(fixture)
    assert {:ok, ^height} = rpc("eth_blockNumber", [], arguments)
  end

  # Optional manual integration mode; the mandatory gate never reads an
  # environment variable or contacts a host outside its own loopback listener.
  def live_node_case(_kind, nil), do: {:skip, :live_url_unset}

  def live_node_case(kind, url) when kind in ["LIVE", "REFUSED", "MALFORMED"] do
    assert %URI{scheme: scheme, host: host, path: path} = URI.parse(url)
    assert scheme in ["http", "https"] and is_binary(host)
    assert path in [nil, "", "/"], "launchpad probes use root node JSON-RPC"
    probe_node(arguments(url))
  end

  defp node_case(kind) do
    fixture = @fixtures |> Path.join(String.downcase(kind) <> ".json") |> File.read!() |> Jason.decode!()
    assert fixture["state"] == kind

    exchanges =
      Map.new(fixture["exchanges"], fn %{"request" => request, "response" => response} ->
        assert request["jsonrpc"] == "2.0"
        assert response["jsonrpc"] == "2.0"
        assert response["id"] == request["id"]
        {{request["method"], request["params"]}, response}
      end)

    {:ok, _} = Application.ensure_all_started(:plug_cowboy)
    ref = {:launchpad_recording, make_ref()}

    start_supervised!(
      {Plug.Cowboy,
       scheme: :http, plug: {RecordedRPC, {exchanges, self()}}, options: [ip: {127, 0, 0, 1}, port: 0, ref: ref]}
    )

    {arguments, height, response} = probe_node(arguments("http://127.0.0.1:#{:ranch.get_port(ref)}/"))
    assert height == fixture["height"]
    {arguments, height, response, fixture}
  end

  defp probe_node(arguments) do
    assert {:ok, height} = rpc("eth_blockNumber", [], arguments)
    block_number = EthereumJSONRPC.quantity_to_integer(height)
    assert is_integer(block_number) and block_number >= 0

    request = Contract.eth_call_request(@selector, @launchpad, 407, block_number, nil)
    assert request.params == [%{to: @launchpad, data: @selector}, EthereumJSONRPC.integer_to_quantity(block_number)]
    assert {:ok, [%{id: 407} = response]} = EthereumJSONRPC.json_rpc([request], arguments)

    {arguments, height, response}
  end

  defp assert_probe_requests(fixture) do
    assert_received {:capability_batch, requests}
    assert length(requests) == 4
    assert requests |> Enum.map(& &1["id"]) |> Enum.uniq() |> length() == 4

    expected =
      for %{"request" => %{"method" => "eth_call"} = request} <- fixture["exchanges"] do
        Map.take(request, ["method", "params", "jsonrpc"])
      end

    assert Enum.sort(Enum.map(requests, &Map.take(&1, ["method", "params", "jsonrpc"]))) == Enum.sort(expected)
  end

  defp arguments(url) do
    [
      transport: HTTP,
      transport_options: [
        http: HTTP.HTTPoison,
        urls: [url],
        http_options: [timeout: 5_000, recv_timeout: 5_000]
      ]
    ]
  end

  defp rpc(method, params, arguments) do
    %{id: 1, method: method, params: params}
    |> EthereumJSONRPC.request()
    |> EthereumJSONRPC.json_rpc(arguments)
  end

  defp abi_word?("0x" <> digits), do: byte_size(digits) == 64 and String.match?(digits, ~r/\A[0-9a-fA-F]{64}\z/)
  defp abi_word?(_), do: false
end
