defmodule BlockScoutWeb.Repair22_33_394Test do
  @moduledoc """
  Repair record 22.33.394 for audit 22.30.142.
  8: "finding 8: missing JSON and unreadable uploads return explicit errors"
  and "finding 8: acceptance requires a persisted job and an unavailable queue returns an error".
  9: "finding 9: counters reject malformed input before querying and preserve ranks".
  11: "finding 11: topic search applies restrictions and carries authorization through paging".
  12: "finding 12: write settings govern verified and custom ABI pages".
  24: "finding 24: regular and custom AJAX functions and reads enforce address access".
  25: "finding 25: counts match the selected regular custom and proxy ABI before reading".
  26: "finding 26: twelve indexed elements retain numeric order through HTTP and ABI decoding".
  33: "finding 33: mounted verified search filters and pages real persisted contracts".

  Chain addresses and transaction fields use the committed transaction importer
  transcript. Contracts use the committed SimpleStorage fixture; balanceOfBatch
  uses the ERC1155 interface in fixture/smart_contract/issue_4758.sol. The loopback
  peer receives production Reader/HTTP/ABI encoded eth_call requests and emits
  standard JSON-RPC responses. Requests traverse WebRouter and real persistence.
  No component under repair or transport is replaced.
  Baseline verification was not run: the explicit ordered bounded gates require
  implementation before hf-build and hf-done, and forbid a manual verify run.
  No baseline failures are claimed. The task/spec/gotcha ledgers are outside the
  writable scope, so this module carries the repair record and gate evidence is
  recorded by hf-done.
  First build at 2e459e82a2b641c572e4c13d36004de51f69539a failed with exit 1
  (services/explorer/deploy/tools/mix-in-builder.sh compile, /out/logs/build-1.log):
  Phoenix names search route helpers after their controller. Paging now uses
  verified_contracts_path and address_logs_path with the declared search action.
  First verify at 946fd966f8faf2ec69329b21352ba1fd0aa0403a failed with exit 1
  (cg spec done 22.33.394 -f layerx-merge --root /work/paxeer-monorepo,
  /out/logs/verify-1.log): the ranking fixture compared a raw hash to checksum HTML
  and the custom-ABI address lacked a persisted proxy snapshot. Those fixtures
  are completed with all assertions retained; the upload also traverses the real
  POST parser and awaits its notification, with the peer's version endpoint supplied.
  Continuation verify at 5bdcf85c6c6c05616e48730a44e722db00e882b5 failed with exit 1
  (cg spec done 22.33.394 -f layerx-merge --root /work/paxeer-monorepo,
  /out/logs/verify-1.log): eight tests passed; the custom write-page fixture
  inherited verified bytecode through twin resolution. It now uses the committed
  modern-compiler SimpleStorage bytecode to exercise the unverified custom branch.
  Wiring request: domain 14 (explorer web) must preserve the authorized key in
  services/explorer/backend/apps/block_scout_web/assets/js/pages/address/logs.js
  when constructing the initial /search-logs request; controller paging retains it.
  """

  use BlockScoutWeb.ConnCase, async: false

  alias BlockScoutWeb.Account.AuthController
  alias BlockScoutWeb.AddressContractVerificationController
  alias Explorer.Account.CustomABI
  alias Explorer.Chain.Address
  alias Explorer.Repo

  @endpoint BlockScoutWeb.Routers.WebRouter
  @key "repair+key&22=33?394"
  @topic "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
  @recording Path.expand(
               "../../../explorer/test/support/fixture/vcr_cassettes/transaction_importer_binds_internal_transactions.json",
               __DIR__
             )

  defmodule Peer do
    import Plug.Conn
    def init(owner), do: owner

    def call(conn, owner) do
      {:ok, body, conn} = read_body(conn)
      requests = Jason.decode!(body)

      responses =
        Enum.map(List.wrap(requests), fn request ->
          send(owner, {:rpc, request})
          %{"method" => "eth_call", "params" => [%{"data" => data}, "latest"]} = request

          result =
            if String.starts_with?(data, "0x4e1273f4") do
              ABI.TypeEncoder.encode([Enum.to_list(101..112)], [{:array, {:uint, 256}}])
            else
              ABI.TypeEncoder.encode([42], [{:uint, 256}])
            end

          %{jsonrpc: "2.0", id: request["id"], result: "0x" <> Base.encode16(result, case: :lower)}
        end)

      response = if is_list(requests), do: responses, else: hd(responses)
      conn |> put_resp_content_type("application/json") |> send_resp(200, Jason.encode!(response))
    end
  end

  defmodule VerifierPeer do
    import Plug.Conn
    def init(owner), do: owner

    def call(%{method: "GET", request_path: "/api/v2/verifier/solidity/versions"} = conn, _owner) do
      conn
      |> put_resp_content_type("application/json")
      |> send_resp(200, Jason.encode!(%{compilerVersions: ["v0.4.24+commit.e67f0147"]}))
    end

    def call(conn, owner) do
      {:ok, body, conn} = read_body(conn)
      send(owner, {:verification, conn.request_path, Jason.decode!(body)})

      conn
      |> put_resp_content_type("application/json")
      |> send_resp(200, Jason.encode!(%{status: "FAILURE", message: "Recorded compiler input rejected"}))
    end
  end

  setup %{conn: conn} do
    for {app, key} <- [
          {:block_scout_web, :contract},
          {:explorer, Explorer.Account},
          {:explorer, :addresses_blacklist},
          {:explorer, :addresses_blacklist_key},
          {:explorer, :json_rpc_named_arguments},
          {:explorer, :http_client},
          {:explorer, Explorer.SmartContract.RustVerifierInterfaceBehaviour}
        ] do
      previous = Application.fetch_env(app, key)
      on_exit(fn -> restore(app, key, previous) end)
    end

    Application.put_env(:explorer, :addresses_blacklist, "")
    Application.put_env(:explorer, :addresses_blacklist_key, @key)
    account = Application.get_env(:explorer, Explorer.Account, [])
    Application.put_env(:explorer, Explorer.Account, Keyword.put(account, :enabled, true))
    interaction(false)

    [recording] = @recording |> File.read!() |> Jason.decode!()
    %{"method" => "eth_getTransactionByHash", "params" => [hash]} = Jason.decode!(recording["request"]["body"])
    %{"result" => transaction} = Jason.decode!(recording["response"]["body"])
    assert transaction["hash"] == hash

    conn =
      conn
      |> Plug.Test.init_test_session(%{})
      |> put_private(:phoenix_endpoint, BlockScoutWeb.Endpoint)
      |> Map.put(:secret_key_base, BlockScoutWeb.Endpoint.config(:secret_key_base))

    {:ok, conn: conn, recorded: transaction}
  end

  test "finding 8: missing JSON and unreadable uploads return explicit errors", %{conn: conn, recorded: recorded} do
    params = %{"smart_contract" => %{"address_hash" => recorded["to"]}, "verification_type" => "json:standard"}

    for files <- [
          nil,
          %{},
          %{"0" => %Plug.Upload{path: __ENV__.file, filename: "source.sol", content_type: "text/plain"}}
        ] do
      input = if is_nil(files), do: params, else: Map.put(params, "file", files)
      response = AddressContractVerificationController.create(conn, input)
      assert Phoenix.ConnTest.json_response(response, 422) == %{"error" => "Please attach a JSON compiler input file."}
    end

    for path <- [__ENV__.file <> ".missing", __DIR__] do
      file = %Plug.Upload{path: path, filename: "input.json", content_type: "application/json"}
      response = AddressContractVerificationController.create(conn, Map.put(params, "file", %{"0" => file}))
      assert Phoenix.ConnTest.json_response(response, 422) == %{"error" => "Unable to read JSON compiler input file."}
    end
  end

  test "finding 8: acceptance requires a persisted job and an unavailable queue returns an error", %{conn: conn} do
    contract = simple_storage()
    ref = make_ref()
    start_supervised!({Plug.Cowboy, scheme: :http, plug: {VerifierPeer, self()}, options: [port: 0, ref: ref]})
    url = "http://127.0.0.1:#{:ranch.get_port(ref)}"
    Application.put_env(:explorer, :http_client, HTTPoison)

    Application.put_env(:explorer, Explorer.SmartContract.RustVerifierInterfaceBehaviour,
      enabled: true,
      service_url: url
    )

    path = Path.expand("../../../explorer/test/support/fixture/smart_contract/standard_input.json", __DIR__)
    upload = %Plug.Upload{path: path, filename: "input.json", content_type: "application/json"}

    params = %{
      "smart_contract" => %{
        "address_hash" => to_string(contract.address_hash),
        "compiler_version" => contract.compiler_version,
        "name" => contract.name
      },
      "verification_type" => "json:standard",
      "file" => %{"0" => upload}
    }

    worker = Explorer.SmartContract.Solidity.PublisherWorker
    before_ids = Que.Persistence.all(worker) |> Enum.map(& &1.id)
    topic = "addresses_old:" <> to_string(contract.address_hash)
    :ok = BlockScoutWeb.Endpoint.subscribe(topic)
    on_exit(fn -> BlockScoutWeb.Endpoint.unsubscribe(topic) end)

    response =
      Phoenix.ConnTest.dispatch(
        conn,
        BlockScoutWeb.Endpoint,
        :post,
        "/verify_smart_contract/contract_verifications",
        params
      )

    assert response.status == 204
    [job] = Enum.reject(Que.Persistence.all(worker), &(&1.id in before_ids))
    {"json_web", queued_contract, queued_input, queued_conn} = job.arguments
    assert queued_contract == params["smart_contract"]
    assert queued_input == File.read!(path)
    assert queued_conn.params["verification_type"] == "json:standard"
    assert_receive {:verification, request_path, request}, 5_000
    assert URI.decode(request_path) == "/api/v2/verifier/solidity/sources:verify-standard-json"
    assert request["input"] == File.read!(path)
    assert request["bytecode"] == to_string(Repo.get!(Address, contract.address_hash).contract_code)
    assert request["metadata"]["contractAddress"] == to_string(contract.address_hash)
    assert_job_completed(job.id, 100)

    assert_receive %Phoenix.Socket.Broadcast{
                     topic: ^topic,
                     event: "verification_result",
                     payload: %{result: {:error, rendered_error}}
                   },
                   5_000

    assert rendered_error =~ "Recorded compiler input rejected"
    assert rendered_error =~ "Standard input JSON"
    Que.Persistence.destroy(job.id)

    :ok = Application.stop(:que)
    on_exit(fn -> Application.ensure_all_started(:que) end)
    unavailable = AddressContractVerificationController.create(conn, params)
    assert Phoenix.ConnTest.json_response(unavailable, 503) == %{"error" => "Unable to queue verification."}
    {:ok, _} = Application.ensure_all_started(:que)
  end

  test "finding 9: counters reject malformed input before querying and preserve ranks", %{
    conn: conn,
    recorded: recorded
  } do
    address = insert(:address, hash: recorded["from"], fetched_coin_balance: 1_000_000_000_000_000_000)
    observe_queries()

    for value <- ["abc", "1tail", "-1", "", "9223372036854775808", ["1"], %{"n" => "1"}] do
      assert conn |> get("/accounts", %{"type" => "JSON", "items_count" => value}) |> html_response(422)
      refute_received {:query, :addresses}
    end

    for {count, rank} <- [{nil, "1"}, {"0", "1"}, {"50", "51"}] do
      params = %{"type" => "JSON"}
      params = if count, do: Map.put(params, "items_count", count), else: params
      body = conn |> get("/accounts", params) |> json_response(200)
      assert_received {:query, :addresses}
      item = Enum.find(body["items"], &String.contains?(&1, Address.checksum(address.hash)))
      assert item

      assert item
             |> Floki.parse_fragment!()
             |> Floki.find("td:first-child > span.color-lighten")
             |> Floki.text()
             |> String.trim() == rank
    end
  end

  test "finding 11: topic search applies restrictions and carries authorization through paging", %{
    conn: conn,
    recorded: recorded
  } do
    transaction = persist_transaction(recorded)
    contract = insert(:smart_contract, address_hash: transaction.to_address_hash, abi: transfer_abi())
    insert(:proxy_implementation, proxy_address_hash: contract.address_hash, address_hashes: [], names: [])
    address = Repo.get!(Address, transaction.to_address_hash)

    for index <- 0..50 do
      insert(:log,
        address: address,
        transaction: transaction,
        block: Repo.get!(Explorer.Chain.Block, transaction.block_hash),
        block_number: transaction.block_number,
        index: index,
        first_topic: @topic,
        second_topic: padded_address(recorded["from"]),
        third_topic: padded_address(recorded["to"]),
        data: "0x" <> String.duplicate("0", 63) <> "1"
      )
    end

    hash = Address.checksum(address.hash)
    Application.put_env(:explorer, :addresses_blacklist, String.upcase(hash))
    observe_queries()
    params = %{"topic" => @topic, "address_id" => hash}

    for query <- [params, Map.put(params, "key", "wrong")] do
      assert conn |> get("/search-logs", query) |> html_response(404)
      refute_received {:query, :logs}
    end

    body = conn |> get("/search-logs", Map.put(params, "key", @key)) |> json_response(200)
    assert length(body["items"]) == 50
    assert Enum.all?(body["items"], &String.contains?(&1, @topic))
    uri = URI.parse(body["next_page_path"])
    assert uri.path == "/search-logs"
    assert URI.decode_query(uri.query)["key"] == @key
    assert URI.decode_query(uri.query)["topic"] == @topic
    assert URI.decode_query(uri.query)["address_id"] == hash
    next = conn |> get(body["next_page_path"]) |> json_response(200)
    assert length(next["items"]) == 1
    assert next["next_page_path"] == nil
    unauthorized = URI.decode_query(uri.query) |> Map.delete("key")
    assert conn |> get(uri.path, unauthorized) |> html_response(404)
  end

  test "finding 12: write settings govern verified and custom ABI pages", %{conn: conn} do
    contract = simple_storage()
    unverified = insert(:contract_address, contract_code: contract_code_info_modern_compiler().bytecode)
    insert(:proxy_implementation, proxy_address_hash: unverified.hash, address_hashes: [], names: [])
    custom_conn = custom_session(conn, unverified.hash, contract.abi)

    for disabled <- [true, false] do
      interaction(disabled)

      for {request_conn, hash, custom?} <- [
            {conn, contract.address_hash, false},
            {custom_conn, contract.address_hash, false},
            {custom_conn, unverified.hash, true}
          ] do
        response = get(request_conn, "/address/#{Address.checksum(hash)}/write-contract")

        if disabled do
          assert html_response(response, 404)
          refute response.resp_body =~ "data-smart-contract-functions"
        else
          assert html_response(response, 200) =~ ~s(data-action="write")
          assert response.assigns.non_custom_abi == !custom?
        end
      end

      assert conn |> get("/address/#{Address.checksum(unverified.hash)}/write-contract") |> html_response(404)
    end
  end

  test "finding 24: regular and custom AJAX functions and reads enforce address access", %{conn: conn} do
    rpc_peer()
    contract = simple_storage()
    hash = Address.checksum(contract.address_hash)
    conn = custom_session(conn, contract.address_hash, contract.abi)
    Application.put_env(:explorer, :addresses_blacklist, String.upcase(hash))
    observe_queries()

    for custom? <- ["false", "true"] do
      index = %{"hash" => hash, "type" => "regular", "action" => "read", "is_custom_abi" => custom?}

      show = %{
        "type" => "regular",
        "is_custom_abi" => custom?,
        "method_id" => "6d4ce63c",
        "function_name" => "get",
        "args_count" => "0"
      }

      for key <- [nil, "wrong"] do
        for {path, params} <- [{"/smart-contracts", index}, {"/smart-contracts/#{hash}", show}] do
          params = if key, do: Map.put(params, "key", key), else: params
          assert ajax(conn, path, params) |> html_response(404)
          refute_received {:rpc, _}
          refute_received {:query, :smart_contracts}
          refute_received {:query, :account_custom_abis}
        end
      end

      listing = ajax(conn, "/smart-contracts", Map.put(index, "key", @key))
      assert html_response(listing, 200) =~ "get"
      assert Map.get(listing.assigns, :custom_abi, false) == (custom? == "true")
      assert_receive {:rpc, %{"method" => "eth_call"}}
      response = ajax(conn, "/smart-contracts/#{hash}", Map.put(show, "key", @key))
      assert html_response(response, 200)
      assert response.assigns.outputs == [%{"type" => "uint256", "value" => 42}]
      assert_receive {:rpc, %{"method" => "eth_call"}}
      flush_queries()
    end
  end

  test "finding 25: counts match the selected regular custom and proxy ABI before reading", %{conn: conn} do
    rpc_peer()
    contract = simple_storage()
    hash = Address.checksum(contract.address_hash)
    custom_conn = custom_session(conn, contract.address_hash, contract.abi)
    proxy = simple_storage()
    proxy |> Ecto.Changeset.change(abi: []) |> Repo.update!()

    Repo.get_by!(Explorer.Chain.SmartContract.Proxy.Models.Implementation, proxy_address_hash: proxy.address_hash)
    |> Ecto.Changeset.change(address_hashes: [contract.address_hash], names: [contract.name])
    |> Repo.update!()

    for {request_conn, target, extra} <- [
          {conn, hash, %{"type" => "regular"}},
          {custom_conn, hash, %{"type" => "regular", "is_custom_abi" => "true"}},
          {conn, Address.checksum(proxy.address_hash), %{"type" => "proxy"}}
        ] do
      params = Map.merge(extra, %{"method_id" => "6d4ce63c", "function_name" => "get"})

      for count <- ["abc", "0trailing", "-1", "1", "1000000000", String.duplicate("9", 100), ["0"]] do
        assert ajax(request_conn, "/smart-contracts/#{target}", Map.put(params, "args_count", count))
               |> html_response(422)

        refute_received {:rpc, _}
      end

      response = ajax(request_conn, "/smart-contracts/#{target}", Map.put(params, "args_count", "0"))
      assert html_response(response, 200)
      assert response.assigns.outputs == [%{"type" => "uint256", "value" => 42}]
      assert_receive {:rpc, %{"method" => "eth_call"}}
    end

    # The stored custom ABI has a different arity from the verified contract.
    batch_conn = custom_session(conn, contract.address_hash, batch_abi())

    batch_params = %{
      "is_custom_abi" => "true",
      "method_id" => "4e1273f4",
      "function_name" => "balanceOfBatch",
      "arg_0" => %{"0" => hash},
      "arg_1" => %{"0" => "1"}
    }

    for count <- ["0", "1", "3", "2tail"] do
      assert ajax(batch_conn, "/smart-contracts/#{hash}", Map.put(batch_params, "args_count", count))
             |> html_response(422)

      refute_received {:rpc, _}
    end

    batch_response = ajax(batch_conn, "/smart-contracts/#{hash}", Map.put(batch_params, "args_count", "2"))
    assert html_response(batch_response, 200)
    assert batch_response.assigns.outputs == [%{"type" => "uint256[]", "value" => Enum.to_list(101..112)}]
    assert_receive {:rpc, %{"method" => "eth_call"}}

    bad_method = %{"method_id" => "ffffffff", "args_count" => "0"}
    assert ajax(conn, "/smart-contracts/#{hash}", bad_method) |> html_response(422)
    refute_received {:rpc, _}
  end

  test "finding 26: twelve indexed elements retain numeric order through HTTP and ABI decoding", %{
    conn: conn,
    recorded: recorded
  } do
    rpc_peer()
    contract = insert(:smart_contract, abi: batch_abi())
    hash = Address.checksum(contract.address_hash)
    accounts = Enum.map(0..11, fn index -> if rem(index, 2) == 0, do: recorded["from"], else: recorded["to"] end)
    ids = Enum.to_list(1..12)

    indexed = fn values ->
      values |> Enum.with_index() |> Map.new(fn {value, index} -> {to_string(index), to_string(value)} end)
    end

    params = %{
      "method_id" => "4e1273f4",
      "function_name" => "balanceOfBatch",
      "args_count" => "2",
      "arg_0" => indexed.(accounts),
      "arg_1" => indexed.(ids)
    }

    for args <- [
          params,
          params
          |> Map.drop(["args_count", "arg_0", "arg_1"])
          |> Map.put("args", %{"0" => indexed.(accounts), "1" => indexed.(ids)})
        ] do
      response = ajax(conn, "/smart-contracts/#{hash}", args)
      assert html_response(response, 200)
      assert response.assigns.outputs == [%{"type" => "uint256[]", "value" => Enum.to_list(101..112)}]

      assert_receive {:rpc,
                      %{
                        "method" => "eth_call",
                        "params" => [%{"data" => "0x4e1273f4" <> encoded, "to" => target}, "latest"]
                      }}

      assert String.downcase(target) == String.downcase(hash)
      payload = Base.decode16!(encoded, case: :mixed)
      [actual_accounts, actual_ids] = ABI.TypeDecoder.decode(payload, [{:array, :address}, {:array, {:uint, 256}}])
      assert actual_ids == ids
      assert actual_accounts == Enum.map(accounts, fn "0x" <> hex -> Base.decode16!(hex, case: :mixed) end)
    end

    for invalid <- [%{"0" => "1", "2" => "3"}, %{"-1" => "1"}, %{"one" => "1"}, %{"0" => "1", "00" => "2"}] do
      assert ajax(conn, "/smart-contracts/#{hash}", Map.put(params, "arg_1", invalid)) |> html_response(422)
      refute_received {:rpc, _}
    end
  end

  test "finding 33: mounted verified search filters and pages real persisted contracts", %{conn: conn} do
    contracts =
      insert_list(51, :smart_contract, name: "Repair394Storage", language: :solidity, is_vyper_contract: false)

    insert(:smart_contract, name: "OtherContract", language: :solidity)
    insert(:smart_contract, name: "Repair394Storage", language: :vyper, is_vyper_contract: true)
    response = get(conn, "/search-verified-contracts", %{"q" => "Repair394Storage", "filter" => "solidity"})
    assert response.private.phoenix_action == :search_verified_contracts
    body = json_response(response, 200)
    assert length(body["items"]) == 50
    uri = URI.parse(body["next_page_path"])
    assert URI.decode_query(uri.query)["q"] == "Repair394Storage"
    assert URI.decode_query(uri.query)["filter"] == "solidity"
    assert uri.path == "/search-verified-contracts"
    next = conn |> get(body["next_page_path"]) |> json_response(200)
    assert length(next["items"]) == 1
    assert next["next_page_path"] == nil
    returned = Enum.join(body["items"] ++ next["items"])
    for contract <- contracts, do: assert(returned =~ to_string(contract.address_hash))

    exact =
      conn
      |> get("/search-verified-contracts", %{"search" => to_string(hd(contracts).address_hash)})
      |> json_response(200)

    assert length(exact["items"]) == 1
    assert hd(exact["items"]) =~ to_string(hd(contracts).address_hash)
  end

  defp assert_job_completed(id, attempts) do
    case Que.Persistence.find(id) do
      %{status: :completed} ->
        :ok

      job when attempts == 0 ->
        flunk("verification job did not complete: #{inspect(job)}")

      _ ->
        Process.sleep(10)
        assert_job_completed(id, attempts - 1)
    end
  end

  defp simple_storage do
    contract = insert(:smart_contract, language: :solidity, is_vyper_contract: false)
    insert(:proxy_implementation, proxy_address_hash: contract.address_hash, address_hashes: [], names: [])
    contract
  end

  defp custom_session(conn, hash, abi) do
    logged_in =
      conn
      |> Phoenix.Controller.fetch_flash([])
      |> assign(:ueberauth_auth, build(:auth))
      |> AuthController.callback(%{})

    user = get_session(logged_in, :current_user)
    assert user.email_verified
    attrs = %{identity_id: user.id, address_hash: to_string(hash), name: "Recorded custom ABI", abi: Jason.encode!(abi)}
    custom_abi = %CustomABI{} |> CustomABI.changeset(attrs) |> Repo.account_repo().insert!()
    assert custom_abi.abi == abi
    Plug.Test.init_test_session(conn, current_user: user)
  end

  defp rpc_peer do
    ref = make_ref()
    start_supervised!({Plug.Cowboy, scheme: :http, plug: {Peer, self()}, options: [port: 0, ref: ref]})
    url = "http://127.0.0.1:#{:ranch.get_port(ref)}"

    Application.put_env(:explorer, :json_rpc_named_arguments,
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        http_options: [timeout: 2_000, recv_timeout: 2_000],
        urls: [url]
      ]
    )
  end

  defp ajax(conn, path, params), do: conn |> put_req_header("x-requested-with", "xmlhttprequest") |> get(path, params)

  defp interaction(disabled) do
    config = Application.get_env(:block_scout_web, :contract, [])
    Application.put_env(:block_scout_web, :contract, Keyword.put(config, :disable_interaction, disabled))
  end

  defp restore(app, key, {:ok, value}), do: Application.put_env(app, key, value)
  defp restore(app, key, :error), do: Application.delete_env(app, key)

  def record_query(_event, _measurements, metadata, owner) do
    if self() == owner do
      for table <- [:addresses, :logs, :smart_contracts, :account_custom_abis] do
        if String.contains?(metadata.query, ~s(FROM "#{table}")), do: send(owner, {:query, table})
      end
    end
  end

  defp observe_queries do
    for repo <- [Repo, Repo.Account] do
      id = {__MODULE__, repo, self()}
      prefix = repo.config()[:telemetry_prefix] || [:explorer, :repo]
      :ok = :telemetry.attach(id, prefix ++ [:query], &__MODULE__.record_query/4, self())
      on_exit(fn -> :telemetry.detach(id) end)
    end
  end

  defp flush_queries do
    receive do
      {:query, _} -> flush_queries()
    after
      0 -> :ok
    end
  end

  defp persist_transaction(recorded) do
    params = recorded |> EthereumJSONRPC.Transaction.to_elixir() |> EthereumJSONRPC.Transaction.elixir_to_params()
    insert(:address, hash: params.from_address_hash)
    insert(:address, hash: params.to_address_hash)
    block = insert(:block, hash: params.block_hash, number: params.block_number)

    params =
      Map.merge(params, %{
        block_consensus: true,
        block_timestamp: block.timestamp,
        status: :ok,
        gas_used: 21_000,
        cumulative_gas_used: 21_000
      })

    %Explorer.Chain.Transaction{} |> Explorer.Chain.Transaction.changeset(params) |> Repo.insert!()
  end

  defp padded_address("0x" <> address), do: "0x" <> String.duplicate("0", 24) <> address

  defp transfer_abi do
    [
      %{
        "type" => "event",
        "name" => "Transfer",
        "anonymous" => false,
        "inputs" => [
          %{"name" => "from", "type" => "address", "indexed" => true},
          %{"name" => "to", "type" => "address", "indexed" => true},
          %{"name" => "value", "type" => "uint256", "indexed" => false}
        ]
      }
    ]
  end

  defp batch_abi do
    [
      %{
        "type" => "function",
        "name" => "balanceOfBatch",
        "constant" => true,
        "stateMutability" => "view",
        "inputs" => [
          %{"name" => "accounts", "type" => "address[]"},
          %{"name" => "ids", "type" => "uint256[]"}
        ],
        "outputs" => [%{"name" => "", "type" => "uint256[]"}]
      }
    ]
  end
end
