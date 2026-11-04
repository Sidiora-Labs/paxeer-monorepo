defmodule BlockScoutWeb.Repair22_33_399Test do
  @moduledoc """
  Finding 22: real key controller -> certificate-verified HTTPS -> real import
  controller -> production SQL importer. The relay transaction and signing key
  are recordings used by the existing Optimism controller/queue tests. Only the
  optional Optimism table is supplied as a transaction-scoped store. No adapter
  or component under repair is replaced. Cache restarts use its OTP supervisor.
  """
  use BlockScoutWeb.ConnCase, async: false

  alias BlockScoutWeb.API.V2.OptimismController
  alias Ecto.Adapters.SQL
  alias Ecto.Adapters.SQL.Sandbox
  alias Explorer.Chain.Cache.ChainId
  alias Explorer.Chain.Optimism.InteropMessage
  alias Explorer.Repo
  alias Indexer.Fetcher.Optimism.Interop.MessageQueue

  @old_key <<51, 169, 186, 160, 251, 109, 12, 35, 225, 13, 110, 62, 216, 253, 27, 181, 187, 222, 222, 75, 79, 84, 185,
             24, 245, 213, 28, 21, 76, 179, 162, 16>>
  @new_key <<2::256>>
  @transaction "0xf463ce43ac52251f60f437a0346b7168874e6eb13a9689d7efde4ce907826897"

  defmodule Peer do
    @moduledoc false
    alias BlockScoutWeb.API.V2.{OptimismController, OptimismView}
    alias Ecto.Adapters.SQL.Sandbox
    alias Explorer.Repo

    def init(owner), do: owner

    def call(conn, owner) do
      conn =
        Plug.Parsers.call(conn, Plug.Parsers.init(parsers: [:json], json_decoder: Jason, pass: ["application/json"]))

      conn = conn |> Phoenix.Controller.put_view(OptimismView) |> Phoenix.Controller.put_format("json")

      case {conn.method, conn.request_path} do
        {"GET", "/api/v2/optimism/interop/public-key"} ->
          send(owner, :key_requested)
          OptimismController.interop_public_key(conn, %{})

        {"POST", "/api/v2/import/optimism/interop/"} ->
          :ok = Sandbox.allow(Repo, owner, self())
          OptimismController.interop_import(conn, conn.body_params)
      end
    end
  end

  @moduletag :tmp_dir
  setup %{tmp_dir: tmp_dir} do
    for {app, key} <- [{:indexer, MessageQueue}, {:explorer, :chain_type}] do
      previous = Application.fetch_env(app, key)

      on_exit(fn ->
        case previous do
          {:ok, value} -> Application.put_env(app, key, value)
          :error -> Application.delete_env(app, key)
        end
      end)
    end

    previous_id = ConCache.get(ChainId.cache_name(), :id)
    ChainId.set_id(23)
    on_exit(fn -> ChainId.set_id(previous_id) end)
    Application.put_env(:explorer, :chain_type, :optimism)

    SQL.query!(
      Repo,
      """
      CREATE TEMP TABLE op_interop_messages (
        sender_address_hash bytea, target_address_hash bytea, nonce bigint NOT NULL, init_chain_id integer NOT NULL,
        init_transaction_hash bytea, block_number bigint, timestamp timestamp(6), relay_chain_id integer NOT NULL,
        relay_transaction_hash bytea, payload bytea, failed boolean, transfer_token_address_hash bytea,
        transfer_from_address_hash bytea, transfer_to_address_hash bytea, transfer_amount numeric,
        sent_to_multichain boolean NOT NULL DEFAULT false, inserted_at timestamp(6) NOT NULL,
        updated_at timestamp(6) NOT NULL, PRIMARY KEY (nonce, init_chain_id)
      )
      """,
      []
    )

    cache = InteropMessage.interop_instance_api_url_to_public_key_cache()

    {supervisor, child_id} =
      if Process.whereis(cache) do
        {Explorer.Supervisor, {ConCache, cache}}
      else
        start_supervised!(Supervisor.child_spec({ConCache, name: cache, ttl_check_interval: false}, id: cache))
        {:ok, supervisor} = ExUnit.fetch_test_supervisor()
        {supervisor, cache}
      end

    tls = tls_fixture()
    ca_file = Path.join(tmp_dir, "trusted-ca.pem")
    File.write!(ca_file, :public_key.pem_encode([{:Certificate, tls.root.cert, :not_encrypted}]))
    :ok = :public_key.cacerts_load(String.to_charlist(ca_file))
    on_exit(fn -> :public_key.cacerts_load() end)

    key_ref = make_ref()
    start_key_peer(key_ref, tls.options, 0)
    key_port = :ranch.get_port(key_ref)
    key_url = "https://localhost:#{key_port}"
    import_ref = make_ref()

    start_supervised!(
      Supervisor.child_spec(
        {Plug.Cowboy, scheme: :http, plug: {Peer, self()}, options: [port: 0, ref: import_ref]},
        id: import_ref
      )
    )

    import_url = "http://127.0.0.1:#{:ranch.get_port(import_ref)}"

    Application.put_env(:indexer, MessageQueue,
      private_key: hex(@old_key),
      connect_timeout: 2,
      recv_timeout: 2,
      chainscout_fallback_map: %{"45" => key_url}
    )

    on_exit(fn ->
      if Process.whereis(cache), do: ConCache.delete(cache, key_url)
    end)

    {:ok,
     cache: cache,
     supervisor: supervisor,
     child_id: child_id,
     key_url: key_url,
     import_url: import_url,
     key_ref: key_ref,
     key_port: key_port}
  end

  test "finding 22: authenticated remote rotation replaces the key version and retires the old key", ctx do
    assert {200, %{"init_transaction_hash" => nil}} = post_interop(ctx, signed(1, @old_key))
    assert_receive :key_requested
    old = ConCache.get(ctx.cache, ctx.key_url)
    {:ok, old_public} = ExSecp256k1.create_public_key(@old_key)
    assert old.public_key == old_public
    assert old.key_version == :crypto.hash(:sha256, old_public)
    assert old.expires_at > System.monotonic_time(:millisecond)
    assert old.expires_at <= System.monotonic_time(:millisecond) + :timer.minutes(5)

    rotate(@new_key)
    assert {200, %{"init_transaction_hash" => nil}} = post_interop(ctx, signed(2, @new_key))
    assert_receive :key_requested
    refute_received :key_requested
    current = ConCache.get(ctx.cache, ctx.key_url)
    {:ok, new_public} = ExSecp256k1.create_public_key(@new_key)
    assert current.public_key == new_public
    assert current.key_version == :crypto.hash(:sha256, new_public)
    refute current.key_version == old.key_version

    assert {401, %{"message" => "Wrong signature"}} = post_interop(ctx, signed(3, @old_key))
    assert_receive :key_requested
    refute_received :key_requested
    assert ConCache.get(ctx.cache, ctx.key_url).key_version == current.key_version
    altered = signed(4, @new_key) |> Map.put("failed", true)
    assert {401, %{"message" => "Wrong signature"}} = post_interop(ctx, altered)
    assert_receive :key_requested
    refute_received :key_requested
    assert Repo.get_by(InteropMessage, init_chain_id: 23, nonce: 3) == nil
    assert Repo.get_by(InteropMessage, init_chain_id: 23, nonce: 4) == nil
    assert_stored(1)
    assert_stored(2)
  end

  test "finding 22: expiry retires an old key even when only old-key traffic arrives", ctx do
    assert {200, _} = post_interop(ctx, signed(1, @old_key))
    assert_receive :key_requested
    expire(ctx)
    rotate(@new_key)
    assert {401, %{"message" => "Wrong signature"}} = post_interop(ctx, signed(2, @old_key))
    assert_receive :key_requested
    refute_received :key_requested
    assert Repo.get_by(InteropMessage, init_chain_id: 23, nonce: 2) == nil
    assert {200, _} = post_interop(ctx, signed(2, @new_key))
    refute_received :key_requested
    assert_stored(2)

    expire(ctx)
    rotate(nil)
    assert {500, %{"message" => "Unable to get public key"}} = post_interop(ctx, signed(3, @new_key))
    assert_receive :key_requested
    assert ConCache.get(ctx.cache, ctx.key_url) == nil
    assert Repo.get_by(InteropMessage, init_chain_id: 23, nonce: 3) == nil
    rotate(@new_key)
    assert {200, _} = post_interop(ctx, signed(3, @new_key))
    assert_receive :key_requested
    assert_stored(3)
  end

  test "finding 22: stop and reopen the cache recovers the current remote key without reviving the retired key", ctx do
    assert {200, _} = post_interop(ctx, signed(1, @old_key))
    assert_receive :key_requested
    previous_pid = Process.whereis(ctx.cache)
    rotate(@new_key)
    assert :ok = Supervisor.terminate_child(ctx.supervisor, ctx.child_id)
    assert Process.whereis(ctx.cache) == nil
    assert {:ok, restarted} = Supervisor.restart_child(ctx.supervisor, ctx.child_id)
    refute restarted == previous_pid
    assert Process.whereis(ctx.cache) == restarted
    assert ConCache.get(ctx.cache, ctx.key_url) == nil
    assert_stored(1)

    assert {401, %{"message" => "Wrong signature"}} = post_interop(ctx, signed(2, @old_key))
    assert_receive :key_requested
    assert {200, _} = post_interop(ctx, signed(2, @new_key))
    refute_received :key_requested
    assert_stored(2)
  end

  test "finding 22: a refresh rejects an untrusted TLS authority and never imports or uses a stale key", ctx do
    assert {200, _} = post_interop(ctx, signed(1, @old_key))
    assert_receive :key_requested
    rotate(@new_key)
    stop_supervised!(ctx.key_ref)
    untrusted = tls_fixture()
    start_key_peer(ctx.key_ref, untrusted.options, ctx.key_port)

    assert {401, %{"message" => "Wrong signature"}} = post_interop(ctx, signed(2, @new_key))
    refute_received :key_requested
    assert ConCache.get(ctx.cache, ctx.key_url) == nil
    assert {500, %{"message" => "Unable to get public key"}} = post_interop(ctx, signed(3, @old_key))
    refute_received :key_requested
    assert Repo.get_by(InteropMessage, init_chain_id: 23, nonce: 2) == nil
    assert Repo.get_by(InteropMessage, init_chain_id: 23, nonce: 3) == nil
    assert_stored(1)
  end

  test "finding 22: legacy unbounded entries are reauthenticated and HTTP cannot replace a trust key", ctx do
    {:ok, old_public} = ExSecp256k1.create_public_key(@old_key)
    ConCache.put(ctx.cache, ctx.key_url, old_public)
    rotate(@new_key)
    assert {401, %{"message" => "Wrong signature"}} = post_interop(ctx, signed(1, @old_key))
    assert_receive :key_requested
    assert {200, _} = post_interop(ctx, signed(1, @new_key))
    refute_received :key_requested
    assert_stored(1)

    config = Application.fetch_env!(:indexer, MessageQueue)

    Application.put_env(
      :indexer,
      MessageQueue,
      Keyword.put(config, :chainscout_fallback_map, %{"45" => ctx.import_url})
    )

    assert {500, %{"message" => "Unable to get public key"}} = post_interop(ctx, signed(2, @new_key))
    refute_received :key_requested
    assert Repo.get_by(InteropMessage, init_chain_id: 23, nonce: 2) == nil
  end

  defp tls_fixture do
    curve = {:namedCurve, {1, 2, 840, 10045, 3, 1, 7}}
    root = :public_key.pkix_test_root_cert(~c"Repair trust root", key: curve, digest: :sha256)

    options =
      :public_key.pkix_test_data(%{
        root: root,
        peer: [
          key: curve,
          digest: :sha256,
          extensions: [{:Extension, {2, 5, 29, 17}, false, [{:dNSName, ~c"localhost"}]}]
        ]
      })

    %{root: root, options: options}
  end

  defp start_key_peer(ref, tls_options, port) do
    start_supervised!(
      Supervisor.child_spec(
        {Plug.Cowboy,
         scheme: :https, plug: {Peer, self()}, options: [port: port, ref: ref, ip: {127, 0, 0, 1}] ++ tls_options},
        id: ref
      )
    )
  end

  defp rotate(nil) do
    config = Application.fetch_env!(:indexer, MessageQueue)
    Application.put_env(:indexer, MessageQueue, Keyword.put(config, :private_key, nil))
  end

  defp rotate(key) do
    config = Application.fetch_env!(:indexer, MessageQueue)
    Application.put_env(:indexer, MessageQueue, Keyword.put(config, :private_key, hex(key)))
  end

  defp expire(ctx) do
    assert :ok =
             ConCache.update(ctx.cache, ctx.key_url, fn entry ->
               {:ok, %{entry | expires_at: System.monotonic_time(:millisecond) - 1}}
             end)
  end

  defp signed(nonce, key) do
    params = %{
      "protocol_version" => 1,
      "nonce" => nonce,
      "init_chain_id" => 23,
      "relay_chain_id" => 45,
      "relay_transaction_hash" => @transaction,
      "failed" => false
    }

    {:ok, data} = OptimismController.interop_signing_data(params)
    {:ok, {signature, _}} = data |> ExKeccak.hash_256() |> ExSecp256k1.sign_compact(key)
    {23, transport} = MessageQueue.set_post_data_signature(23, %{signature: nil}, signature)
    Map.put(params, "signature", transport.signature)
  end

  defp post_interop(ctx, params) do
    client = Tesla.client([], Tesla.Adapter.Mint)

    {:ok, response} =
      Tesla.post(client, ctx.import_url <> "/api/v2/import/optimism/interop/", Jason.encode!(params),
        headers: [{"content-type", "application/json"}]
      )

    {response.status, Jason.decode!(response.body)}
  end

  defp assert_stored(nonce) do
    stored = Repo.get_by!(InteropMessage, init_chain_id: 23, nonce: nonce)
    assert to_string(stored.relay_transaction_hash) == @transaction
    assert stored.relay_chain_id == 45
    assert stored.failed == false
  end

  defp hex(bytes), do: "0x" <> Base.encode16(bytes, case: :lower)
end
