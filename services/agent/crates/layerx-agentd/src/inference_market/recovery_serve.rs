use super::super::laps::recovery::RecoveryRecord;
use super::{Cursor, ServeError, ServeRoutes};

impl ServeRoutes {
    pub fn resume_recovery(&self, record: &RecoveryRecord) -> Result<(), ServeError> {
        let frame = record
            .replacement
            .as_ref()
            .ok_or_else(ServeError::unavailable)?;
        let mut journal = self.journal.lock().map_err(|_| ServeError::unavailable())?;
        if let Some(cursor) = journal.cursors.get(&frame.ask_id) {
            if cursor.stream != record.stream_id
                || cursor.cumulative != record.resume.cumulative_tokens
            {
                return Err(ServeError::upstream());
            }
            if cursor.deal == frame.deal_id {
                return if cursor.next_lap == record.resume.next_lap_index {
                    Ok(())
                } else {
                    Err(ServeError::upstream())
                };
            }
            if cursor.deal != record.previous_deal.deal_id {
                return Err(ServeError::upstream());
            }
        }
        journal.cursors.insert(
            frame.ask_id,
            Cursor {
                stream: record.stream_id,
                deal: frame.deal_id,
                next_lap: record.resume.next_lap_index,
                cumulative: record.resume.cumulative_tokens,
            },
        );
        Ok(())
    }
}

#[cfg(test)]
pub mod tests {
    use super::super::InferenceBootGate;
    use super::*;
    use std::sync::{Arc, Mutex};

    fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
        result.unwrap_or_else(|error| panic!("{error:?}"))
    }

    fn cursor(routes: &ServeRoutes, ask: &[u8; 32]) -> ([u8; 32], [u8; 32], u64, u64) {
        let journal = must(routes.journal.lock());
        let cursor = journal
            .cursors
            .get(ask)
            .unwrap_or_else(|| panic!("retained completion cursor"));
        (
            cursor.stream,
            cursor.deal,
            cursor.next_lap,
            cursor.cumulative,
        )
    }

    pub fn assert_resume_cursor(record: &RecoveryRecord) {
        let frame = record
            .replacement
            .as_ref()
            .unwrap_or_else(|| panic!("replacement deal"));
        let routes = must(ServeRoutes::new(
            frame.room_id,
            frame.model_digest,
            None,
            None,
            Arc::new(Mutex::new(InferenceBootGate::default())),
        ));
        let previous = Cursor {
            stream: record.stream_id,
            deal: record.previous_deal.deal_id,
            next_lap: record.resume.next_lap_index,
            cumulative: record.resume.cumulative_tokens,
        };
        must(routes.journal.lock())
            .cursors
            .insert(frame.ask_id, previous);
        must(routes.resume_recovery(record));
        let resumed = cursor(&routes, &frame.ask_id);
        assert_eq!(
            resumed,
            (
                record.stream_id,
                frame.deal_id,
                record
                    .resume
                    .last_countersigned_lap
                    .unwrap_or_else(|| panic!("countersigned lap"))
                    + 1,
                record.resume.cumulative_tokens,
            )
        );
        must(routes.resume_recovery(record));
        assert_eq!(cursor(&routes, &frame.ask_id), resumed);

        for deal in [previous.deal, frame.deal_id] {
            let valid = Cursor { deal, ..previous };
            let mut wrong_deal = valid;
            wrong_deal.deal[0] ^= 1;
            let mut wrong_stream = valid;
            wrong_stream.stream[0] ^= 1;
            let mut wrong_cumulative = valid;
            wrong_cumulative.cumulative += 1;
            for invalid in [wrong_deal, wrong_stream, wrong_cumulative] {
                must(routes.journal.lock())
                    .cursors
                    .insert(frame.ask_id, invalid);
                let before = cursor(&routes, &frame.ask_id);
                assert_eq!(routes.resume_recovery(record), Err(ServeError::upstream()));
                assert_eq!(cursor(&routes, &frame.ask_id), before);
            }
        }
        must(routes.journal.lock()).cursors.clear();
        must(routes.resume_recovery(record));
        assert_eq!(cursor(&routes, &frame.ask_id), resumed);
    }
}
