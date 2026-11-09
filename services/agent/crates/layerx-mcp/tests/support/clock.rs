use std::sync::Mutex;
use std::time::Duration;

use layerx_types::clock::{Clock, ClockError, ClockReading};
use layerx_types::test_support::DeterministicClock;

pub struct TestClock {
    pub ticks: Mutex<DeterministicClock>,
    pub generation: [u8; 16],
}

impl TestClock {
    #[must_use]
    pub const fn new(seconds: u64) -> Self {
        Self {
            ticks: Mutex::new(DeterministicClock::new(seconds)),
            generation: [1; 16],
        }
    }

    /// # Panics
    /// Panics if another test thread poisoned the deterministic clock lock.
    #[must_use]
    pub fn now(&self) -> u64 {
        match self.ticks.lock() {
            Ok(ticks) => ticks.now(),
            Err(error) => panic!("test clock: {error:?}"),
        }
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
