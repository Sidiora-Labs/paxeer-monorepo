defmodule BlockScoutWeb.API.V2.PaxeerX.OpenApiSpecTest do
  use BlockScoutWeb.ConnCase

  alias Explorer.Chain.Address
  alias Explorer.Chain.PaxeerX.AccountBinding
  alias Explorer.Repo
  alias OpenApiSpex.{Cast, MediaType, Operation, PathItem, Response, Schema}

  @pax_address "pax1005qwm6w5jj26zq8tsjs3eyp6my5d5fthrlqk3"
  @kernel_key "3f1a9b0c5d2e4f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8"
  @asset_id "0x" <> String.duplicate("0", 63) <> "1"

  @paxeer_x_paths [
    "/v2/addresses/{address_hash_param}/unified",
    "/v2/paxeer-x/capabilities",
    "/v2/paxeer-x/archive/anchors",
    "/v2/paxeer-x/archive/receipts",
    "/v2/paxeer-x/archive/receipts/{id}",
    "/v2/paxeer-x/feed/markets",
    "/v2/paxeer-x/feed/markets/{market_id}/updates",
    "/v2/paxeer-x/feed/updates",
    "/v2/paxeer-x/feed/updates/{hash}",
    "/v2/transactions/{transaction_hash_param}/status",
    "/v2/transactions/{transaction_hash_param}/signature"
  ]

  test "openapi_lists_archive_paths_not_live_kernel_paths" do
    spec = BlockScoutWeb.Specs.Public.spec()

    refute Map.has_key?(spec.paths, "/v2/paxeer-x/anchors")
    refute Map.has_key?(spec.paths, "/v2/paxeer-x/receipts")
    refute Map.has_key?(spec.paths, "/v2/paxeer-x/receipts/{id}")

    for path <- [
          "/v2/paxeer-x/archive/anchors",
          "/v2/paxeer-x/archive/receipts",
          "/v2/paxeer-x/archive/receipts/{id}"
        ] do
      assert %PathItem{get: %Operation{}} = Map.get(spec.paths, path),
             "the specification carries no archive GET operation for #{path}"
    end
  end

  describe "the generated specification" do
    test "lists the eleven Paxeer X paths with no registration beyond the API router" do
      spec = BlockScoutWeb.Specs.Public.spec()

      for path <- @paxeer_x_paths do
        assert %PathItem{get: %Operation{} = operation} = Map.get(spec.paths, path),
               "the specification carries no GET operation for #{path}"

        assert operation.summary
        assert "paxeer-x" in operation.tags
        assert Enum.any?(operation.parameters, &(&1.name == :apikey))
        assert %Response{content: %{"application/json" => %MediaType{}}} = success_response(operation)
      end
    end
  end

  describe "the declared response schemas" do
    test "reject a unified account document that lost a field", %{conn: conn} do
      address = insert(:address)
      block = insert(:block, number: 300)
      transaction = :transaction |> insert(from_address: address) |> with_block(block)

      insert_binding(address, block, transaction)

      insert_custody_event(block, transaction,
        log_index: 1,
        kind: :custody_deposit,
        direction: :deposit,
        amount: Decimal.new(42),
        address_hash: to_string(address.hash),
        account: "0x" <> @kernel_key
      )

      path = "/api/v2/addresses/#{Address.checksum(address.hash)}/unified"
      response = json_response(get(conn, path), 200)

      refute Enum.empty?(response["balances"])
      refute Enum.empty?(response["activity"])

      {spec, schema} = declared_schema("/v2/addresses/{address_hash_param}/unified")

      assert_absent_rejected(response, schema, spec)
      assert_absent_rejected_in_list(response, "balances", schema, spec)
      assert_absent_rejected_in_list(response, "activity", schema, spec)
    end

    test "reject a capabilities document that lost a field", %{conn: conn} do
      response = json_response(get(conn, "/api/v2/paxeer-x/capabilities"), 200)
      {spec, schema} = declared_schema("/v2/paxeer-x/capabilities")

      assert_absent_rejected(response, schema, spec)
    end

    test "reject a transaction status that lost a field", %{conn: conn} do
      block = insert(:block, number: 4_242)
      transaction = :transaction |> insert() |> with_block(block)

      response = json_response(get(conn, "/api/v2/transactions/#{transaction.hash}/status"), 200)
      {spec, schema} = declared_schema("/v2/transactions/{transaction_hash_param}/status")

      assert_absent_rejected(response, schema, spec)
    end
  end

  defp declared_schema(path) do
    spec = BlockScoutWeb.Specs.Public.spec()

    %PathItem{get: %Operation{} = operation} = Map.fetch!(spec.paths, path)

    %Response{content: %{"application/json" => %MediaType{schema: schema}}} = success_response(operation)

    {spec, resolve(schema, spec)}
  end

  defp success_response(%Operation{responses: responses}),
    do: Map.get(responses, 200) || Map.get(responses, "200")

  defp resolve(schema, spec), do: OpenApiSpex.resolve_schema(schema, spec.components.schemas)

  defp assert_absent_rejected(document, %Schema{} = schema, spec) do
    assert {:ok, _document} = cast(document, schema, spec)

    for field <- required_fields(schema) do
      assert {:error, _errors} = cast(Map.delete(document, field), schema, spec),
             "the declared schema accepts a document without #{field}"
    end
  end

  defp assert_absent_rejected_in_list(document, key, %Schema{} = schema, spec) do
    item_schema = schema.properties |> Map.fetch!(String.to_existing_atom(key)) |> Map.fetch!(:items) |> resolve(spec)

    [item | rest] = Map.fetch!(document, key)

    for field <- required_fields(item_schema) do
      broken = Map.put(document, key, [Map.delete(item, field) | rest])

      assert {:error, _errors} = cast(broken, schema, spec),
             "the declared schema accepts a #{key} item without #{field}"
    end
  end

  defp required_fields(%Schema{required: required}) when is_list(required),
    do: Enum.map(required, &to_string/1)

  defp cast(document, %Schema{} = schema, spec),
    do: Cast.cast(%Cast{value: document, schema: schema, schemas: spec.components.schemas})

  defp insert_binding(address, block, transaction) do
    did = "did:layerx:" <> @kernel_key

    attributes = %{
      transaction_hash: transaction.hash,
      log_index: 0,
      block_hash: block.hash,
      block_number: block.number,
      block_consensus: true,
      evm_address_hash: to_string(address.hash),
      pax_address: @pax_address,
      layerx_did: did,
      layerx_account: "agent:" <> did <> ":main",
      bound: true,
      nonce: 1
    }

    %AccountBinding{}
    |> AccountBinding.changeset(attributes)
    |> Repo.insert!()
  end

  defp insert_custody_event(block, transaction, fields) do
    attributes =
      Map.merge(
        %{
          transaction_hash: transaction.hash,
          block_hash: block.hash,
          block_number: block.number,
          block_consensus: true,
          asset_id: @asset_id
        },
        Map.new(fields)
      )

    Repo.query!(
      """
      INSERT INTO lx_custody_events (
        transaction_hash, log_index, block_hash, block_number, block_consensus,
        kind, direction, asset_id, amount, address_hash, account, inserted_at, updated_at
      ) VALUES (
        $1, $2, $3, $4, $5, $6::lx_custody_event_kind, $7::lx_custody_event_direction,
        decode(substr($8, 3), 'hex'), $9, decode(substr($10, 3), 'hex'),
        decode(substr($11, 3), 'hex'), NOW(), NOW()
      )
      """,
      [
        attributes.transaction_hash.bytes,
        attributes.log_index,
        attributes.block_hash.bytes,
        attributes.block_number,
        attributes.block_consensus,
        to_string(attributes.kind),
        to_string(attributes.direction),
        attributes.asset_id,
        attributes[:amount],
        attributes[:address_hash],
        attributes[:account]
      ]
    )
  end
end
