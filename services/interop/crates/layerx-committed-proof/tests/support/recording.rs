use layerx_committed_proof::header::Timestamp;

pub fn recorded_base64(encoded: &str) -> Vec<u8> {
    const ALPHABET: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    assert_eq!(encoded.len() % 4, 0);
    let mut decoded = Vec::new();
    let chunks = encoded.as_bytes().chunks_exact(4);
    let count = chunks.len();
    for (index, chunk) in chunks.enumerate() {
        let padding = chunk.iter().rev().take_while(|byte| **byte == b'=').count();
        assert!(padding <= 2);
        assert!(padding == 0 || index + 1 == count);
        let mut word = 0_u32;
        for (position, byte) in chunk.iter().enumerate() {
            let digit = if position >= 4 - padding {
                assert_eq!(*byte, b'=');
                0
            } else {
                ALPHABET
                    .iter()
                    .position(|candidate| candidate == byte)
                    .unwrap_or_else(|| panic!("invalid recorded base64")) as u32
            };
            word = (word << 6) | digit;
        }
        assert_eq!(word & ((1 << (padding * 8)) - 1), 0);
        decoded.extend_from_slice(&word.to_be_bytes()[1..4 - padding]);
    }
    decoded
}

pub fn recorded_timestamp(encoded: &str) -> Timestamp {
    assert!(encoded.is_ascii());
    let utc = encoded
        .strip_suffix('Z')
        .unwrap_or_else(|| panic!("non-UTC recording"));
    let (date, time) = utc
        .split_once('T')
        .unwrap_or_else(|| panic!("invalid recorded time"));
    let date: Vec<u32> = date.split('-').map(|part| part.parse().unwrap()).collect();
    assert_eq!(date.len(), 3);
    let (year, month, day) = (date[0], date[1], date[2]);
    assert!((1..=9999).contains(&year));
    assert!((1..=12).contains(&month));
    let leap = year % 4 == 0 && (year % 100 != 0 || year % 400 == 0);
    let months = [
        31,
        if leap { 29 } else { 28 },
        31,
        30,
        31,
        30,
        31,
        31,
        30,
        31,
        30,
        31,
    ];
    assert!((1..=months[month as usize - 1]).contains(&day));
    let (whole, fraction) = time.split_once('.').unwrap_or((time, ""));
    assert!(fraction.len() <= 9 && fraction.bytes().all(|byte| byte.is_ascii_digit()));
    let time: Vec<u32> = whole.split(':').map(|part| part.parse().unwrap()).collect();
    assert_eq!(time.len(), 3);
    assert!(time[0] < 24 && time[1] < 60 && time[2] < 60);
    let days_before_year = |year: u32| {
        let previous = i64::from(year - 1);
        365 * previous + previous / 4 - previous / 100 + previous / 400
    };
    let days = days_before_year(year) - days_before_year(1970)
        + i64::from(months[..month as usize - 1].iter().sum::<u32>())
        + i64::from(day - 1);
    Timestamp {
        seconds: days * 86_400 + i64::from(time[0] * 3600 + time[1] * 60 + time[2]),
        nanos: if fraction.is_empty() {
            0
        } else {
            fraction.parse::<u32>().unwrap() * 10_u32.pow(9 - fraction.len() as u32)
        },
    }
}
