defmodule BlockScoutWeb.SourcifyServer do
  @moduledoc false

  import ExUnit.Assertions
  import Plug.Conn

  def start(address) do
    bypass = Bypass.open()
    normalized_address = String.downcase(address)
    previous = Application.fetch_env!(:explorer, Explorer.ThirdPartyIntegrations.Sourcify)

    previous_verifier =
      Application.get_env(:explorer, Explorer.SmartContract.RustVerifierInterfaceBehaviour)

    Application.put_env(
      :explorer,
      Explorer.ThirdPartyIntegrations.Sourcify,
      Keyword.merge(previous, server_url: "http://localhost:#{bypass.port}/server", enabled: true, chain_id: "1")
    )

    Application.put_env(:explorer, Explorer.SmartContract.RustVerifierInterfaceBehaviour, enabled: false)

    ExUnit.Callbacks.on_exit(fn ->
      Application.put_env(:explorer, Explorer.ThirdPartyIntegrations.Sourcify, previous)

      Application.put_env(
        :explorer,
        Explorer.SmartContract.RustVerifierInterfaceBehaviour,
        previous_verifier
      )
    end)

    Bypass.expect_once(bypass, "GET", "/server/check-by-addresses", fn conn ->
      conn = fetch_query_params(conn)
      assert Map.get(conn.query_params, "chainIds") == "1"
      assert String.downcase(Map.get(conn.query_params, "addresses")) == normalized_address
      json(conn, [%{"status" => "perfect"}])
    end)

    Bypass.expect_once(bypass, "GET", "/server/files/1/:address", fn conn ->
      assert String.downcase(conn.params["address"]) == normalized_address
      json(conn, metadata())
    end)

    bypass
  end

  def metadata do
    abi = [
      %{
        "inputs" => [],
        "name" => "retrieve",
        "outputs" => [%{"internalType" => "uint256", "name" => "", "type" => "uint256"}],
        "stateMutability" => "view",
        "type" => "function"
      },
      %{
        "inputs" => [%{"internalType" => "uint256", "name" => "_number", "type" => "uint256"}],
        "name" => "store",
        "outputs" => [],
        "stateMutability" => "nonpayable",
        "type" => "function"
      }
    ]

    metadata = %{
      "compiler" => %{"version" => "0.7.6+commit.7338295f"},
      "output" => %{"abi" => abi},
      "settings" => %{
        "compilationTarget" => %{"Storage.sol" => "Storage"},
        "evmVersion" => "istanbul",
        "optimizer" => %{"enabled" => false, "runs" => 200},
        "libraries" => %{}
      }
    }

    source = """
    pragma solidity ^0.7.6;
    contract Storage {
      uint256 number;
      function store(uint256 _number) public { number = _number; }
      function retrieve() public view returns (uint256) { return number; }
    }
    """

    [
      %{"name" => "metadata.json", "content" => Jason.encode!(metadata), "path" => "/Storage/metadata.json"},
      %{"name" => "Storage.sol", "content" => source, "path" => "/Storage/Storage.sol"}
    ]
  end

  defp json(conn, body) do
    conn
    |> put_resp_content_type("application/json")
    |> resp(200, Jason.encode!(body))
  end
end
