defmodule Explorer.Repair.Repair2233509Test.Auth0Transcript do
  @moduledoc false
  import Plug.Conn

  def init(options), do: options

  def call(conn, {owner, transcript}) do
    {:ok, body, conn} = read_body(conn)
    send(owner, {:auth0_request, conn.method, conn.request_path, conn.req_headers, body})
    response = Map.fetch!(transcript, "#{conn.method} #{conn.request_path}")

    conn
    |> put_resp_content_type("application/json")
    |> send_resp(response["status"], Jason.encode!(response["body"]))
  end
end

defmodule Explorer.Repair.Repair2233509Test do
  use ExUnit.Case, async: false

  import ExUnit.CaptureLog

  alias Explorer.Account
  alias Explorer.Account.{Identity, TagAddress}
  alias Explorer.Repo
  alias Explorer.ThirdPartyIntegrations.{Auth0, Dynamic, Keycloak}
  alias Ueberauth.Strategy.Auth0.OAuth

  @fixtures Path.expand("../../lib/mix/tasks/auth0_to_keycloak_migrate/fixtures", __DIR__)
  @export Path.join(@fixtures, "duplicate_users.jsonl")
  @status_fixtures Path.join(@fixtures, "status")
  @legacy_error "Migration did not report an explicit success status. Review migration errors before retrying."

  setup do
    :ok = Ecto.Adapters.SQL.Sandbox.checkout(Repo.account_repo())
    replace_env(:ueberauth, OAuth, domain: nil, auth0_application_id: "")
    replace_env(:explorer, Keycloak, [])
    replace_env(:explorer, Dynamic, enabled: false)
    replace_env(:explorer, Account, enabled: true)

    @export
    |> File.read!()
    |> String.split("\n", trim: true)
    |> Enum.each(fn line ->
      user = Jason.decode!(line)

      identity =
        %Identity{}
        |> Identity.changeset(%{uid: user["user_id"], email: user["email"]})
        |> Repo.account_repo().insert!()

      address = "0x0000000000000000000000000000000000000125"
      {:ok, hash} = Explorer.Chain.string_to_address_hash(address)

      Repo.account_repo().insert!(%TagAddress{
        identity_id: identity.id,
        name: "recorded migration address",
        address_hash: hash,
        address_hash_hash: address
      })
    end)

    :ok
  end

  test "migration-options rejects misspelled preview switches before app.start" do
    for args <- [["--dryrun"], ["--dry-rnu"], ["--dry-run=yes"], ["--unknown"]] do
      assert_rejected_before_start(args, ~r/Invalid migration options/)
    end
  end

  test "migration-options rejects positional input before app.start" do
    for args <- [["unexpected"], ["--dry-run", "unexpected"], ["--", "--dry-run"]] do
      assert_rejected_before_start(args, ~r/Unexpected migration arguments/)
    end
  end

  test "migration-options rejects missing and noninteger batch sizes before app.start" do
    for args <- [["--batch-size"], ["--batch-size", "many"], ["--batch-size=1.5"], ["-b", "many"]] do
      assert_rejected_before_start(args, ~r/Invalid migration options/)
    end
  end

  test "migration-options rejects nonpositive and excessive batch sizes before app.start" do
    for size <- ["0", "-1", "1001", "999999999999999999999999999999999999"] do
      assert_rejected_before_start(["--batch-size=#{size}"], ~r/--batch-size must be an integer from 1 to 1000/)
    end
  end

  test "migration-options rejects execution with an offline export before app.start" do
    assert_rejected_before_start(["--auth0-export", @export], ~r/--auth0-export requires --dry-run/)
    assert_rejected_before_start(["--no-dry-run", "--auth0-export", @export], ~r/--auth0-export requires --dry-run/)
  end

  test "migration-options accepts the default, aliases and batch bounds along the real preview path" do
    before = snapshot()

    for options <- [[], ["-b", "1"], ["--batch-size", "500"], ["--batch-size=1000"]] do
      log = capture_log(fn -> assert run_cli(["-n", "--auth0-export", @export] ++ options) == [] end)
      assert log =~ "Would merge duplicate email preview@example.com"
      assert log =~ "auth0|preview-wallet"
      assert snapshot() == before
    end
  end

  test "migration-status distinguishes a successful preview no-op from missing configuration" do
    Repo.account_repo().delete_all(TagAddress)
    Repo.account_repo().delete_all(Identity)

    assert capture_log(fn -> assert run_cli(["--dry-run", "--auth0-export", @export]) == [] end) =~
             "Would delete 0 identities"

    assert_raise Mix.Error, "Auth0 is not configured. Cannot read source users.", fn ->
      run_cli([])
    end

    assert Application.get_env(:explorer, Account)[:enabled]
  end

  test "migration-status missing Auth0 and Keycloak fail visibly without account writes" do
    before = snapshot()

    assert_raise Mix.Error, "Auth0 is not configured. Cannot read source users.", fn ->
      run_cli(["--no-dry-run"])
    end

    Application.put_env(:ueberauth, OAuth, domain: "configured.invalid", auth0_application_id: "")
    assert Auth0.enabled?()
    refute Keycloak.enabled?()

    assert_raise Mix.Error, "Keycloak is not configured. Cannot create target users.", fn ->
      run_cli([])
    end

    assert snapshot() == before
  end

  for {scenario, failure} <- [
        {"create_failed", "Failed to create Auth0 export job"},
        {"poll_failed", "Auth0 export job failed"}
      ] do
    @scenario scenario
    @failure failure
    test "migration-status #{@scenario} propagates a real runner phase failure to the command" do
      start_auth0_transcript(@scenario)
      start_token_cache()

      Application.put_env(:explorer, Keycloak,
        domain: "http://127.0.0.1:1",
        realm: "fixture",
        client_id: "fixture",
        client_secret: "fixture"
      )

      assert Auth0.enabled?()
      assert Keycloak.enabled?()
      before = snapshot()

      log =
        capture_log(fn ->
          assert_raise Mix.Error, @legacy_error, fn -> run_cli([]) end
        end)

      assert log =~ "Phase 1: Exporting Auth0 users"
      assert log =~ "Migration failed:"
      assert log =~ @failure
      assert log =~ "Disabling account access for migration"
      refute log =~ "Phase 3:"
      assert snapshot() == before

      assert_receive {:auth0_request, "POST", "/api/v2/jobs/users-exports", headers, body}
      assert {"authorization", "Bearer recorded-management-token"} in headers
      assert {"content-type", "application/json"} in headers
      request = Jason.decode!(body)
      assert request["format"] == "json"

      assert Enum.map(request["fields"], & &1["name"]) ==
               ~w(user_id email email_verified username nickname name given_name family_name picture user_metadata app_metadata identities)

      if @scenario == "poll_failed" do
        assert_receive {:auth0_request, "GET", "/api/v2/jobs/job_recorded_export", headers, ""}
        assert {"authorization", "Bearer recorded-management-token"} in headers
      end

      key = Explorer.Helper.redis_key(Auth0.Internal.redis_key())
      assert_receive {:token_cache_get, ^key}
    end
  end

  test "migration-status CLI documents the effective default and bounded batch size" do
    {:docs_v1, _, _, _, %{"en" => documentation}, _, _} = Code.fetch_docs(Mix.Tasks.Auth0ToKeycloakMigrate)
    assert documentation =~ "from 1 to 1000 (default: 500)"
  end

  defp run_cli(args), do: Mix.Task.rerun("auth0_to_keycloak_migrate", ["--no-start" | args])

  defp assert_rejected_before_start(args, message) do
    before = snapshot()
    tracer = spawn_link(fn -> collect_start_calls([]) end)
    :erlang.trace_pattern({Mix.Task, :run, 2}, true, [:local])
    :erlang.trace(self(), true, [:call, {:tracer, tracer}])

    try do
      assert_raise Mix.Error, message, fn -> Mix.Task.rerun("auth0_to_keycloak_migrate", args) end
      delivered = :erlang.trace_delivered(self())
      assert_receive {:trace_delivered, _, ^delivered}
      reference = make_ref()
      send(tracer, {:calls, self(), reference})
      assert_receive {^reference, calls}
      assert calls == []
    after
      :erlang.trace(self(), false, [:call])
      :erlang.trace_pattern({Mix.Task, :run, 2}, false, [:local])
      send(tracer, :stop)
    end

    assert snapshot() == before
  end

  defp collect_start_calls(calls) do
    receive do
      {:trace, _, :call, {Mix.Task, :run, ["app.start", args]}} -> collect_start_calls([args | calls])
      {:trace, _, :call, _} -> collect_start_calls(calls)
      {:calls, caller, reference} -> send(caller, {reference, Enum.reverse(calls)})
      :stop -> :ok
    end
  end

  defp start_auth0_transcript(scenario) do
    transcript =
      @status_fixtures |> Path.join("export_jobs.json") |> File.read!() |> Jason.decode!() |> Map.fetch!(scenario)

    ref = {__MODULE__, make_ref()}

    start_supervised!(
      {Plug.Cowboy,
       scheme: :https,
       plug: {__MODULE__.Auth0Transcript, {self(), transcript}},
       options: [
         ref: ref,
         ip: {127, 0, 0, 1},
         port: 0,
         certfile: Path.join(@status_fixtures, "localhost-cert.pem"),
         keyfile: Path.join(@status_fixtures, "localhost-key.pem")
       ]}
    )

    replace_env(:oauth2, :adapter, Tesla.Adapter.Httpc)
    replace_env(:oauth2, :middleware, [])

    Application.put_env(:ueberauth, OAuth,
      domain: "localhost:#{:ranch.get_port(ref)}",
      auth0_application_id: "",
      client_id: "recorded-client",
      client_secret: "recorded-secret",
      request_opts: [ssl: [verify: :verify_peer, cacertfile: Path.join(@status_fixtures, "ca-cert.pem")]]
    )
  end

  defp start_token_cache do
    {:ok, listener} = :gen_tcp.listen(0, [:binary, active: false, packet: :line, ip: {127, 0, 0, 1}])
    {:ok, {_, port}} = :inet.sockname(listener)
    owner = self()

    start_supervised!(
      {Task,
       fn ->
         {:ok, socket} = :gen_tcp.accept(listener)
         serve_token_cache(socket, owner)
       end}
    )

    previous = Process.whereis(:redix)
    if previous, do: Process.unregister(:redix)
    {:ok, redix} = Redix.start_link(host: "127.0.0.1", port: port, name: :redix, sync_connect: true)
    Process.unlink(redix)

    on_exit(fn ->
      if Process.alive?(redix), do: GenServer.stop(redix)
      if previous && Process.alive?(previous), do: Process.register(previous, :redix)
      :gen_tcp.close(listener)
    end)
  end

  defp serve_token_cache(socket, owner) do
    case :gen_tcp.recv(socket, 0) do
      {:ok, "*2\r\n"} ->
        {:ok, "$3\r\n"} = :gen_tcp.recv(socket, 0)
        {:ok, "GET\r\n"} = :gen_tcp.recv(socket, 0)
        {:ok, "$" <> _} = :gen_tcp.recv(socket, 0)
        {:ok, key} = :gen_tcp.recv(socket, 0)
        send(owner, {:token_cache_get, String.trim_trailing(key, "\r\n")})
        :ok = :gen_tcp.send(socket, "$25\r\nrecorded-management-token\r\n")
        serve_token_cache(socket, owner)

      {:error, :closed} ->
        :ok
    end
  end

  defp snapshot do
    {
      Repo.account_repo().all(Identity),
      Repo.account_repo().all(TagAddress),
      Application.get_env(:explorer, Account)
    }
  end

  defp replace_env(app, key, value) do
    previous = Application.fetch_env(app, key)
    Application.put_env(app, key, value)

    on_exit(fn ->
      case previous do
        {:ok, config} -> Application.put_env(app, key, config)
        :error -> Application.delete_env(app, key)
      end
    end)
  end
end
