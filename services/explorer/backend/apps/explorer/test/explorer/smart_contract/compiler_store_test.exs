defmodule Explorer.SmartContract.CompilerStoreTest do
  use ExUnit.Case, async: false

  import Bitwise

  alias Explorer.SmartContract.{CompilerDownload, SolcDownloader, VyperDownloader}

  @moduletag :compiler

  test "every compiler in the inventory is available with its recorded digest and mode" do
    entries =
      "/opt/explorer-compilers/compilers.tsv"
      |> File.read!()
      |> String.split("\n", trim: true)

    assert entries != []

    for entry <- entries do
      [compiler, version, url, digest] = String.split(entry, "\t")
      assert String.starts_with?(url, "https://")

      {downloader, filename, mode} =
        case compiler do
          "solc" -> {SolcDownloader, version <> ".js", 0o644}
          "vyper" -> {VyperDownloader, version, 0o755}
        end

      path = Application.app_dir(:explorer, "priv/#{compiler}_compilers/bundled/#{filename}")
      assert File.regular?(path), "Missing compiler: #{compiler} #{version}"
      assert :ok = CompilerDownload.sha256(File.read!(path), digest)
      assert {:ok, stat} = File.stat(path)
      assert (stat.mode &&& 0o777) == mode
      assert downloader.ensure_exists(version) == path
    end
  end
end
