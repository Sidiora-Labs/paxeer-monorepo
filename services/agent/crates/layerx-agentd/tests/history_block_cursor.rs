#[path = "history.rs"]
mod history_integration;

use history_integration::{
    context, history_limits, page_result, position, result, rows, HistoryServer,
};
use layerx_agentd::read::{history, Cursor, HistoryLimits, HistoryReadError};
use layerx_client::read::{ReadError, Requested};
use layerx_types::verify::VerificationLevel;
use serde_json::json;

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
    let server = HistoryServer::new(vec![(None, 3, page_result(None, 3))]);
    let mut transport = server.connect();
    let limits = HistoryLimits {
        maximum_items: 3,
        maximum_response_bytes: 1_048_576,
    };
    assert_eq!(
        history(&mut transport, 12, 10, None, limits, context()),
        Err(HistoryReadError::InvalidRange)
    );
    assert_eq!(
        history(&mut transport, 10, 11, Some(cursor), limits, context()),
        Err(HistoryReadError::Cursor)
    );
    assert_eq!(
        history(
            &mut transport,
            10,
            12,
            None,
            HistoryLimits {
                maximum_items: 0,
                ..limits
            },
            context()
        ),
        Err(HistoryReadError::InvalidLimits)
    );
    assert_eq!(
        history(
            &mut transport,
            10,
            12,
            None,
            HistoryLimits {
                maximum_response_bytes: 0,
                ..limits
            },
            context()
        ),
        Err(HistoryReadError::InvalidLimits)
    );
    let mut verified_context = context();
    verified_context.requested = Requested::new(VerificationLevel::BATCH_INCLUDED);
    assert_eq!(
        history(&mut transport, 10, 12, None, limits, verified_context),
        Err(HistoryReadError::Read(ReadError::MissingEvidence {
            requested: VerificationLevel::BATCH_INCLUDED,
            achieved: VerificationLevel::UNVERIFIED,
        }))
    );
    let page = history(&mut transport, 10, 12, None, limits, context()).unwrap();
    assert_eq!(page.items.len(), 3);
    server.finish();
}

#[test]
fn history_pages_resume_from_the_returned_cursor() {
    let expected_next = position(11, 0, 0);
    let server = HistoryServer::new(vec![
        (None, 3, page_result(None, 3)),
        (Some(expected_next), 3, page_result(Some(expected_next), 3)),
        (None, 5, page_result(None, 5)),
    ]);
    let first = server.page(None, 3);
    assert_eq!(first.items.len(), 3);
    assert!(first.response_bytes <= 1_048_576);
    let next = first
        .next_cursor
        .unwrap_or_else(|| panic!("first page must have a cursor"));
    assert_eq!(first.items.last().map(|item| item.position), Some(next));
    assert_eq!(next, expected_next);
    let encoded = next.to_hex();
    let resumed = server.page(
        Some(Cursor::from_hex(&encoded).unwrap_or_else(|error| panic!("cursor: {error:?}"))),
        3,
    );
    assert_eq!(resumed.items.len(), 2);
    assert!(resumed.response_bytes <= 1_048_576);
    assert!(resumed.next_cursor.is_none());
    let joined: Vec<_> = first.items.into_iter().chain(resumed.items).collect();
    assert_eq!(joined.len(), 5);
    assert!(joined
        .windows(2)
        .all(|pair| pair[0].position < pair[1].position));
    assert_eq!(
        joined.first().map(|item| item.position.block_height),
        Some(10)
    );
    assert_eq!(
        joined.last().map(|item| item.position.block_height),
        Some(12)
    );
    let complete = server.page(None, 5);
    assert_eq!(joined, complete.items);
    assert!(complete.next_cursor.is_none());
    server.finish();

    let oversized = HistoryServer::new(vec![(None, 3, result(rows(), None))]);
    assert_eq!(
        oversized.read(None, history_limits(3)),
        Err(HistoryReadError::Oversized)
    );
    oversized.finish();

    let byte_bound = HistoryServer::new(vec![(None, 3, page_result(None, 3))]);
    assert_eq!(
        byte_bound.read(
            None,
            HistoryLimits {
                maximum_response_bytes: 20,
                ..history_limits(3)
            }
        ),
        Err(HistoryReadError::Oversized)
    );
    byte_bound.finish();

    let mut unordered = rows()[..3].to_vec();
    unordered.swap(0, 1);
    let mut duplicate = rows()[..3].to_vec();
    duplicate[1] = duplicate[0].clone();
    let mut overflow = rows()[..1].to_vec();
    overflow[0]["txIndex"] = json!("4294967296");
    let mut invalid_kind = rows()[..1].to_vec();
    invalid_kind[0]["kind"] = json!("Unknown");
    for (response, expected) in [
        (result(unordered, None), HistoryReadError::Cursor),
        (result(duplicate, None), HistoryReadError::Cursor),
        (result(overflow, None), HistoryReadError::Cursor),
        (result(invalid_kind, None), HistoryReadError::Malformed),
        (
            result(rows()[..3].to_vec(), Some(position(12, 0, 0))),
            HistoryReadError::Cursor,
        ),
        (
            result(Vec::new(), Some(expected_next)),
            HistoryReadError::Cursor,
        ),
    ] {
        let refused = HistoryServer::new(vec![(None, 3, response)]);
        assert_eq!(refused.read(None, history_limits(3)), Err(expected));
        refused.finish();
    }
    let repeated = HistoryServer::new(vec![(
        Some(expected_next),
        3,
        result(rows()[2..].to_vec(), None),
    )]);
    assert_eq!(
        repeated.read(Some(expected_next), history_limits(3)),
        Err(HistoryReadError::Cursor)
    );
    repeated.finish();

    let within_block = position(12, 0, 0);
    let last_log = HistoryServer::new(vec![(
        Some(within_block),
        3,
        page_result(Some(within_block), 3),
    )]);
    let last = last_log.page(Some(within_block), 3);
    assert_eq!(last.items.len(), 1);
    assert_eq!(last.items[0].position, position(12, 0, 1));
    assert!(last.next_cursor.is_none());
    last_log.finish();

    let empty = HistoryServer::new(vec![(None, 3, result(Vec::new(), None))]);
    let page = empty.page(None, 3);
    assert!(page.items.is_empty());
    assert!(page.next_cursor.is_none());
    empty.finish();
}
