from kernel_retirement import assert_retired


def test_paxeer_withdrawal_codec_is_retired():
    assert_retired(["contracts/libraries/PaxeerWithdrawalCodec.sol"])
