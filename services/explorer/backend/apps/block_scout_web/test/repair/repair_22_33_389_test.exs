defmodule BlockScoutWeb.Repair2233389Test do
  @moduledoc """
  Repair record 22.33.389, audit 22.30.141: redis-session-revocation-unchecked
  is proved by the failed-deletion/recovery/retry, failed-lookup, failed-write
  and already-absent deletion cases below. They exercise AuthController's real
  login and logout, Plug.Session, signed cookies and Redix over loopback RESP.
  The Redis peer keeps acknowledged state across transport/client restarts.
  Policy: unsuccessful persistence aborts the response; retain the cookie and
  explicitly retry logout after recovery. No successful revocation is claimed
  on an error, and no cookie is issued on an unsuccessful write.
  Wiring request: domain 14 owns services/explorer/backend/apps/block_scout_web/lib/block_scout_web/controllers/account/auth_controller.ex;
  present RedisCookie.PersistenceError as a retryable logout failure, preserving
  the original cookie and never sending the success redirect on that error.
  Baseline verification was not run: the bounded gate order requires complete
  implementation before hf-build and hf-done; no baseline failures are claimed.
  The runner's evidence record names the verified candidate revision.
  """

  use BlockScoutWeb.ConnCase, async: false

  alias BlockScoutWeb.Account.AuthController
  alias BlockScoutWeb.Plug.RedisCookie
  alias BlockScoutWeb.Plug.RedisCookie.PersistenceError

  # Recorded owner from the committed CryptoKitties metadata fixture.
  @address "0x7b9ea9ac69b8fde875554321472c732eeff06ca0"
  @cookie_key "_explorer_key"

  defmodule RedisPeer do
    def serve(listener, store) do
      case :gen_tcp.accept(listener) do
        {:ok, socket} ->
          serve_commands(socket, store)
          serve(listener, store)

        {:error, :closed} ->
          :ok
      end
    end

    defp serve_commands(socket, store) do
      case read_command(socket) do
        :closed ->
          :ok

        command ->
          reply = Agent.get_and_update(store, &execute(command, &1))

          case reply do
            :disconnect ->
              :gen_tcp.close(socket)

            response ->
              :ok = :gen_tcp.send(socket, response)
              serve_commands(socket, store)
          end
      end
    end

    defp read_command(socket) do
      :ok = :inet.setopts(socket, packet: :line)

      case :gen_tcp.recv(socket, 0) do
        {:error, :closed} ->
          :closed

        {:ok, "*" <> count} ->
          for _ <- 1..String.to_integer(String.trim(count)) do
            {:ok, "$" <> length} = :gen_tcp.recv(socket, 0)
            size = String.to_integer(String.trim(length))
            :ok = :inet.setopts(socket, packet: :raw)
            {:ok, data} = :gen_tcp.recv(socket, size + 2)
            <<argument::binary-size(size), "\r\n">> = data
            :ok = :inet.setopts(socket, packet: :line)
            argument
          end
      end
    end

    defp execute([operation | _] = command, state) do
      state = %{state | commands: state.commands ++ [command]}

      case Map.get(state.failures, operation) do
        :disconnect -> {:disconnect, state}
        :readonly -> {"-READONLY You can't write against a read only replica.\r\n", state}
        nil -> apply_command(command, state)
      end
    end

    defp apply_command(["SET", key, value, "EX", ttl], state) do
      expires = System.monotonic_time(:second) + String.to_integer(ttl)
      {"+OK\r\n", %{state | values: Map.put(state.values, key, {value, expires})}}
    end

    defp apply_command(["GET", key], state) do
      case Map.get(state.values, key) do
        {value, expires} ->
          if expires > System.monotonic_time(:second) do
            {["$", Integer.to_string(byte_size(value)), "\r\n", value, "\r\n"], state}
          else
            {"$-1\r\n", %{state | values: Map.delete(state.values, key)}}
          end

        nil ->
          {"$-1\r\n", state}
      end
    end

    defp apply_command(["DEL", key], state) do
      count = if Map.has_key?(state.values, key), do: 1, else: 0
      {[":", Integer.to_string(count), "\r\n"], %{state | values: Map.delete(state.values, key)}}
    end
  end

  setup do
    previous = Process.whereis(:redix)
    if previous, do: Process.unregister(:redix)

    on_exit(fn ->
      if previous && Process.alive?(previous), do: Process.register(previous, :redix)
    end)

    store = start_supervised!({Agent, fn -> %{values: %{}, failures: %{}, commands: []} end})
    listener = start_transport(store)
    {:ok, store: store, listener: listener}
  end

  for failure <- [:readonly, :disconnect] do
    test "redis-session-revocation-unchecked: #{failure} deletion fails explicitly, recovers and retries", context do
      %{store: store, listener: listener} = context
      {cookie, user} = login()
      assert_received {:plug_conn, :sent}
      assert user.address_hash == @address
      assert get_session(request(cookie), :current_user) == user

      fail_command(store, "DEL", unquote(failure))

      error = assert_raise PersistenceError, fn -> logout(cookie) end
      assert error.operation == :delete
      assert {:error, _} = error.reason
      refute_received {:plug_conn, :sent}
      assert Enum.any?(commands(store), &match?(["DEL", _], &1))

      # A failed logout never claimed revocation: recovery still requires retry.
      listener = recover(store, listener)
      assert get_session(request(cookie), :current_user) == user
      response = logout(cookie)
      assert response.state == :sent
      assert response.status == 302
      assert get_resp_header(response, "location") == [Explorer.Repo.ConfigHelper.network_path()]
      assert response.resp_cookies[@cookie_key].max_age == 0
      assert get_session(request(cookie), :current_user) == nil

      # Reopen both sides again: an acknowledged deletion cannot resurrect.
      recover(store, listener)
      assert get_session(request(cookie), :current_user) == nil
      assert Agent.get(store, & &1.values) == %{}
    end
  end

  test "redis-session-revocation-unchecked: lookup failure cannot turn logout into success", context do
    %{store: store, listener: listener} = context
    {cookie, user} = login()
    assert_received {:plug_conn, :sent}
    fail_command(store, "GET", :disconnect)

    error = assert_raise PersistenceError, fn -> logout(cookie) end
    assert error.operation == :read
    refute_received {:plug_conn, :sent}
    refute Enum.any?(commands(store), &match?(["DEL", _], &1))

    recover(store, listener)
    assert get_session(request(cookie), :current_user) == user
    assert logout(cookie).status == 302
    assert get_session(request(cookie), :current_user) == nil
  end

  test "redis-session-revocation-unchecked: failed login persistence sends no cookie or redirect", context do
    %{store: store, listener: listener} = context
    fail_command(store, "SET", :readonly)

    error = assert_raise PersistenceError, fn -> login() end
    assert error.operation == :write
    assert {:error, %Redix.Error{}} = error.reason
    refute_received {:plug_conn, :sent}
    assert Agent.get(store, & &1.values) == %{}

    recover(store, listener)
    {cookie, user} = login()
    assert get_session(request(cookie), :current_user) == user
    ttl = to_string(Application.fetch_env!(:block_scout_web, :session_cookie_ttl))
    assert Enum.any?(commands(store), &match?(["SET", _, "1", "EX", ^ttl], &1))
    assert logout(cookie).status == 302
    assert get_session(request(cookie), :current_user) == nil
  end

  test "redis-session-revocation-unchecked: deletion is idempotent when another request already revoked", %{
    store: store
  } do
    {cookie, _user} = login()
    conn = request(cookie)
    ["SET", key, "1", "EX", _ttl] = Enum.find(commands(store), &match?(["SET" | _], &1))
    assert {:ok, 1} = Redix.command(:redix, ["DEL", key])

    response = AuthController.logout(conn, %{})
    assert response.status == 302
    assert response.resp_cookies[@cookie_key].max_age == 0
    assert get_session(request(cookie), :current_user) == nil
  end

  defp start_transport(store) do
    {:ok, listener} = :gen_tcp.listen(0, [:binary, active: false, ip: {127, 0, 0, 1}, reuseaddr: true])
    {:ok, {_, port}} = :inet.sockname(listener)
    start_supervised!(%{id: :redis_peer, start: {Task, :start_link, [fn -> RedisPeer.serve(listener, store) end]}})
    start_supervised!({Redix, host: "127.0.0.1", port: port, name: :redix, sync_connect: true})
    listener
  end

  defp recover(store, listener) do
    :ok = stop_supervised(Redix)
    :ok = stop_supervised(:redis_peer)
    :ok = :gen_tcp.close(listener)
    Agent.update(store, &%{&1 | failures: %{}})
    start_transport(store)
  end

  defp fail_command(store, command, failure),
    do: Agent.update(store, &%{&1 | failures: Map.put(&1.failures, command, failure)})

  defp commands(store), do: Agent.get(store, & &1.commands)

  defp request(cookie \\ nil, path \\ "/auth/logout") do
    options = Plug.Session.init(store: RedisCookie, key: @cookie_key, signing_salt: "iC2ksJHS")
    conn = Plug.Test.conn(:get, path)
    conn = %{conn | secret_key_base: BlockScoutWeb.Endpoint.config(:secret_key_base)}
    conn = if cookie, do: put_req_header(conn, "cookie", @cookie_key <> "=" <> cookie), else: conn

    conn
    |> Plug.Session.call(options)
    |> fetch_session()
  end

  defp login do
    auth = build(:auth)
    auth = put_in(auth.extra.raw_info.user["user_metadata"]["web3_address_hash"], @address)

    conn =
      request(nil, "/auth/auth0/callback")
      |> assign(:ueberauth_auth, auth)
      |> AuthController.callback(%{})

    assert conn.state == :sent
    assert conn.status == 302
    cookie = conn.resp_cookies[@cookie_key].value
    assert is_binary(cookie)
    {cookie, get_session(conn, :current_user)}
  end

  defp logout(cookie), do: AuthController.logout(request(cookie), %{})
end
