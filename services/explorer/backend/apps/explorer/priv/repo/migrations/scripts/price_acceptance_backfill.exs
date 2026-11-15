defmodule Explorer.Repo.Migrations.PriceAcceptanceBackfill do
  use GenServer

  import Ecto.Query

  alias Explorer.Chain.Block
  alias Explorer.PaxeerX.CosmosRest
  alias Explorer.Repo

  @table "paxeer_x_price_acceptance_backfill"

  def start_link(options \\ []), do: GenServer.start_link(__MODULE__, options)

  def retain(server, number, transactions, results),
    do: GenServer.call(server, {:retain, number, transactions, results}, :infinity)

  def step(server, batch_size) when is_integer(batch_size) and batch_size > 0,
    do: GenServer.call(server, {:step, batch_size}, :infinity)

  def pending? do
    %{rows: [[pending]]} =
      Repo.query!("""
      SELECT EXISTS (
        SELECT 1 FROM #{@table} j JOIN blocks b ON b.hash = j.block_hash
        WHERE j.completed = false AND b.consensus = true
      )
      """)

    pending
  end

  def held_jobs do
    case Repo.query!("""
         SELECT count(*) OVER (), j.block_number, j.error
         FROM #{@table} j JOIN blocks b ON b.hash = j.block_hash
         WHERE j.completed = false AND b.consensus = true AND j.error IS NOT NULL
         ORDER BY j.block_number, j.block_hash LIMIT 1
         """).rows do
      [] -> nil
      [[count, number, error]] -> {count, number, error}
    end
  end

  @impl GenServer
  def init(options), do: {:ok, Keyword.put_new(options, :acceptance_backfill_concurrency, 4)}

  @impl GenServer
  def handle_call({:retain, number, transactions, results}, _from, options) do
    result =
      with {:ok, _updates} <- decode(transactions, results) do
        case Repo.query!(
               """
               UPDATE #{@table} SET transactions = $2, results = $3, error = NULL
               WHERE block_number = $1 AND completed = false
                 AND transactions IS NULL AND results IS NULL
               """,
               [number, transactions, %{"txs_results" => results}]
             ) do
          %{num_rows: 1} -> :ok
          _ -> {:error, :not_awaiting_retained_data}
        end
      end

    {:reply, result, options}
  end

  def handle_call({:step, batch_size}, _from, options) do
    {:reply, run_step(batch_size, options), options}
  end

  defp run_step(batch_size, options) do
    concurrency = Keyword.get(options, :acceptance_backfill_concurrency, 4)

    case next_jobs(concurrency) do
      [] ->
        case Repo.query!("SELECT EXISTS (SELECT 1 FROM #{@table} WHERE completed = false)").rows do
          [[true]] -> :deferred
          [[false]] -> :complete
        end

      jobs ->
        jobs
        |> read_jobs(options, concurrency)
        |> apply_jobs(batch_size)
    end
  rescue
    error -> {:error, {:replay_failed, Exception.message(error)}}
  end

  defp read_jobs(jobs, options, concurrency) do
    results =
      Task.async_stream(jobs, &read_job(&1, options),
        max_concurrency: concurrency,
        ordered: true,
        timeout: :infinity
      )

    jobs
    |> Enum.zip(results)
    |> Enum.map(fn {job, result} ->
      case result do
        {:ok, read_result} -> {job, read_result}
        {:exit, reason} -> {job, {:error, {:replay_failed, inspect(reason)}}}
      end
    end)
    |> Enum.sort_by(fn {[hash, number, _transactions, _results], _result} -> {number, hash} end)
  end

  defp apply_jobs(reads, batch_size) do
    {count, reason} =
      Enum.reduce(reads, {0, nil}, fn {[_hash, _number, _transactions, _results] = job, result}, {count, reason} ->
        case apply_job(job, result, batch_size) do
          {:ok, completed} -> {count + completed, reason}
          {:error, failure} -> {count, reason || failure}
          :deferred -> {count, reason || :deferred}
        end
      end)

    finalize_step(count, reason)
  end

  defp apply_job([hash, _number, _transactions, _results], {:ok, updates}, batch_size) do
    case apply_batch(hash, updates, batch_size) do
      {:error, reason} -> hold_job(hash, reason)
      result -> result
    end
  rescue
    error -> hold_job(hash, {:replay_failed, Exception.message(error)})
  end

  defp apply_job([hash, _number, _transactions, _results], {:error, reason}, _batch_size),
    do: hold_job(hash, reason)

  defp read_job([hash, number, transactions, results], options) do
    with {:ok, transactions, results} <- retained(hash, transactions, results, number, options),
         {:ok, updates} <- decode(transactions, results) do
      {:ok, updates}
    end
  rescue
    error -> {:error, {:replay_failed, Exception.message(error)}}
  end

  defp finalize_step(_count, :deferred), do: :deferred
  defp finalize_step(_count, reason) when not is_nil(reason), do: {:error, reason}
  defp finalize_step(count, nil), do: {:ok, count}

  defp next_jobs(limit) do
    case Repo.query!(
           """
           SELECT j.block_hash, j.block_number, j.transactions, j.results
           FROM #{@table} j JOIN blocks b ON b.hash = j.block_hash
           WHERE j.completed = false AND b.consensus = true
           ORDER BY (j.error IS NOT NULL), j.block_number, j.block_hash LIMIT $1
           """,
           [limit]
         ).rows do
      jobs -> jobs
    end
  end

  defp hold_job(hash, reason) do
    Repo.query!("UPDATE #{@table} SET error = $2 WHERE block_hash = $1", [hash, inspect(reason)])
    {:error, reason}
  end

  defp retained(hash, nil, nil, number, options) do
    with {:ok, transactions} <- CosmosRest.fetch_block_txs(Keyword.get(options, :cometbft_rpc_urls, []), number),
         {:ok, results} <- CosmosRest.fetch_block_results(Keyword.get(options, :cometbft_rpc_urls, []), number),
         {:ok, _updates} <- decode(transactions, results) do
      Repo.query!(
        """
        UPDATE #{@table} SET transactions = $2, results = $3
        WHERE block_hash = $1 AND transactions IS NULL AND results IS NULL
        """,
        [hash, transactions, %{"txs_results" => results}]
      )

      %{rows: [[stored_transactions, %{"txs_results" => stored_results}]]} =
        Repo.query!("SELECT transactions, results FROM #{@table} WHERE block_hash = $1", [hash])

      {:ok, stored_transactions, stored_results}
    end
  end

  defp retained(_hash, transactions, %{"txs_results" => results}, _number, _options),
    do: {:ok, transactions, results}

  defp decode(%{"txs" => txs, "raw" => raw} = transactions, results)
       when is_list(txs) and is_list(raw) and is_list(results) and length(txs) == length(raw),
       do: decode_entries(transactions, results)

  defp decode(%{"txs" => _}, _results), do: {:error, :malformed_retained_block}

  defp decode(%{"raw" => raw} = transactions, results) when is_list(raw) and is_list(results),
    do: decode_entries(transactions, results)

  defp decode(_transactions, _results), do: {:error, :malformed_retained_block}

  defp decode_entries(transactions, results) do
    with {:ok, submissions} <- CosmosRest.decode_block_txs(transactions) do
      CosmosRest.decode_price_updates(submissions, results, length(transactions["raw"]))
    end
  end

  defp apply_batch(hash, updates, batch_size) do
    decoded =
      for update <- updates,
          observation <- update.observations,
          into: %{} do
        <<"0x", hex::binary>> = update.hash
        key = {Base.decode16!(hex, case: :mixed), update.index, observation.market_id}
        {key, {update.status, observation}}
      end

    Repo.transaction(fn ->
      %{rows: [[through_index, through_market_id, completed]]} =
        Repo.query!(
          """
          SELECT through_index, through_market_id, completed FROM #{@table}
          WHERE block_hash = $1 FOR UPDATE
          """,
          [hash]
        )

      block = Repo.one(from(b in Block, where: b.hash == ^hash and b.consensus == true, lock: "FOR SHARE"))
      if is_nil(block), do: Repo.rollback(:non_consensus_block)
      if completed, do: Repo.rollback(:already_completed)

      # A job that has not advanced its cursor yet resets this block's observations
      # to unknown here, inside the same transaction as its first batch, so the
      # reset is bounded to one block, never runs as a whole-table statement in a
      # migration, and is redone with the batch if the transaction does not commit.
      if through_index == -1 and through_market_id == -1 do
        Repo.query!(
          """
          UPDATE paxeer_x_price_observations o SET accepted = NULL
          FROM paxeer_x_price_updates u
          WHERE u.hash = o.update_hash AND u.block_hash = $1
          """,
          [hash]
        )
      end

      %{rows: rows} =
        Repo.query!(
          """
          SELECT o.update_hash, u.index, o.market_id, u.status,
                 o.sequence, o.index_price, o.mark_price, o.source_timestamp_ms
          FROM paxeer_x_price_observations o
          JOIN paxeer_x_price_updates u ON u.hash = o.update_hash
          WHERE u.block_hash = $1 AND o.accepted IS NULL
            AND (u.index, o.market_id) > ($2, $3)
          ORDER BY u.index, o.market_id LIMIT $4 FOR UPDATE OF o, u
          """,
          [hash, through_index, through_market_id, batch_size]
        )

      Enum.each(rows, fn [update_hash, index, market_id, status, sequence, index_price, mark_price, timestamp] ->
        case Map.fetch(decoded, {update_hash, index, market_id}) do
          {:ok, {^status, observation}} ->
            unless observation.sequence == sequence and
                     Decimal.equal?(index_price, Decimal.new(observation.index_price)) and
                     Decimal.equal?(mark_price, Decimal.new(observation.mark_price)) and
                     observation.source_timestamp_ms == timestamp do
              Repo.rollback(:retained_observation_mismatch)
            end

            Repo.query!(
              """
              UPDATE paxeer_x_price_observations SET accepted = $3
              WHERE update_hash = $1 AND market_id = $2
              """,
              [update_hash, market_id, observation.accepted]
            )

          _ ->
            Repo.rollback(:retained_submission_mismatch)
        end
      end)

      {index, market_id} =
        case List.last(rows) do
          nil -> {through_index, through_market_id}
          [_hash, index, market_id | _] -> {index, market_id}
        end

      %{rows: [[remaining]]} =
        Repo.query!(
          """
          SELECT count(*) FROM paxeer_x_price_observations o
          JOIN paxeer_x_price_updates u ON u.hash = o.update_hash
          WHERE u.block_hash = $1 AND o.accepted IS NULL
          """,
          [hash]
        )

      if rows == [] and remaining > 0, do: Repo.rollback(:backfill_cursor_conflict)

      Repo.query!(
        """
        UPDATE #{@table}
        SET through_index = $2, through_market_id = $3, completed = $4, error = NULL
        WHERE block_hash = $1
        """,
        [hash, index, market_id, remaining == 0]
      )

      length(rows)
    end)
  end
end
