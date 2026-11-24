#[allow(unused_imports)]
pub(crate) use approvals::owner::hex;

#[path = "approvals.rs"]
mod approvals;

#[path = "decide.rs"]
mod decide;

#[test]
fn daemon_budget_snapshot_survives_reopen_through_human_decoder() {
    let mut fixture = approvals::owner::Fixture::new(&[1, 2]);
    fixture.assert_reservations(2, 51, 0);
    fixture.assert_snapshot_recovery();
    fixture.assert_reservations(2, 51, 0);
}
