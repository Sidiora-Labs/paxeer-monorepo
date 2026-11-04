defmodule Explorer.Repair.Repair2233390Test do
  use ExUnit.Case, async: false

  import ExUnit.CaptureLog

  alias Explorer.Chain
  alias Explorer.Chain.Fetcher.AddressesBlacklist
  alias Explorer.Chain.Fetcher.AddressesBlacklist.Blockaid

  @ofac "0x8576acc5c05d6ce88f4e49bf65bdf0c62f91353c"
  @malicious "0x098b716b8aaf21512996dc57eb0615e2383e2f96"
  @replacement "0x910cbd523d972eb0a6f4cae4618ad62622b39dbf"
  @snapshot ~s({"OFAC":["#{@ofac}"],"Malicious":["#{@malicious}"]})
  @recovered ~s({"OFAC":["#{@replacement}"],"Malicious":[]})
  @empty ~s({"OFAC":[],"Malicious":[]})

  setup do
    blacklist_config = Application.fetch_env!(:explorer, AddressesBlacklist)
    tesla_adapter = Application.fetch_env!(:tesla, :adapter)
    logger_level = Logger.level()
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    Logger.configure(level: :info)

    on_exit(fn ->
      Application.put_env(:explorer, AddressesBlacklist, blacklist_config)
      Application.put_env(:tesla, :adapter, tesla_adapter)
      Logger.configure(level: logger_level)
    end)

    :ok
  end

  for {name, status, body, reason} <- [
        {"HTTP outage", 503, ~s({"error":"service unavailable"}), {:http_status, 503}},
        {"malformed JSON", 200, "{\"OFAC\":", :invalid_json},
        {"non-object JSON", 200, "[]", :invalid_schema},
        {"missing categories", 200, ~s({"error":"temporarily unavailable"}), :invalid_schema},
        {"null category", 200, ~s({"OFAC":null,"Malicious":[]}), :invalid_schema},
        {"wrong category type", 200, ~s({"OFAC":{},"Malicious":[]}), :invalid_schema},
        {"invalid address", 200, ~s({"OFAC":["not-an-address"],"Malicious":[]}), :invalid_schema},
        {"non-string address", 200, ~s({"OFAC":[123],"Malicious":[]}), :invalid_schema}
      ] do
    @failure {status, body, reason}

    test "blacklist-outage-clears-cache retains the snapshot after #{name} and retries to recovery" do
      {status, body, reason} = @failure
      owner = self()
      server = Bypass.open()
      requests = start_supervised!({Agent, fn -> 0 end})
      responses = [{200, @snapshot}, {status, body}, {200, @recovered}, {200, @empty}]

      Bypass.expect(server, "GET", "/blacklist.json", fn conn ->
        index = Agent.get_and_update(requests, fn index -> {index, index + 1} end)
        assert conn.query_string == ""
        assert index < length(responses), "Unexpected additional blacklist request"
        send(owner, {:blacklist_request, index, self()})

        receive do
          :respond -> :ok
        after
          5_000 -> flunk("Test did not release blacklist response #{index}")
        end

        {response_status, response_body} = Enum.fetch!(responses, index)

        conn
        |> Plug.Conn.put_resp_content_type("application/json")
        |> Plug.Conn.resp(response_status, response_body)
      end)

      Application.put_env(:explorer, AddressesBlacklist,
        url: "http://127.0.0.1:#{server.port}/blacklist.json",
        enabled: true,
        provider: "blockaid",
        update_interval: 60_000,
        retry_interval: 50
      )

      ofac = Chain.string_to_address_hash_or_nil(@ofac)
      malicious = Chain.string_to_address_hash_or_nil(@malicious)
      replacement = Chain.string_to_address_hash_or_nil(@replacement)
      refute is_nil(ofac)
      refute is_nil(malicious)
      refute is_nil(replacement)

      log =
        capture_log(fn ->
          fetcher = start_supervised!(AddressesBlacklist)
          respond_to_request(0)

          await(fn -> AddressesBlacklist.blacklisted?(ofac) && AddressesBlacklist.blacklisted?(malicious) end)
          refute AddressesBlacklist.blacklisted?(replacement)
          snapshot = :ets.tab2list(:addresses_blacklist) |> MapSet.new()
          assert snapshot == MapSet.new([{ofac, nil}, {malicious, nil}])

          GenServer.cast(fetcher, :fetch)
          assert_receive {:blacklist_request, 1, handler}, 5_000
          {:monitors, monitors} = Process.info(fetcher, :monitors)
          [{:process, task}] = Enum.filter(monitors, &match?({:process, pid} when is_pid(pid), &1))
          monitor = Process.monitor(task)
          send(handler, :respond)

          assert_receive {:DOWN, ^monitor, :process, ^task, {%Blockaid.FetchError{reason: ^reason}, _stack}}, 5_000
          assert Process.alive?(fetcher)
          assert MapSet.new(:ets.tab2list(:addresses_blacklist)) == snapshot
          assert AddressesBlacklist.blacklisted?(ofac)
          assert AddressesBlacklist.blacklisted?(malicious)

          assert_receive {:blacklist_request, 2, recovery_handler}, 5_000
          assert MapSet.new(:ets.tab2list(:addresses_blacklist)) == snapshot
          send(recovery_handler, :respond)

          await(fn ->
            AddressesBlacklist.blacklisted?(replacement) && !AddressesBlacklist.blacklisted?(ofac) &&
              !AddressesBlacklist.blacklisted?(malicious)
          end)

          GenServer.cast(fetcher, :fetch)
          respond_to_request(3)
          await(fn -> :ets.info(:addresses_blacklist, :size) == 0 end)
          refute AddressesBlacklist.blacklisted?(replacement)
          assert Agent.get(requests, & &1) == 4
          stop_supervised!(AddressesBlacklist)
        end)

      assert log =~ "status=stale_or_unavailable"
      assert log =~ "retaining last successful snapshot"
      assert log =~ "retrying after 50ms"
      assert log =~ "reason=#{inspect(reason)}"
      assert log =~ "status=fresh"
    end
  end

  test "blacklist-outage-clears-cache reports a transport failure explicitly" do
    server = Bypass.open()
    Bypass.down(server)

    Application.put_env(:explorer, AddressesBlacklist,
      url: "http://127.0.0.1:#{server.port}/blacklist.json",
      retry_interval: 50
    )

    assert {:error, :transport} = Blockaid.fetch_result()
  end

  defp respond_to_request(index) do
    assert_receive {:blacklist_request, ^index, handler}, 5_000
    send(handler, :respond)
  end

  defp await(condition, attempts \\ 500)
  defp await(condition, 0), do: assert(condition.(), "Blacklist cache did not reach the expected state")

  defp await(condition, attempts) do
    unless condition.() do
      Process.sleep(10)
      await(condition, attempts - 1)
    end
  end
end
