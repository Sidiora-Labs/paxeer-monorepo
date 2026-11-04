Code.require_file("../indexer/fetcher/token_instance/helper_test.exs", __DIR__)
Code.require_file("../indexer/fetcher/token_instance/realtime_test.exs", __DIR__)

defmodule Indexer.Repair2233559Test do
  @moduledoc """
  Task 22.33.559 repair record, audit 22.30.158:
  * 015: base URI and IPFS cases restore complete configuration after assertion failure.
  * 016: retry deadlines follow exponential backoff and its cap in both directions.
  * 055: realtime retry restores the original adapter after assertion failure.

  Baseline: not run; the task's ordered gate instructions require complete implementation
  before hf-build and hf-done, and prohibit invoking verify_cmd separately.
  No out-of-path wiring is required. The runner records the verified revision and logs.
  The original HelperTest and RealtimeTest cases also execute under this focused gate.
  """
  use Explorer.DataCase, async: false

  alias Explorer.Chain.Token.Instance
  alias Indexer.Fetcher.TokenInstance.{Helper, HelperTest, Realtime, RealtimeTest, Retry}
  alias Plug.Conn

  @address "0x5caebd3b32e210e85ce3e9d51638b9c445481567"
  @cid "bafybeig6nlmyzui7llhauc52j2xo5hoy4lzp6442lkve5wysdvjkizxonu"
  @failure "deliberate metadata assertion failure"

  setup do
    original =
      for {app, key} <- [
            {:explorer, :json_rpc_named_arguments},
            {:indexer, Helper},
            {:indexer, :ipfs},
            {:indexer, Retry},
            {:indexer, Realtime},
            {:tesla, :adapter}
          ] do
        {app, key, Application.fetch_env(app, key)}
      end

    on_exit(fn ->
      for {app, key, value} <- original do
        case value do
          {:ok, config} -> Application.put_env(app, key, config)
          :error -> Application.delete_env(app, key)
        end
      end
    end)

    bypass = Bypass.open()
    url = "http://localhost:#{bypass.port}"

    Application.put_env(:explorer, :json_rpc_named_arguments,
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        http_options: [],
        urls: [url <> "/rpc"]
      ]
    )

    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    {:ok, bypass: bypass, url: url}
  end

  test "015: base URI configuration survives a failed metadata assertion", %{bypass: bypass, url: url} do
    token = insert(:token, contract_address: build(:address, hash: @address), type: "ERC-721")
    original_helper = Application.get_env(:indexer, Helper)
    original_ipfs = Application.get_env(:indexer, :ipfs)

    transcript =
      rpc(bypass, [
        {token_data(777), {:error, %{code: -32015, data: "Reverted 0x", message: "execution reverted"}}},
        {"0x6c0360eb", {:ok, url <> "/base/"}}
      ])

    Bypass.expect_once(bypass, "GET", "/base/777", &Conn.resp(&1, 200, ~s({"name":"123"})))

    fail_after_fetch(&HelperTest.preserve_configuration/1, fn ->
      Application.put_env(:indexer, Helper, Keyword.put(original_helper, :base_uri_retry?, true))
      Application.put_env(:indexer, :ipfs, Keyword.put(original_ipfs, :gateway_url, url))

      assert [%Instance{metadata: %{"name" => "123"}}] =
               Helper.batch_fetch_instances([{token.contract_address_hash, 777}])
    end)

    assert Application.get_env(:indexer, Helper) == original_helper
    assert Application.get_env(:indexer, :ipfs) == original_ipfs
    assert persisted(token, 777).metadata == %{"name" => "123"}
    assert Agent.get(transcript, & &1) == []
  end

  test "015: IPFS credentials and full configuration survive a failed assertion", %{bypass: bypass, url: url} do
    token = insert(:token, contract_address: build(:address, hash: @address), type: "ERC-721")
    original_helper = Application.get_env(:indexer, Helper)
    original_ipfs = Application.get_env(:indexer, :ipfs)
    transcript = rpc(bypass, [{token_data(777), {:ok, "ipfs://" <> @cid}}])

    Bypass.expect_once(bypass, "GET", "/" <> @cid, fn conn ->
      conn = Conn.fetch_query_params(conn)
      assert conn.query_params == %{"secret_key" => "secret_value"}
      conn |> Conn.put_resp_content_type("image/jpg") |> Conn.resp(200, "img")
    end)

    fail_after_fetch(&HelperTest.preserve_configuration/1, fn ->
      Application.put_env(
        :indexer,
        :ipfs,
        Keyword.merge(original_ipfs,
          gateway_url: url,
          public_gateway_url: "https://ipfs.io/ipfs",
          gateway_url_param_key: "secret_key",
          gateway_url_param_value: "secret_value",
          gateway_url_param_location: :query
        )
      )

      assert [%Instance{metadata: %{"image" => image}}] =
               Helper.batch_fetch_instances([{token.contract_address_hash, 777}])

      assert image == "ipfs://" <> @cid
      refute image =~ "secret_key"
      refute image =~ "secret_value"
    end)

    assert Application.get_env(:indexer, Helper) == original_helper
    assert Application.get_env(:indexer, :ipfs) == original_ipfs
    assert persisted(token, 777).metadata == %{"image" => "ipfs://" <> @cid}
    assert Agent.get(transcript, & &1) == []
  end

  test "016: retry deadlines follow exponential backoff and its cap in both directions", %{bypass: bypass, url: url} do
    Application.put_env(
      :indexer,
      Retry,
      Keyword.merge(Application.get_env(:indexer, Retry),
        exp_timeout_coeff: 1,
        exp_timeout_base: 2,
        max_refetch_interval: 8_000
      )
    )

    token = insert(:token, contract_address: build(:address, hash: @address), type: "ERC-721")

    for {previous_count, id, delay} <- [{nil, 777, 1_000}, {0, 778, 2_000}, {1, 779, 4_000}, {1000, 780, 8_000}] do
      if previous_count do
        insert(:token_instance,
          token_contract_address_hash: token.contract_address_hash,
          token_id: id,
          metadata: nil,
          error: "request error: 429",
          retries_count: previous_count
        )
      end

      transcript = rpc(bypass, [{token_data(id), {:ok, url <> "/rate-limited/#{id}"}}])
      Bypass.expect_once(bypass, "GET", "/rate-limited/#{id}", &Conn.resp(&1, 429, "429 Too many requests"))
      assert :ok = Retry.run([%{contract_address_hash: token.contract_address_hash, token_id: id}], nil)
      instance = persisted(token, id)
      expected = DateTime.add(DateTime.utc_now(), delay, :millisecond)

      HelperTest.assert_refetch_after(expected, instance.refetch_after)
      assert instance.retries_count == if(is_nil(previous_count), do: 0, else: previous_count + 1)
      assert instance.error == "request error: 429"
      refute instance.is_banned
      assert is_nil(instance.metadata)
      assert Agent.get(transcript, & &1) == []

      for offset <- [-86_400, 86_400] do
        assert_raise ExUnit.AssertionError, fn ->
          HelperTest.assert_refetch_after(expected, DateTime.add(instance.refetch_after, offset, :second))
        end
      end
    end
  end

  test "055: realtime retry restores the original adapter after assertion failure", %{bypass: bypass, url: url} do
    original_adapter = {Tesla.Adapter.Mint, []}
    Application.put_env(:tesla, :adapter, original_adapter)

    Application.put_env(
      :indexer,
      Realtime,
      Keyword.merge(Application.get_env(:indexer, Realtime), retry_with_cooldown?: true, retry_timeout: 100)
    )

    start_supervised!({Realtime.Supervisor, []})
    token = insert(:token, contract_address: build(:address, hash: @address), type: "ERC-1155")

    insert(:token_instance,
      token_contract_address_hash: token.contract_address_hash,
      token_id: 777,
      metadata: nil,
      error: nil
    )

    data = "0x0e89341c" <> String.pad_leading("309", 64, "0")
    transcript = rpc(bypass, List.duplicate({data, {:ok, url <> "/api/card/{id}"}}, 2))
    path = "/api/card/" <> String.pad_leading("309", 64, "0")
    responses = start_supervised!({Agent, fn -> [{404, "Not found"}, {200, ~s({"name":"name"})}] end}, id: make_ref())

    Bypass.expect(bypass, "GET", path, fn conn ->
      {status, body} = Agent.get_and_update(responses, fn [head | tail] -> {head, tail} end)
      Conn.resp(conn, status, body)
    end)

    fail_after_fetch(&RealtimeTest.preserve_adapter/1, fn ->
      Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

      assert :ok =
               Realtime.async_fetch([
                 %{token_contract_address_hash: token.contract_address_hash, token_ids: [Decimal.new(777)]}
               ])

      instance = await_metadata(token, 777)
      assert instance.metadata == %{"name" => "name"}
      assert is_nil(instance.error)
      assert instance.retries_count == 2
    end)

    assert Application.get_env(:tesla, :adapter) == original_adapter
    assert persisted(token, 777).metadata == %{"name" => "name"}
    assert Agent.get(transcript, & &1) == []
    assert Agent.get(responses, & &1) == []
  end

  defp fail_after_fetch(preserve, fetch) do
    parent = self()

    {pid, monitor} =
      spawn_monitor(fn ->
        ExUnit.OnExitHandler.register(self())
        preserve.(nil)

        try do
          fetch.()
          flunk(@failure)
        rescue
          error in ExUnit.AssertionError -> send(parent, {:assertion_failure, self(), error.message})
        end
      end)

    assert_receive {:assertion_failure, ^pid, @failure}, 5_000
    assert_receive {:DOWN, ^monitor, :process, ^pid, :normal}, 5_000
    assert :ok = ExUnit.OnExitHandler.run(pid, 5_000)
  end

  defp rpc(bypass, transcript) do
    recorded = start_supervised!({Agent, fn -> transcript end}, id: make_ref())

    Bypass.expect(bypass, "POST", "/rpc", fn conn ->
      {data, response} = Agent.get_and_update(recorded, fn [head | tail] -> {head, tail} end)
      {:ok, body, conn} = Conn.read_body(conn)
      [request] = Jason.decode!(body)

      assert %{"jsonrpc" => "2.0", "method" => "eth_call", "params" => [%{"to" => @address, "data" => ^data}, "latest"]} =
               request

      result =
        case response do
          {:ok, uri} -> %{result: "0x" <> Base.encode16(ABI.TypeEncoder.encode([uri], [:string]), case: :lower)}
          {:error, error} -> %{error: error}
        end

      reply = Map.merge(result, %{jsonrpc: "2.0", id: request["id"]})
      conn |> Conn.put_resp_content_type("application/json") |> Conn.resp(200, Jason.encode!([reply]))
    end)

    recorded
  end

  defp token_data(id), do: "0xc87b56dd" <> String.pad_leading(Integer.to_string(id, 16) |> String.downcase(), 64, "0")

  defp persisted(token, id), do: id |> Instance.token_instance_query(token.contract_address_hash) |> Repo.one!()

  defp await_metadata(token, id, remaining \\ 100) do
    instance = persisted(token, id)

    if instance.metadata do
      instance
    else
      assert remaining > 0, "realtime retry did not persist metadata"
      Process.sleep(10)
      await_metadata(token, id, remaining - 1)
    end
  end
end
