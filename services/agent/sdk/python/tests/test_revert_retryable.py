from layerx_sdk.revert import LXP_RETRYABLE


def test_site_codes_present_and_not_retryable():
    site_codes = (
        "LXP_PRICEFEED_DELEGATECALL",
        "LXP_PRICEFEED_STATICCALL",
        "LXP_PRICEFEED_KEEPER_MISSING",
        "LXP_PRICEFEED_BATCH_TOO_LARGE",
        "LXP_AGENT_NOT_ACTIVE",
        "LXP_BANK_STATICCALL",
        "LXP_BANK_DELEGATECALL",
        "LXP_BANK_ZERO_VALUE",
    )
    for code in site_codes:
        assert code in LXP_RETRYABLE, code
        assert LXP_RETRYABLE[code] is False, code
