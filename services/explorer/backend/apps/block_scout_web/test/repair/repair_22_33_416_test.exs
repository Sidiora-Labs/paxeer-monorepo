Code.require_file("../block_scout_web/controllers/api/v2/paxeer_x/repair_evidence.exs", __DIR__)
Code.require_file("../block_scout_web/controllers/api/v2/paxeer_x/capabilities_controller_test.exs", __DIR__)
Code.require_file("../block_scout_web/controllers/api/v2/paxeer_x/feed_controller_test.exs", __DIR__)
Code.require_file("../block_scout_web/controllers/api/v2/paxeer_x/unified_account_controller_test.exs", __DIR__)

defmodule BlockScoutWeb.Repair.Repair2233416Test do
  use BlockScoutWeb.ConnCase, async: false

  alias BlockScoutWeb.API.V2.PaxeerX.{CapabilitiesControllerTest, FeedControllerTest, UnifiedAccountControllerTest}

  test "capability_nonproducer_abi_canonical_response", %{conn: conn} do
    CapabilitiesControllerTest.prove_capability(conn, :canonical)
  end

  for valid <- [:nonempty_string, :word_aligned_string] do
    test "capability_nonproducer_abi_accepts_#{valid}", %{conn: conn} do
      CapabilitiesControllerTest.prove_capability(conn, unquote(valid))
    end
  end

  for malformed <- [
        :refused,
        :wrong_selector,
        :zero_offset,
        :unaligned_offset,
        :outside_offset,
        :missing_tail,
        :truncated_length,
        :truncated_string,
        :invalid_address_padding,
        :invalid_string_padding,
        :extra_tail
      ] do
    test "capability_nonproducer_abi_rejects_#{malformed}", %{conn: conn} do
      CapabilitiesControllerTest.prove_capability(conn, unquote(malformed))
    end
  end

  test "feed_producer_restart_proof", %{conn: conn} do
    FeedControllerTest.prove_producer_restart(conn)
  end

  for variant <- [:refused, :failed, :mixed] do
    test "feed_refused_market_observations_#{variant}", %{conn: conn} do
      FeedControllerTest.prove_refused_observations(conn, unquote(variant))
    end
  end

  test "unified_impossible_did_fixture_restart_and_unbind", %{conn: conn} do
    UnifiedAccountControllerTest.prove_identity_restart_and_unbind(conn)
  end
end
