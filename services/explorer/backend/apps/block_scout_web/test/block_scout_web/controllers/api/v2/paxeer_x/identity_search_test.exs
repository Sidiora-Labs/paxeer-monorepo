defmodule BlockScoutWeb.API.V2.PaxeerX.IdentitySearchTest do
  use BlockScoutWeb.ConnCase

  alias Explorer.Chain.Address
  alias Explorer.Chain.Import
  alias Explorer.Chain.PaxeerX.Identity
  alias Indexer.Transform.PaxeerXLogs

  @pax_address "pax1005qwm6w5jj26zq8tsjs3eyp6my5d5fthrlqk3"
  @kernel_key "3f1a9b0c5d2e4f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8"

  setup do
    {:ok, address_bytes} = Identity.pax_address_bytes(@pax_address)
    address = insert(:address, hash: address_bytes)
    did = "did:layerx:" <> @kernel_key
    kernel_account = "agent:" <> did <> ":main"

    import_binding(address, did, kernel_account, true, 0)

    {:ok, address: address, did: did, kernel_account: kernel_account}
  end

  describe "GET /api/v2/search" do
    test "a pax bech32 string finds the account it is bound to", %{conn: conn, address: address} do
      response = json_response(get(conn, "/api/v2/search", %{"q" => @pax_address}), 200)

      assert [%{"type" => "address", "address_hash" => address_hash}] = response["items"]
      assert address_hash == Address.checksum(address.hash)
    end

    test "a DID finds the account it is bound to", %{conn: conn, address: address, did: did} do
      response = json_response(get(conn, "/api/v2/search", %{"q" => did}), 200)

      assert [%{"type" => "address", "address_hash" => address_hash}] = response["items"]
      assert address_hash == Address.checksum(address.hash)
    end

    test "a kernel account id finds the account it is bound to", %{
      conn: conn,
      address: address,
      kernel_account: kernel_account
    } do
      response = json_response(get(conn, "/api/v2/search", %{"q" => kernel_account}), 200)

      assert [%{"type" => "address", "address_hash" => address_hash}] = response["items"]
      assert address_hash == Address.checksum(address.hash)
    end

    test "a bare kernel key finds the account it is bound to", %{conn: conn, address: address} do
      response = json_response(get(conn, "/api/v2/search", %{"q" => @kernel_key}), 200)

      assert [%{"type" => "address", "address_hash" => address_hash}] = response["items"]
      assert address_hash == Address.checksum(address.hash)
    end

    test "an unbound DID finds nothing", %{conn: conn} do
      unbound = "did:layerx:" <> String.duplicate("ab", 32)

      response = json_response(get(conn, "/api/v2/search", %{"q" => unbound}), 200)

      assert response["items"] == []
    end

    test "an identity the newest log unbound finds nothing", %{conn: conn, address: address, did: did} do
      kernel_account = "agent:" <> did <> ":main"

      import_binding(address, did, kernel_account, false, 1)

      response = json_response(get(conn, "/api/v2/search", %{"q" => did}), 200)

      assert response["items"] == []
    end
  end

  def import_binding(address, did, _kernel_account, bound, log_index) do
    block = insert(:block, number: 10 + log_index)
    transaction = :transaction |> insert() |> with_block(block)
    "did:layerx:" <> key = did
    "0x" <> evm = to_string(address.hash)

    # ABI event input, as emitted by addr; neither native-identity column is injected.
    log = %{
      transaction_hash: transaction.hash,
      index: log_index,
      block_hash: block.hash,
      block_number: block.number,
      address_hash: "0x0000000000000000000000000000000000001004",
      first_topic:
        if(bound,
          do: "0x79b5f3e386759c93c56bf67a868c5f49afe6afaf286aa99bbc8e800ca4b7c856",
          else: "0xce3f8d62ef8d76530f28a650078b3a45e48fbc97596eeb704b1bb00d3946847d"
        ),
      second_topic: "0x" <> String.duplicate("0", 24) <> evm,
      third_topic: "0x" <> key,
      fourth_topic: nil,
      data: "0x" <> Base.encode16(<<log_index::256>>, case: :lower)
    }

    assert %{lx_account_bindings: [row], undecoded_log_count: 0} = PaxeerXLogs.parse([log])

    assert {:ok, %{insert_paxeer_x_account_bindings: [_]}} =
             Import.all(%{paxeer_x_account_bindings: %{params: [row]}})

    %{row: row, block: block, transaction: transaction}
  end
end
