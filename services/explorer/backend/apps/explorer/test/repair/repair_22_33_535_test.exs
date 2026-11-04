defmodule Explorer.Repair.Repair2233535Test do
  use ExUnit.Case, async: true

  alias Explorer.ThirdPartyIntegrations.UniversalProxy.SocketHandler

  @recording Path.expand("../../lib/explorer/chain/paxeer_x/fixtures/capability_discovery.json", __DIR__)

  defmodule RecordedUpstream do
    @behaviour :cowboy_websocket

    @impl true
    def init(req, state), do: {:cowboy_websocket, req, state}

    @impl true
    def websocket_init(state) do
      send(state.observer, {:upstream_connected, self()})
      {:ok, state}
    end

    @impl true
    def websocket_handle({type, payload} = frame, state) when type in [:text, :binary] do
      send(state.observer, {:upstream_frame, self(), frame})
      {:reply, {type, Map.fetch!(state.transcript, payload)}, state}
    end

    def websocket_handle(_frame, state), do: {:ok, state}

    @impl true
    def websocket_info({:close_upstream, code}, state) do
      {:reply, {:close, code, "Recorded upstream closed"}, state}
    end

    @impl true
    def terminate(reason, _req, state) do
      send(state.observer, {:upstream_terminated, self(), reason})
      :ok
    end
  end

  defmodule Client do
    use WebSockex

    @impl true
    def handle_frame(frame, observer) do
      send(observer, {:client_frame, self(), frame})
      {:ok, observer}
    end

    @impl true
    def handle_cast(:close, observer), do: {:close, observer}

    @impl true
    def handle_disconnect(status, observer) do
      send(observer, {:client_disconnected, self(), status.reason})
      {:ok, observer}
    end
  end

  setup do
    {:ok, _} = Application.ensure_all_started(:cowboy)
    head = @recording |> File.read!() |> Jason.decode!() |> Map.fetch!("head")
    request = Jason.encode!(%{"jsonrpc" => "2.0", "id" => 1, "method" => head["method"], "params" => head["params"]})
    response = Jason.encode!(Map.merge(head["response"], %{"jsonrpc" => "2.0", "id" => 1}))
    transcript = %{request => response, <<0, 255, 128, 1, 0>> => <<255, 0, 128, 2>>, "" => ""}
    upstream_url = listen(RecordedUpstream, %{observer: self(), transcript: transcript})
    proxy_url = listen(SocketHandler, url: upstream_url)
    {:ok, client} = WebSockex.start(proxy_url, Client, self())

    on_exit(fn ->
      if Process.alive?(client), do: Process.exit(client, :kill)
    end)

    assert_receive {:upstream_connected, upstream}, 2000
    %{client: client, upstream: upstream, request: request, response: response}
  end

  test "proxy-binary-drop: recorded chain requests and replies preserve binary and text frames", ctx do
    for type <- [:binary, :text] do
      assert :ok = WebSockex.send_frame(ctx.client, {type, ctx.request})
      assert_receive {:upstream_frame, upstream, {^type, request}}, 2000
      assert upstream == ctx.upstream
      assert request == ctx.request
      assert_receive {:client_frame, client, {^type, response}}, 2000
      assert client == ctx.client
      assert response == ctx.response
      assert Jason.decode!(response)["result"] == "0x16c1320"
    end
  end

  test "proxy-binary-drop: opaque and empty binary payloads survive a real round trip", ctx do
    for {request, response} <- [{<<0, 255, 128, 1, 0>>, <<255, 0, 128, 2>>}, {"", ""}] do
      assert :ok = WebSockex.send_frame(ctx.client, {:binary, request})
      assert_receive {:upstream_frame, upstream, {:binary, ^request}}, 2000
      assert upstream == ctx.upstream
      assert_receive {:client_frame, client, {:binary, ^response}}, 2000
      assert client == ctx.client
    end
  end

  for code <- [1000, 1011] do
    @close_code code
    test "proxy-binary-drop: upstream close #{@close_code} closes the downstream connection cleanly", ctx do
      client_ref = Process.monitor(ctx.client)
      upstream_ref = Process.monitor(ctx.upstream)
      send(ctx.upstream, {:close_upstream, @close_code})

      assert_receive {:client_disconnected, client, {:remote, 1011, "Target connection lost"}}, 2000
      assert client == ctx.client
      assert_receive {:DOWN, ^client_ref, :process, ^client, _reason}, 2000
      assert_receive {:upstream_terminated, upstream, _reason}, 2000
      assert upstream == ctx.upstream
      assert_receive {:DOWN, ^upstream_ref, :process, ^upstream, _reason}, 2000
    end
  end

  test "proxy-binary-drop: downstream disconnect releases the upstream connection", ctx do
    client_ref = Process.monitor(ctx.client)
    upstream_ref = Process.monitor(ctx.upstream)
    WebSockex.cast(ctx.client, :close)

    assert_receive {:DOWN, ^client_ref, :process, client, :normal}, 2000
    assert client == ctx.client
    assert_receive {:upstream_terminated, upstream, _reason}, 2000
    assert upstream == ctx.upstream
    assert_receive {:DOWN, ^upstream_ref, :process, ^upstream, _reason}, 2000
  end

  defp listen(handler, options) do
    ref = {__MODULE__, make_ref()}
    dispatch = :cowboy_router.compile([{:_, [{"/", handler, options}]}])
    {:ok, _} = :cowboy.start_clear(ref, [ip: {127, 0, 0, 1}, port: 0], %{env: %{dispatch: dispatch}})
    on_exit(fn -> :cowboy.stop_listener(ref) end)
    "ws://127.0.0.1:#{:ranch.get_port(ref)}/"
  end
end
