use std::sync::Mutex;
use std::time::Duration;

use layerx_types::clock::{Clock, ClockError, ClockReading};
use layerx_types::test_support::DeterministicClock;

pub struct TestClock {
    pub(crate) ticks: Mutex<DeterministicClock>,
    pub(crate) generation: [u8; 16],
}

impl TestClock {
    pub const fn new(seconds: u64) -> Self {
        Self {
            ticks: Mutex::new(DeterministicClock::new(seconds)),
            generation: [1; 16],
        }
    }

    pub fn now(&self) -> u64 {
        self.ticks.lock().expect("test clock").now()
    }
}

impl Clock for TestClock {
    fn sample(&self, _: Duration) -> Result<ClockReading, ClockError> {
        let seconds = self.now();
        Ok(ClockReading {
            generation: self.generation,
            unix_milliseconds: seconds.checked_mul(1_000).ok_or(ClockError::Overflow)?,
            monotonic_nanoseconds: seconds
                .checked_mul(1_000_000_000)
                .ok_or(ClockError::Overflow)?,
        })
    }
}
