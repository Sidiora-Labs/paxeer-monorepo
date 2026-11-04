Code.require_file("../explorer/smart_contract/compiler_version_fixture.exs", __DIR__)
Code.require_file("../explorer/smart_contract/repair_22_33_544_fixtures.exs", __DIR__)

defmodule Explorer.Repair.Repair2233544Test do
  use ExUnit.Case

  alias Explorer.SmartContract.{CompilerVersion, CompilerVersionFixture, Repair2233544Fixtures}

  setup do
    replace_env(:explorer, Explorer.SmartContract.RustVerifierInterfaceBehaviour, enabled: false)
    replace_env(:tesla, :adapter, Tesla.Adapter.Mint)
    previous = Application.fetch_env(:explorer, :solc_bin_api_url)
    on_exit(fn -> restore_env(:explorer, :solc_bin_api_url, previous) end)
    :ok
  end

  test "compiler-url-leak restores the ordinary compiler lookup after the fixture closes" do
    ordinary = compiler_server()
    ordinary_url = "http://localhost:#{ordinary.port}"
    Application.put_env(:explorer, :solc_bin_api_url, ordinary_url)

    on_exit(fn ->
      assert Application.fetch_env(:explorer, :solc_bin_api_url) == {:ok, ordinary_url}
      assert Application.fetch_env(:tesla, :adapter) == {:ok, Tesla.Adapter.Mint}
      assert {:ok, versions} = CompilerVersion.fetch_versions(:solc)
      assert "v0.4.9+commit.364da425" in versions
      assert hd(versions) == "latest"
    end)

    {:ok, bypass: fixture, restore: _} = CompilerVersionFixture.install()
    serve_versions(fixture)
    assert {:ok, versions} = CompilerVersion.fetch_versions(:solc)
    assert "v0.4.9+commit.364da425" in versions
    :ok = Bypass.down(fixture)
  end

  test "compiler-url-leak deletes the fixture key when the original key was absent" do
    Application.delete_env(:explorer, :solc_bin_api_url)
    Application.delete_env(:tesla, :adapter)

    on_exit(fn ->
      assert Application.fetch_env(:explorer, :solc_bin_api_url) == :error
      assert Application.fetch_env(:tesla, :adapter) == :error
    end)

    {:ok, bypass: fixture, restore: _} = CompilerVersionFixture.install()
    serve_versions(fixture)
    assert {:ok, versions} = CompilerVersion.fetch_versions(:solc)
    assert hd(versions) == "latest"
    :ok = Bypass.down(fixture)
  end

  test "disabled-binary-getter-regression preserves bytes beginning with ASCII 0x over real HTTP" do
    Repair2233544Fixtures.assert_binary_getter()
  end

  defp compiler_server do
    bypass = Bypass.open()
    serve_versions(bypass)
    bypass
  end

  defp serve_versions(bypass) do
    body = File.read!(Path.expand("../support/fixture/smart_contract/solc_bin.json", __DIR__))

    Bypass.expect_once(bypass, "GET", "/bin/list.json", fn conn ->
      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, body)
    end)
  end

  defp replace_env(app, key, value) do
    previous = Application.fetch_env(app, key)
    on_exit(fn -> restore_env(app, key, previous) end)
    Application.put_env(app, key, value)
  end

  defp restore_env(app, key, {:ok, value}), do: Application.put_env(app, key, value)
  defp restore_env(app, key, :error), do: Application.delete_env(app, key)
end
