#[allow(dead_code)]
mod activity_chain {
    include!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/tests/activity_chain.rs"
    ));
}
#[allow(dead_code)]
mod expiry_chain {
    include!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/tests/expiry_chain.rs"
    ));
}
#[allow(dead_code)]
mod snapshot {
    include!(concat!(env!("CARGO_MANIFEST_DIR"), "/tests/snapshot.rs"));
}
