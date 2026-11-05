defmodule EthereumJSONRPC.HTTP.Throttle do
  @moduledoc """
  Per-endpoint guard in front of the JSON-RPC HTTP transport.

  The indexer talks to a single full node, so an unbounded fan-out of archive-style requests is
  enough to take that node down. Every HTTP call made by `EthereumJSONRPC.HTTP` asks this process
  for a slot first and only then opens the connection.

  Four limits are applied per endpoint URL:

    * `:max_inflight` - how many JSON-RPC HTTP requests may be in flight at once;
    * `:rps` - a token bucket, refilled continuously, that caps requests per second. A batch costs
      one token per sub-request, clamped to the bucket capacity, so one large batch cannot consume
      more than a second of budget;
    * `:trace_max_inflight` - a lower concurrency cap for tracing calls (`debug_trace*`, `trace_*`);
    * `:archive_max_inflight` - a lower concurrency cap for historical state reads, that is
      `eth_getBalance`, `eth_getCode`, `eth_getStorageAt` and `eth_call` against a block other than
      `latest` or `pending`.

  A batch counts as a single in-flight request, and callers that cannot be admitted are queued in
  arrival order up to `:max_queue` waiting calls across all endpoints (default 1,024). The
  queue is scanned in order on every change, and an entry that cannot be admitted is skipped rather
  than blocking the ones behind it, so a saturated trace cap never stalls ordinary requests.

  Identical single requests that arrive while the same request is already in flight are coalesced:
  the later callers wait for the in-flight call and receive its response instead of opening another
  connection. Only the safelisted read methods in `@coalescable_methods` and tracing calls are
  coalesced, and only single requests, never batches. `EthereumJSONRPC.HTTP` reads `result`/`error`
  out of a single-request response and ignores the response `id`, so sharing one response between
  callers that used different ids is transparent to them.

  `:request_timeout` (default 60,000 ms) is one absolute budget for admission and HTTP execution.
  Full queues return `{:error, {:throttle, :overloaded}}`; an absent guard returns
  `{:error, {:throttle, :unavailable}}`; expired work returns
  `{:error, {:throttle, :deadline_exceeded}}`. Expired and abandoned executions are stopped before
  their slots are released. No request is ever let through unmetered.

  Outstanding admissions and rate budgets live in an ETS ledger whose heir is the supervising
  process. A replacement guard recovers the active executions before admitting new work, including
  across a terminate/restart of its child. Completed tickets retain their unique references, so
  delayed completion messages cannot release a replacement admission.

  Queue depth, in-flight counts and admission wait time are exported through
  `EthereumJSONRPC.HTTP.Throttle.Metrics`.
  """

  use GenServer

  alias EthereumJSONRPC.HTTP.Throttle.Metrics

  @classes [:default, :archive, :trace]

  @default_max_inflight 32
  @default_rps 200
  @default_trace_max_inflight 4
  @default_archive_max_inflight 8
  @default_coalesce? true
  @default_max_queue 1_024
  @default_request_timeout 60_000
  @ledger __MODULE__.Admissions

  # Upper bound on how long the guard sleeps before it re-examines a non-empty queue.
  @max_pump_interval 50

  @trace_method_prefixes ~w(debug_trace trace_)

  @historical_block_parameter_index %{
    "eth_call" => 1,
    "eth_getBalance" => 1,
    "eth_getCode" => 1,
    "eth_getStorageAt" => 2
  }

  @latest_block_parameters ~w(latest pending)

  @coalescable_methods ~w(
    eth_blockNumber
    eth_call
    eth_chainId
    eth_getBalance
    eth_getBlockByHash
    eth_getBlockByNumber
    eth_getBlockReceipts
    eth_getCode
    eth_getLogs
    eth_getStorageAt
    eth_getTransactionByHash
    eth_getTransactionCount
    eth_getTransactionReceipt
    eth_getUncleByBlockHashAndIndex
    eth_getUncleByBlockNumberAndIndex
    eth_syncing
  )

  @type class :: :default | :archive | :trace

  @type ticket :: %{
          url: String.t() | nil,
          class: class(),
          weight: pos_integer(),
          key: term(),
          ref: reference(),
          deadline: integer()
        }

  @spec start_link(keyword()) :: GenServer.on_start()
  def start_link(opts \\ []) do
    {name, opts} = Keyword.pop(opts, :name, __MODULE__)

    GenServer.start_link(__MODULE__, opts, name: name)
  end

  @doc """
  Runs `fun` once a slot for `request` on `url` is available.

  `request` is the single JSON-RPC request map or the batch about to be sent, and `fun` is the
  function that performs the HTTP call. Execution shares the admission deadline, including when
  the HTTP client has a longer timeout. Results and exceptions of `fun` are preserved.
  """
  @spec request(String.t() | nil, map() | [map()], (-> result)) :: result when result: term()
  def request(url, request, fun) when is_function(fun, 0) do
    deadline = monotonic_ms() + build_config([]).request_timeout
    caller = self()
    callers = [caller | Process.get(:"$callers", [])]
    result_ref = make_ref()

    {worker, monitor} =
      spawn_monitor(fn ->
        Process.put(:"$callers", callers)
        watch_execution(caller, self(), deadline)

        outcome =
          try do
            {:return, execute(url, request, fun, deadline)}
          catch
            kind, reason -> {:raise, kind, reason, __STACKTRACE__}
          end

        send(caller, {result_ref, outcome})
      end)

    receive do
      {^result_ref, outcome} ->
        receive do
          {:DOWN, ^monitor, :process, ^worker, _reason} -> :ok
        end

        case outcome do
          {:return, result} -> result
          {:raise, kind, reason, stacktrace} -> :erlang.raise(kind, reason, stacktrace)
        end

      {:DOWN, ^monitor, :process, ^worker, _reason} ->
        execution_error(deadline)
    after
      remaining(deadline) ->
        Process.exit(worker, :kill)

        receive do
          {:DOWN, ^monitor, :process, ^worker, _reason} -> :ok
        end

        receive do
          {^result_ref, _outcome} -> :ok
        after
          0 -> :ok
        end

        {:error, {:throttle, :deadline_exceeded}}
    end
  end

  defp execute(url, request, fun, deadline) do
    {class, weight} = classify(request)
    key = coalesce_key(url, request)

    case acquire(url, class, weight, key, deadline) do
      {:run, ticket} -> run(ticket, fun)
      {:coalesced, result} -> result
      {:error, _} = error -> error
    end
  end

  defp watch_execution(caller, worker, deadline) do
    spawn(fn ->
      caller_monitor = Process.monitor(caller)
      worker_monitor = Process.monitor(worker)

      receive do
        {:DOWN, ^caller_monitor, :process, ^caller, _reason} -> Process.exit(worker, :kill)
        {:DOWN, ^worker_monitor, :process, ^worker, _reason} -> :ok
      after
        remaining(deadline) -> Process.exit(worker, :kill)
      end
    end)
  end

  defp execution_error(deadline) do
    if remaining(deadline) == 0,
      do: {:error, {:throttle, :deadline_exceeded}},
      else: {:error, {:throttle, :unavailable}}
  end

  @doc """
  Current in-flight counts and queue depth for `url`.
  """
  @spec stats(String.t() | nil) :: %{
          inflight: %{class() => non_neg_integer()},
          inflight_total: non_neg_integer(),
          queue_depth: non_neg_integer()
        }
  def stats(url) do
    GenServer.call(__MODULE__, {:stats, url})
  end

  @doc """
  Classifies `request` into the class whose cap applies to it and the number of rate budget tokens
  it costs.
  """
  @spec classify(map() | [map()]) :: {class(), pos_integer()}
  def classify(request) when is_map(request), do: {classify_method(request), 1}

  def classify(batch) when is_list(batch) do
    {class, weight} =
      Enum.reduce(batch, {:default, 0}, fn request, {class, weight} ->
        {heavier(class, classify_method(request)), weight + 1}
      end)

    {class, max(weight, 1)}
  end

  @impl GenServer
  def init(opts) do
    ensure_ledger()
    state = recover(%{config: build_config(opts), endpoints: %{}, waiters: %{}, monitors: %{}, entries: %{}})
    publish(state)
    {:ok, state}
  end

  @impl GenServer
  def handle_call({:acquire, url, class, weight, key, deadline}, {pid, _tag} = from, state) do
    state = pump(state, url)
    endpoint = endpoint(state, url)

    entry = %{
      from: from,
      pid: pid,
      class: class,
      weight: min(weight, state.config.rps),
      key: key,
      queued_at: monotonic_ms(),
      deadline: deadline
    }

    waiting? =
      (state.config.coalesce? and coalescable_entry?(endpoint, entry)) or
        not admissible?(endpoint, state.config, entry)

    cond do
      remaining(deadline) == 0 ->
        {:reply, {:error, {:throttle, :deadline_exceeded}}, state}

      waiting? and waiting_count(state) >= state.config.max_queue ->
        {:reply, {:error, {:throttle, :overloaded}}, state}

      true ->
        ref = Process.monitor(pid)
        timer = Process.send_after(self(), {:expire, ref}, remaining(deadline))
        entry = Map.merge(entry, %{ref: ref, timer: timer})
        state = %{state | entries: Map.put(state.entries, ref, entry), monitors: Map.put(state.monitors, ref, ref)}

        state =
          state
          |> put_endpoint(url, %{endpoint | queue: [entry | endpoint.queue]})
          |> put_waiter(ref, {:queued, url})
          |> pump(url)

        {:noreply, state}
    end
  end

  def handle_call({:stats, url}, _from, state) do
    endpoint = endpoint(state, url)

    stats = %{
      inflight: endpoint.inflight,
      inflight_total: endpoint.inflight_total,
      queue_depth:
        length(endpoint.queue) +
          Enum.reduce(endpoint.leaders, 0, fn {_key, leader}, sum -> sum + length(leader.followers) end)
    }

    {:reply, stats, state}
  end

  @impl GenServer
  def handle_cast({:complete, ticket, result}, state) do
    {:noreply, state |> finish(ticket, {:coalesced, result}) |> pump(ticket.url)}
  end

  def handle_cast({:release, ticket}, state) do
    {:noreply, state |> finish(ticket, :release) |> pump(ticket.url)}
  end

  def handle_cast({:abandon, ticket}, state) do
    {:noreply, state |> finish(ticket, :promote) |> pump(ticket.url)}
  end

  @impl GenServer
  def handle_info({:pump, url}, state) do
    endpoint = %{endpoint(state, url) | timer: nil}

    {:noreply, state |> put_endpoint(url, endpoint) |> pump(url)}
  end

  def handle_info({:DOWN, ref, :process, _pid, _reason}, state) do
    ref = Map.get(state.monitors, ref, ref)
    {:noreply, remove_waiter(state, ref)}
  end

  def handle_info({:expire, ref}, state) do
    case Map.fetch(state.entries, ref) do
      {:ok, entry} ->
        case Map.get(state.waiters, ref) do
          {:queued, _url} ->
            GenServer.reply(entry.from, {:error, {:throttle, :deadline_exceeded}})
            {:noreply, remove_waiter(state, ref)}

          {:follower, _url, _key} ->
            GenServer.reply(entry.from, {:error, {:throttle, :deadline_exceeded}})
            {:noreply, remove_waiter(state, ref)}

          _ ->
            # DOWN, rather than the timer, releases an execution's slot after cancellation.
            Process.exit(entry.pid, :kill)
            {:noreply, state}
        end

      :error ->
        {:noreply, state}
    end
  end

  defp remove_waiter(state, ref) do
    case Map.fetch(state.waiters, ref) do
      {:ok, {:queued, url}} ->
        endpoint = endpoint(state, url)
        queue = Enum.reject(endpoint.queue, &(&1.ref == ref))

        state |> drop_waiter(ref) |> put_endpoint(url, %{endpoint | queue: queue}) |> pump(url)

      {:ok, {:running, url, class, weight}} ->
        ticket = %{url: url, class: class, weight: weight, key: nil, ref: ref, deadline: state.entries[ref].deadline}

        state |> finish(ticket, :promote) |> pump(url)

      {:ok, {:leader, url, key, class, weight}} ->
        ticket = %{url: url, class: class, weight: weight, key: key, ref: ref, deadline: state.entries[ref].deadline}

        state |> finish(ticket, :promote) |> pump(url)

      {:ok, {:follower, url, key}} ->
        endpoint = endpoint(state, url)

        leaders =
          case Map.fetch(endpoint.leaders, key) do
            {:ok, leader} ->
              Map.put(endpoint.leaders, key, %{
                leader
                | followers:
                    Enum.reject(leader.followers, fn {_from, follower_ref} ->
                      follower_ref == ref
                    end)
              })

            :error ->
              endpoint.leaders
          end

        state |> drop_waiter(ref) |> put_endpoint(url, %{endpoint | leaders: leaders}) |> publish()

      :error ->
        state
    end
  end

  defp acquire(url, class, weight, key, deadline) do
    GenServer.call(__MODULE__, {:acquire, url, class, weight, key, deadline}, remaining(deadline))
  catch
    :exit, {:timeout, _} -> {:error, {:throttle, :deadline_exceeded}}
    :exit, _reason -> {:error, {:throttle, :unavailable}}
  end

  defp run(ticket, fun) do
    result = fun.()
    :ets.update_element(@ledger, ticket.ref, {4, :completed})

    # Only a request that can be coalesced has to hand its response to the guard, so a batch
    # response, which is the largest payload the transport returns, is never copied into the
    # guard's mailbox.
    if is_nil(ticket.key) do
      GenServer.cast(__MODULE__, {:release, ticket})
    else
      GenServer.cast(__MODULE__, {:complete, ticket, result})
    end

    result
  catch
    kind, reason ->
      :ets.update_element(@ledger, ticket.ref, {4, :completed})
      GenServer.cast(__MODULE__, {:abandon, ticket})

      :erlang.raise(kind, reason, __STACKTRACE__)
  end

  defp build_config(opts) do
    merged = Keyword.merge(Application.get_env(:ethereum_jsonrpc, __MODULE__, []), opts)

    %{
      max_inflight: positive(merged[:max_inflight], @default_max_inflight),
      rps: positive(merged[:rps], @default_rps),
      trace_max_inflight: positive(merged[:trace_max_inflight], @default_trace_max_inflight),
      archive_max_inflight: positive(merged[:archive_max_inflight], @default_archive_max_inflight),
      max_queue: positive(merged[:max_queue], @default_max_queue),
      request_timeout: positive(merged[:request_timeout], @default_request_timeout),
      coalesce?: if(is_boolean(merged[:coalesce?]), do: merged[:coalesce?], else: @default_coalesce?)
    }
  end

  defp positive(value, _default) when is_integer(value) and value > 0, do: value
  defp positive(_value, default), do: default

  defp pump(state, url) do
    config = state.config
    endpoint = state |> endpoint(url) |> refill(config)

    arrival_order = Enum.reverse(endpoint.queue)

    {state, drained, kept} =
      Enum.reduce(arrival_order, {state, endpoint, []}, fn entry, {acc_state, acc_endpoint, kept} ->
        cond do
          remaining(entry.deadline) == 0 or not Process.alive?(entry.pid) ->
            GenServer.reply(entry.from, {:error, {:throttle, :deadline_exceeded}})
            {drop_waiter(acc_state, entry.ref), acc_endpoint, kept}

          config.coalesce? and coalescable_entry?(acc_endpoint, entry) ->
            {acc_state, acc_endpoint} = attach_follower(acc_state, acc_endpoint, url, entry)
            {acc_state, acc_endpoint, kept}

          admissible?(acc_endpoint, config, entry) ->
            {acc_state, acc_endpoint} = admit(acc_state, acc_endpoint, config, url, entry)
            {acc_state, acc_endpoint, kept}

          true ->
            {acc_state, acc_endpoint, [entry | kept]}
        end
      end)

    pumped = schedule_pump(%{drained | queue: kept}, url, config)

    state |> put_endpoint(url, pumped) |> publish()
  end

  defp admissible?(endpoint, config, entry) do
    endpoint.inflight_total < config.max_inflight and
      Map.fetch!(endpoint.inflight, entry.class) < class_limit(config, entry.class) and
      endpoint.tokens >= entry.weight
  end

  defp class_limit(config, :trace), do: config.trace_max_inflight
  defp class_limit(config, :archive), do: config.archive_max_inflight
  defp class_limit(config, :default), do: config.max_inflight

  defp coalescable_entry?(endpoint, entry) do
    not is_nil(entry.key) and Map.has_key?(endpoint.leaders, entry.key)
  end

  defp attach_follower(state, endpoint, url, entry) do
    Metrics.coalesced(entry.class)

    leaders =
      Map.update!(endpoint.leaders, entry.key, fn leader ->
        %{leader | followers: [{entry.from, entry.ref} | leader.followers]}
      end)

    {put_waiter(state, entry.ref, {:follower, url, entry.key}), %{endpoint | leaders: leaders}}
  end

  defp admit(state, endpoint, config, url, entry) do
    Metrics.wait_time(entry.class, monotonic_ms() - entry.queued_at)

    ticket = %{
      url: url,
      class: entry.class,
      weight: entry.weight,
      key: if(config.coalesce?, do: entry.key),
      ref: entry.ref,
      deadline: entry.deadline
    }

    endpoint = reserve(endpoint, entry)
    persist_rate(url, endpoint)
    :ets.insert(@ledger, {entry.ref, entry.pid, ticket, :running})
    GenServer.reply(entry.from, {:run, ticket})

    if config.coalesce? and not is_nil(entry.key) do
      leaders = Map.put(endpoint.leaders, entry.key, %{ref: entry.ref, followers: []})

      {put_waiter(state, entry.ref, {:leader, url, entry.key, entry.class, entry.weight}),
       %{endpoint | leaders: leaders}}
    else
      {put_waiter(state, entry.ref, {:running, url, entry.class, entry.weight}), endpoint}
    end
  end

  defp reserve(endpoint, entry) do
    %{
      endpoint
      | inflight_total: endpoint.inflight_total + 1,
        inflight: Map.update!(endpoint.inflight, entry.class, &(&1 + 1)),
        tokens: endpoint.tokens - entry.weight
    }
  end

  defp release(endpoint, class) do
    %{
      endpoint
      | inflight_total: max(endpoint.inflight_total - 1, 0),
        inflight: Map.update!(endpoint.inflight, class, &max(&1 - 1, 0))
    }
  end

  defp finish(state, %{url: url, class: class, key: key, ref: ref} = ticket, disposition) do
    waiter = Map.get(state.waiters, ref)

    if match?({:running, _, _, _}, waiter) or match?({:leader, _, _, _, _}, waiter) do
      :ets.delete(@ledger, ref)
      state = drop_waiter(state, ref)
      endpoint = endpoint(state, url)

      case key && Map.get(endpoint.leaders, key) do
        nil ->
          put_endpoint(state, url, release(endpoint, class))

        %{ref: leader_ref} when leader_ref != ref ->
          put_endpoint(state, url, release(endpoint, class))

        %{followers: followers} ->
          resolve_leader(state, url, endpoint, ticket, followers, disposition)
      end
    else
      state
    end
  end

  defp resolve_leader(
         state,
         url,
         endpoint,
         %{class: class, key: key},
         followers,
         {:coalesced, _} = reply
       ) do
    state =
      Enum.reduce(followers, state, fn {from, follower_ref}, acc ->
        Process.demonitor(follower_ref, [:flush])
        GenServer.reply(from, reply)
        drop_waiter(acc, follower_ref)
      end)

    endpoint = %{endpoint | leaders: Map.delete(endpoint.leaders, key)}

    put_endpoint(state, url, release(endpoint, class))
  end

  defp resolve_leader(state, url, endpoint, %{class: class, key: key}, [], :promote) do
    endpoint = %{endpoint | leaders: Map.delete(endpoint.leaders, key)}

    put_endpoint(state, url, release(endpoint, class))
  end

  defp resolve_leader(state, url, endpoint, ticket, followers, :promote) do
    %{class: class, weight: weight, key: key} = ticket
    [{from, follower_ref} | rest] = Enum.reverse(followers)
    entry = Map.fetch!(state.entries, follower_ref)

    if remaining(entry.deadline) == 0 or not Process.alive?(entry.pid) do
      GenServer.reply(from, {:error, {:throttle, :deadline_exceeded}})
      resolve_leader(drop_waiter(state, follower_ref), url, endpoint, ticket, Enum.reverse(rest), :promote)
    else
      ticket = %{ticket | ref: follower_ref, deadline: entry.deadline}
      :ets.insert(@ledger, {follower_ref, entry.pid, ticket, :running})
      GenServer.reply(from, {:run, ticket})

      endpoint = %{
        endpoint
        | leaders: Map.put(endpoint.leaders, key, %{ref: follower_ref, followers: Enum.reverse(rest)})
      }

      state
      |> put_waiter(follower_ref, {:leader, url, key, class, weight})
      |> put_endpoint(url, endpoint)
    end
  end

  defp refill(endpoint, config) do
    now = monotonic_ms()
    elapsed = now - endpoint.refilled_at

    if elapsed > 0 do
      tokens = min(config.rps * 1.0, endpoint.tokens + elapsed * config.rps / 1000)

      %{endpoint | tokens: tokens, refilled_at: now}
    else
      endpoint
    end
  end

  defp schedule_pump(%{queue: []} = endpoint, _url, _config), do: endpoint

  defp schedule_pump(%{timer: timer} = endpoint, _url, _config) when is_reference(timer),
    do: endpoint

  defp schedule_pump(endpoint, url, config) do
    cheapest = endpoint.queue |> Enum.map(& &1.weight) |> Enum.min()
    deficit = cheapest - endpoint.tokens

    delay =
      if deficit <= 0 do
        @max_pump_interval
      else
        deficit
        |> Kernel.*(1000)
        |> Kernel./(config.rps)
        |> ceil()
        |> min(@max_pump_interval)
        |> max(1)
      end

    %{endpoint | timer: Process.send_after(self(), {:pump, url}, delay)}
  end

  defp publish(state) do
    # Class-only labels represent the sum across every endpoint, including coalesced waiters.
    Enum.each(@classes, fn class ->
      queued =
        Enum.count(state.waiters, fn {ref, waiter} ->
          waiting?(waiter) and Map.fetch!(state.entries, ref).class == class
        end)

      inflight = Enum.reduce(state.endpoints, 0, fn {_url, endpoint}, sum -> sum + endpoint.inflight[class] end)
      Metrics.queue_depth(class, queued)
      Metrics.inflight(class, inflight)
    end)

    state
  end

  defp endpoint(state, url) do
    Map.get_lazy(state.endpoints, url, fn -> new_endpoint(state.config) end)
  end

  defp new_endpoint(config) do
    %{
      inflight: Map.new(@classes, &{&1, 0}),
      inflight_total: 0,
      tokens: config.rps * 1.0,
      refilled_at: monotonic_ms(),
      queue: [],
      leaders: %{},
      timer: nil
    }
  end

  defp put_endpoint(state, url, endpoint) do
    persist_rate(url, endpoint)
    %{state | endpoints: Map.put(state.endpoints, url, endpoint)}
  end

  defp put_waiter(state, ref, waiter) do
    %{state | waiters: Map.put(state.waiters, ref, waiter)}
  end

  defp drop_waiter(state, ref) do
    case Map.get(state.entries, ref) do
      %{timer: timer} -> Process.cancel_timer(timer)
      nil -> :ok
    end

    monitors =
      Enum.reduce(state.monitors, state.monitors, fn {monitor, entry_ref}, acc ->
        if entry_ref == ref do
          Process.demonitor(monitor, [:flush])
          Map.delete(acc, monitor)
        else
          acc
        end
      end)

    %{state | waiters: Map.delete(state.waiters, ref), entries: Map.delete(state.entries, ref), monitors: monitors}
  end

  defp waiting?({:queued, _url}), do: true
  defp waiting?({:follower, _url, _key}), do: true
  defp waiting?(_waiter), do: false

  defp waiting_count(state), do: Enum.count(state.waiters, fn {_ref, waiter} -> waiting?(waiter) end)

  defp ensure_ledger do
    if :ets.whereis(@ledger) == :undefined do
      [parent | _] = Process.get(:"$ancestors")
      parent = GenServer.whereis(parent)
      :ets.new(@ledger, [:named_table, :public, :set, {:heir, parent, :throttle_admissions}])
    end
  end

  defp persist_rate(url, endpoint) do
    :ets.insert(@ledger, {{:rate, url}, endpoint.tokens, endpoint.refilled_at})
  end

  defp recover(state) do
    rows = :ets.tab2list(@ledger)

    state =
      Enum.reduce(rows, state, fn
        {{:rate, url}, tokens, refilled_at}, acc ->
          endpoint = %{new_endpoint(acc.config) | tokens: min(tokens, acc.config.rps * 1.0), refilled_at: refilled_at}
          %{acc | endpoints: Map.put(acc.endpoints, url, endpoint)}

        _admission, acc ->
          acc
      end)

    Enum.reduce(rows, state, fn
      {ref, pid, ticket, :running}, acc ->
        if Process.alive?(pid) do
          monitor = Process.monitor(pid)
          timer = Process.send_after(self(), {:expire, ref}, remaining(ticket.deadline))
          endpoint = endpoint(acc, ticket.url)

          endpoint = %{
            endpoint
            | inflight_total: endpoint.inflight_total + 1,
              inflight: Map.update!(endpoint.inflight, ticket.class, &(&1 + 1))
          }

          {endpoint, waiter} =
            if is_nil(ticket.key) do
              {endpoint, {:running, ticket.url, ticket.class, ticket.weight}}
            else
              {%{endpoint | leaders: Map.put(endpoint.leaders, ticket.key, %{ref: ref, followers: []})},
               {:leader, ticket.url, ticket.key, ticket.class, ticket.weight}}
            end

          entry = %{pid: pid, timer: timer, deadline: ticket.deadline, class: ticket.class}
          acc = %{acc | entries: Map.put(acc.entries, ref, entry), monitors: Map.put(acc.monitors, monitor, ref)}
          acc |> put_waiter(ref, waiter) |> put_endpoint(ticket.url, endpoint)
        else
          :ets.delete(@ledger, ref)
          acc
        end

      {ref, _pid, _ticket, :completed}, acc ->
        :ets.delete(@ledger, ref)
        acc

      _rate, acc ->
        acc
    end)
  end

  defp classify_method(request) when is_map(request) do
    method = field(request, :method)
    params = field(request, :params) || []

    cond do
      not is_binary(method) -> :default
      String.starts_with?(method, @trace_method_prefixes) -> :trace
      historical_state_read?(method, params) -> :archive
      true -> :default
    end
  end

  defp classify_method(_request), do: :default

  defp historical_state_read?(method, params) when is_list(params) do
    case Map.fetch(@historical_block_parameter_index, method) do
      {:ok, index} -> params |> Enum.at(index) |> historical_block?()
      :error -> false
    end
  end

  defp historical_state_read?(_method, _params), do: false

  defp historical_block?(nil), do: false
  defp historical_block?(block) when block in @latest_block_parameters, do: false
  defp historical_block?(block) when is_binary(block), do: true
  defp historical_block?(_block), do: false

  defp heavier(:trace, _other), do: :trace
  defp heavier(_class, :trace), do: :trace
  defp heavier(:archive, _other), do: :archive
  defp heavier(_class, :archive), do: :archive
  defp heavier(_class, _other), do: :default

  defp coalesce_key(_url, batch) when is_list(batch), do: nil

  defp coalesce_key(url, request) when is_map(request) do
    method = field(request, :method)

    if is_binary(method) and coalescable?(method) do
      {url, method, field(request, :params) || []}
    end
  end

  defp coalescable?(method) do
    method in @coalescable_methods or String.starts_with?(method, @trace_method_prefixes)
  end

  defp field(request, key) do
    case Map.fetch(request, key) do
      {:ok, value} -> value
      :error -> Map.get(request, Atom.to_string(key))
    end
  end

  defp monotonic_ms, do: System.monotonic_time(:millisecond)
  defp remaining(deadline), do: max(deadline - monotonic_ms(), 0)
end
