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
    Author, Conversation, ConversationState, CreateConversation, Shell, SupportError,
    SupportService, MAX_ADMITTED_CONVERSATIONS,
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
        std::fs::create_dir_all(&root).req("unexpected error");
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
        self.store
            .as_mut()
            .unwrap_or_else(|| panic!("missing value"))
    }

    fn restart(&mut self) {
        drop(self.store.take().unwrap_or_else(|| panic!("missing value")));
        self.store = Some(
            PrincipalStore::open(&self.root, retention_uniform(self.retention), self.digest)
                .req("unexpected error"),
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
        id.strip_prefix("sup_")
            .unwrap_or_else(|| panic!("missing value"))
    ))
    .req("unexpected error")
}

#[test]
fn frame_registry_real_trait_and_byte_maximum_round_trip() {
    let fixture = Fixture::new("repair-22-33-43-frame", u64::MAX);
    let socket = fixture.root.join("frame.sock");
    let listener = UnixListener::bind(&socket).req("unexpected error");
    let limits = Limits {
        maximum_frame_bytes: 256,
        maximum_connections: 1,
        maximum_streams: 1,
        maximum_queued_bytes: 256,
        deadline: Duration::from_secs(1),
    };
    let gate = ConnectionGate::new(1);
    let mut client = Uds::connect(&socket, &gate, limits).req("unexpected error");
    let (mut peer, _) = listener.accept().req("unexpected error");
    peer.set_read_timeout(Some(limits.deadline))
        .req("unexpected error");
    peer.set_write_timeout(Some(limits.deadline))
        .req("unexpected error");
    let transport: &mut dyn FrameTransport = &mut client;
    let payload: Vec<_> = (0..=u8::MAX).collect();
    transport.send(&payload).req("unexpected error");
    let received = read_frame(&mut peer, limits.maximum_frame_bytes).req("unexpected error");
    assert_eq!(received, payload);
    write_frame(&mut peer, &received, limits.maximum_frame_bytes).req("unexpected error");
    assert_eq!(transport.receive().req("unexpected error"), payload);
    transport.send(&[]).req("unexpected error");
    assert_eq!(
        read_frame(&mut peer, limits.maximum_frame_bytes).req("unexpected error"),
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
    write_frame(&mut output, &[1; 5], 5).req("unexpected error");
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
    input.read_to_end(&mut body).req("unexpected error");
    assert_eq!(body, vec![1; 5]);
    assert_eq!(
        read_frame(&mut Cursor::new([0, 0]), 10),
        Err(TransportError::Frame(FrameViolation::TruncatedPrefix))
    );
    let mut truncated = Vec::new();
    write_frame(&mut truncated, b"body", 4).req("unexpected error");
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
        let mut scope = fixture
            .store()
            .principal(&principal("alice"))
            .req("unexpected error");
        for i in 0..140_u64 {
            let request =
                CreateConversation::new(format!("{i}:{}", "x".repeat(1_900)), Shell::Desktop)
                    .req("unexpected error");
            let conversation =
                SupportService::create(&mut scope, i + 1, &format!("create-{i}"), &request)
                    .req("unexpected error");
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
                .req("unexpected error");
        }
        let listed = SupportService::list(&scope).req("unexpected error");
        assert_eq!(listed.len(), 100);
        assert_eq!(
            listed
                .iter()
                .map(layerx_human_service::support::Conversation::conversation_id)
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
        let mut scope = fixture
            .store()
            .principal(&principal("alice"))
            .req("unexpected error");
        let listed = SupportService::list(&scope).req("unexpected error");
        assert_eq!(listed.len(), 100);
        assert_eq!(
            listed
                .first()
                .unwrap_or_else(|| panic!("missing value"))
                .conversation_id(),
            ids[139]
        );
        assert_eq!(
            listed
                .last()
                .unwrap_or_else(|| panic!("missing value"))
                .conversation_id(),
            ids[40]
        );
        let updated =
            SupportService::reply(&mut scope, 1_000, &ids[40], "reply", "Please check again")
                .req("unexpected error");
        assert_eq!(updated.updated_at(), 1_000);
    }
    fixture.restart();
    let mut scope = fixture
        .store()
        .principal(&principal("alice"))
        .req("unexpected error");
    let listed = SupportService::list(&scope).req("unexpected error");
    assert_eq!(listed.len(), 100);
    assert_eq!(listed[0].conversation_id(), ids[40]);
    assert_eq!(
        listed[0]
            .messages()
            .last()
            .unwrap_or_else(|| panic!("missing value"))
            .body(),
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
        .req("unexpected error");
    assert!(matches!(
        SupportService::list(&scope),
        Err(SupportError::Corrupt)
    ));
}

fn assert_expired_conversation(
    fixture: &mut Fixture,
    request: &CreateConversation,
    first: &Conversation,
) {
    fixture.restart();
    {
        let mut scope = fixture
            .store()
            .principal(&principal("alice"))
            .req("unexpected error");
        assert!(SupportService::list(&scope)
            .req("unexpected error")
            .is_empty());
        assert!(matches!(
            SupportService::create(&mut scope, 21, "identity-0", request),
            Err(SupportError::ConversationExpired)
        ));
        let changed =
            CreateConversation::new("Please help", Shell::Desktop).req("unexpected error");
        assert!(matches!(
            SupportService::create(&mut scope, 21, "identity-0", &changed),
            Err(SupportError::Conflict)
        ));
        assert!(matches!(
            SupportService::create(&mut scope, 21, "overflow", request),
            Err(SupportError::ConversationLimit)
        ));
        assert!(matches!(
            SupportService::status(&scope, first.conversation_id()),
            Err(SupportError::ConversationExpired)
        ));
        assert_eq!(
            scope.expire(u64::MAX).req("unexpected error").expired_rows,
            0
        );
    }
}

#[test]
fn support_admission_and_durable_retry_tombstones_survive_expiry_and_reopen() {
    let mut fixture = Fixture::new("repair-22-33-43-tombstones", 10);
    let request = CreateConversation::new("Please help", Shell::Mobile).req("unexpected error");
    let first;
    {
        let mut scope = fixture
            .store()
            .principal(&principal("alice"))
            .req("unexpected error");
        first =
            SupportService::create(&mut scope, 1, "identity-0", &request).req("unexpected error");
        let mut admitted = 1;
        for i in 1..MAX_ADMITTED_CONVERSATIONS {
            SupportService::create(&mut scope, 1, &format!("identity-{i}"), &request)
                .req("unexpected error");
            admitted += 1;
        }
        assert!(admitted > 100);
        assert_eq!(
            SupportService::list(&scope).req("unexpected error").len(),
            100
        );
        assert!(matches!(
            SupportService::create(&mut scope, 2, "overflow", &request),
            Err(SupportError::ConversationLimit)
        ));
        assert_eq!(
            SupportService::create(&mut scope, 2, "identity-0", &request).req("unexpected error"),
            first
        );
        let changed =
            CreateConversation::new("Changed help", Shell::Mobile).req("unexpected error");
        assert!(matches!(
            SupportService::create(&mut scope, 2, "identity-0", &changed),
            Err(SupportError::Conflict)
        ));
        assert_eq!(
            scope.expire(20).req("unexpected error").expired_rows,
            MAX_ADMITTED_CONVERSATIONS
        );
        assert!(SupportService::list(&scope)
            .req("unexpected error")
            .is_empty());
        assert_eq!(scope.keys(Table::Support).len(), 1);
        let index = scope
            .get(
                Table::Support,
                &RowKey::new("support_recent_index_v1").req("unexpected error"),
            )
            .unwrap_or_else(|| panic!("missing value"));
        assert!(!std::str::from_utf8(index.bytes())
            .req("unexpected error")
            .contains("Please help"));
    }
    assert_expired_conversation(&mut fixture, &request, &first);
    let mut other = fixture
        .store()
        .principal(&principal("bob"))
        .req("unexpected error");
    let theirs =
        SupportService::create(&mut other, 21, "identity-0", &request).req("unexpected error");
    assert_ne!(theirs.conversation_id(), first.conversation_id());
    assert_eq!(
        SupportService::list(&other).req("unexpected error"),
        vec![theirs]
    );
}

fn resolved_conversation(fixture: &mut Fixture) -> Conversation {
    let mut scope = fixture
        .store()
        .principal(&principal("alice"))
        .req("unexpected error");
    let conversation = SupportService::create(
        &mut scope,
        1,
        "create",
        &CreateConversation::new("Help", Shell::Mobile).req("unexpected error"),
    )
    .req("unexpected error");
    let accepted = SupportService::reply(
        &mut scope,
        2,
        conversation.conversation_id(),
        "reply",
        " My reply \n",
    )
    .req("unexpected error");
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
    .req("unexpected error");
    assert_eq!(support.state(), ConversationState::WaitingForYou);
    let resolved = SupportService::resolve(&mut scope, 4, conversation.conversation_id())
        .req("unexpected error");
    assert_eq!(resolved.state(), ConversationState::Resolved);
    resolved
}

#[test]
fn support_reply_replay_converges_after_resolve_and_reopen() {
    let mut fixture = Fixture::new("repair-22-33-43-reply", u64::MAX);
    let resolved = resolved_conversation(&mut fixture);
    fixture.restart();
    {
        let mut scope = fixture
            .store()
            .principal(&principal("alice"))
            .req("unexpected error");
        let before = scope
            .get(
                Table::Support,
                &conversation_key(resolved.conversation_id()),
            )
            .unwrap_or_else(|| panic!("missing value"))
            .bytes()
            .to_vec();
        let replay = SupportService::reply(
            &mut scope,
            500,
            resolved.conversation_id(),
            "reply",
            "My reply",
        )
        .req("unexpected error");
        assert_eq!(replay, resolved);
        assert_eq!(
            scope
                .get(
                    Table::Support,
                    &conversation_key(resolved.conversation_id())
                )
                .unwrap_or_else(|| panic!("missing value"))
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
            .req("unexpected error"),
            resolved
        );
        assert_eq!(
            SupportService::list(&scope).req("unexpected error"),
            vec![resolved.clone()]
        );
    }
    fixture.restart();
    let scope = fixture
        .store()
        .principal(&principal("alice"))
        .req("unexpected error");
    assert_eq!(
        SupportService::list(&scope).req("unexpected error"),
        vec![resolved]
    );
}

#[test]
fn support_index_and_body_commit_atomically() {
    let mut fixture = Fixture::new("repair-22-33-43-atomic", u64::MAX);
    let original;
    {
        let mut scope = fixture
            .store()
            .principal(&principal("alice"))
            .req("unexpected error");
        original = SupportService::create(
            &mut scope,
            1,
            "create",
            &CreateConversation::new("Help", Shell::Mobile).req("unexpected error"),
        )
        .req("unexpected error");
        scope
            .append_audit(
                RowKey::new("pin-index").req("unexpected error"),
                2,
                b"index evidence".to_vec(),
                AuditDisposition::Exportable {
                    evidence: vec![EvidenceRef::new(
                        Table::Support,
                        RowKey::new("support_recent_index_v1").req("unexpected error"),
                    )],
                },
            )
            .req("unexpected error");
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
                &CreateConversation::new("New conversation", Shell::Mobile).req("unexpected error")
            ),
            Err(SupportError::Store(_))
        ));
        assert_eq!(
            SupportService::list(&scope).req("unexpected error"),
            vec![original.clone()]
        );
    }
    fixture.restart();
    let scope = fixture
        .store()
        .principal(&principal("alice"))
        .req("unexpected error");
    assert_eq!(
        SupportService::list(&scope).req("unexpected error"),
        vec![original]
    );
    assert_eq!(scope.keys(Table::Support).len(), 2);
}

#[test]
fn support_legacy_index_is_persisted_once_and_ties_are_stable() {
    let mut fixture = Fixture::new("repair-22-33-43-legacy", u64::MAX);
    let request = CreateConversation::new("Legacy", Shell::Desktop).req("unexpected error");
    let mut conversations = Vec::new();
    {
        let mut scope = fixture
            .store()
            .principal(&principal("alice"))
            .req("unexpected error");
        for key in ["one", "two", "three"] {
            conversations
                .push(SupportService::create(&mut scope, 1, key, &request).req("unexpected error"));
        }
        scope
            .remove(
                Table::Support,
                &RowKey::new("support_recent_index_v1").req("unexpected error"),
            )
            .req("unexpected error");
    }
    fixture.restart();
    {
        let mut scope = fixture
            .store()
            .principal(&principal("alice"))
            .req("unexpected error");
        conversations.sort_by(|a, b| b.conversation_id().cmp(a.conversation_id()));
        assert_eq!(
            SupportService::list(&scope).req("unexpected error"),
            conversations
        );
        assert_eq!(scope.keys(Table::Support).len(), 4);
        assert_eq!(
            SupportService::create(&mut scope, 9, "one", &request)
                .req("unexpected error")
                .created_at(),
            1
        );
        let index_key = RowKey::new("support_recent_index_v1").req("unexpected error");
        let mut index: serde_json::Value = serde_json::from_slice(
            scope
                .get(Table::Support, &index_key)
                .unwrap_or_else(|| panic!("missing value"))
                .bytes(),
        )
        .req("unexpected error");
        index["entries"]
            .as_array_mut()
            .unwrap_or_else(|| panic!("missing value"))
            .swap(0, 1);
        scope
            .put(
                Table::Support,
                index_key,
                9,
                serde_json::to_vec(&index).req("unexpected error"),
            )
            .req("unexpected error");
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
        let mut scope = fixture
            .store()
            .principal(&principal("alice"))
            .req("unexpected error");
        let original = SupportService::create(
            &mut scope,
            1,
            "create",
            &CreateConversation::new("First", Shell::Mobile).req("unexpected error"),
        )
        .req("unexpected error");
        id = original.conversation_id().to_owned();
        for i in 1..200_u64 {
            let conversation = SupportService::reply(
                &mut scope,
                i + 1,
                &id,
                &format!("reply-{i}"),
                format!("Reply {i}"),
            )
            .req("unexpected error");
            assert_eq!(
                conversation.messages().len(),
                usize::try_from(i + 1).req("unexpected error")
            );
        }
        assert!(matches!(
            SupportService::reply(&mut scope, 201, &id, "overflow", "No room"),
            Err(SupportError::ConversationFull)
        ));
        let replay = SupportService::reply(&mut scope, 201, &id, "reply-199", "Reply 199")
            .req("unexpected error");
        assert_eq!(replay.messages().len(), 200);
        assert_eq!(replay.updated_at(), 200);
        SupportService::resolve(&mut scope, 202, &id).req("unexpected error");
    }
    fixture.restart();
    let mut scope = fixture
        .store()
        .principal(&principal("alice"))
        .req("unexpected error");
    let replay = SupportService::reply(&mut scope, 203, &id, "reply-199", "Reply 199")
        .req("unexpected error");
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

trait Require<T> {
    fn req(self, context: &str) -> T;
}

impl<T, E: std::fmt::Debug> Require<T> for Result<T, E> {
    fn req(self, context: &str) -> T {
        self.unwrap_or_else(|error| panic!("{context}: {error:?}"))
    }
}
