defmodule Indexer.Fetcher.Arbitrum.Workers.BackfillRecovery do
  @moduledoc false

  import Ecto.Query
  alias EthereumJSONRPC.{Block, Blocks, Receipt, Receipts}
  alias Explorer.Repo

  # ArbitrumLegacyTxType receipts predate Nitro's gasUsedForL1 field.
  @legacy_transaction_type 120

  def blocks_with_missing_fields(first, last) do
    Repo.all(
      from(b in "blocks",
        left_join: t in "transactions",
        on: t.block_hash == b.hash,
        where: b.consensus == true and b.number >= ^first and b.number <= ^last,
        where:
          is_nil(b.send_count) or is_nil(b.send_root) or is_nil(b.l1_block_number) or
            (not is_nil(t.hash) and is_nil(t.gas_used_for_l1) and
               (is_nil(t.type) or t.type != ^@legacy_transaction_type)),
        distinct: true,
        order_by: b.number,
        select: b.number
      )
    )
  end

  def fetch_blocks(numbers, arguments) do
    params = Map.new(numbers, &{&1, %{number: &1}})
    requests = Blocks.requests(params, &Block.ByNumber.request(&1, false))

    with {:ok, responses} <- EthereumJSONRPC.json_rpc(requests, arguments),
         %Blocks{errors: [], blocks_params: blocks} <- Blocks.from_responses(responses, params),
         true <- Enum.sort(Enum.map(blocks, & &1.number)) == Enum.sort(numbers) do
      raw = Map.new(responses, fn %{result: block} -> {block["hash"], block} end)

      {:ok,
       Enum.map(blocks, fn block ->
         data = raw[block.hash]

         Map.merge(block, %{
           send_count: quantity(data["sendCount"]),
           send_root: data["sendRoot"],
           l1_block_number: quantity(data["l1BlockNumber"])
         })
       end)}
    else
      {:error, reason} -> {:error, reason}
      _ -> {:error, :incomplete_blocks}
    end
  end

  def fetch_by_block_numbers(numbers, arguments) do
    requests = Enum.map(numbers, &Receipts.ByBlockNumber.request(%{id: &1, number: &1}))
    fetch_receipts(requests, arguments, true)
  end

  def fetch(transactions, arguments) do
    requests =
      transactions
      |> Enum.with_index()
      |> Enum.map(fn {tx, id} -> Receipts.ByTransactionHash.request(id, tx.hash) end)

    fetch_receipts(requests, arguments, false)
  end

  defp fetch_receipts(requests, arguments, blocks?) do
    with {:ok, responses} <- EthereumJSONRPC.json_rpc(requests, arguments),
         true <- Enum.sort(Enum.map(responses, & &1.id)) == Enum.sort(Enum.map(requests, & &1.id)),
         true <- Enum.all?(responses, &match?(%{result: result} when not is_nil(result), &1)) do
      raw = Enum.flat_map(responses, fn %{result: result} -> if blocks?, do: result, else: [result] end)

      receipts =
        Enum.map(raw, fn receipt ->
          elixir = Receipt.to_elixir(receipt)

          elixir
          |> Receipt.elixir_to_params()
          |> Map.merge(%{
            gas_used_for_l1: elixir["gasUsedForL1"],
            block_hash: elixir["blockHash"],
            block_number: elixir["blockNumber"]
          })
        end)

      {:ok, %{receipts: receipts}}
    else
      {:error, reason} -> {:error, reason}
      _ -> {:error, :incomplete_receipts}
    end
  end

  def validate(blocks, receipts) do
    numbers = Enum.map(blocks, & &1.number)

    expected_blocks =
      Repo.all(
        from(b in "blocks",
          where: b.consensus == true and b.number in ^numbers,
          select: {b.number, b.hash},
          lock: "FOR SHARE"
        )
      )

    actual_blocks = Enum.map(blocks, &{&1.number, bytes(&1.hash)})

    transactions =
      Repo.all(
        from(t in "transactions",
          join: b in "blocks",
          on: b.hash == t.block_hash,
          where: b.consensus == true and b.number in ^numbers,
          select: {t.hash, t.block_hash, t.block_number, t.type}
        )
      )

    expected = Enum.map(transactions, fn {hash, block_hash, number, _} -> {hash, block_hash, number} end)
    actual = Enum.map(receipts, &{bytes(&1.transaction_hash), bytes(&1.block_hash), &1.block_number})
    legacy = MapSet.new(for {hash, _, _, @legacy_transaction_type} <- transactions, do: hash)

    if Enum.sort(expected_blocks) == Enum.sort(actual_blocks) and
         Enum.sort(expected) == Enum.sort(actual) and
         Enum.all?(blocks, &(is_integer(&1.send_count) and not is_nil(&1.send_root) and is_integer(&1.l1_block_number))) and
         Enum.all?(receipts, &(is_integer(&1.gas_used_for_l1) or MapSet.member?(legacy, bytes(&1.transaction_hash)))) do
      :ok
    else
      {:error, :incomplete_backfill}
    end
  end

  def bytes(hash) do
    {:ok, cast} = Explorer.Chain.Hash.Full.cast(hash)
    cast.bytes
  end

  defp quantity(nil), do: nil
  defp quantity(value), do: EthereumJSONRPC.quantity_to_integer(value)
end
