defmodule Explorer.Repair.Repair22_33_333Test do
  @moduledoc """
  Task 22.33.333 repair record (audit 22.30.137).

  reader_options_dropped: regular and proxy reads preserve caller transport options.
  reader_empty_abi: empty and event-only ABIs return the method-not-found error.
  solc_partial_download_recovery: solc recovers owned temporary files after restart.
  solc_mirror_bypass: solc discovery and artifact retrieval use only the configured mirror.
  solc_http_error_cached: solc rejects HTTP errors and corrupt artifacts before retrying.
  vyper_partial_download_recovery: vyper recovers owned temporary files after restart.
  vyper_http_error_cached: vyper rejects release errors, artifact errors, truncated responses and corrupt artifacts.

  These test names below are the proving cases. No out-of-path wiring is required.
  Baseline verification was not run: the task's ordered gate instructions require
  complete implementation before the single hf-build/hf-done qualification pass.
  The runner's gate ledger records the verified revision and private evidence.
  """
  use Explorer.DataCase, async: false

  alias Explorer.Chain.Hash
  alias Explorer.SmartContract.{CompilerVersion, Reader, SolcDownloader, VyperDownloader}

  @solc_version "v0.8.21+commit.d9974bed"
  @vyper_version "v0.3.10"
  @solc_body "var Module = {version: '0.8.21+commit.d9974bed'};\n"
  @vyper_body <<0x7F, "ELF", 2, 1, 1, 0, "vyper-0.3.10-release-transcript">>
  @abi [
    %{
      "type" => "function",
      "name" => "get",
      "inputs" => [],
      "outputs" => [%{"type" => "uint256", "name" => "value"}],
      "stateMutability" => "view"
    }
  ]
  @event %{"type" => "event", "name" => "Changed", "inputs" => [], "anonymous" => false}
  @params %{method_id: "6d4ce63c", args: []}
  @result "0x000000000000000000000000000000000000000000000000000000000000002a"

  setup do
    server = start_transcript_server()
    put_env(:explorer, :http_client, HTTPoison)
    put_env(:explorer, :solc_bin_api_url, server.url)
    put_env(:explorer, :vyper_releases_url, server.url <> "/releases?per_page=100")
    put_env(:explorer, Explorer.SmartContract.RustVerifierInterfaceBehaviour, enabled: false)

    for {module, path} <- [{SolcDownloader, solc_path()}, {VyperDownloader, vyper_path()}] do
      :ok = Supervisor.terminate_child(Explorer.Supervisor, module)
      for file <- [path, path <> ".download", legacy_path(module)], do: File.rm(file)
      {:ok, _} = Supervisor.restart_child(Explorer.Supervisor, module)
    end

    on_exit(fn ->
      for {module, path} <- [{SolcDownloader, solc_path()}, {VyperDownloader, vyper_path()}] do
        Supervisor.terminate_child(Explorer.Supervisor, module)
        for file <- [path, path <> ".download", legacy_path(module)], do: File.rm(file)
        Supervisor.restart_child(Explorer.Supervisor, module)
      end
    end)

    install_solc_transcript(server)
    install_vyper_transcript(server)
    {:ok, server: server}
  end

  test "reader_options_dropped: regular and proxy reads preserve caller transport options", %{server: server} do
    implementation = insert(:smart_contract, abi: @abi, contract_code_md5: "implementation")
    proxy = insert(:smart_contract, abi: [], contract_code_md5: "proxy")

    insert(:proxy_implementation,
      proxy_address_hash: proxy.address_hash,
      address_hashes: [implementation.address_hash],
      names: [implementation.name],
      proxy_type: :eip1967
    )

    # The default endpoint must never receive these reads. The per-call options
    # select the loopback HTTP server through the production JSON-RPC transport.
    put_env(:explorer, :json_rpc_named_arguments, rpc_options(server.url <> "/wrong-endpoint"))
    options = rpc_options(server.url <> "/rpc")
    caller = "0x0000000000000000000000000000000000000001"

    assert [%{"type" => "uint256", "value" => 42}] ==
             Reader.query_function(implementation.address_hash, @params, :regular, caller, true, options)

    assert %{output: [%{"type" => "uint256", "value" => 42}], names: ["value"]} ==
             Reader.query_function_with_names(proxy.address_hash, @params, :proxy, caller, [], true, options)

    assert %{output: [%{"type" => "uint256", "value" => 42}], names: ["value"]} ==
             Reader.query_function_with_names_custom_abi(implementation.address_hash, @params, caller, @abi, options)

    calls = requests(server) |> Enum.filter(&(&1.path == "/rpc"))
    assert length(calls) == 3
    assert Enum.all?(calls, &(&1.method == "POST"))

    for {call, target} <-
          Enum.zip(calls, [implementation.address_hash, proxy.address_hash, implementation.address_hash]) do
      assert [%{"method" => "eth_call", "params" => [transaction, "latest"]}] = Jason.decode!(call.body)
      assert transaction == %{"to" => Hash.to_string(target), "from" => caller, "data" => "0x6d4ce63c"}
    end

    refute Enum.any?(requests(server), &(&1.path == "/wrong-endpoint"))
  end

  for {label, abi} <- [{"empty", []}, {"event-only", [@event]}] do
    test "reader_empty_abi: #{label} ABIs return the method-not-found error", %{server: server} do
      abi = unquote(Macro.escape(abi))
      contract = insert(:smart_contract, abi: abi, contract_code_md5: "missing-method")
      expected = {:error, "method_id does not exist"}

      assert Reader.query_function(contract.address_hash, @params, :regular, nil, true) == expected
      assert Reader.query_function_with_custom_abi(contract.address_hash, @params, nil, true, abi) == expected
      assert Reader.query_function_with_names(contract.address_hash, @params, :regular, nil, abi, true) == expected
      assert Reader.query_function_with_names_custom_abi(contract.address_hash, @params, nil, abi) == expected
      refute Enum.any?(requests(server), &(&1.method == "POST"))
    end
  end

  test "reader_empty_abi: a proxy with no function ABI propagates the named-output error" do
    implementation = insert(:smart_contract, abi: [], contract_code_md5: "empty-implementation")
    proxy = insert(:smart_contract, abi: [], contract_code_md5: "empty-proxy")

    insert(:proxy_implementation,
      proxy_address_hash: proxy.address_hash,
      address_hashes: [implementation.address_hash],
      names: [implementation.name],
      proxy_type: :eip1967
    )

    assert Reader.query_function_with_names(proxy.address_hash, @params, :proxy, nil, [], true) ==
             {:error, "method_id does not exist"}
  end

  test "solc_mirror_bypass: solc discovery and artifact retrieval use only the configured mirror", %{server: server} do
    assert {:ok, versions} = CompilerVersion.fetch_versions(:solc)
    assert @solc_version in versions
    assert SolcDownloader.ensure_exists(@solc_version) == solc_path()
    assert File.read!(solc_path()) == @solc_body
    assert Enum.any?(requests(server), &(&1.path == "/bin/list.json"))
    assert Enum.any?(requests(server), &(&1.path == "/bin/soljson-#{@solc_version}.js"))
    before = length(requests(server))
    assert SolcDownloader.ensure_exists(@solc_version) == solc_path()
    assert length(requests(server)) == before
  end

  test "solc_partial_download_recovery: solc recovers owned temporary files after restart" do
    assert_restart_recovery(SolcDownloader, @solc_version, solc_path(), @solc_body)
  end

  test "solc_http_error_cached: solc rejects HTTP errors and corrupt artifacts before retrying", %{server: server} do
    artifact = "/bin/soljson-#{@solc_version}.js"
    route(server, artifact, {503, "<html>unavailable</html>"})
    refute SolcDownloader.ensure_exists(@solc_version)
    refute File.exists?(solc_path())
    assert Process.alive?(Process.whereis(SolcDownloader))

    route(server, artifact, {200, String.duplicate("x", byte_size(@solc_body))})
    refute SolcDownloader.ensure_exists(@solc_version)
    refute File.exists?(solc_path())
    refute File.exists?(solc_path() <> ".download")

    route(server, artifact, {200, @solc_body})
    assert SolcDownloader.ensure_exists(@solc_version) == solc_path()
    assert File.read!(solc_path()) == @solc_body
  end

  test "solc_http_error_cached: invalid distribution metadata cannot publish a compiler", %{server: server} do
    route(
      server,
      "/bin/list.json",
      {200, Jason.encode!(%{"builds" => [%{"version" => "0.8.21", "path" => "soljson-#{@solc_version}.js"}]})}
    )

    refute SolcDownloader.ensure_exists(@solc_version)
    refute File.exists?(solc_path())
    refute Enum.any?(requests(server), &(&1.path == "/bin/soljson-#{@solc_version}.js"))
    install_solc_transcript(server)
    assert SolcDownloader.ensure_exists(@solc_version) == solc_path()
  end

  test "vyper_partial_download_recovery: vyper recovers owned temporary files after restart" do
    assert_restart_recovery(VyperDownloader, @vyper_version, vyper_path(), @vyper_body)
    assert Bitwise.band(File.stat!(vyper_path()).mode, 0o777) == 0o755
  end

  test "vyper_http_error_cached: vyper rejects release errors, artifact errors, truncated responses and corrupt artifacts",
       %{server: server} do
    route(server, "/releases?per_page=100", {503, Jason.encode!(%{"error" => "unavailable"})})
    refute VyperDownloader.ensure_exists(@vyper_version)
    refute File.exists?(vyper_path())
    install_vyper_transcript(server)

    route(server, "/vyper-linux", {502, "bad gateway"})
    refute VyperDownloader.ensure_exists(@vyper_version)
    refute File.exists?(vyper_path())

    route(server, "/vyper-linux", {200, binary_part(@vyper_body, 0, 6), byte_size(@vyper_body)})
    refute VyperDownloader.ensure_exists(@vyper_version)
    refute File.exists?(vyper_path())
    assert Process.alive?(Process.whereis(VyperDownloader))

    route(server, "/vyper-linux", {200, binary_part(@vyper_body, 0, 6)})
    refute VyperDownloader.ensure_exists(@vyper_version)
    refute File.exists?(vyper_path())

    route(server, "/vyper-linux", {200, String.duplicate("x", byte_size(@vyper_body))})
    refute VyperDownloader.ensure_exists(@vyper_version)
    refute File.exists?(vyper_path())
    refute File.exists?(vyper_path() <> ".download")

    route(server, "/vyper-linux", {200, @vyper_body})
    assert VyperDownloader.ensure_exists(@vyper_version) == vyper_path()
    assert File.read!(vyper_path()) == @vyper_body
    assert Bitwise.band(File.stat!(vyper_path()).mode, 0o777) == 0o755
  end

  test "vyper_http_error_cached: older release assets without digests still require a complete recorded size", %{
    server: server
  } do
    install_vyper_transcript(server, nil)
    route(server, "/vyper-linux", {200, "partial"})
    refute VyperDownloader.ensure_exists(@vyper_version)
    refute File.exists?(vyper_path())
    route(server, "/vyper-linux", {200, @vyper_body})
    assert VyperDownloader.ensure_exists(@vyper_version) == vyper_path()
    assert File.read!(vyper_path()) == @vyper_body
  end

  test "publication errors leave downloads retryable without returning an installed compiler" do
    for {module, version, path} <- [
          {SolcDownloader, @solc_version, solc_path()},
          {VyperDownloader, @vyper_version, vyper_path()}
        ] do
      # An owned staging path that cannot be removed must fail explicitly.
      File.mkdir!(path <> ".download")
      refute module.ensure_exists(version)
      refute File.exists?(path)
      File.rmdir!(path <> ".download")
      assert module.ensure_exists(version) == path
    end
  end

  defp assert_restart_recovery(module, version, path, body) do
    old_pid = Process.whereis(module)
    File.write!(legacy_path(module), "interrupted legacy download")
    File.write!(path <> ".download", "interrupted owned download")
    :ok = Supervisor.terminate_child(Explorer.Supervisor, module)
    refute Process.alive?(old_pid)
    {:ok, new_pid} = Supervisor.restart_child(Explorer.Supervisor, module)
    assert new_pid != old_pid
    refute File.exists?(legacy_path(module))
    refute File.exists?(path <> ".download")
    refute File.exists?(path)
    assert module.ensure_exists(version) == path
    assert File.read!(path) == body
  end

  defp solc_path, do: Application.app_dir(:explorer, "priv/solc_compilers/#{@solc_version}.js")
  defp vyper_path, do: Application.app_dir(:explorer, "priv/vyper_compilers/#{@vyper_version}")

  defp legacy_path(SolcDownloader), do: String.replace_suffix(solc_path(), ".js", "-tmp.js")
  defp legacy_path(VyperDownloader), do: vyper_path() <> "-tmp"

  defp rpc_options(url) do
    [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        urls: [url],
        http_options: [recv_timeout: 2_000, timeout: 2_000]
      ]
    ]
  end

  defp install_solc_transcript(server) do
    metadata = %{
      "builds" => [
        %{"version" => "0.8.21", "path" => "soljson-#{@solc_version}.js", "sha256" => "0x" <> digest(@solc_body)}
      ],
      "releases" => %{"0.8.21" => "soljson-#{@solc_version}.js"},
      "latestRelease" => "0.8.21"
    }

    route(server, "/bin/list.json", {200, Jason.encode!(metadata)})
    route(server, "/bin/soljson-#{@solc_version}.js", {200, @solc_body})
  end

  defp install_vyper_transcript(server, checksum \\ "sha256:" <> digest(@vyper_body)) do
    releases = [
      %{
        "tag_name" => @vyper_version,
        "assets" => [
          %{
            "name" => "vyper-linux",
            "browser_download_url" => server.url <> "/vyper-linux",
            "size" => byte_size(@vyper_body),
            "digest" => checksum
          }
        ]
      }
    ]

    route(server, "/releases?per_page=100", {200, Jason.encode!(releases)})
    route(server, "/vyper-linux", {200, @vyper_body})
  end

  defp digest(body), do: Base.encode16(:crypto.hash(:sha256, body), case: :lower)

  defp put_env(app, key, value) do
    previous = Application.fetch_env(app, key)
    Application.put_env(app, key, value)

    on_exit(fn ->
      case previous do
        {:ok, original} -> Application.put_env(app, key, original)
        :error -> Application.delete_env(app, key)
      end
    end)
  end

  defp start_transcript_server do
    state = start_supervised!({Agent, fn -> %{routes: %{}, requests: []} end})
    {:ok, listener} = :gen_tcp.listen(0, [:binary, active: false, packet: :raw, reuseaddr: true, ip: {127, 0, 0, 1}])
    {:ok, {_, port}} = :inet.sockname(listener)
    acceptor = spawn(fn -> accept_requests(listener, state) end)

    on_exit(fn ->
      :gen_tcp.close(listener)
      Process.exit(acceptor, :kill)
    end)

    %{url: "http://127.0.0.1:#{port}", state: state}
  end

  defp route(server, path, response) do
    Agent.update(server.state, &put_in(&1, [:routes, path], response))
  end

  defp requests(server), do: Agent.get(server.state, &Enum.reverse(&1.requests))

  defp accept_requests(listener, state) do
    case :gen_tcp.accept(listener) do
      {:ok, socket} ->
        serve_request(socket, state)
        :gen_tcp.close(socket)
        accept_requests(listener, state)

      {:error, :closed} ->
        :ok
    end
  end

  defp serve_request(socket, state) do
    {header, initial_body} = receive_headers(socket, "")
    [request_line | headers] = String.split(header, "\r\n")
    [method, path, _] = String.split(request_line, " ")

    length =
      Enum.find_value(headers, 0, fn line ->
        case String.split(line, ":", parts: 2) do
          [name, value] -> if String.downcase(name) == "content-length", do: String.to_integer(String.trim(value))
          _ -> nil
        end
      end)

    body = receive_body(socket, initial_body, length)

    response =
      Agent.get_and_update(state, fn current ->
        {current.routes[path], %{current | requests: [%{method: method, path: path, body: body} | current.requests]}}
      end)

    response =
      if path == "/rpc" do
        batch = Jason.decode!(body)
        {200, Jason.encode!(Enum.map(batch, &%{"id" => &1["id"], "jsonrpc" => "2.0", "result" => @result}))}
      else
        response || {404, "unrecorded request"}
      end

    {status, response_body, declared_size} =
      case response do
        {status, response_body} -> {status, response_body, byte_size(response_body)}
        triple -> triple
      end

    :gen_tcp.send(socket, [
      "HTTP/1.1 #{status} Response\r\nContent-Type: application/json\r\nContent-Length: #{declared_size}\r\nConnection: close\r\n\r\n",
      response_body
    ])
  end

  defp receive_headers(socket, data) do
    case :binary.split(data, "\r\n\r\n") do
      [header, body] ->
        {header, body}

      [_] ->
        {:ok, more} = :gen_tcp.recv(socket, 0, 5_000)
        receive_headers(socket, data <> more)
    end
  end

  defp receive_body(_socket, body, length) when byte_size(body) >= length, do: body

  defp receive_body(socket, body, length) do
    {:ok, more} = :gen_tcp.recv(socket, 0, 5_000)
    receive_body(socket, body <> more, length)
  end
end
