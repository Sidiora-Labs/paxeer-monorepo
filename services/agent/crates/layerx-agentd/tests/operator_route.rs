mod support;

use std::fs;

use layerx_agentd::admin::{OperatorCommand, ProtectedMutation, Surface};
use layerx_agentd::audit::verify_chain;
use layerx_agentd::human::HumanOperationError;
use layerx_agentd::human_runtime::route_operator_command;
use layerx_agentd::outbox::{Outbox, SubmissionState};
use layerx_agentd::store::Store;

use support::{directory, signed_transaction, tenant};

const OPERATOR: &str = "operator:incident-response";

fn enqueue_unknown(store: &mut Store, outbox: &mut Outbox, id: u8) -> [u8; 32] {
    let hash = outbox
        .enqueue(store, tenant(), signed_transaction(id))
        .unwrap_or_else(|error| panic!("enqueue: {error:?}"));
    outbox
        .transition(
            store,
            hash,
            SubmissionState::Submitted,
            "real boundary accepted bytes",
            None,
        )
        .unwrap_or_else(|error| panic!("submitted: {error:?}"));
    outbox
        .transition(
            store,
            hash,
            SubmissionState::Unknown,
            "acknowledgement was not observed",
            None,
        )
        .unwrap_or_else(|error| panic!("unknown: {error:?}"));
    hash
}

#[test]
fn human_socket_operator_commands_reach_the_audited_admin_dispatch() {
    let root = directory("operator-route");
    let mut store = Store::open(&root).unwrap_or_else(|error| panic!("store: {error}"));
    let mut outbox = Outbox::default();
    let hash = enqueue_unknown(&mut store, &mut outbox, 1);
    let activity_id = outbox
        .status(hash)
        .unwrap_or_else(|| panic!("unknown missing"))
        .activity_id;

    let inspected = route_operator_command(
        &root,
        &tenant(),
        &outbox,
        OPERATOR,
        [7; 32],
        OperatorCommand::InspectUnknown(hash),
    )
    .unwrap_or_else(|error| panic!("inspect unknown: {error:?}"));
    let bytes = inspected.bytes();
    assert_eq!(bytes.len(), 74);
    assert_eq!(bytes[0], 1);
    assert_eq!(&bytes[1..33], &activity_id);
    assert_eq!(&bytes[33..65], &hash);
    assert_eq!(bytes[65], 5);
    assert_eq!(&bytes[66..74], &1_u64.to_be_bytes());

    assert_eq!(
        route_operator_command(
            &root,
            &tenant(),
            &outbox,
            OPERATOR,
            [8; 32],
            OperatorCommand::InspectUnknown([2; 32]),
        )
        .err(),
        Some(HumanOperationError::Refused)
    );
    assert_eq!(
        route_operator_command(
            &root,
            &tenant(),
            &outbox,
            OPERATOR,
            [9; 32],
            OperatorCommand::AttemptProtectedMutation {
                target: hash,
                mutation: ProtectedMutation::MarkUnknownExecuted,
            },
        )
        .err(),
        Some(HumanOperationError::Refused)
    );
    assert_eq!(
        outbox
            .status(hash)
            .unwrap_or_else(|| panic!("unknown missing"))
            .state,
        SubmissionState::Unknown
    );

    let planned = route_operator_command(
        &root,
        &tenant(),
        &outbox,
        OPERATOR,
        [10; 32],
        OperatorCommand::ResolveUnknown(hash),
    )
    .unwrap_or_else(|error| panic!("resolve plan: {error:?}"));
    assert_eq!(
        planned.bytes(),
        [&[2_u8][..], &4_u64.to_be_bytes()].concat()
    );

    let surface =
        Surface::open(&root, &tenant()).unwrap_or_else(|error| panic!("surface: {error:?}"));
    assert_eq!(
        verify_chain(surface.audit_path())
            .unwrap_or_else(|error| panic!("verify audit: {error}"))
            .entries,
        4
    );
    let _ = fs::remove_dir_all(root);
}
