use layerx_human_service::audit::{
    verify_export, verify_export_anchored, AuditChain, AuditError, AuditEvent, AuthMethod,
    ChainHead, Decision, EXPORT_IMPORT_MAX_BYTES, EXPORT_IMPORT_MAX_ENTRIES,
    EXPORT_IMPORT_MAX_EVIDENCE_ROWS,
};
use layerx_human_service::store::{EvidenceRef, PrincipalId, PrincipalStore, Table};
use layerx_human_service::trace::TraceId;
use layerx_human_test_support::{
    directory, install_and_open, principal, retention_uniform, row_key, tenancy,
};

// Committed fixture inputs go through the production store, append, export and
// import codecs. The head is retained directly from the writer, out of band.
struct Fixture {
    principal: PrincipalId,
    empty: Vec<u8>,
    empty_head: ChainHead,
    prefix: Vec<u8>,
    bundle: Vec<u8>,
    head: ChainHead,
}

fn exported_fixture(name: &str, receipt: &[u8], with_evidence: bool) -> Fixture {
    let root = directory("repair-22-33-60");
    let map = tenancy(&[(name, "tenant-a")]);
    let (mut store, digest) = install_and_open(&root, &map, retention_uniform(100));
    let id = principal(name);
    let (empty, empty_head, prefix, bundle, head) = {
        let mut scope = store
            .principal(&id)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        let key = row_key("receipt-1");
        scope
            .put(Table::Journeys, key.clone(), 1, receipt.to_vec())
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        let references = if with_evidence {
            vec![EvidenceRef::new(Table::Journeys, key)]
        } else {
            Vec::new()
        };
        let mut chain =
            AuditChain::open(&scope).unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        let empty = chain
            .export(&scope)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        let empty_head = chain.head();
        chain
            .append(
                &mut scope,
                2,
                &TraceId::mint([1; 16]),
                &AuditEvent::Authentication {
                    method: AuthMethod::Passkey,
                    outcome: Decision::Granted,
                },
                &references,
            )
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        let prefix = chain
            .export(&scope)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        chain
            .append(
                &mut scope,
                3,
                &TraceId::mint([2; 16]),
                &AuditEvent::Authentication {
                    method: AuthMethod::StepUp,
                    outcome: Decision::Refused,
                },
                &references,
            )
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        (
            empty,
            empty_head,
            prefix,
            chain
                .export(&scope)
                .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
            chain.head(),
        )
    };
    drop(store);
    let mut reopened = PrincipalStore::open(&root, retention_uniform(100), digest)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    {
        let scope = reopened
            .principal(&id)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        let chain = AuditChain::open_anchored(&scope, head)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
        assert_eq!(
            chain
                .export(&scope)
                .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
            bundle
        );
    }
    drop(reopened);
    std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    Fixture {
        principal: id,
        empty,
        empty_head,
        prefix,
        bundle,
        head,
    }
}

fn read_count(bytes: &[u8], offset: usize) -> usize {
    u32::from_be_bytes(
        bytes[offset..offset + 4]
            .try_into()
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}")),
    ) as usize
}

fn skip_frame(bytes: &[u8], offset: usize) -> usize {
    offset + 4 + read_count(bytes, offset)
}

fn entry_count_offset(bytes: &[u8]) -> usize {
    // LXAX, version, framed principal, framed head.
    skip_frame(bytes, skip_frame(bytes, 5))
}

fn evidence_count_offset(bytes: &[u8]) -> usize {
    let count_offset = entry_count_offset(bytes);
    let mut offset = count_offset + 4;
    for _ in 0..read_count(bytes, count_offset) {
        offset = skip_frame(bytes, offset);
    }
    offset
}

fn set_count(bytes: &mut [u8], offset: usize, count: usize) {
    bytes[offset..offset + 4].copy_from_slice(
        &u32::try_from(count)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
            .to_be_bytes(),
    );
}

fn assert_refused(fixture: &Fixture, bytes: &[u8], reason: &'static str) {
    for result in [
        verify_export(bytes),
        verify_export_anchored(bytes, &fixture.principal, fixture.head),
    ] {
        assert!(
            matches!(result, Err(AuditError::Corrupt(actual)) if actual == reason),
            "expected {reason}, got {result:?}"
        );
    }
}

#[test]
fn audit_export_authentication_anchor_accepts_retained_commitment_after_reopen() {
    let fixture = exported_fixture("alice", b"committed-receipt", true);
    let report = verify_export_anchored(&fixture.bundle, &fixture.principal, fixture.head)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    assert_eq!(report.principal(), &fixture.principal);
    assert_eq!(report.head(), fixture.head);
    assert_eq!(report.entries(), 2);
    assert_eq!(report.evidence_rows(), 1);
    assert_eq!(
        report,
        verify_export(&fixture.bundle)
            .unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
    );
    let empty = verify_export_anchored(&fixture.empty, &fixture.principal, fixture.empty_head)
        .unwrap_or_else(|error| panic!("unexpected error: {error:?}"));
    assert_eq!(empty.entries(), 0);
    assert_eq!(empty.evidence_rows(), 0);
}

#[test]
fn audit_export_authentication_anchor_refuses_replacement_and_valid_prefix() {
    let fixture = exported_fixture("alice", b"committed-receipt", true);
    let other_principal = exported_fixture("bob", b"committed-receipt", true);
    let replacement = exported_fixture("alice", b"different-receipt", true);
    assert_eq!(replacement.head.length(), fixture.head.length());
    assert_ne!(replacement.head.link(), fixture.head.link());
    // These attacks are internally consistent, so the old API is explicitly
    // only a consistency check. None may satisfy the retained commitment.
    for (bytes, foreign_principal) in [
        (&other_principal.bundle, true),
        (&replacement.bundle, false),
        (&fixture.prefix, false),
        (&fixture.empty, false),
    ] {
        assert!(verify_export(bytes).is_ok());
        let result = verify_export_anchored(bytes, &fixture.principal, fixture.head);
        if foreign_principal {
            assert!(matches!(
                result,
                Err(AuditError::Corrupt(
                    "export principal does not match anchor"
                ))
            ));
        } else {
            assert!(
                matches!(result, Err(AuditError::HeadMismatch { expected, .. }) if expected == fixture.head)
            );
        }
    }
    // Principal and head comparisons are independent: even a supplied head
    // matching the foreign bundle cannot bypass the principal commitment.
    assert!(matches!(
        verify_export_anchored(
            &other_principal.bundle,
            &fixture.principal,
            other_principal.head
        ),
        Err(AuditError::Corrupt(
            "export principal does not match anchor"
        ))
    ));
}

#[test]
fn audit_export_count_allocation_refuses_short_entry_and_evidence_counts() {
    let fixture = exported_fixture("alice", b"committed-receipt", true);
    for (offset, budget, budget_reason, frame_reason) in [
        (
            entry_count_offset(&fixture.bundle),
            EXPORT_IMPORT_MAX_ENTRIES,
            "audit export exceeds import entry budget",
            "export entry count exceeds framed bytes",
        ),
        (
            evidence_count_offset(&fixture.bundle),
            EXPORT_IMPORT_MAX_EVIDENCE_ROWS,
            "audit export exceeds import evidence budget",
            "export evidence count exceeds framed bytes",
        ),
    ] {
        for count in [1, 2, budget, budget + 1, u32::MAX as usize] {
            let mut short = fixture.bundle[..offset + 4].to_vec();
            set_count(&mut short, offset, count);
            if offset == entry_count_offset(&fixture.bundle) {
                // Retain the mandatory evidence-count frame, but no entries.
                short.extend_from_slice(&0_u32.to_be_bytes());
            }
            assert_refused(
                &fixture,
                &short,
                if count > budget {
                    budget_reason
                } else {
                    frame_reason
                },
            );
        }
    }
    let offset = entry_count_offset(&fixture.bundle);
    let missing = &fixture.bundle[..offset + 4];
    assert_refused(&fixture, missing, "missing export evidence count");
}

#[test]
fn audit_export_count_allocation_enforces_budgets_even_with_framed_space() {
    let fixture = exported_fixture("alice", b"committed-receipt", true);
    for (offset, count, frame_size, reason) in [
        (
            entry_count_offset(&fixture.bundle),
            EXPORT_IMPORT_MAX_ENTRIES + 1,
            4,
            "audit export exceeds import entry budget",
        ),
        (
            evidence_count_offset(&fixture.bundle),
            EXPORT_IMPORT_MAX_EVIDENCE_ROWS + 1,
            17,
            "audit export exceeds import evidence budget",
        ),
    ] {
        let mut oversized = fixture.bundle[..offset + 4].to_vec();
        set_count(&mut oversized, offset, count);
        oversized.resize(offset + 4 + count * frame_size + 4, 0);
        assert_refused(&fixture, &oversized, reason);
    }
    let mut oversized = fixture.bundle.clone();
    oversized.resize(EXPORT_IMPORT_MAX_BYTES + 1, 0);
    assert_refused(
        &fixture,
        &oversized,
        "audit export exceeds import byte budget",
    );
}

#[test]
fn audit_export_count_allocation_refuses_nested_binding_counts() {
    let fixture = exported_fixture("alice", b"unused", false);
    let first_entry = entry_count_offset(&fixture.bundle) + 4;
    let nested_count = skip_frame(&fixture.bundle, first_entry) - 4;
    let mut altered = fixture.bundle.clone();
    set_count(&mut altered, nested_count, u32::MAX as usize);
    assert_refused(
        &fixture,
        &altered,
        "evidence binding count exceeds framed bytes",
    );
}

#[test]
fn audit_export_authentication_anchor_keeps_evidence_and_framing_checks() {
    let fixture = exported_fixture("alice", b"committed-receipt", true);
    let mut altered = fixture.bundle.clone();
    *altered
        .last_mut()
        .unwrap_or_else(|| panic!("missing value")) ^= 1;
    assert!(matches!(
        verify_export_anchored(&altered, &fixture.principal, fixture.head),
        Err(AuditError::EvidenceDigestMismatch { sequence: 0 })
    ));
    let mut trailing = fixture.bundle.clone();
    trailing.push(0);
    assert_refused(&fixture, &trailing, "trailing audit export bytes");

    let offset = evidence_count_offset(&fixture.bundle);
    let mut missing = fixture.bundle[..offset + 4].to_vec();
    set_count(&mut missing, offset, 0);
    assert!(matches!(
        verify_export_anchored(&missing, &fixture.principal, fixture.head),
        Err(AuditError::EvidenceMismatch { sequence: 0 })
    ));

    let row = &fixture.bundle[offset + 4..];
    let mut duplicate = fixture.bundle.clone();
    duplicate.extend_from_slice(row);
    set_count(&mut duplicate, offset, 2);
    assert_refused(&fixture, &duplicate, "duplicate export evidence");

    let mut extra_row = row.to_vec();
    let key_end = skip_frame(&extra_row, 1);
    extra_row[key_end - 1] = b'2';
    let mut extraneous = fixture.bundle.clone();
    extraneous.extend_from_slice(&extra_row);
    set_count(&mut extraneous, offset, 2);
    assert_refused(&fixture, &extraneous, "unreferenced export evidence");
}
