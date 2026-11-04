defmodule Explorer.SmartContract.CompilerVersionFixture do
  import ExUnit.Callbacks, only: [on_exit: 1]

  def install do
    compiler_url = Application.fetch_env(:explorer, :solc_bin_api_url)
    adapter = Application.fetch_env(:tesla, :adapter)
    bypass = Bypass.open()

    restore = fn ->
      restore_env(:explorer, :solc_bin_api_url, compiler_url)
      restore_env(:tesla, :adapter, adapter)
    end

    on_exit(restore)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)
    Application.put_env(:explorer, :solc_bin_api_url, "http://localhost:#{bypass.port}")
    {:ok, bypass: bypass, restore: restore}
  end

  defp restore_env(app, key, {:ok, value}), do: Application.put_env(app, key, value)
  defp restore_env(app, key, :error), do: Application.delete_env(app, key)
end
