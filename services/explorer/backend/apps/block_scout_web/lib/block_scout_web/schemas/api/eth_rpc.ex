defmodule BlockScoutWeb.Schemas.API.EthRPC do
  @moduledoc "JSON-RPC envelopes and Ethereum result types."
  alias BlockScoutWeb.Schemas.API.V2.General
  alias OpenApiSpex.Schema

  def response do
    entry = %Schema{oneOf: [success(), error()]}
    %Schema{oneOf: [entry, %Schema{type: :array, items: entry}]}
  end

  defp success do
    object(%{jsonrpc: version(), id: id(), result: result()}, [:jsonrpc, :id, :result])
  end

  defp error do
    object(
      %{
        jsonrpc: version(),
        id: id(),
        error: %Schema{
          oneOf: [
            %Schema{type: :string},
            object(%{code: %Schema{type: :integer}, message: %Schema{type: :string}}, [:code, :message])
          ]
        }
      },
      [:jsonrpc, :id, :error]
    )
  end

  defp version, do: %Schema{type: :string, enum: ["2.0"]}
  defp id, do: %Schema{oneOf: [%Schema{type: :integer}, %Schema{type: :string}]}
  defp quantity, do: %Schema{type: :string, pattern: ~r/^0x[0-9a-fA-F]+$/, nullable: true}

  defp result do
    %Schema{nullable: true, anyOf: [General.HexString, %Schema{type: :array, items: log()}, transaction(), receipt()]}
  end

  defp log do
    object(
      %{
        address: General.AddressHash,
        blockHash: General.FullHash,
        blockNumber: quantity(),
        data: General.HexString,
        logIndex: quantity(),
        removed: %Schema{type: :boolean},
        topics: %Schema{type: :array, items: General.FullHash, maxItems: 4},
        transactionHash: General.FullHash,
        transactionIndex: quantity()
      },
      ~w(address blockHash blockNumber data logIndex removed topics transactionHash transactionIndex)a
    )
  end

  defp transaction do
    fields =
      Map.new(
        ~w(blockNumber gas gasPrice nonce transactionIndex value type chainId v r s maxFeePerGas maxPriorityFeePerGas yParity maxFeePerBlobGas)a,
        &{&1, quantity()}
      )

    fields =
      Map.merge(fields, %{
        blockHash: %Schema{allOf: [General.FullHash], nullable: true},
        hash: General.FullHash,
        from: General.AddressHash,
        to: General.AddressHashNullable,
        input: General.HexString,
        accessList: %Schema{
          type: :array,
          items:
            object(%{address: General.AddressHash, storageKeys: %Schema{type: :array, items: General.FullHash}}, [
              :address,
              :storageKeys
            ])
        },
        blobVersionedHashes: %Schema{type: :array, items: General.FullHash},
        authorizationList: %Schema{
          type: :array,
          items:
            object(
              %{
                chainId: quantity(),
                nonce: quantity(),
                address: General.AddressHash,
                r: quantity(),
                s: quantity(),
                yParity: quantity()
              },
              ~w(chainId nonce address r s yParity)a
            )
        }
      })

    object(
      fields,
      ~w(blockHash blockNumber from gas gasPrice hash input nonce to transactionIndex value type chainId v r s)a
    )
  end

  defp receipt do
    fields =
      Map.new(
        ~w(blockNumber cumulativeGasUsed effectiveGasPrice gasUsed status transactionIndex type blobGasPrice blobGasUsed)a,
        &{&1, quantity()}
      )

    fields =
      Map.merge(fields, %{
        blockHash: General.FullHash,
        contractAddress: General.AddressHashNullable,
        from: General.AddressHash,
        to: General.AddressHashNullable,
        transactionHash: General.FullHash,
        logs: %Schema{type: :array, items: log()},
        logsBloom: General.HexString
      })

    object(
      fields,
      ~w(blockHash blockNumber contractAddress cumulativeGasUsed effectiveGasPrice from gasUsed logs logsBloom status to transactionHash transactionIndex type)a
    )
  end

  defp object(properties, required),
    do: %Schema{type: :object, properties: properties, required: required, additionalProperties: false}
end
