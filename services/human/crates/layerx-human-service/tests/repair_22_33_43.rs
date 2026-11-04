use std::io::{Cursor, Read as _};
use std::os::unix::net::UnixListener;
use std::path::PathBuf;
use std::time::Duration;

use layerx_human_service::frame::{
    read_frame, write_frame, ConnectionGate, FrameTransport, FrameViolation, Limits,
    TransportError, Uds,
};
use layerx_human_service::store::{
    AuditDisposition, EvidenceRef, PrincipalStore, RowKey, Table, TenancyDigest,
};
use layerx_human_service::support::{
    Author, ConversationState, CreateConversation, Shell, SupportError, SupportService,
    MAX_ADMITTED_CONVERSATIONS,
};
use layerx_human_test_support::{
    directory, install_and_open, principal, retention_uniform, tenancy,
};

struct Fixture {
    root: PathBuf,
    store: Option<PrincipalStore>,
    digest: TenancyDigest,
    retention: u64,
}

impl Fixture {
    fn new(label: &str, retention: u64) -> Self {
        let root = directory(label);
        std::fs::create_dir_all(&root).unwrap();
        let (store, digest) = install_and_open(
            &root,
            &tenancy(&[("alice", "tenant-alice"), ("bob", "tenant-bob")]),
            retention_uniform(retention),
        );
        Self {
            root,
            store: Some(store),
            digest,
            retention,
        }
    }

    fn store(&mut self) -> &mut PrincipalStore {
        self.store.as_mut().unwrap()
    }

    fn restart(&mut self) {
        drop(self.store.take().unwrap());
        self.store = Some(
            PrincipalStore::open(&self.root, retention_uniform(self.retention), self.digest)
                .unwrap(),
        );
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        drop(self.store.take());
        let _ = std::fs::remove_dir_all(&self.root);
    }
}

fn conversation_key(id: &str) -> RowKey {
    RowKey::new(format!(
        "support_conversation_{}",
        id.strip_prefix("sup_").unwrap()
    ))
    .unwrap()
}

#[test]
fn frame_registry_real_trait_and_byte_maximum_round_trip() {
    let fixture = Fixture::new("repair-22-33-43-frame", u64::MAX);
    let socket = fixture.root.join("frame.sock");
    let listener = UnixListener::bind(&socket).unwrap();
    let limits = Limits {
        maximum_frame_bytes: 256,
        maximum_connections: 1,
        maximum_streams: 1,
        maximum_queued_bytes: 256,
        deadline: Duration::from_secs(1),
    };
    let gate = ConnectionGate::new(1);
    let mut client = Uds::connect(&socket, &gate, limits).unwrap();
    let (mut peer, _) = listener.accept().unwrap();
    peer.set_read_timeout(Some(limits.deadline)).unwrap();
    peer.set_write_timeout(Some(limits.deadline)).unwrap();
    let transport: &mut dyn FrameTransport = &mut client;
    let payload: Vec<_> = (0..=u8::MAX).collect();
    transport.send(&payload).unwrap();
    let received = read_frame(&mut peer, limits.maximum_frame_bytes).unwrap();
    assert_eq!(received, payload);
    write_frame(&mut peer, &received, limits.maximum_frame_bytes).unwrap();
    assert_eq!(transport.receive().unwrap(), payload);
    transport.send(&[]).unwrap();
    assert_eq!(
        read_frame(&mut peer, limits.maximum_frame_bytes).unwrap(),
        Vec::<u8>::new()
    );
    assert!(matches!(
        Uds::connect(&socket, &gate, limits),
        Err(TransportError::ConnectionLimit)
    ));
    assert_eq!(gate.active(), 1);
    assert_eq!(client.receive(), Err(TransportError::Deadline));
    drop(peer);
    assert_eq!(client.receive(), Err(TransportError::PeerShutdown));
    drop(client);
    assert_eq!(gate.active(), 0);
}

#[test]
fn frame_registry_preserves_bounds_and_typed_errors() {
    let mut output = Vec::new();
    assert_eq!(
        write_frame(&mut output, &[1; 5], 4),
        Err(TransportError::Frame(FrameViolation::Oversized {
            declared: 5,
            maximum: 4
        }))
    );
    assert!(output.is_empty());
    write_frame(&mut output, &[1; 5], 5).unwrap();
    let mut input = Cursor::new(output);
    assert_eq!(
        read_frame(&mut input, 4),
        Err(TransportError::Frame(FrameViolation::Oversized {
            declared: 5,
            maximum: 4
        }))
    );
    assert_eq!(input.position(), 4);
    let mut body = Vec::new();
    input.read_to_end(&mut body).unwrap();
    assert_eq!(body, vec![1; 5]);
    assert_eq!(
        read_frame(&mut Cursor::new([0, 0]), 10),
        Err(TransportError::Frame(FrameViolation::TruncatedPrefix))
    );
    let mut truncated = Vec::new();
    write_frame(&mut truncated, b"body", 4).unwrap();
    truncated.pop();
    assert_eq!(
        read_frame(&mut Cursor::new(truncated), 4),
        Err(TransportError::Frame(FrameViolation::TruncatedBody))
    );
    assert_eq!(
        read_frame(&mut Cursor::new([]), 4),
        Err(TransportError::PeerShutdown)
    );
}

#[test]
fn support_unbounded_list_uses_recent_index_after_reopen() {
    let mut fixture = Fixture::new("repair-22-33-43-list", u64::MAX);
    let mut ids = Vec::new();
    {
        let mut scope = fixture.store().principal(&principal("alice")).unwrap();
        for i in 0..140_u64 {
            let request =
                CreateConversation::new(format!("{i}:{}", "x".repeat(1_900)), Shell::Desktop)
                    .unwrap();
            let conversation =
                SupportService::create(&mut scope, i + 1, &format!("create-{i}"), &request)
                    .unwrap();
            ids.push(conversation.conversation_id().to_owned());
        }
        // Off-page corruption would fail any implementation that decoded all
        // bodies to sort them, including one that did so while reopening.
        for id in &ids[..40] {
            scope
                .put(
                    Table::Support,
                    conversation_key(id),
                    1,
                    b"invalid old body".to_vec(),
                )
                .unwrap();
        }
        let listed = SupportService::list(&scope).unwrap();
        assert_eq!(listed.len(), 100);
        assert_eq!(
            listed
                .iter()
                .map(|c| c.conversation_id())
                .collect::<Vec<_>>(),
            ids[40..]
                .iter()
                .rev()
                .map(String::as_str)
                .collect::<Vec<_>>()
        );
    }
    fixture.restart();
    {
        let mut scope = fixture.store().principal(&principal("alice")).unwrap();
        let listed = SupportService::list(&scope).unwrap();
        assert_eq!(listed.len(), 100);
        assert_eq!(listed.first().unwrap().conversation_id(), ids[139]);
        assert_eq!(listed.last().unwrap().conversation_id(), ids[40]);
        let updated =
            SupportService::reply(&mut scope, 1_000, &ids[40], "reply", "Please check again")
                .unwrap();
        assert_eq!(updated.updated_at(), 1_000);
    }
    fixture.restart();
    let mut scope = fixture.store().principal(&principal("alice")).unwrap();
    let listed = SupportService::list(&scope).unwrap();
    assert_eq!(listed.len(), 100);
    assert_eq!(listed[0].conversation_id(), ids[40]);
    assert_eq!(
        listed[0].messages().last().unwrap().body(),
        "Please check again"
    );
    assert_eq!(listed[99].conversation_id(), ids[41]);
    scope
        .put(
            Table::Support,
            conversation_key(&ids[40]),
            1_000,
            b"invalid selected body".to_vec(),
        )
        .unwrap();
    assert!(matches!(
        SupportService::list(&scope),
        Err(SupportError::Corrupt)
    ));
}

#[test]
fn support_admission_and_durable_retry_tombstones_survive_expiry_and_reopen() {
    let mut fixture = Fixture::new("repair-22-33-43-tombstones", 10);
    let request = CreateConversation::new("Please help", Shell::Mobile).unwrap();
    let first;
    {
        let mut scope = fixture.store().principal(&principal("alice")).unwrap();
        first = SupportService::create(&mut scope, 1, "identity-0", &request).unwrap();
        for i in 1..MAX_ADMITTED_CONVERSATIONS {
            SupportService::create(&mut scope, 1, &format!("identity-{i}"), &request).unwrap();
        }
        assert!(MAX_ADMITTED_CONVERSATIONS > 100);
        assert_eq!(SupportService::list(&scope).unwrap().len(), 100);
        assert!(matches!(
            SupportService::create(&mut scope, 2, "overflow", &request),
            Err(SupportError::ConversationLimit)
        ));
        assert_eq!(
            SupportService::create(&mut scope, 2, "identity-0", &request).unwrap(),
            first
        );
        let changed = CreateConversation::new("Changed help", Shell::Mobile).unwrap();
        assert!(matches!(
            SupportService::create(&mut scope, 2, "identity-0", &changed),
            Err(SupportError::Conflict)
        ));
        assert_eq!(
            scope.expire(20).unwrap().expired_rows,
            MAX_ADMITTED_CONVERSATIONS
        );
        assert!(SupportService::list(&scope).unwrap().is_empty());
        assert_eq!(scope.keys(Table::Support).len(), 1);
        let index = scope
            .get(
                Table::Support,
                &RowKey::new("support_recent_index_v1").unwrap(),
            )
            .unwrap();
        assert!(!std::str::from_utf8(index.bytes())
            .unwrap()
            .contains("Please help"));
    }
    fixture.restart();
    {
        let mut scope = fixture.store().principal(&principal("alice")).unwrap();
        assert!(SupportService::list(&scope).unwrap().is_empty());
        assert!(matches!(
            SupportService::create(&mut scope, 21, "identity-0", &request),
            Err(SupportError::ConversationExpired)
        ));
        let changed = CreateConversation::new("Please help", Shell::Desktop).unwrap();
        assert!(matches!(
            SupportService::create(&mut scope, 21, "identity-0", &changed),
            Err(SupportError::Conflict)
        ));
        assert!(matches!(
            SupportService::create(&mut scope, 21, "overflow", &request),
            Err(SupportError::ConversationLimit)
        ));
        assert!(matches!(
            SupportService::status(&scope, first.conversation_id()),
            Err(SupportError::ConversationExpired)
        ));
        assert_eq!(scope.expire(u64::MAX).unwrap().expired_rows, 0);
    }
    let mut other = fixture.store().principal(&principal("bob")).unwrap();
    let theirs = SupportService::create(&mut other, 21, "identity-0", &request).unwrap();
    assert_ne!(theirs.conversation_id(), first.conversation_id());
    assert_eq!(SupportService::list(&other).unwrap(), vec![theirs]);
}

#[test]
fn support_reply_replay_converges_after_resolve_and_reopen() {
    let mut fixture = Fixture::new("repair-22-33-43-reply", u64::MAX);
    let resolved;
    {
        let mut scope = fixture.store().principal(&principal("alice")).unwrap();
        let conversation = SupportService::create(
            &mut scope,
            1,
            "create",
            &CreateConversation::new("Help", Shell::Mobile).unwrap(),
        )
        .unwrap();
        let accepted = SupportService::reply(
            &mut scope,
            2,
            conversation.conversation_id(),
            "reply",
            " My reply \n",
        )
        .unwrap();
        assert_eq!(accepted.messages().len(), 2);
        assert_eq!(accepted.messages()[1].author(), Author::You);
        assert_eq!(accepted.messages()[1].body(), "My reply");
        let support = SupportService::record_support_reply(
            &mut scope,
            3,
            conversation.conversation_id(),
            "operator",
            "All fixed",
        )
        .unwrap();
        assert_eq!(support.state(), ConversationState::WaitingForYou);
        resolved = SupportService::resolve(&mut scope, 4, conversation.conversation_id()).unwrap();
        assert_eq!(resolved.state(), ConversationState::Resolved);
    }
    fixture.restart();
    {
        let mut scope = fixture.store().principal(&principal("alice")).unwrap();
        let before = scope
            .get(
                Table::Support,
                &conversation_key(resolved.conversation_id()),
            )
            .unwrap()
            .bytes()
            .to_vec();
        let replay = SupportService::reply(
            &mut scope,
            500,
            resolved.conversation_id(),
            "reply",
            "My reply",
        )
        .unwrap();
        assert_eq!(replay, resolved);
        assert_eq!(
            scope
                .get(
                    Table::Support,
                    &conversation_key(resolved.conversation_id())
                )
                .unwrap()
                .bytes(),
            before
        );
        assert!(matches!(
            SupportService::reply(
                &mut scope,
                501,
                resolved.conversation_id(),
                "reply",
                "Changed reply"
            ),
            Err(SupportError::Conflict)
        ));
        assert!(matches!(
            SupportService::record_support_reply(
                &mut scope,
                501,
                resolved.conversation_id(),
                "reply",
                "My reply"
            ),
            Err(SupportError::Conflict)
        ));
        assert!(matches!(
            SupportService::reply(
                &mut scope,
                501,
                resolved.conversation_id(),
                "new-reply",
                "New reply"
            ),
            Err(SupportError::ConversationResolved)
        ));
        assert_eq!(
            SupportService::record_support_reply(
                &mut scope,
                501,
                resolved.conversation_id(),
                "operator",
                "All fixed"
            )
            .unwrap(),
            resolved
        );
        assert_eq!(
            SupportService::list(&scope).unwrap(),
            vec![resolved.clone()]
        );
    }
    fixture.restart();
    let scope = fixture.store().principal(&principal("alice")).unwrap();
    assert_eq!(SupportService::list(&scope).unwrap(), vec![resolved]);
}

#[test]
fn support_index_and_body_commit_atomically() {
    let mut fixture = Fixture::new("repair-22-33-43-atomic", u64::MAX);
    let original;
    {
        let mut scope = fixture.store().principal(&principal("alice")).unwrap();
        original = SupportService::create(
            &mut scope,
            1,
            "create",
            &CreateConversation::new("Help", Shell::Mobile).unwrap(),
        )
        .unwrap();
        scope
            .append_audit(
                RowKey::new("pin-index").unwrap(),
                2,
                b"index evidence".to_vec(),
                AuditDisposition::Exportable {
                    evidence: vec![EvidenceRef::new(
                        Table::Support,
                        RowKey::new("support_recent_index_v1").unwrap(),
                    )],
                },
            )
            .unwrap();
        assert!(matches!(
            SupportService::reply(
                &mut scope,
                3,
                original.conversation_id(),
                "reply",
                "New reply"
            ),
            Err(SupportError::Store(_))
        ));
        assert!(matches!(
            SupportService::create(
                &mut scope,
                3,
                "new",
                &CreateConversation::new("New conversation", Shell::Mobile).unwrap()
            ),
            Err(SupportError::Store(_))
        ));
        assert_eq!(
            SupportService::list(&scope).unwrap(),
            vec![original.clone()]
        );
    }
    fixture.restart();
    let scope = fixture.store().principal(&principal("alice")).unwrap();
    assert_eq!(SupportService::list(&scope).unwrap(), vec![original]);
    assert_eq!(scope.keys(Table::Support).len(), 2);
}

#[test]
fn support_legacy_index_is_persisted_once_and_ties_are_stable() {
    let mut fixture = Fixture::new("repair-22-33-43-legacy", u64::MAX);
    let request = CreateConversation::new("Legacy", Shell::Desktop).unwrap();
    let mut conversations = Vec::new();
    {
        let mut scope = fixture.store().principal(&principal("alice")).unwrap();
        for key in ["one", "two", "three"] {
            conversations.push(SupportService::create(&mut scope, 1, key, &request).unwrap());
        }
        scope
            .remove(
                Table::Support,
                &RowKey::new("support_recent_index_v1").unwrap(),
            )
            .unwrap();
    }
    fixture.restart();
    {
        let mut scope = fixture.store().principal(&principal("alice")).unwrap();
        conversations.sort_by(|a, b| b.conversation_id().cmp(a.conversation_id()));
        assert_eq!(SupportService::list(&scope).unwrap(), conversations);
        assert_eq!(scope.keys(Table::Support).len(), 4);
        assert_eq!(
            SupportService::create(&mut scope, 9, "one", &request)
                .unwrap()
                .created_at(),
            1
        );
        let index_key = RowKey::new("support_recent_index_v1").unwrap();
        let mut index: serde_json::Value =
            serde_json::from_slice(scope.get(Table::Support, &index_key).unwrap().bytes()).unwrap();
        index["entries"].as_array_mut().unwrap().swap(0, 1);
        scope
            .put(
                Table::Support,
                index_key,
                9,
                serde_json::to_vec(&index).unwrap(),
            )
            .unwrap();
        assert!(matches!(
            SupportService::list(&scope),
            Err(SupportError::Corrupt)
        ));
    }
}

#[test]
fn support_full_conversation_preserves_retry_and_message_bounds() {
    let mut fixture = Fixture::new("repair-22-33-43-full", u64::MAX);
    let id;
    {
        let mut scope = fixture.store().principal(&principal("alice")).unwrap();
        let original = SupportService::create(
            &mut scope,
            1,
            "create",
            &CreateConversation::new("First", Shell::Mobile).unwrap(),
        )
        .unwrap();
        id = original.conversation_id().to_owned();
        for i in 1..200_u64 {
            let conversation = SupportService::reply(
                &mut scope,
                i + 1,
                &id,
                &format!("reply-{i}"),
                format!("Reply {i}"),
            )
            .unwrap();
            assert_eq!(
                conversation.messages().len(),
                usize::try_from(i + 1).unwrap()
            );
        }
        assert!(matches!(
            SupportService::reply(&mut scope, 201, &id, "overflow", "No room"),
            Err(SupportError::ConversationFull)
        ));
        let replay = SupportService::reply(&mut scope, 201, &id, "reply-199", "Reply 199").unwrap();
        assert_eq!(replay.messages().len(), 200);
        assert_eq!(replay.updated_at(), 200);
        SupportService::resolve(&mut scope, 202, &id).unwrap();
    }
    fixture.restart();
    let mut scope = fixture.store().principal(&principal("alice")).unwrap();
    let replay = SupportService::reply(&mut scope, 203, &id, "reply-199", "Reply 199").unwrap();
    assert_eq!(replay.messages().len(), 200);
    assert_eq!(replay.updated_at(), 202);
    assert_eq!(replay.state(), ConversationState::Resolved);
    assert!(matches!(
        SupportService::reply(&mut scope, 203, &id, "reply-199", "Changed"),
        Err(SupportError::Conflict)
    ));
    assert!(matches!(
        CreateConversation::new("x".repeat(2_001), Shell::Mobile),
        Err(SupportError::InvalidBody)
    ));
}
