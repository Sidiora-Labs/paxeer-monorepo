defmodule BlockScoutWeb.Repair2231388Test do
  use ExUnit.Case, async: false

  import Phoenix.ChannelTest

  alias BlockScoutWeb.RealtimeEventHandlers.Main
  alias Explorer.Chain.Events.Publisher
  alias Explorer.Chain.Token.Instance
  alias Phoenix.Socket.Message

  @endpoint BlockScoutWeb.Endpoint
  @contract "0x06012c8cf97bead5deae237070f9587f8e7a266d"
  @other_contract "0x5caebd3b32e210e85ce3e9d51638b9c445481567"

  # Recorded CryptoKitties response from Indexer.Fetcher.TokenInstance.HelperTest.
  @recorded_metadata ~S"""
  {
    "id": 100500,
    "name": "KittyBlue_2_Lemonade",
    "generation": 20,
    "genes": "623509754227292470437941473598751240781530569131665917719736997423495595",
    "created_at": "2017-12-06T01:56:27.000Z",
    "birthday": "2017-12-06T00:00:00.000Z",
    "image_url": "https://img.cryptokitties.co/0x06012c8cf97bead5deae237070f9587f8e7a266d/100500.svg",
    "image_url_cdn": "https://img.cn.cryptokitties.co/0x06012c8cf97bead5deae237070f9587f8e7a266d/100500.svg",
    "color": "strawberry",
    "background_color": "#ffe0e5",
    "bio": "Shalom! I'm KittyBlue_2_Lemonade. I'm a professional Foreign Film Director and I love cantaloupe. I'm convinced that the world is flat. One day I'll prove it. It's pawesome to meet you!",
    "kitty_type": null,
    "is_fancy": false,
    "is_exclusive": false,
    "is_special_edition": false,
    "fancy_type": null,
    "language": "en",
    "is_prestige": false,
    "prestige_type": null,
    "prestige_ranking": null,
    "prestige_time_limit": null,
    "status": {
      "is_ready": true,
      "is_gestating": false,
      "cooldown": 1410310201506,
      "dynamic_cooldown": 1475064986478,
      "cooldown_index": 10,
      "cooldown_end_block": 0,
      "pending_transaction_type": null,
      "pending_tx_since": null
    },
    "purrs": {
      "count": 1,
      "is_purred": false
    },
    "watchlist": {
      "count": 0,
      "is_watchlisted": false
    },
    "hatcher": {
      "address": "0x7b9ea9ac69b8fde875554321472c732eeff06ca0",
      "image": "14",
      "nickname": "KittyBlu",
      "hasDapper": false,
      "twitter_id": null,
      "twitter_image_url": null,
      "twitter_handle": null
    },
    "auction": {},
    "offer": {},
    "owner": {
      "address": "0x7b9ea9ac69b8fde875554321472c732eeff06ca0",
      "hasDapper": false,
      "twitter_id": null,
      "twitter_image_url": null,
      "twitter_handle": null,
      "image": "14",
      "nickname": "KittyBlu"
    },
    "matron": {
      "id": 46234,
      "name": "KittyBlue_1_Limegreen",
      "generation": 10,
      "enhanced_cattributes": [
        {
          "type": "body",
          "kittyId": 19631,
          "position": 105,
          "description": "cymric"
        },
        {
          "type": "coloreyes",
          "kittyId": 40356,
          "position": 263,
          "description": "limegreen"
        },
        {
          "type": "eyes",
          "kittyId": 3185,
          "position": 16,
          "description": "raisedbrow"
        },
        {
          "type": "pattern",
          "kittyId": 46234,
          "position": -1,
          "description": "totesbasic"
        },
        {
          "type": "mouth",
          "kittyId": 46234,
          "position": -1,
          "description": "happygokitty"
        },
        {
          "type": "colorprimary",
          "kittyId": 46234,
          "position": -1,
          "description": "greymatter"
        },
        {
          "type": "colorsecondary",
          "kittyId": 46234,
          "position": -1,
          "description": "lemonade"
        },
        {
          "type": "colortertiary",
          "kittyId": 46234,
          "position": -1,
          "description": "granitegrey"
        }
      ],
      "owner_wallet_address": "0x7b9ea9ac69b8fde875554321472c732eeff06ca0",
      "owner": {
        "address": "0x7b9ea9ac69b8fde875554321472c732eeff06ca0"
      },
      "created_at": "2017-12-03T21:29:17.000Z",
      "image_url": "https://img.cryptokitties.co/0x06012c8cf97bead5deae237070f9587f8e7a266d/46234.svg",
      "image_url_cdn": "https://img.cn.cryptokitties.co/0x06012c8cf97bead5deae237070f9587f8e7a266d/46234.svg",
      "color": "limegreen",
      "is_fancy": false,
      "kitty_type": null,
      "is_exclusive": false,
      "is_special_edition": false,
      "fancy_type": null,
      "status": {
        "is_ready": true,
        "is_gestating": false,
        "cooldown": 1486487069384
      },
      "hatched": true,
      "wrapped": false,
      "image_url_png": "https://img.cryptokitties.co/0x06012c8cf97bead5deae237070f9587f8e7a266d/46234.png"
    },
    "sire": {
      "id": 82090,
      "name": null,
      "generation": 19,
      "enhanced_cattributes": [
        {
          "type": "body",
          "kittyId": 82090,
          "position": -1,
          "description": "himalayan"
        },
        {
          "type": "coloreyes",
          "kittyId": 82090,
          "position": -1,
          "description": "strawberry"
        },
        {
          "type": "eyes",
          "kittyId": 82090,
          "position": -1,
          "description": "thicccbrowz"
        },
        {
          "type": "pattern",
          "kittyId": 82090,
          "position": -1,
          "description": "totesbasic"
        },
        {
          "type": "mouth",
          "kittyId": 82090,
          "position": -1,
          "description": "pouty"
        },
        {
          "type": "colorprimary",
          "kittyId": 82090,
          "position": -1,
          "description": "aquamarine"
        },
        {
          "type": "colorsecondary",
          "kittyId": 82090,
          "position": -1,
          "description": "chocolate"
        },
        {
          "type": "colortertiary",
          "kittyId": 82090,
          "position": -1,
          "description": "granitegrey"
        }
      ],
      "owner_wallet_address": "0x798fdad0cedc4b298fc7d53a982fa0c5f447eaa5",
      "owner": {
        "address": "0x798fdad0cedc4b298fc7d53a982fa0c5f447eaa5"
      },
      "created_at": "2017-12-05T06:30:05.000Z",
      "image_url": "https://img.cryptokitties.co/0x06012c8cf97bead5deae237070f9587f8e7a266d/82090.svg",
      "image_url_cdn": "https://img.cn.cryptokitties.co/0x06012c8cf97bead5deae237070f9587f8e7a266d/82090.svg",
      "color": "strawberry",
      "is_fancy": false,
      "is_exclusive": false,
      "is_special_edition": false,
      "fancy_type": null,
      "status": {
        "is_ready": true,
        "is_gestating": false,
        "cooldown": 1486619010030
      },
      "kitty_type": null,
      "hatched": true,
      "wrapped": false,
      "image_url_png": "https://img.cryptokitties.co/0x06012c8cf97bead5deae237070f9587f8e7a266d/82090.png"
    },
    "children": [],
    "hatched": true,
    "wrapped": false,
    "enhanced_cattributes": [
      {
        "type": "colorprimary",
        "description": "greymatter",
        "position": null,
        "kittyId": 100500
      },
      {
        "type": "coloreyes",
        "description": "strawberry",
        "position": null,
        "kittyId": 100500
      },
      {
        "type": "body",
        "description": "himalayan",
        "position": null,
        "kittyId": 100500
      },
      {
        "type": "colorsecondary",
        "description": "lemonade",
        "position": null,
        "kittyId": 100500
      },
      {
        "type": "mouth",
        "description": "pouty",
        "position": null,
        "kittyId": 100500
      },
      {
        "type": "pattern",
        "description": "totesbasic",
        "position": null,
        "kittyId": 100500
      },
      {
        "type": "eyes",
        "description": "thicccbrowz",
        "position": null,
        "kittyId": 100500
      },
      {
        "type": "colortertiary",
        "description": "kittencream",
        "position": null,
        "kittyId": 100500
      },
      {
        "type": "secret",
        "description": "se5",
        "position": -1,
        "kittyId": 100500
      },
      {
        "type": "purrstige",
        "description": "pu20",
        "position": -1,
        "kittyId": 100500
      }
    ],
    "variation": null,
    "variation_ranking": null,
    "image_url_png": "https://img.cryptokitties.co/0x06012c8cf97bead5deae237070f9587f8e7a266d/100500.png",
    "items": []
  }
  """

  test "legacy-token-instance-topic: /socket joins and delivers metadata on the legacy topic" do
    prove_metadata_delivery("/socket", BlockScoutWeb.UserSocket, "token_instances_old:")
  end

  test "legacy-token-instance-topic: /socket/v2 retains join and metadata delivery" do
    prove_metadata_delivery("/socket/v2", BlockScoutWeb.V2.UserSocket, "token_instances:")
  end

  defp prove_metadata_delivery(path, handler, prefix) do
    assert {^path, ^handler, options} = List.keyfind(@endpoint.__sockets__(), path, 0)
    assert Keyword.has_key?(options, :websocket)
    assert {:ok, connection} = connect(handler, %{"locale" => "en"})
    topic = prefix <> @contract
    assert {:ok, %{}, socket} = join(connection, topic, %{})
    assert socket.channel == BlockScoutWeb.TokenInstanceChannel
    assert socket.topic == topic
    assert is_pid(Process.whereis(Main))

    {:ok, contract_hash} = Explorer.Chain.string_to_address_hash(@contract)

    instance = %Instance{
      token_contract_address_hash: contract_hash,
      token_id: Decimal.new(100_500),
      metadata: Jason.decode!(@recorded_metadata)
    }

    token_id = EthereumJSONRPC.NFT.prepare_token_id(instance.token_id)
    metadata = instance.metadata

    Publisher.broadcast(
      %{fetched_token_instance_metadata: [to_string(instance.token_contract_address_hash), token_id, metadata]},
      :on_demand
    )

    assert_receive %Message{
                     topic: ^topic,
                     event: "fetched_token_instance_metadata",
                     payload: %{token_id: "100500", fetched_metadata: ^metadata}
                   },
                   5_000

    for reason <- ["error", "retry_cooldown"] do
      Publisher.broadcast(
        %{not_fetched_token_instance_metadata: [to_string(instance.token_contract_address_hash), token_id, reason]},
        :on_demand
      )

      assert_receive %Message{
                       topic: ^topic,
                       event: "not_fetched_token_instance_metadata",
                       payload: %{token_id: "100500", reason: ^reason}
                     },
                     5_000
    end

    Publisher.broadcast(
      %{fetched_token_instance_metadata: [@other_contract, token_id, metadata]},
      :on_demand
    )

    Publisher.broadcast(
      %{not_fetched_token_instance_metadata: [@other_contract, token_id, "error"]},
      :on_demand
    )

    refute_receive %Message{event: "fetched_token_instance_metadata"}
    refute_receive %Message{event: "not_fetched_token_instance_metadata"}
    assert Process.alive?(socket.channel_pid)
  end
end
