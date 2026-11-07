#[cfg(test)]
mod streaming_resumability_tests {
    use layerx_sdk::production::{SdkErrorCode, StreamEvent, StreamPage};
    use serde_json::{json, Value};

    type StreamCursor = layerx_sdk::production::StreamCursor;
    type ResumableStream = layerx_sdk::production::ResumableStream;

    fn log_cursor(block_height: u64, tx_index: u32, log_index: u32) -> StreamCursor {
        StreamCursor::new(format!("{block_height:016x}{tx_index:08x}{log_index:08x}"))
            .unwrap_or_else(|error| panic!("cursor: {error:?}"))
    }

    fn log_event(previous_cursor: &StreamCursor, position: (u64, u32, u32)) -> StreamEvent<Value> {
        let (block_height, tx_index, log_index) = position;
        let cursor = log_cursor(block_height, tx_index, log_index);
        StreamEvent {
            event_id: format!("log:{}", cursor.as_str()),
            previous_cursor: previous_cursor.clone(),
            cursor,
            value: json!({
                "jsonrpc": "2.0",
                "method": "eth_subscription",
                "params": {
                    "subscription": "0x1",
                    "result": {
                        "blockNumber": format!("0x{block_height:x}"),
                        "transactionIndex": format!("0x{tx_index:x}"),
                        "logIndex": format!("0x{log_index:x}"),
                        "address": "0x0000000000000000000000000000000000000000",
                        "topics": [],
                        "data": "0x",
                        "removed": false,
                    },
                },
            }),
        }
    }

    #[test]
    fn log_cursor_resumes_after_last_delivered_log() {
        let initial_cursor = log_cursor(9, 0, 0);
        let mut stream = ResumableStream::new(initial_cursor.clone());
        let first = log_event(&initial_cursor, (10, 0, 0));
        let second = log_event(&first.cursor, (10, 0, 1));
        let delivered = stream
            .accept(StreamPage {
                requested_cursor: initial_cursor,
                events: vec![first.clone(), second.clone()],
                next_cursor: second.cursor.clone(),
            })
            .unwrap_or_else(|error| panic!("initial page: {error:?}"));
        assert_eq!(delivered, vec![first, second.clone()]);
        assert_eq!(stream.cursor(), &log_cursor(10, 0, 1));

        let saved_cursor = stream.cursor().clone();
        let mut resumed = ResumableStream::new(saved_cursor.clone());
        let third = log_event(&saved_cursor, (11, 2, 0));
        let resumed_logs = resumed
            .accept(StreamPage {
                requested_cursor: saved_cursor.clone(),
                events: vec![third.clone()],
                next_cursor: third.cursor.clone(),
            })
            .unwrap_or_else(|error| panic!("resumed log: {error:?}"));
        assert_eq!(resumed_logs, vec![third.clone()]);
        assert_eq!(resumed.cursor(), &log_cursor(11, 2, 0));
        assert_eq!(
            delivered
                .iter()
                .chain(&resumed_logs)
                .map(|event| event.cursor.clone())
                .collect::<Vec<_>>(),
            vec![log_cursor(10, 0, 0), log_cursor(10, 0, 1), third.cursor]
        );

        let mut reconnected = ResumableStream::new(saved_cursor.clone());
        for position in [(10, 0, 0), (10, 0, 1)] {
            let replayed = log_event(&saved_cursor, position);
            let result = reconnected.accept(StreamPage {
                requested_cursor: saved_cursor.clone(),
                next_cursor: replayed.cursor.clone(),
                events: vec![replayed],
            });
            let refusal =
                result.expect_err("reconnected stream delivered a log at or before its cursor");
            assert_eq!(refusal.code, SdkErrorCode::DecodeFailure);
            assert_eq!(reconnected.cursor(), &saved_cursor);
        }

        let positions = [(10, 0, 2), (10, 1, 0), (11, 0, 0)];
        let mut previous = saved_cursor.clone();
        let events: Vec<_> = positions
            .into_iter()
            .map(|position| {
                let event = log_event(&previous, position);
                previous = event.cursor.clone();
                event
            })
            .collect();
        let accepted = reconnected
            .accept(StreamPage {
                requested_cursor: saved_cursor.clone(),
                events: events.clone(),
                next_cursor: previous.clone(),
            })
            .unwrap_or_else(|error| panic!("resumed page: {error:?}"));
        assert_eq!(accepted, events);
        assert!(accepted
            .iter()
            .all(|event| event.cursor.as_str() > saved_cursor.as_str()));
        assert_eq!(reconnected.cursor(), &log_cursor(11, 0, 0));
        let empty: StreamPage<Value> = StreamPage {
            requested_cursor: previous.clone(),
            events: vec![],
            next_cursor: previous.clone(),
        };
        assert!(reconnected
            .accept(empty)
            .unwrap_or_else(|error| panic!("empty page: {error:?}"))
            .is_empty());
        assert_eq!(reconnected.cursor(), &previous);
    }

    #[test]
    fn log_cursor_refuses_regressing_position() {
        let cases = [
            (None, (11, 1, 5)),
            (None, (11, 2, 0)),
            (None, (11, 1, u32::MAX)),
            (None, (10, u32::MAX, u32::MAX)),
            (Some((11, 2, 2)), (11, 2, 1)),
            (Some((11, 2, 1)), (11, 2, 1)),
            (Some((11, 3, 0)), (11, 2, 5)),
            (Some((12, 0, 0)), (11, 3, 5)),
        ];
        for (prefix_position, regressing_position) in cases {
            let requested = log_cursor(11, 2, 0);
            let mut stream = ResumableStream::new(requested.clone());
            let prefix: Vec<_> = prefix_position
                .into_iter()
                .map(|position| log_event(&requested, position))
                .collect();
            let previous = prefix.last().map_or(&requested, |event| &event.cursor);
            let mut regressing = log_event(previous, regressing_position);
            regressing.event_id = "regressing".to_string();
            let mut events = prefix.clone();
            events.push(regressing.clone());
            let result = stream.accept(StreamPage {
                requested_cursor: requested.clone(),
                events,
                next_cursor: regressing.cursor,
            });
            let refusal = result.expect_err("stream delivered a non-increasing log position");
            assert_eq!(refusal.code, SdkErrorCode::DecodeFailure);
            assert_eq!(stream.cursor(), &requested);

            let corrected = log_event(previous, (12, 1, 0));
            let mut corrected_events = prefix.clone();
            corrected_events.push(corrected.clone());
            let accepted = stream
                .accept(StreamPage {
                    requested_cursor: requested,
                    events: corrected_events.clone(),
                    next_cursor: corrected.cursor.clone(),
                })
                .unwrap_or_else(|error| panic!("corrected page: {error:?}"));
            assert_eq!(accepted, corrected_events);
            assert_eq!(stream.cursor(), &corrected.cursor);
        }
    }

    #[test]
    fn stream_cursor_constructs_valid_cursors() {
        let cursor = log_cursor(10, 2, 1);
        assert_eq!(cursor.as_str(), "000000000000000a0000000200000001");
    }

    #[test]
    fn stream_cursor_refuses_empty_cursors() {
        let cursor = StreamCursor::new("");
        let Err(refusal) = cursor else {
            panic!("empty cursor accepted")
        };
        assert_eq!(refusal.code, SdkErrorCode::InvalidArgument);
    }

    #[test]
    fn stream_cursor_refuses_nul_containing_cursors() {
        let cursor = StreamCursor::new("has\0null");
        assert!(cursor.is_err());
    }

    #[test]
    fn resumable_stream_accepts_empty_page() {
        let initial_cursor = log_cursor(9, 0, 0);
        let mut stream = ResumableStream::new(initial_cursor.clone());
        let page: StreamPage<StreamEvent<&str>> = StreamPage {
            requested_cursor: initial_cursor.clone(),
            events: vec![],
            next_cursor: initial_cursor.clone(),
        };
        let accepted = stream
            .accept(page)
            .unwrap_or_else(|error| panic!("accept: {error:?}"));
        assert_eq!(accepted.len(), 0);
        assert_eq!(stream.cursor(), &log_cursor(9, 0, 0));
    }

    #[test]
    fn resumable_stream_accepts_single_event() {
        let initial_cursor = log_cursor(9, 0, 0);
        let mut stream = ResumableStream::new(initial_cursor.clone());
        let next_cursor = log_cursor(10, 0, 0);
        let page = StreamPage {
            requested_cursor: initial_cursor.clone(),
            events: vec![StreamEvent {
                event_id: "e1".to_string(),
                previous_cursor: initial_cursor.clone(),
                cursor: next_cursor.clone(),
                value: "first",
            }],
            next_cursor: next_cursor.clone(),
        };
        let accepted = stream
            .accept(page)
            .unwrap_or_else(|error| panic!("accept: {error:?}"));
        assert_eq!(accepted.len(), 1);
        assert_eq!(accepted[0].value, "first");
        assert_eq!(stream.cursor(), &log_cursor(10, 0, 0));
    }

    #[test]
    fn resumable_stream_accepts_multiple_events_forming_chain() {
        let initial_cursor = log_cursor(9, 0, 0);
        let mut stream = ResumableStream::new(initial_cursor.clone());
        let c1 = log_cursor(10, 0, 0);
        let c2 = log_cursor(10, 0, 1);
        let c3 = log_cursor(11, 2, 0);
        let page = StreamPage {
            requested_cursor: initial_cursor.clone(),
            events: vec![
                StreamEvent {
                    event_id: "e1".to_string(),
                    previous_cursor: initial_cursor.clone(),
                    cursor: c1.clone(),
                    value: "first",
                },
                StreamEvent {
                    event_id: "e2".to_string(),
                    previous_cursor: c1.clone(),
                    cursor: c2.clone(),
                    value: "second",
                },
                StreamEvent {
                    event_id: "e3".to_string(),
                    previous_cursor: c2.clone(),
                    cursor: c3.clone(),
                    value: "third",
                },
            ],
            next_cursor: c3.clone(),
        };
        let accepted = stream
            .accept(page)
            .unwrap_or_else(|error| panic!("accept: {error:?}"));
        assert_eq!(accepted.len(), 3);
        assert_eq!(stream.cursor(), &log_cursor(11, 2, 0));
    }

    #[test]
    fn resumable_stream_refuses_page_with_wrong_requested_cursor() {
        let initial_cursor = log_cursor(9, 0, 0);
        let mut stream = ResumableStream::new(initial_cursor.clone());
        let wrong_cursor = log_cursor(8, 0, 0);
        let page: StreamPage<StreamEvent<&str>> = StreamPage {
            requested_cursor: wrong_cursor.clone(),
            events: vec![],
            next_cursor: wrong_cursor.clone(),
        };
        let result = stream.accept(page);
        let Err(refusal) = result else {
            panic!("page with wrong requested cursor accepted")
        };
        assert_eq!(refusal.code, SdkErrorCode::DecodeFailure);
    }

    #[test]
    fn resumable_stream_refuses_gap_in_cursor_chain() {
        let initial_cursor = log_cursor(9, 0, 0);
        let mut stream = ResumableStream::new(initial_cursor.clone());
        let c1 = log_cursor(10, 0, 0);
        let c2 = log_cursor(10, 0, 1);
        let page = StreamPage {
            requested_cursor: initial_cursor.clone(),
            events: vec![StreamEvent {
                event_id: "e1".to_string(),
                previous_cursor: c1.clone(),
                cursor: c2.clone(),
                value: "skipped",
            }],
            next_cursor: c2.clone(),
        };
        let result = stream.accept(page);
        assert!(result.is_err());
    }

    #[test]
    fn resumable_stream_refuses_duplicate_event_ids_across_pages() {
        let initial_cursor = log_cursor(9, 0, 0);
        let mut stream = ResumableStream::new(initial_cursor.clone());
        let c1 = log_cursor(10, 0, 0);
        let c2 = log_cursor(10, 0, 1);
        let first_page = StreamPage {
            requested_cursor: initial_cursor.clone(),
            events: vec![StreamEvent {
                event_id: "e1".to_string(),
                previous_cursor: initial_cursor.clone(),
                cursor: c1.clone(),
                value: "first",
            }],
            next_cursor: c1.clone(),
        };
        stream
            .accept(first_page)
            .unwrap_or_else(|error| panic!("accept: {error:?}"));
        let duplicate_page = StreamPage {
            requested_cursor: c1.clone(),
            events: vec![StreamEvent {
                event_id: "e1".to_string(),
                previous_cursor: c1.clone(),
                cursor: c2.clone(),
                value: "duplicate",
            }],
            next_cursor: c2.clone(),
        };
        let result = stream.accept(duplicate_page);
        assert!(result.is_err());
    }

    #[test]
    fn resumable_stream_refuses_duplicate_event_ids_within_page() {
        let initial_cursor = log_cursor(9, 0, 0);
        let mut stream = ResumableStream::new(initial_cursor.clone());
        let c1 = log_cursor(10, 0, 0);
        let c2 = log_cursor(10, 0, 1);
        let page = StreamPage {
            requested_cursor: initial_cursor.clone(),
            events: vec![
                StreamEvent {
                    event_id: "e1".to_string(),
                    previous_cursor: initial_cursor.clone(),
                    cursor: c1.clone(),
                    value: "first",
                },
                StreamEvent {
                    event_id: "e1".to_string(),
                    previous_cursor: c1.clone(),
                    cursor: c2.clone(),
                    value: "duplicate",
                },
            ],
            next_cursor: c2.clone(),
        };
        let result = stream.accept(page);
        assert!(result.is_err());
    }

    #[test]
    fn resumable_stream_refuses_empty_event_id() {
        let initial_cursor = log_cursor(9, 0, 0);
        let mut stream = ResumableStream::new(initial_cursor.clone());
        let c1 = log_cursor(10, 0, 0);
        let page = StreamPage {
            requested_cursor: initial_cursor.clone(),
            events: vec![StreamEvent {
                event_id: String::new(),
                previous_cursor: initial_cursor.clone(),
                cursor: c1.clone(),
                value: "no-id",
            }],
            next_cursor: c1.clone(),
        };
        let result = stream.accept(page);
        assert!(result.is_err());
    }

    #[test]
    fn resumable_stream_refuses_mismatched_next_cursor() {
        let initial_cursor = log_cursor(9, 0, 0);
        let mut stream = ResumableStream::new(initial_cursor.clone());
        let c1 = log_cursor(10, 0, 0);
        let c2 = log_cursor(10, 0, 1);
        let page = StreamPage {
            requested_cursor: initial_cursor.clone(),
            events: vec![StreamEvent {
                event_id: "e1".to_string(),
                previous_cursor: initial_cursor.clone(),
                cursor: c1.clone(),
                value: "first",
            }],
            next_cursor: c2.clone(),
        };
        let result = stream.accept(page);
        assert!(result.is_err());
    }

    #[test]
    fn resumable_stream_supports_resumption_after_disconnection() {
        let initial_cursor = log_cursor(9, 0, 0);
        let mut stream = ResumableStream::new(initial_cursor.clone());
        let c1 = log_cursor(10, 0, 0);
        let c2 = log_cursor(10, 0, 1);
        let first_page = StreamPage {
            requested_cursor: initial_cursor.clone(),
            events: vec![StreamEvent {
                event_id: "e1".to_string(),
                previous_cursor: initial_cursor.clone(),
                cursor: c1.clone(),
                value: "before-disconnect",
            }],
            next_cursor: c1.clone(),
        };
        stream
            .accept(first_page)
            .unwrap_or_else(|error| panic!("accept: {error:?}"));
        let resumed_page = StreamPage {
            requested_cursor: c1.clone(),
            events: vec![StreamEvent {
                event_id: "e2".to_string(),
                previous_cursor: c1.clone(),
                cursor: c2.clone(),
                value: "after-reconnect",
            }],
            next_cursor: c2.clone(),
        };
        let accepted = stream
            .accept(resumed_page)
            .unwrap_or_else(|error| panic!("accept: {error:?}"));
        assert_eq!(accepted.len(), 1);
        assert_eq!(accepted[0].value, "after-reconnect");
        assert_eq!(stream.cursor(), &log_cursor(10, 0, 1));
    }
}
