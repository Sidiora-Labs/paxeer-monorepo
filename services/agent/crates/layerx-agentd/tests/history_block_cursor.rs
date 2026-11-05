use layerx_agentd::read::{Cursor, HistoryLimits, HistoryPage, HistoryReadError};
use layerx_client::sequencer::SequencerEndpoint;

#[test]
fn history_cursor_is_the_block_position_triple() {
    let cursor = Cursor {
        block_height: 12,
        tx_index: 3,
        log_index: 4,
    };
    assert_eq!(cursor.to_hex(), "000000000000000c0000000300000004");
    let distinct_bytes = Cursor {
        block_height: 0x0102_0304_0506_0708,
        tx_index: 0x090a_0b0c,
        log_index: 0x0d0e_0f10,
    };
    assert_eq!(distinct_bytes.to_hex(), "0102030405060708090a0b0c0d0e0f10");
    for value in [
        cursor,
        distinct_bytes,
        Cursor {
            block_height: 0,
            tx_index: 0,
            log_index: 0,
        },
        Cursor {
            block_height: u64::MAX,
            tx_index: u32::MAX,
            log_index: u32::MAX,
        },
    ] {
        assert_eq!(Cursor::from_hex(&value.to_hex()), Ok(value));
    }
    for length in (0..=112).filter(|length| *length != 32) {
        assert_eq!(
            Cursor::from_hex(&"0".repeat(length)),
            Err(HistoryReadError::Cursor)
        );
    }
    for invalid in [
        "000000000000000c000000030000000G",
        "000000000000000C0000000300000004",
        "é0000000000000c0000000300000004",
        "0x000000000000000c0000000300000004",
    ] {
        assert_eq!(Cursor::from_hex(invalid), Err(HistoryReadError::Cursor));
    }

    let endpoint = SequencerEndpoint::new("http://127.0.0.1:1/", &[]).unwrap();
    let limits = HistoryLimits {
        maximum_items: 3,
        maximum_response_bytes: 1_048_576,
    };
    assert_eq!(
        HistoryPage::read_account(&endpoint, "account", 12, 10, None, limits),
        Err(HistoryReadError::InvalidRange)
    );
    assert_eq!(
        HistoryPage::read_account(&endpoint, "account", 10, 11, Some(cursor), limits),
        Err(HistoryReadError::Cursor)
    );
    assert_eq!(
        HistoryPage::read_account(
            &endpoint,
            "account",
            10,
            12,
            None,
            HistoryLimits {
                maximum_items: 0,
                ..limits
            },
        ),
        Err(HistoryReadError::InvalidLimits)
    );
    assert_eq!(
        HistoryPage::read_account(
            &endpoint,
            "account",
            10,
            12,
            None,
            HistoryLimits {
                maximum_response_bytes: 0,
                ..limits
            },
        ),
        Err(HistoryReadError::InvalidLimits)
    );
}
