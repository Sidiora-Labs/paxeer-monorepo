defmodule Explorer.Repair.Repair2231473Test do
  use ExUnit.Case, async: false

  @backend_dir Path.expand("../../../..", __DIR__)
  @runtime_config Path.join(@backend_dir, "apps/explorer/config/runtime/prod.exs")
  @ca_env "DATABASE_SSL_CA_CERT_FILE"

  setup do
    ca_file = System.get_env(@ca_env)

    on_exit(fn ->
      if ca_file, do: System.put_env(@ca_env, ca_file), else: System.delete_env(@ca_env)
    end)

    :ok
  end

  test "production umbrella build configuration contains only writable terms and no repo ssl_opts" do
    config = production_build_config()
    assert_release_term(config, [:config])

    text = :io_lib.format(~c"~tp.", [config]) |> List.flatten()
    assert {:ok, tokens, _} = :erl_scan.string(text)
    assert {:ok, ^config} = :erl_parse.parse_term(tokens)

    repos = repo_configs(config)
    assert Keyword.has_key?(repos, Explorer.Repo)

    for {repo, opts} <- repos do
      refute Keyword.has_key?(opts, :ssl_opts), "#{inspect(repo)} has build-time ssl_opts"
    end
  end

  test "runtime configuration supplies peer and hostname verification to every production repo at startup" do
    build_config = production_build_config()
    build_repos = repo_configs(build_config)
    assert Keyword.has_key?(build_repos, Explorer.Repo)
    ca_path = Path.expand("../../config/repair_22_33_471/fixtures/trusted-ca.pem", __DIR__)

    for ca_file <- [nil, ca_path] do
      if ca_file, do: System.put_env(@ca_env, ca_file), else: System.delete_env(@ca_env)
      runtime_config = Config.Reader.read!(@runtime_config, env: :prod, imports: :disabled)
      runtime_repos = repo_configs(runtime_config)
      assert Enum.sort(Keyword.keys(runtime_repos)) == Enum.sort(Keyword.keys(build_repos))
      merged = Config.Reader.merge(build_config, runtime_config)
      expected_ca = String.to_charlist(ca_file || "/etc/ssl/certs/ca-certificates.crt")

      for {repo, opts} <- runtime_repos do
        ssl_opts = Keyword.fetch!(opts, :ssl_opts)
        assert Keyword.fetch!(ssl_opts, :verify) == :verify_peer
        assert Keyword.fetch!(ssl_opts, :cacertfile) == expected_ca
        hostname_check = Keyword.fetch!(ssl_opts, :customize_hostname_check)
        assert is_function(Keyword.fetch!(hostname_check, :match_fun), 2)
        refute Keyword.has_key?(ssl_opts, :server_name_indication)
        assert merged[:explorer][repo][:ssl_opts] == ssl_opts
      end
    end
  end

  test "production release writes a readable system configuration without build-time repo ssl_opts" do
    release_path = Path.join([Mix.Project.build_path(), "rel", "blockscout"])
    [_erts_version, version] = release_path |> Path.join("releases/start_erl.data") |> File.read!() |> String.split()
    sys_config = Path.join([release_path, "releases", version, "sys.config"])
    assert File.regular?(sys_config)
    assert {:ok, [config]} = :file.consult(String.to_charlist(sys_config))
    release_repos = repo_configs(config)

    for {repo, _opts} <- repo_configs(production_build_config()) do
      opts = Keyword.fetch!(release_repos, repo)
      refute Keyword.has_key?(opts, :ssl_opts), "#{inspect(repo)} has ssl_opts in #{sys_config}"
    end
  end

  defp production_build_config do
    File.cd!(@backend_dir, fn -> Config.Reader.read!("config/config.exs", env: :prod) end)
  end

  defp repo_configs(config) do
    for {repo, opts} <- Keyword.fetch!(config, :explorer),
        is_atom(repo),
        repo == Explorer.Repo or String.starts_with?(Atom.to_string(repo), "Elixir.Explorer.Repo.") do
      {repo, opts}
    end
  end

  defp assert_release_term(term, _path)
       when is_atom(term) or is_number(term) or is_bitstring(term),
       do: :ok

  defp assert_release_term([], _path), do: :ok

  defp assert_release_term([head | tail], path) do
    assert_release_term(head, [:head | path])
    assert_release_term(tail, [:tail | path])
  end

  defp assert_release_term(term, path) when is_tuple(term) do
    term
    |> Tuple.to_list()
    |> Enum.with_index()
    |> Enum.each(fn {value, index} -> assert_release_term(value, [index | path]) end)
  end

  defp assert_release_term(term, path) when is_map(term) do
    term
    |> Map.to_list()
    |> Enum.each(fn {key, value} ->
      assert_release_term(key, [:map_key | path])
      assert_release_term(value, [key | path])
    end)
  end

  defp assert_release_term(_term, path) do
    flunk("production build configuration contains a function or unwritable term at #{inspect(Enum.reverse(path))}")
  end
end
