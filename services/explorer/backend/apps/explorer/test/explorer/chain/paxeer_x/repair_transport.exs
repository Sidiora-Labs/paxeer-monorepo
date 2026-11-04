defmodule Explorer.Chain.PaxeerX.Repair2233523.Transport do
  import ExUnit.Assertions

  def arguments(url) do
    [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        urls: [url],
        http_options: [timeout: 5_000, recv_timeout: 5_000]
      ]
    ]
  end

  def rpc(method, params, arguments) do
    %{id: 523, method: method, params: params}
    |> EthereumJSONRPC.request()
    |> EthereumJSONRPC.json_rpc(arguments)
  end

  def capability_recovery do
    alias Explorer.Chain.PaxeerX.Capabilities
    fixture = Path.join(__DIR__, "fixtures/launchpad_rpc/live.json") |> File.read!() |> Jason.decode!()

    exchanges =
      Map.new(fixture["exchanges"], fn exchange ->
        request = exchange["request"]
        {{request["method"], request["params"]}, exchange["response"]}
      end)

    parent = self()
    server = Bypass.open()
    url = "http://127.0.0.1:#{server.port}/"

    Bypass.expect(server, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      request = Jason.decode!(body)

      response =
        if is_list(request) do
          send(parent, {:views, request})

          request
          |> Enum.map(fn request ->
            Map.fetch!(exchanges, {request["method"], request["params"]}) |> Map.put("id", request["id"])
          end)
          |> Enum.reverse()
        else
          assert request["method"] == "eth_blockNumber"
          assert request["params"] == []
          send(parent, {:head, self()})

          receive do
            :release_head -> :ok
          after
            5_000 -> flunk("head response was never released")
          end

          Map.fetch!(exchanges, {"eth_blockNumber", []}) |> Map.put("id", request["id"])
        end

      Plug.Conn.resp(conn, 200, Jason.encode!(response))
    end)

    old_arguments = Application.get_env(:explorer, :json_rpc_named_arguments)
    old_config = Application.get_env(:explorer, Capabilities)
    Application.put_env(:explorer, :json_rpc_named_arguments, arguments(url))
    Application.put_env(:explorer, Capabilities, refresh_interval_seconds: 3600)

    try do
      snapshots =
        for _ <- 1..2 do
          {:ok, pid} = Capabilities.start_link([])

          try do
            assert_receive {:head, handler}, 5_000
            assert Capabilities.all().checked_at == nil
            assert Enum.all?(Capabilities.surfaces(), &(Capabilities.live?(&1) == false))
            send(handler, :release_head)
            assert_receive {:views, requests}, 5_000
            assert length(requests) == 4

            assert Enum.all?(requests, fn request ->
                     request["method"] == "eth_call" and List.last(request["params"]) == fixture["height"]
                   end)

            # The call is serialized behind the boot probe, without issuing another RPC.
            :sys.get_state(pid)
            snapshot = Capabilities.all()
            assert %DateTime{} = snapshot.checked_at

            assert Map.drop(snapshot, [:checked_at]) == %{
                     unified_account: true,
                     exchange: true,
                     bridge: true,
                     launchpad: true
                   }

            snapshot
          after
            GenServer.stop(pid)
          end
        end

      [first, second] = snapshots
      assert DateTime.compare(second.checked_at, first.checked_at) in [:eq, :gt]
      assert Capabilities.all().checked_at == nil
    after
      restore(:explorer, :json_rpc_named_arguments, old_arguments)
      restore(:explorer, Capabilities, old_config)
    end
  end

  def restore(app, key, nil), do: Application.delete_env(app, key)
  def restore(app, key, value), do: Application.put_env(app, key, value)
end
