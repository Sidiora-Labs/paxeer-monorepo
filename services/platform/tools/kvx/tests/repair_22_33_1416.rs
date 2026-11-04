//! Task 22.33.1416 closes audit 22.30.79 finding 034 with the tests below.
//! Baseline at e3d7e5828b39e4182e9559fb6bc28d74c2f37e0f was not run: the
//! task's gate order requires implementation before its sole verification run.
//! No out-of-path wiring is required; qualification records the verified revision.

use layerx_platform_kvx::{parse, quote, string_list, unquote};

#[test]
fn finding_034_quote_unquote_round_trip() {
    for (plain, encoded) in [
        ("", "\"\""),
        ("a,b", r#""a,b""#),
        (r#"say "hello, world""#, r#""say \"hello, world\"""#),
        (r"C:\data\", r#""C:\\data\\""#),
        (r#"\",\"#, r#""\\\",\\""#),
        ("雪, café", "\"雪, café\""),
    ] {
        assert_eq!(quote(plain), encoded, "encoding {plain:?}");
        assert_eq!(unquote(encoded), Ok(plain.to_owned()), "decoding {encoded}");
        let source = format!("[record]\nvalue = {}\n", quote(plain));
        let document = parse(&source).unwrap_or_else(|error| panic!("parse: {error}"));
        let recovered = unquote(
            document
                .required("record", "value")
                .unwrap_or_else(|error| panic!("required: {error}")),
        );
        assert_eq!(recovered, Ok(plain.to_owned()));
    }
}

#[test]
fn finding_034_list_round_trip_through_document() {
    let values = [
        "a,b",
        r#"say "yes, please""#,
        r#"\",still one item"#,
        r"ends in a slash\",
        r"ends in two slashes\\",
        r#""quoted at both ends""#,
        "",
        "  surrounding whitespace  ",
        "雪, café [brackets] # literal",
    ];
    let encoded = format!("[ {} ]", values.map(quote).join(", \t"));
    let source = format!("[record]\nitems = {encoded}\nnext = \"intact\"\n");
    let document = parse(&source).unwrap_or_else(|error| panic!("parse: {error}"));
    assert_eq!(
        string_list(
            document
                .required("record", "items")
                .unwrap_or_else(|error| panic!("required: {error}")),
        ),
        Ok(values.map(str::to_owned).to_vec())
    );
    assert_eq!(document.get("record", "next"), Some("\"intact\""));
}

#[test]
fn finding_034_escape_and_delimiter_combinations_round_trip() {
    let parts = ["", "a", ",", "\"", "\\", " ", "[", "]", "é"];
    let mut values = Vec::new();
    for first in parts {
        for second in parts {
            for third in parts {
                let plain = format!("{first}{second}{third}");
                assert_eq!(unquote(&quote(&plain)), Ok(plain.clone()));
                values.push(plain);
            }
        }
    }
    let encoded = values
        .iter()
        .map(|value| quote(value))
        .collect::<Vec<_>>()
        .join(",");
    let document = parse(&format!("[record]\nitems = [{encoded}]\n"))
        .unwrap_or_else(|error| panic!("parse: {error}"));
    assert_eq!(
        string_list(
            document
                .required("record", "items")
                .unwrap_or_else(|error| panic!("required: {error}")),
        ),
        Ok(values)
    );
}

#[test]
fn finding_034_escaped_quoted_key_round_trip() {
    let key = r#"say "hello, world"\path"#;
    let source = format!("[record]\n{} = {}\n", quote(key), quote("value, intact"));
    let document = parse(&source).unwrap_or_else(|error| panic!("parse: {error}"));
    assert_eq!(document.section_entries("record").len(), 1);
    assert_eq!(
        document.get("record", key).map(unquote),
        Some(Ok("value, intact".to_owned()))
    );
}

#[test]
fn finding_034_rejects_malformed_strings_and_lists() {
    for invalid in [
        "",
        "bare",
        "\"",
        r#""inner"quote""#,
        r#""trailing\""#,
        r#""unknown\q""#,
        r#""valid" trailing"#,
    ] {
        assert!(unquote(invalid).is_err(), "accepted string {invalid:?}");
    }
    for invalid in [
        "",
        r#""a,b""#,
        "[bare]",
        r#"["a",]"#,
        r#"[,"a"]"#,
        r#"["a",,"b"]"#,
        r#"["a" "b"]"#,
        r#"["unterminated]"#,
        r#"["trailing\"]"#,
        r#"["unknown\q"]"#,
        r#"["a"bare]"#,
    ] {
        assert!(string_list(invalid).is_err(), "accepted list {invalid:?}");
    }
    for empty in ["[]", "[ \t ]"] {
        assert_eq!(string_list(empty), Ok(Vec::new()));
    }
}
