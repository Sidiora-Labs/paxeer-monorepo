defmodule Explorer.Repair.Repair2233471Test do
  use ExUnit.Case, async: false

  alias Explorer.Chain.{Address, Block}
  alias Explorer.Repo

  @config_dir Path.expand("../../config", __DIR__)
  @fixtures Path.join(@config_dir, "repair_22_33_471/fixtures")
  @ca_env "DATABASE_SSL_CA_CERT_FILE"
  @repos [
    Explorer.Repo,
    Explorer.Repo.Replica1,
    Explorer.Repo.Account,
    Explorer.Repo.BridgedTokens,
    Explorer.Repo.ShrunkInternalTransactions,
    Explorer.Repo.EventNotifications,
    Explorer.Repo.Arbitrum,
    Explorer.Repo.Beacon,
    Explorer.Repo.Blackfort,
    Explorer.Repo.Celo,
    Explorer.Repo.Filecoin,
    Explorer.Repo.Mud,
    Explorer.Repo.Optimism,
    Explorer.Repo.PolygonEdge,
    Explorer.Repo.PolygonZkevm,
    Explorer.Repo.RSK,
    Explorer.Repo.Scroll,
    Explorer.Repo.Shibarium,
    Explorer.Repo.Stability,
    Explorer.Repo.Suave,
    Explorer.Repo.Zilliqa,
    Explorer.Repo.ZkSync,
    Explorer.Repo.Neon
  ]

  setup do
    {:ok, _} = Application.ensure_all_started(:ssl)
    ca_env = System.get_env(@ca_env)
    repo_config = Application.fetch_env!(:explorer, Repo)
    System.put_env(@ca_env, Path.join(@fixtures, "trusted-ca.pem"))

    on_exit(fn ->
      if ca_env, do: System.put_env(@ca_env, ca_env), else: System.delete_env(@ca_env)
      Application.put_env(:explorer, Repo, repo_config)
    end)

    :ok
  end

  test "production-database-peer-verification: every production repo inherits authenticated TLS" do
    System.delete_env(@ca_env)
    config = Config.Reader.read!(Path.join(@config_dir, "prod.exs"))
    configured_repos = for {repo, opts} <- config[:explorer], Keyword.has_key?(opts, :ssl_opts), do: repo
    assert Enum.sort(configured_repos) == Enum.sort(@repos)

    for repo <- @repos do
      opts = config[:explorer][repo][:ssl_opts]
      assert opts[:verify] == :verify_peer
      assert opts[:cacertfile] == ~c"/etc/ssl/certs/ca-certificates.crt"
      assert is_function(opts[:customize_hostname_check][:match_fun], 2)
      refute Keyword.has_key?(opts, :server_name_indication)
    end
  end

  test "production-database-peer-verification: runtime CA selection overrides the build configuration" do
    System.delete_env(@ca_env)
    build_config = Config.Reader.read!(Path.join(@config_dir, "prod.exs"))
    ca_path = Path.join(@fixtures, "trusted-ca.pem")
    System.put_env(@ca_env, ca_path)
    runtime_config = Config.Reader.read!(Path.join(@config_dir, "runtime/prod.exs"), imports: :disabled)
    merged = Config.Reader.merge(build_config, runtime_config)

    for repo <- @repos do
      opts = merged[:explorer][repo][:ssl_opts]
      assert opts[:verify] == :verify_peer
      assert opts[:cacertfile] == String.to_charlist(ca_path)
      assert is_function(opts[:customize_hostname_check][:match_fun], 2)
    end
  end

  test "production-database-peer-verification: a trusted TLS database stores and returns a recorded block schema" do
    {port, ref} = tls_database("trusted-server")
    opts = connection_opts(port, Repo)
    {:ok, tls_repo} = Repo.start_link(opts ++ [name: nil, pool: DBConnection.ConnectionPool, pool_size: 1])
    old_dynamic_repo = Repo.put_dynamic_repo(tls_repo)

    try do
      assert {:error, :proof_complete} =
               Repo.transaction(fn ->
                 recorded = @fixtures |> Path.join("new_head_full.json") |> File.read!() |> Jason.decode!()

                 params =
                   recorded["block"]
                   |> EthereumJSONRPC.Block.to_elixir()
                   |> EthereumJSONRPC.Block.elixir_to_params()
                   |> Map.put(:consensus, true)

                 address = Address.changeset(%Address{}, %{hash: params.miner_hash})
                 assert address.valid?
                 Repo.insert!(address, on_conflict: :nothing)
                 changeset = Block.changeset(%Block{}, params)
                 assert changeset.valid?
                 block = Repo.insert!(changeset)
                 recovered = Repo.get!(Block, block.hash)
                 assert to_string(recovered.hash) == recorded["block"]["hash"]
                 assert recovered.number == EthereumJSONRPC.quantity_to_integer(recorded["block"]["number"])
                 assert recovered.miner_hash == block.miner_hash
                 assert DateTime.compare(recovered.timestamp, params.timestamp) == :eq
                 Repo.rollback(:proof_complete)
               end)

      assert_receive {:forwarded, ^ref, <<_length::32, 196_608::32, startup::binary>>}, 5_000
      assert startup =~ "user"
      assert startup =~ "database"
    after
      Repo.put_dynamic_repo(old_dynamic_repo)
      Supervisor.stop(tls_repo)
    end
  end

  test "production-database-peer-verification: an untrusted database is rejected before startup data" do
    {port, ref} = tls_database("untrusted-server")
    opts = connection_opts(port, Repo.Replica1)
    assert {:error, %DBConnection.ConnectionError{message: message}} = connect(opts)
    assert message =~ "unknown_ca"
    refute_receive {:forwarded, ^ref, _}, 100
  end

  test "production-database-peer-verification: a trusted certificate for another hostname is rejected" do
    {port, ref} = tls_database("wrong-host-server")
    opts = connection_opts(port, Repo.Account)
    assert {:error, %DBConnection.ConnectionError{message: message}} = connect(opts)
    assert message =~ "hostname_check_failed"
    refute_receive {:forwarded, ^ref, _}, 100
  end

  defp connection_opts(port, repo) do
    runtime = Config.Reader.read!(Path.join(@config_dir, "runtime/prod.exs"), imports: :disabled)

    Repo.config()
    |> Keyword.take([:username, :password, :database])
    |> Keyword.merge(runtime[:explorer][repo])
    |> Keyword.merge(hostname: "localhost", port: port, ssl: true, timeout: 5_000, connect_timeout: 5_000)
    |> Postgrex.Utils.default_opts()
  end

  defp connect(opts) do
    Task.async(fn -> Postgrex.Protocol.connect(opts) end) |> Task.await(10_000)
  end

  # This peer terminates real PostgreSQL TLS and forwards the PostgreSQL stream
  # unchanged to the disposable database provisioned by the declared gate.
  defp tls_database(certificate) do
    {:ok, listener} = :gen_tcp.listen(0, [:binary, active: false, ip: {127, 0, 0, 1}, reuseaddr: true])
    {:ok, {_, port}} = :inet.sockname(listener)
    parent = self()
    ref = make_ref()
    db_config = Repo.config()

    pid =
      spawn(fn ->
        {:ok, socket} = :gen_tcp.accept(listener, 10_000)
        {:ok, <<8::32, 80_877_103::32>>} = :gen_tcp.recv(socket, 8, 5_000)
        :ok = :gen_tcp.send(socket, "S")

        server_opts = [
          certfile: String.to_charlist(Path.join(@fixtures, certificate <> ".pem")),
          keyfile: String.to_charlist(Path.join(@fixtures, certificate <> "-key.pem")),
          active: false
        ]

        case :ssl.handshake(socket, server_opts, 5_000) do
          {:ok, tls} ->
            {:ok, upstream} =
              :gen_tcp.connect(
                String.to_charlist(Keyword.fetch!(db_config, :hostname)),
                Keyword.get(db_config, :port, 5432),
                [:binary, active: false],
                5_000
              )

            :ok = :ssl.setopts(tls, active: :once)
            :ok = :inet.setopts(upstream, active: :once)
            relay(tls, upstream, parent, ref, false)

          {:error, _reason} ->
            :gen_tcp.close(socket)
        end
      end)

    on_exit(fn ->
      Process.exit(pid, :kill)
      :gen_tcp.close(listener)
    end)

    {port, ref}
  end

  defp relay(tls, upstream, parent, ref, forwarded?) do
    receive do
      {:ssl, ^tls, data} ->
        unless forwarded?, do: send(parent, {:forwarded, ref, data})
        :ok = :gen_tcp.send(upstream, data)
        :ok = :ssl.setopts(tls, active: :once)
        relay(tls, upstream, parent, ref, true)

      {:tcp, ^upstream, data} ->
        :ok = :ssl.send(tls, data)
        :ok = :inet.setopts(upstream, active: :once)
        relay(tls, upstream, parent, ref, forwarded?)

      {kind, _socket} when kind in [:ssl_closed, :tcp_closed] ->
        :ssl.close(tls)
        :gen_tcp.close(upstream)

      {kind, _socket, _reason} when kind in [:ssl_error, :tcp_error] ->
        :ssl.close(tls)
        :gen_tcp.close(upstream)
    after
      15_000 ->
        :ssl.close(tls)
        :gen_tcp.close(upstream)
    end
  end
end

