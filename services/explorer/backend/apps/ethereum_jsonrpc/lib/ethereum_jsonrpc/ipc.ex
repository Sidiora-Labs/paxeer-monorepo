defmodule EthereumJSONRPC.IPC do
  use GenServer
  @moduledoc false

  import EthereumJSONRPC.HTTP, only: [standardize_response: 1]

  @default_timeout 600_000

  def start_link(opts) do
    GenServer.start_link(__MODULE__, Keyword.put(opts, :socket, nil))
  end

  @impl GenServer
  def init(state) do
    case connect(state, deadline(timeout(state))) do
      {:ok, state} -> {:ok, state}
      {:error, reason} -> {:stop, reason}
    end
  end

  def post(pid, request), do: post_until(pid, request, deadline(timeout([])))

  defp post_until(pid, request, deadline) do
    if remaining(deadline) == 0 do
      {:error, :timeout}
    else
      GenServer.call(pid, {:request, request, deadline}, remaining(deadline))
    end
  catch
    :exit, {:timeout, _} -> {:error, :timeout}
    :exit, {:noproc, _} -> {:error, :closed}
  end

  def receive_response(data, socket, timeout, result \\ <<>>) do
    receive_until(data, socket, deadline(timeout), result)
  end

  defp receive_until({:error, reason}, _socket, _deadline, _result), do: {:error, reason}

  defp receive_until(:ok, socket, deadline, result) do
    with time when time > 0 <- remaining(deadline),
         {:ok, response} <- :gen_tcp.recv(socket, 0, time) do
      new_result = result <> response

      cond do
        remaining(deadline) == 0 -> {:error, :timeout}
        String.ends_with?(new_result, "\n") -> {:ok, new_result}
        true -> receive_until(:ok, socket, deadline, new_result)
      end
    else
      0 -> {:error, :timeout}
      {:error, _} = error -> error
    end
  end

  @impl GenServer
  def handle_call({:request, request, deadline}, _from, state) do
    case connect(state, deadline) do
      {:ok, state} ->
        socket = Keyword.fetch!(state, :socket)

        response =
          with time when time > 0 <- remaining(deadline),
               :ok <- :inet.setopts(socket, send_timeout: time, send_timeout_close: true),
               :ok <- :gen_tcp.send(socket, request) do
            receive_until(:ok, socket, deadline, <<>>)
          else
            0 -> {:error, :timeout}
            {:error, _} = error -> error
          end

        case response do
          {:ok, _} -> {:reply, response, state}
          {:error, _} -> {:reply, response, invalidate(state)}
        end

      {:error, reason} ->
        {:reply, {:error, reason}, invalidate(state)}
    end
  end

  @impl GenServer
  def terminate(_reason, state) do
    invalidate(state)
    :ok
  end

  defp connect(state, deadline) do
    cond do
      remaining(deadline) == 0 ->
        {:error, :timeout}

      state[:socket] != nil ->
        {:ok, state}

      true ->
        case :gen_tcp.connect({:local, state[:path]}, 0, [:binary, active: false], remaining(deadline)) do
          {:ok, socket} -> {:ok, Keyword.put(state, :socket, socket)}
          {:error, _} = error -> error
        end
    end
  end

  defp invalidate(state) do
    if state[:socket], do: :gen_tcp.close(state[:socket])
    Keyword.put(state, :socket, nil)
  end

  def request(pid, payload), do: request_until(pid, payload, deadline(timeout([])))

  defp request_until(pid, payload, deadline) do
    with {:ok, response} <- post_until(pid, Jason.encode!(payload), deadline) do
      result = decode_response(response)
      if remaining(deadline) == 0, do: {:error, :timeout}, else: result
    end
  end

  defp decode_response(response) do
    case Jason.decode(response) do
      {:ok, responses} when is_list(responses) ->
        {:ok, Enum.map(responses, &standardize_response/1)}

      {:ok, response} ->
        case standardize_response(response) do
          %{error: error} -> {:error, error}
          %{result: result} -> {:ok, result}
        end

      {:error, %Jason.DecodeError{data: ""}} ->
        {:error, :empty_response}

      {:error, error} ->
        {:error, {:invalid_json, error}}
    end
  end

  def json_rpc(payload, opts) do
    deadline = deadline(timeout(opts))

    :poolboy.transaction(
      Keyword.get(opts, :pool, :ipc_worker),
      fn pid -> request_until(pid, payload, deadline) end,
      remaining(deadline)
    )
  catch
    :exit, {:timeout, _} -> {:error, :timeout}
    :exit, {:noproc, _} -> {:error, :closed}
  end

  defp timeout(opts) do
    configured = Application.get_env(:ethereum_jsonrpc, __MODULE__, [])
    value = Keyword.get(opts, :timeout, Keyword.get(configured, :timeout, @default_timeout))

    if is_integer(value) and value > 0 do
      value
    else
      raise ArgumentError, "IPC timeout must be a positive integer in milliseconds"
    end
  end

  defp deadline(timeout), do: System.monotonic_time(:millisecond) + timeout
  defp remaining(deadline), do: max(deadline - System.monotonic_time(:millisecond), 0)
end
