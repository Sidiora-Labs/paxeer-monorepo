defmodule BlockScoutWeb.AddressContractVerificationController do
  use BlockScoutWeb, :controller

  alias BlockScoutWeb.Controller
  alias Explorer.Chain.Events.Publisher, as: EventsPublisher
  alias Explorer.Chain.SmartContract
  alias Explorer.SmartContract.{CompilerVersion, Solidity.CodeCompiler}
  alias Explorer.SmartContract.Solidity.PublisherWorker, as: SolidityPublisherWorker
  alias Explorer.SmartContract.Solidity.PublishHelper
  alias Explorer.SmartContract.Vyper.PublisherWorker, as: VyperPublisherWorker
  alias Explorer.ThirdPartyIntegrations.Sourcify

  def new(conn, %{"address_id" => address_hash_string}) do
    if SmartContract.verified_with_full_match?(address_hash_string) do
      address_contract_path =
        conn
        |> address_contract_path(:index, address_hash_string)
        |> Controller.full_path()

      redirect(conn, to: address_contract_path)
    else
      changeset =
        SmartContract.changeset(
          %SmartContract{address_hash: address_hash_string},
          %{}
        )

      compiler_versions =
        case CompilerVersion.fetch_versions(:solc) do
          {:ok, compiler_versions} ->
            compiler_versions

          {:error, _} ->
            []
        end

      render(conn, "new.html",
        changeset: changeset,
        compiler_versions: compiler_versions,
        evm_versions: CodeCompiler.evm_versions(:solidity),
        address_hash: address_hash_string
      )
    end
  end

  def create(
        conn,
        %{
          "smart_contract" => smart_contract,
          "external_libraries" => external_libraries,
          "file" => files,
          "verification_type" => "multi-part-files"
        }
      ) do
    files_array =
      files
      |> Map.values()
      |> PublishHelper.read_files()

    Que.add(SolidityPublisherWorker, {"multipart", smart_contract, files_array, external_libraries, conn})

    send_resp(conn, 204, "")
  end

  def create(
        conn,
        %{
          "smart_contract" => smart_contract,
          "external_libraries" => external_libraries
        }
      ) do
    Que.add(SolidityPublisherWorker, {"flattened", smart_contract, external_libraries, conn})

    send_resp(conn, 204, "")
  end

  # sobelow_skip ["Traversal.FileModule"]
  def create(
        conn,
        %{
          "smart_contract" => smart_contract,
          "verification_type" => "json:standard"
        } = params
      ) do
    files_array = PublishHelper.prepare_files_array(params["file"])

    with %Plug.Upload{path: path} <- PublishHelper.get_one_json(files_array),
         {:read, {:ok, json_input}} <- {:read, File.read(path)},
         {:queue, {:ok, %Que.Job{}}} <-
           {:queue, enqueue_standard_json(smart_contract, json_input, conn)} do
      send_resp(conn, 204, "")
    else
      nil ->
        conn |> put_status(:unprocessable_entity) |> json(%{error: "Please attach a JSON compiler input file."})

      {:read, {:error, _}} ->
        conn |> put_status(:unprocessable_entity) |> json(%{error: "Unable to read JSON compiler input file."})

      _ ->
        conn |> put_status(:service_unavailable) |> json(%{error: "Unable to queue verification."})
    end
  end

  def create(
        conn,
        %{
          "smart_contract" => smart_contract,
          "verification_type" => "vyper"
        }
      ) do
    Que.add(VyperPublisherWorker, {smart_contract["address_hash"], smart_contract, conn})

    send_resp(conn, 204, "")
  end

  def create(
        conn,
        %{
          "address_hash" => address_hash_string,
          "file" => files,
          "verification_type" => "json:metadata"
        }
      ) do
    files_array = PublishHelper.prepare_files_array(files)

    json_file = PublishHelper.get_one_json(files_array)

    if json_file do
      if SmartContract.verified_with_full_match?(address_hash_string) do
        EventsPublisher.broadcast(
          PublishHelper.prepare_verification_error(
            "This contract already verified in Blockscout.",
            address_hash_string,
            conn
          ),
          :on_demand
        )
      else
        check_in_sourcify(address_hash_string, files_array, conn)
      end
    else
      EventsPublisher.broadcast(
        PublishHelper.prepare_verification_error(
          "Please attach JSON file with metadata of contract's compilation.",
          address_hash_string,
          conn
        ),
        :on_demand
      )
    end

    send_resp(conn, 204, "")
  end

  def create(conn, _params) do
    Que.add(SolidityPublisherWorker, {"", %{}, %{}, conn})

    send_resp(conn, 204, "")
  end

  defp enqueue_standard_json(smart_contract, json_input, conn) do
    Que.add(SolidityPublisherWorker, {"json_web", smart_contract, json_input, conn})
  catch
    :exit, _ -> {:error, :queue_unavailable}
  end

  defp check_in_sourcify(address_hash_string, files_array, conn) do
    case Sourcify.check_by_address(address_hash_string) do
      {:ok, _verified_status} ->
        PublishHelper.get_metadata_and_publish(address_hash_string, conn)

      _ ->
        PublishHelper.verify_and_publish(address_hash_string, files_array, conn, false)
    end
  end
end
