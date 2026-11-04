defmodule EthereumJSONRPC.WebSocket.WebSocketClient do
  @moduledoc """
  `EthereumJSONRPC.WebSocket` that uses `WebSockex`
  """

  require Logger

  alias EthereumJSONRPC.{Subscription, Transport, WebSocket}
  alias EthereumJSONRPC.WebSocket.{Registration, RetryWorker}
  alias EthereumJSONRPC.WebSocket.Supervisor, as: WebSocketSupervisor
  alias EthereumJSONRPC.WebSocket.WebSocketClient.Options

  @behaviour WebSocket

  @enforce_keys ~w(url)a
  defstruct connected: false,
            request_id_to_registration: %{},
            subscription_id_to_subscription_reference: %{},
            subscription_reference_to_subscription_id: %{},
            subscription_reference_to_subscription: %{},
            subscriber_monitors: %{},
            web_socket: __MODULE__,
            url: nil,
            fallback?: false,
            fallback_url: nil,
            fallback_conn: nil,
            retry: false

  @typedoc """
   * `request_id_to_registration` - maps id of requests in flight to their
     `t:EthereumSJONRPC.WebSocket.Registration.t/0`, so that when the response is received from the server, the caller
     in `from` of the registration can be `GenServer.reply/2`ed to.
   * `subscription_id_to_subscription_reference` - maps id of subscription on the server to the `t:reference/0` used in
     the `t:EthereumJSONRPC.Subscription.t/0`.  Subscriptions use a `t:reference/0` instead of the server-side id, so
     that on reconnect, the id can change, but the subscribe does not need to be notified.
   * `subscription_reference_to_subscription` - maps `t:reference/0` in `t:EthereumJSONRPC.Subscription.t/0` to that
     `t:EthereumJSONRPC.Subscription.t/0`, so that the `subscriber_pid` can be notified of subscription messages.
   * `subscription_reference_to_subscription_id` - maps `t:reference/0` in `t:EthereumJSONRPC.Subscription.t/0  to id of
     the subscription on the server, so that the subscriber can unsubscribe with the `t:reference/0`.
   * `web_socket` - the registered owner name retained when a recovered primary replaces the fallback client.
  """
  @type t :: %__MODULE__{
          request_id_to_registration: %{EthereumJSONRPC.request_id() => Registration.t()},
          subscription_id_to_subscription_reference: %{Subscription.id() => reference()},
          subscription_reference_to_subscription: %{reference() => Subscription.t()},
          subscription_reference_to_subscription_id: %{reference() => Subscription.id()},
          web_socket: WebSocket.name()
        }

  @impl WebSocket
  def child_spec(arg) do
    Supervisor.child_spec(%{id: __MODULE__, start: {__MODULE__, :start_link, [arg]}}, [])
  end

  @impl WebSocket
  def start_link(url, options) do
    case build_conn(url, options) do
      {:ok, conn, opts} ->
        init_state =
          options[:init_state] ||
            %__MODULE__{
              web_socket: options[:name] || __MODULE__,
              url: url,
              fallback_url: options[:fallback_url],
              fallback_conn: build_fallback_conn(options[:fallback_url], options)
            }

        WebSockex.start_link(conn, __MODULE__, init_state, opts)

      _error ->
        Logger.error("Unable to build WS main connection #{redact_endpoint(url)} (invalid endpoint)")
        :ignore
    end
  end

  # Client interface

  @impl WebSocket
  @spec json_rpc(WebSocket.web_socket(), Transport.request()) :: {:ok, Transport.result()} | {:error, reason :: term()}
  def json_rpc(web_socket, request, timeout \\ 5_000) do
    call(web_socket, {:json_rpc, request}, timeout)
  end

  @impl WebSocket
  @spec subscribe(WebSocket.web_socket(), Subscription.event(), Subscription.params()) ::
          {:ok, Subscription.t()} | {:error, reason :: term()}
  def subscribe(web_socket, event, params, timeout \\ 5_000) when is_binary(event) and is_list(params) do
    call(web_socket, {:subscribe, event, params}, timeout)
  end

  @impl WebSocket
  @spec unsubscribe(WebSocket.web_socket(), Subscription.t()) :: :ok | {:error, :not_found}
  def unsubscribe(web_socket, %Subscription{} = subscription, timeout \\ 5_000) do
    call(web_socket, {:unsubscribe, subscription}, timeout)
  end

  defp call(web_socket, request, timeout) when is_integer(timeout) and timeout > 0 do
    deadline = System.monotonic_time(:millisecond) + timeout
    GenServer.call(web_socket, {:request, deadline, request}, timeout)
  end

  def handle_connect(_conn, state) do
    Logger.metadata(fetcher: :websocket_client)
    state = RetryWorker.checkpoint(%{state | connected: true, retry: false})

    if !state.fallback? do
      RetryWorker.deactivate()
      WebSocketSupervisor.stop_other_client(self(), state.web_socket)
    end

    {:ok, reconnect(state)}
  end

  @attempts_to_reconnect 3
  def handle_disconnect(%{attempt_number: attempt}, state) do
    Logger.metadata(fetcher: :websocket_client)

    final_state =
      state.request_id_to_registration
      |> Enum.reduce(state, &disconnect_request_id_registration/2)
      |> Map.merge(%{
        connected: false,
        subscription_id_to_subscription_reference: %{},
        subscription_reference_to_subscription_id: %{}
      })
      |> checkpoint_unless_probe()

    cond do
      state.retry ->
        RetryWorker.activate(final_state)
        Logger.warning("WS endpoint #{redact_endpoint(state.url)} is still unavailable")
        {:ok, final_state}

      attempt < @attempts_to_reconnect ->
        {:reconnect, final_state}

      state.fallback? ->
        RetryWorker.activate(final_state)
        Logger.warning("WS fallback endpoint #{redact_endpoint(state.url)} is unavailable")
        {:ok, final_state}

      not is_nil(state.fallback_conn) ->
        RetryWorker.activate(final_state)

        Logger.warning(
          "WS endpoint #{redact_endpoint(state.url)} is unavailable, switching to fallback #{redact_endpoint(state.fallback_url)}"
        )

        {:reconnect, state.fallback_conn,
         %{final_state | url: state.fallback_url, fallback?: true, fallback_url: nil, fallback_conn: nil}}

      true ->
        RetryWorker.activate(final_state)

        Logger.warning(
          "WS endpoint #{redact_endpoint(state.url)} is unavailable, and no fallback is set, shutting down WS client"
        )

        {:ok, final_state}
    end
  end

  @doc "Endpoint diagnostics retain only scheme, host and port; every credential-bearing component is omitted."
  def redact_endpoint(url) when is_binary(url) do
    case URI.parse(url) do
      %URI{scheme: scheme, host: host, port: port} when scheme in ["ws", "wss"] and is_binary(host) ->
        URI.to_string(%URI{scheme: scheme, host: host, port: port}) <> "/[redacted]"

      _ ->
        "[invalid endpoint]"
    end
  rescue
    _ -> "[invalid endpoint]"
  end

  def redact_endpoint(_), do: "[unset endpoint]"

  defp checkpoint_unless_probe(%{retry: true} = state), do: state
  defp checkpoint_unless_probe(state), do: RetryWorker.checkpoint(state)

  def handle_frame({:text, text}, state) do
    case Jason.decode(text) do
      {:ok, json} ->
        handle_response(json, state)

      {:error, _} = error ->
        broadcast(error, state)
        {:ok, state}
    end
  end

  def handle_ping({:ping, ""}, state) do
    {:reply, {:pong, ""}, state}
  end

  def handle_pong({:pong, _}, state) do
    {:ok, state}
  end

  def handle_info({:"$gen_call", from, {:request, deadline, request}}, state) do
    if System.monotonic_time(:millisecond) < deadline and Process.alive?(elem(from, 0)) do
      register_call(request, from, deadline, state)
    else
      GenServer.reply(from, {:error, :timeout})
      {:ok, state}
    end
  end

  def handle_info({:"$gen_call", from, request}, state) do
    register_call(request, from, System.monotonic_time(:millisecond) + 5_000, state)
  end

  def handle_info({:expire_registration, id}, state) do
    case state.request_id_to_registration[id] do
      %Registration{subscription_reference: reference} when not is_nil(reference) ->
        {:close, {1011, "subscription recovery timed out"}, drop_registration(state, id)}

      _ ->
        {:ok, drop_registration(state, id, {:error, :timeout})}
    end
  end

  def handle_info({:DOWN, monitor, :process, _pid, _reason}, state) do
    state =
      Enum.reduce(state.request_id_to_registration, state, fn {id, registration}, acc ->
        if registration.monitor == monitor, do: drop_registration(acc, id), else: acc
      end)

    state =
      Enum.reduce(state.subscriber_monitors, state, fn {reference, ref}, acc ->
        if ref == monitor, do: forget_subscription(acc, reference), else: acc
      end)

    {:ok, RetryWorker.checkpoint(state)}
  end

  defp register_call({:unsubscribe, %Subscription{reference: reference}}, from, _deadline, %{connected: false} = state) do
    if Map.has_key?(state.subscription_reference_to_subscription, reference) do
      state = state |> forget_subscription(reference) |> RetryWorker.checkpoint()
      GenServer.reply(from, :ok)
      {:ok, state}
    else
      GenServer.reply(from, {:error, :not_found})
      {:ok, state}
    end
  end

  defp register_call(request, from, deadline, state) do
    case register(request, from, state) do
      {:ok, unique_request, updated_state} ->
        id = unique_request.id
        registration = updated_state.request_id_to_registration[id]

        if state.connected or Registration.replayable?(registration) do
          monitor = Process.monitor(elem(from, 0))

          timer =
            Process.send_after(
              self(),
              {:expire_registration, id},
              max(0, deadline - System.monotonic_time(:millisecond))
            )

          registration = %{registration | deadline: deadline, monitor: monitor, timer: timer}
          updated_state = put_in(updated_state.request_id_to_registration[id], registration)
          if state.connected, do: {:reply, frame(unique_request), updated_state}, else: {:ok, updated_state}
        else
          GenServer.reply(from, {:error, :not_connected})
          {:ok, state}
        end

      {:error, _reason} = error ->
        GenServer.reply(from, error)
        {:ok, state}
    end
  end

  def handle_cast({:send_message, id}, state) do
    case state.request_id_to_registration[id] do
      nil ->
        {:ok, state}

      registration ->
        if state.connected and live_registration?(registration) do
          {:reply, frame(registration.request), state}
        else
          {:ok, drop_registration(state, id, {:error, :timeout})}
        end
    end
  end

  @doc "Terminal socket exits use the documented subscription error envelope. Recovery preserves the public handle."
  def terminate(reason, state) do
    Enum.each(state.request_id_to_registration, fn {_, registration} -> release_registration(registration) end)
    broadcast({:error, {:websocket_terminated, reason}}, state)
  end

  defp build_conn(url, options) when is_binary(url) do
    common_opts = [
      name: options[:name] || __MODULE__,
      async: true,
      handle_initial_conn_failure: true
    ]

    additional_opts =
      case url do
        "wss://" <> _ ->
          %URI{host: host} = URI.parse(url)
          host_charlist = String.to_charlist(host)

          :ssl.start()

          [
            insecure: false,
            ssl_options: [
              cacerts: :certifi.cacerts(),
              depth: 99,
              # SNI extension discloses host name in the clear, but allows for compatibility with Virtual Hosting for TLS
              server_name_indication: host_charlist,
              verify_fun: {&:ssl_verify_hostname.verify_fun/3, [check_hostname: host_charlist]},
              customize_hostname_check: [
                match_fun: :public_key.pkix_verify_hostname_match_fun(:https)
              ]
            ]
          ]

        _ ->
          []
      end

    full_opts = Keyword.merge(common_opts, additional_opts)

    # credo:disable-for-next-line Credo.Check.Design.AliasUsage
    case WebSockex.Conn.parse_url(url) do
      # credo:disable-for-next-line Credo.Check.Design.AliasUsage
      {:ok, uri} -> {:ok, WebSockex.Conn.new(uri, full_opts), full_opts}
      error -> error
    end
  end

  defp build_fallback_conn(nil, _options) do
    Logger.info("WS fallback endpoint is not set")
    nil
  end

  defp build_fallback_conn(url, options) do
    case build_conn(url, options) do
      {:ok, conn, _opts} ->
        conn

      _error ->
        Logger.warning(
          "Unable to build WS fallback connection #{redact_endpoint(url)} (invalid endpoint), continuing without fallback"
        )

        nil
    end
  end

  defp broadcast(message, %__MODULE__{subscription_reference_to_subscription: subscription_reference_to_subscription}) do
    subscription_reference_to_subscription
    |> Map.values()
    |> Subscription.broadcast(message)
  end

  defp disconnect_request_id_registration(
         {id, %Registration{type: :unsubscribe, request: %{params: [server_id]}}},
         state
       ) do
    reference = state.subscription_id_to_subscription_reference[server_id]
    state |> drop_registration(id, :ok) |> forget_subscription(reference)
  end

  defp disconnect_request_id_registration({id, %Registration{subscription_reference: reference}}, state)
       when not is_nil(reference), do: drop_registration(state, id)

  defp disconnect_request_id_registration({id, registration}, state) do
    cond do
      not live_registration?(registration) -> drop_registration(state, id, {:error, :timeout})
      Registration.replayable?(registration) -> state
      true -> drop_registration(state, id, {:error, :disconnected})
    end
  end

  defp live_registration?(%Registration{from: {pid, _}, deadline: deadline}) do
    Process.alive?(pid) and (is_nil(deadline) or System.monotonic_time(:millisecond) < deadline)
  end

  defp release_registration(nil), do: :ok

  defp release_registration(registration) do
    if registration.monitor, do: Process.demonitor(registration.monitor, [:flush])
    if registration.timer, do: Process.cancel_timer(registration.timer)
    :ok
  end

  defp drop_registration(state, id, reply \\ nil) do
    {registration, registrations} = Map.pop(state.request_id_to_registration, id)
    release_registration(registration)
    if registration && reply, do: GenServer.reply(registration.from, reply)
    %{state | request_id_to_registration: registrations}
  end

  defp forget_subscription(state, reference) do
    if monitor = state.subscriber_monitors[reference], do: Process.demonitor(monitor, [:flush])
    ids = Map.reject(state.subscription_id_to_subscription_reference, fn {_, ref} -> ref == reference end)

    %{
      state
      | subscription_reference_to_subscription: Map.delete(state.subscription_reference_to_subscription, reference),
        subscription_reference_to_subscription_id:
          Map.delete(state.subscription_reference_to_subscription_id, reference),
        subscription_id_to_subscription_reference: ids,
        subscriber_monitors: Map.delete(state.subscriber_monitors, reference)
    }
  end

  defp frame(request) do
    {:text, Jason.encode!(request)}
  end

  defp handle_response(
         %{"method" => "eth_subscription", "params" => %{"result" => result, "subscription" => subscription_id}},
         %{
           subscription_id_to_subscription_reference: subscription_id_to_subscription_reference,
           subscription_reference_to_subscription: subscription_reference_to_subscription
         } = state
       ) do
    case subscription_id_to_subscription_reference do
      %{^subscription_id => subscription_reference} ->
        %{^subscription_reference => subscription} = subscription_reference_to_subscription
        Subscription.publish(subscription, {:ok, result})

      _ ->
        Logger.error(fn ->
          [
            "Unexpected `eth_subscription` subscription ID (",
            inspect(subscription_id),
            ") result (",
            inspect(result),
            ").  Subscription ID not in known subscription IDs (",
            subscription_id_to_subscription_reference
            |> Map.values()
            |> Enum.map(&inspect/1),
            ")."
          ]
        end)
    end

    {:ok, state}
  end

  defp handle_response(
         %{"id" => id} = response,
         %{request_id_to_registration: request_id_to_registration} = state
       ) do
    {registration, new_request_id_to_registration} = Map.pop(request_id_to_registration, id)
    new_state = %{state | request_id_to_registration: new_request_id_to_registration}
    release_registration(registration)

    if registration && not live_registration?(registration) do
      GenServer.reply(registration.from, {:error, :timeout})
      {:ok, new_state}
    else
      respond_to_registration(registration, response, new_state)
    end
  end

  defp handle_response(response, state) do
    Logger.error(fn ->
      [
        "Unexpected JSON response from web socket\n",
        "\n",
        "  Response:\n",
        "    ",
        inspect(response)
      ]
    end)

    {:ok, state}
  end

  defp reconnect(state) do
    state
    |> rerequest()
    |> resubscribe()
  end

  defp register(
         {:json_rpc, original_request},
         from,
         %{request_id_to_registration: request_id_to_registration} = state
       ) do
    unique_id = unique_request_id(state)
    request = %{original_request | id: unique_id}

    {:ok, request,
     %{
       state
       | request_id_to_registration:
           Map.put(request_id_to_registration, unique_id, %Registration{
             from: from,
             type: :json_rpc,
             request: request
           })
     }}
  end

  defp register(
         {:subscribe, event, params},
         from,
         %{request_id_to_registration: request_id_to_registration} = state
       )
       when is_binary(event) and is_list(params) do
    unique_id = unique_request_id(state)
    request = EthereumJSONRPC.request(%{id: unique_id, method: "eth_subscribe", params: [event | params]})

    {:ok, request,
     %{
       state
       | request_id_to_registration:
           Map.put(request_id_to_registration, unique_id, %Registration{from: from, type: :subscribe, request: request})
     }}
  end

  defp register(
         {:unsubscribe, %Subscription{reference: subscription_reference}},
         from,
         %{
           request_id_to_registration: request_id_to_registration,
           subscription_reference_to_subscription_id: subscription_reference_to_subscription_id
         } = state
       ) do
    case subscription_reference_to_subscription_id do
      %{^subscription_reference => subscription_id} ->
        unique_id = unique_request_id(state)
        request = EthereumJSONRPC.request(%{id: unique_id, method: "eth_unsubscribe", params: [subscription_id]})

        {
          :ok,
          request,
          %{
            state
            | request_id_to_registration:
                Map.put(request_id_to_registration, unique_id, %Registration{
                  from: from,
                  type: :unsubscribe,
                  request: request
                })
          }
        }

      _ ->
        {:error, :not_found}
    end
  end

  defp rerequest(%__MODULE__{request_id_to_registration: registrations} = state) do
    Enum.reduce(registrations, state, fn {id, registration}, acc ->
      if live_registration?(registration) and Registration.replayable?(registration) do
        WebSockex.cast(self(), {:send_message, id})
        acc
      else
        drop_registration(acc, id, {:error, :timeout})
      end
    end)
  end

  defp respond_to_registration(
         %Registration{type: :json_rpc, from: from},
         response,
         state
       ) do
    reply =
      case response do
        %{"result" => result} -> {:ok, result}
        %{"error" => error} -> {:error, error}
      end

    GenServer.reply(from, reply)

    {:ok, state}
  end

  defp respond_to_registration(
         %Registration{
           type: :subscribe,
           from: {subscriber_pid, from_reference} = from,
           request: %{params: [event | params]}
         },
         %{"result" => subscription_id},
         %__MODULE__{
           subscription_id_to_subscription_reference: subscription_id_to_subscription_reference,
           subscription_reference_to_subscription: subscription_reference_to_subscription,
           subscription_reference_to_subscription_id: subscription_reference_to_subscription_id,
           web_socket: web_socket,
           url: url
         } = state
       ) do
    new_state =
      case subscription_reference_to_subscription do
        # resubscribe
        %{
          ^from_reference => %Subscription{
            subscriber_pid: ^subscriber_pid,
            transport_options: %WebSocket{
              web_socket: __MODULE__,
              web_socket_options: %Options{event: ^event, params: ^params}
            }
          }
        } ->
          %__MODULE__{
            state
            | subscription_id_to_subscription_reference:
                subscription_id_to_subscription_reference
                |> Map.reject(fn {_, reference} -> reference == from_reference end)
                |> Map.put(subscription_id, from_reference),
              subscription_reference_to_subscription_id:
                Map.put(subscription_reference_to_subscription_id, from_reference, subscription_id)
          }

        # new subscription
        _ ->
          subscription_reference = make_ref()

          subscription = %Subscription{
            reference: subscription_reference,
            subscriber_pid: subscriber_pid,
            transport: EthereumJSONRPC.WebSocket,
            transport_options: %EthereumJSONRPC.WebSocket{
              web_socket: __MODULE__,
              web_socket_options: %Options{web_socket: web_socket, event: event, params: params},
              url: url
            }
          }

          new_state = %__MODULE__{
            state
            | subscription_reference_to_subscription:
                Map.put(subscription_reference_to_subscription, subscription_reference, subscription),
              subscription_id_to_subscription_reference:
                Map.put(subscription_id_to_subscription_reference, subscription_id, subscription_reference),
              subscription_reference_to_subscription_id:
                Map.put(subscription_reference_to_subscription_id, subscription_reference, subscription_id),
              subscriber_monitors:
                Map.put(state.subscriber_monitors, subscription_reference, Process.monitor(subscriber_pid))
          }

          RetryWorker.checkpoint(new_state)
          GenServer.reply(from, {:ok, subscription})
          new_state
      end

    {:ok, RetryWorker.checkpoint(new_state)}
  end

  defp respond_to_registration(
         %Registration{type: :subscribe, from: from},
         %{"error" => error},
         state
       ) do
    GenServer.reply(from, {:error, error})

    {:ok, state}
  end

  defp respond_to_registration(
         %Registration{
           type: :unsubscribe,
           from: from,
           request: %{method: "eth_unsubscribe", params: [subscription_id]}
         },
         response,
         %__MODULE__{
           subscription_id_to_subscription_reference: subscription_id_to_subscription_reference
         } = state
       ) do
    reply =
      case response do
        %{"result" => true} -> :ok
        %{"result" => false} -> {:error, :not_found}
        %{"error" => %{"message" => "subscription not found"}} -> {:error, :not_found}
        %{"error" => error} -> {:error, error}
      end

    new_state =
      case subscription_id_to_subscription_reference do
        %{^subscription_id => subscription_reference} ->
          forget_subscription(state, subscription_reference)

        _ ->
          state
      end

    new_state = RetryWorker.checkpoint(new_state)
    GenServer.reply(from, reply)
    {:ok, new_state}
  end

  defp respond_to_registration(
         nil,
         response,
         %{request_id_to_registration: request_id_to_registration} = state
       ) do
    Logger.error(fn ->
      [
        "Got response for unregistered request ID: ",
        inspect(response),
        ".  Outstanding request registrations: ",
        inspect(request_id_to_registration)
      ]
    end)

    {:ok, state}
  end

  defp resubscribe(
         %__MODULE__{subscription_reference_to_subscription: subscription_reference_to_subscription} = initial_state
       ) do
    Enum.reduce(subscription_reference_to_subscription, initial_state, fn {subscription_reference,
                                                                           %Subscription{
                                                                             subscriber_pid: subscriber_pid,
                                                                             transport_options: %WebSocket{
                                                                               web_socket: __MODULE__,
                                                                               web_socket_options: %Options{
                                                                                 event: event,
                                                                                 params: params
                                                                               }
                                                                             }
                                                                           }},
                                                                          %__MODULE__{
                                                                            request_id_to_registration:
                                                                              acc_request_id_to_registration
                                                                          } = acc_state ->
      request_id = unique_request_id(acc_state)
      request = EthereumJSONRPC.request(%{id: request_id, method: "eth_subscribe", params: [event | params]})

      WebSockex.cast(self(), {:send_message, request_id})

      %__MODULE__{
        acc_state
        | request_id_to_registration:
            Map.put(acc_request_id_to_registration, request_id, %Registration{
              from: {subscriber_pid, subscription_reference},
              type: :subscribe,
              subscription_reference: subscription_reference,
              deadline: System.monotonic_time(:millisecond) + 5_000,
              timer: Process.send_after(self(), {:expire_registration, request_id}, 5_000),
              monitor: Process.monitor(subscriber_pid),
              request: request
            }),
          subscriber_monitors:
            Map.put_new_lazy(acc_state.subscriber_monitors, subscription_reference, fn ->
              Process.monitor(subscriber_pid)
            end)
      }
    end)
  end

  defp unique_request_id(%{request_id_to_registration: request_id_to_registration} = state) do
    unique_request_id = EthereumJSONRPC.unique_request_id()

    case request_id_to_registration do
      # collision
      %{^unique_request_id => _} ->
        unique_request_id(state)

      _ ->
        unique_request_id
    end
  end
end
