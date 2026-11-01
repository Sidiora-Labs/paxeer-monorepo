use std::fs;

use layerx_human_service::audit::{
    verify_export, AuditChain, AuditError, AuditEvent, AuthMethod, ChainHead, Decision,
    IdentityEvent, JourneyKind, JourneyState, SigningOperation, StepUpEvidence,
};
use layerx_human_service::redaction::Label;
use layerx_human_service::store::PrincipalStore;
use layerx_human_service::trace::TraceId;
use layerx_human_test_support::{
    directory, install_and_open, principal, retention_uniform, tenancy,
};

pub use layerx_human_service::{redaction, store, trace};

fn retired_events() -> [AuditEvent; 3] {
    [
        AuditEvent::SigningDecision {
            operation: SigningOperation::EmergencyExit,
            disclosure_digest: [2; 32],
            step_up: StepUpEvidence::NotRequired,
            outcome: Decision::Granted,
        },
        AuditEvent::JourneyTransition {
            journey: Label::new("wallet-binding").unwrap_or_else(|error| panic!("label: {error}")),
            kind: JourneyKind::WalletBinding,
            from: JourneyState::Processing,
            to: JourneyState::Done,
        },
        AuditEvent::IdentityLifecycle {
            event: IdentityEvent::WalletBinding,
            receipt_digest: [3; 32],
        },
    ]
}

#[test]
fn stored_emergency_exit_audit_record_still_verifies() {
    let root = directory("audit-retired-stored");
    let map = tenancy(&[("alice", "tenant-a")]);
    let (mut store, digest) = install_and_open(&root, &map, retention_uniform(100));
    let alice = principal("alice");
    let trace = TraceId::mint([1; 16]);
    let expected_head;
    let original_bytes;
    {
        let mut scope = store
            .principal(&alice)
            .unwrap_or_else(|error| panic!("scope: {error}"));
        let mut old_chain = historical_head::AuditChain::open(&scope)
            .unwrap_or_else(|error| panic!("HEAD chain: {error}"));
        let legacy_events = [
            historical_head::AuditEvent::SigningDecision {
                operation: historical_head::SigningOperation::EmergencyExit,
                disclosure_digest: [2; 32],
                step_up: historical_head::StepUpEvidence::NotRequired,
                outcome: historical_head::Decision::Granted,
            },
            historical_head::AuditEvent::JourneyTransition {
                journey: Label::new("wallet-binding")
                    .unwrap_or_else(|error| panic!("label: {error}")),
                kind: historical_head::JourneyKind::WalletBinding,
                from: historical_head::JourneyState::Processing,
                to: historical_head::JourneyState::Done,
            },
            historical_head::AuditEvent::IdentityLifecycle {
                event: historical_head::IdentityEvent::WalletBinding,
                receipt_digest: [3; 32],
            },
        ];
        for event in &legacy_events {
            old_chain
                .append(&mut scope, 1, &trace, event, &[])
                .unwrap_or_else(|error| panic!("HEAD append: {error}"));
        }
        expected_head = ChainHead::decode(&old_chain.head().encode())
            .unwrap_or_else(|error| panic!("head: {error}"));
        original_bytes = old_chain
            .entries(&scope)
            .unwrap_or_else(|error| panic!("HEAD entries: {error}"))
            .iter()
            .map(|entry| entry.bytes().to_vec())
            .collect::<Vec<_>>();
    }
    drop(store);
    let mut reopened = PrincipalStore::open(&root, retention_uniform(100), digest)
        .unwrap_or_else(|error| panic!("reopen: {error}"));
    {
        let mut scope = reopened
            .principal(&alice)
            .unwrap_or_else(|error| panic!("scope: {error}"));
        let mut chain = AuditChain::open_anchored(&scope, expected_head)
            .unwrap_or_else(|error| panic!("verify HEAD records: {error}"));
        let entries = chain
            .entries(&scope)
            .unwrap_or_else(|error| panic!("entries: {error}"));
        assert_eq!(entries.len(), 3);
        for ((entry, expected), bytes) in entries.iter().zip(retired_events()).zip(&original_bytes)
        {
            assert_eq!(entry.event(), &expected);
            assert_eq!(entry.bytes(), bytes);
        }
        let bundle = chain
            .export(&scope)
            .unwrap_or_else(|error| panic!("export: {error}"));
        let report =
            verify_export(&bundle).unwrap_or_else(|error| panic!("verify export: {error}"));
        assert_eq!(report.entries(), 3);
        assert_eq!(report.head(), expected_head);
        chain
            .append(
                &mut scope,
                2,
                &trace,
                &AuditEvent::SigningDecision {
                    operation: SigningOperation::AgentArchive,
                    disclosure_digest: [4; 32],
                    step_up: StepUpEvidence::NotRequired,
                    outcome: Decision::Granted,
                },
                &[],
            )
            .unwrap_or_else(|error| panic!("extend legacy chain: {error}"));
        let old_chain = historical_head::AuditChain::open(&scope)
            .unwrap_or_else(|error| panic!("HEAD reads extended chain: {error}"));
        assert_eq!(old_chain.head().encode(), chain.head().encode());
        assert!(matches!(
            old_chain
                .entries(&scope)
                .unwrap_or_else(|error| panic!("HEAD entries: {error}"))[3]
                .event(),
            historical_head::AuditEvent::SigningDecision {
                operation: historical_head::SigningOperation::AgentArchive,
                ..
            }
        ));
        assert_eq!(chain.head().length(), 4);
    }
    drop(reopened);
    fs::remove_dir_all(root).unwrap_or_else(|error| panic!("cleanup: {error}"));
}

#[test]
fn new_wallet_binding_audit_event_is_refused() {
    let root = directory("audit-retired-refused");
    let map = tenancy(&[("alice", "tenant-a")]);
    let (mut store, digest) = install_and_open(&root, &map, retention_uniform(100));
    let alice = principal("alice");
    let trace = TraceId::mint([5; 16]);
    let expected_head;
    {
        let mut scope = store
            .principal(&alice)
            .unwrap_or_else(|error| panic!("scope: {error}"));
        let mut chain = AuditChain::open(&scope).unwrap_or_else(|error| panic!("chain: {error}"));
        chain
            .append(
                &mut scope,
                1,
                &trace,
                &AuditEvent::Authentication {
                    method: AuthMethod::Passkey,
                    outcome: Decision::Granted,
                },
                &[],
            )
            .unwrap_or_else(|error| panic!("active append: {error}"));
        expected_head = chain.head();
        let before = fs::read(root.join("principals/alice/store.bin"))
            .unwrap_or_else(|error| panic!("read before: {error}"));
        for event in retired_events() {
            assert!(matches!(
                chain.append(&mut scope, 2, &trace, &event, &[]),
                Err(AuditError::Corrupt("retired audit event kind"))
            ));
            assert_eq!(chain.head(), expected_head);
            assert_eq!(
                chain
                    .entries(&scope)
                    .unwrap_or_else(|error| panic!("entries: {error}"))
                    .len(),
                1
            );
            assert_eq!(
                fs::read(root.join("principals/alice/store.bin"))
                    .unwrap_or_else(|error| panic!("read after: {error}")),
                before
            );
        }
    }
    drop(store);
    let mut reopened = PrincipalStore::open(&root, retention_uniform(100), digest)
        .unwrap_or_else(|error| panic!("reopen: {error}"));
    {
        let scope = reopened
            .principal(&alice)
            .unwrap_or_else(|error| panic!("scope: {error}"));
        let chain = AuditChain::open_anchored(&scope, expected_head)
            .unwrap_or_else(|error| panic!("reopen chain: {error}"));
        let report = verify_export(
            &chain
                .export(&scope)
                .unwrap_or_else(|error| panic!("export: {error}")),
        )
        .unwrap_or_else(|error| panic!("verify export: {error}"));
        assert_eq!(report.entries(), 1);
        assert_eq!(report.head(), expected_head);
    }
    drop(reopened);
    fs::remove_dir_all(root).unwrap_or_else(|error| panic!("cleanup: {error}"));
}

// Frozen audit producer from dd40942c26124de26d2b64e9fe6547fe757acc14.
// It writes the legacy on-disk records through the real principal store.
#[allow(dead_code, unused_imports)]
mod historical_head {
    //! The append-only hash-chained audit log of the human plane. Every
    //! consequential action - authentication, signing decisions, approvals,
    //! journey transitions, security changes and notification dispatches - is
    //! appended as exportable evidence of record on top of the principal store's
    //! audit namespace, each entry linking to its predecessor and binding the
    //! content of every row it references, so truncation, reordering or
    //! alteration is refused rather than silently served.

    mod event {
        use crate::redaction::Label;

        use super::wire::{push_bytes, Reader};
        use super::AuditError;

        /// How an authentication event was performed.
        #[derive(Clone, Copy, Debug, Eq, PartialEq)]
        pub enum AuthMethod {
            Passkey,
            FallbackCredential,
            StepUp,
        }

        impl AuthMethod {
            const fn code(self) -> u8 {
                match self {
                    Self::Passkey => 1,
                    Self::FallbackCredential => 2,
                    Self::StepUp => 3,
                }
            }

            fn from_code(value: u8) -> Result<Self, AuditError> {
                match value {
                    1 => Ok(Self::Passkey),
                    2 => Ok(Self::FallbackCredential),
                    3 => Ok(Self::StepUp),
                    _ => Err(AuditError::Corrupt("unknown authentication method")),
                }
            }
        }

        /// Whether a guarded decision was granted or refused.
        #[derive(Clone, Copy, Debug, Eq, PartialEq)]
        pub enum Decision {
            Granted,
            Refused,
        }

        impl Decision {
            const fn code(self) -> u8 {
                match self {
                    Self::Granted => 1,
                    Self::Refused => 2,
                }
            }

            fn from_code(value: u8) -> Result<Self, AuditError> {
                match value {
                    1 => Ok(Self::Granted),
                    2 => Ok(Self::Refused),
                    _ => Err(AuditError::Corrupt("unknown decision")),
                }
            }
        }

        /// The step-up evidence recorded with a guarded decision: never the ceremony
        /// material itself, only its digest.
        #[derive(Clone, Copy, Debug, Eq, PartialEq)]
        pub enum StepUpEvidence {
            NotRequired,
            Fresh { ceremony_digest: [u8; 32] },
            Missing,
        }

        impl StepUpEvidence {
            fn encode(self, output: &mut Vec<u8>) {
                match self {
                    Self::NotRequired => output.push(0),
                    Self::Fresh { ceremony_digest } => {
                        output.push(1);
                        output.extend_from_slice(&ceremony_digest);
                    }
                    Self::Missing => output.push(2),
                }
            }

            fn decode(reader: &mut Reader<'_>) -> Result<Self, AuditError> {
                match reader.byte()? {
                    0 => Ok(Self::NotRequired),
                    1 => Ok(Self::Fresh {
                        ceremony_digest: reader.array()?,
                    }),
                    2 => Ok(Self::Missing),
                    _ => Err(AuditError::Corrupt("unknown step-up evidence")),
                }
            }
        }

        /// The operation a signing decision was made for.
        #[derive(Clone, Copy, Debug, Eq, PartialEq)]
        pub enum SigningOperation {
            DidRegistration,
            KeyRotation,
            RecoveryRegistration,
            EvmPayoutBinding,
            LxpSend,
            LxpReceive,
            PayerGrantRegistration,
            BudgetCreate,
            BudgetFund,
            BudgetDefund,
            BridgeDepositCredit,
            BridgeWithdrawRequest,
            SessionProvision,
            ProtocolMutation,
            ApprovalDecision,
            SecuritySettings,
            SecretReveal,
            EmergencyExit,
            AgentArchive,
        }

        impl SigningOperation {
            const fn code(self) -> u8 {
                match self {
                    Self::DidRegistration => 1,
                    Self::KeyRotation => 2,
                    Self::RecoveryRegistration => 3,
                    Self::EvmPayoutBinding => 4,
                    Self::LxpSend => 5,
                    Self::LxpReceive => 6,
                    Self::PayerGrantRegistration => 7,
                    Self::BudgetCreate => 8,
                    Self::BudgetFund => 9,
                    Self::BudgetDefund => 10,
                    Self::BridgeDepositCredit => 11,
                    Self::BridgeWithdrawRequest => 12,
                    Self::SessionProvision => 13,
                    Self::ProtocolMutation => 14,
                    Self::ApprovalDecision => 15,
                    Self::SecuritySettings => 16,
                    Self::SecretReveal => 17,
                    Self::EmergencyExit => 18,
                    Self::AgentArchive => 19,
                }
            }

            fn from_code(value: u8) -> Result<Self, AuditError> {
                match value {
                    1 => Ok(Self::DidRegistration),
                    2 => Ok(Self::KeyRotation),
                    3 => Ok(Self::RecoveryRegistration),
                    4 => Ok(Self::EvmPayoutBinding),
                    5 => Ok(Self::LxpSend),
                    6 => Ok(Self::LxpReceive),
                    7 => Ok(Self::PayerGrantRegistration),
                    8 => Ok(Self::BudgetCreate),
                    9 => Ok(Self::BudgetFund),
                    10 => Ok(Self::BudgetDefund),
                    11 => Ok(Self::BridgeDepositCredit),
                    12 => Ok(Self::BridgeWithdrawRequest),
                    13 => Ok(Self::SessionProvision),
                    14 => Ok(Self::ProtocolMutation),
                    15 => Ok(Self::ApprovalDecision),
                    16 => Ok(Self::SecuritySettings),
                    17 => Ok(Self::SecretReveal),
                    18 => Ok(Self::EmergencyExit),
                    19 => Ok(Self::AgentArchive),
                    _ => Err(AuditError::Corrupt("unknown signing operation")),
                }
            }
        }

        /// The final outcome of an approval hold.
        #[derive(Clone, Copy, Debug, Eq, PartialEq)]
        pub enum ApprovalOutcome {
            Approved,
            Rejected,
            Expired,
            Defective,
        }

        impl ApprovalOutcome {
            const fn code(self) -> u8 {
                match self {
                    Self::Approved => 1,
                    Self::Rejected => 2,
                    Self::Expired => 3,
                    Self::Defective => 4,
                }
            }

            fn from_code(value: u8) -> Result<Self, AuditError> {
                match value {
                    1 => Ok(Self::Approved),
                    2 => Ok(Self::Rejected),
                    3 => Ok(Self::Expired),
                    4 => Ok(Self::Defective),
                    _ => Err(AuditError::Corrupt("unknown approval outcome")),
                }
            }
        }

        /// The journey kinds whose transitions are audited.
        #[derive(Clone, Copy, Debug, Eq, PartialEq)]
        pub enum JourneyKind {
            Onboarding,
            WalletBinding,
            Deposit,
            Withdraw,
            Exit,
            Move,
            AgentCreate,
            AgentFund,
            AgentPause,
            AgentRetire,
        }

        impl JourneyKind {
            const fn code(self) -> u8 {
                match self {
                    Self::Onboarding => 1,
                    Self::WalletBinding => 2,
                    Self::Deposit => 3,
                    Self::Withdraw => 4,
                    Self::Exit => 5,
                    Self::Move => 6,
                    Self::AgentCreate => 7,
                    Self::AgentFund => 8,
                    Self::AgentPause => 9,
                    Self::AgentRetire => 10,
                }
            }

            fn from_code(value: u8) -> Result<Self, AuditError> {
                match value {
                    1 => Ok(Self::Onboarding),
                    2 => Ok(Self::WalletBinding),
                    3 => Ok(Self::Deposit),
                    4 => Ok(Self::Withdraw),
                    5 => Ok(Self::Exit),
                    6 => Ok(Self::Move),
                    7 => Ok(Self::AgentCreate),
                    8 => Ok(Self::AgentFund),
                    9 => Ok(Self::AgentPause),
                    10 => Ok(Self::AgentRetire),
                    _ => Err(AuditError::Corrupt("unknown journey kind")),
                }
            }
        }

        /// The normative journey states a transition moves between.
        #[derive(Clone, Copy, Debug, Eq, PartialEq)]
        pub enum JourneyState {
            GettingReady,
            Sending,
            Processing,
            Done,
            DoneFinalised,
            StillChecking,
            Refused,
            WaitingForYou,
        }

        impl JourneyState {
            const fn code(self) -> u8 {
                match self {
                    Self::GettingReady => 1,
                    Self::Sending => 2,
                    Self::Processing => 3,
                    Self::Done => 4,
                    Self::DoneFinalised => 5,
                    Self::StillChecking => 6,
                    Self::Refused => 7,
                    Self::WaitingForYou => 8,
                }
            }

            fn from_code(value: u8) -> Result<Self, AuditError> {
                match value {
                    1 => Ok(Self::GettingReady),
                    2 => Ok(Self::Sending),
                    3 => Ok(Self::Processing),
                    4 => Ok(Self::Done),
                    5 => Ok(Self::DoneFinalised),
                    6 => Ok(Self::StillChecking),
                    7 => Ok(Self::Refused),
                    8 => Ok(Self::WaitingForYou),
                    _ => Err(AuditError::Corrupt("unknown journey state")),
                }
            }
        }

        /// The audited security-setting changes.
        #[derive(Clone, Copy, Debug, Eq, PartialEq)]
        pub enum SecurityChangeKind {
            DeviceAdded,
            SessionRevoked,
            SignOutEverywhere,
            TwoFactorEnabled,
            TwoFactorDisabled,
            RecoveryInitiated,
            KeyRotation,
            WalletRebinding,
            AgentArchive,
            KeyExported,
        }

        impl SecurityChangeKind {
            const fn code(self) -> u8 {
                match self {
                    Self::DeviceAdded => 1,
                    Self::SessionRevoked => 2,
                    Self::SignOutEverywhere => 3,
                    Self::TwoFactorEnabled => 4,
                    Self::TwoFactorDisabled => 5,
                    Self::RecoveryInitiated => 6,
                    Self::KeyRotation => 7,
                    Self::WalletRebinding => 8,
                    Self::AgentArchive => 9,
                    Self::KeyExported => 10,
                }
            }

            fn from_code(value: u8) -> Result<Self, AuditError> {
                match value {
                    1 => Ok(Self::DeviceAdded),
                    2 => Ok(Self::SessionRevoked),
                    3 => Ok(Self::SignOutEverywhere),
                    4 => Ok(Self::TwoFactorEnabled),
                    5 => Ok(Self::TwoFactorDisabled),
                    6 => Ok(Self::RecoveryInitiated),
                    7 => Ok(Self::KeyRotation),
                    8 => Ok(Self::WalletRebinding),
                    9 => Ok(Self::AgentArchive),
                    10 => Ok(Self::KeyExported),
                    _ => Err(AuditError::Corrupt("unknown security change")),
                }
            }
        }

        /// The notification classes whose dispatches are audited.
        #[derive(Clone, Copy, Debug, Eq, PartialEq)]
        pub enum NotificationClass {
            ApprovalWaiting,
            MoneyArrived,
            JourneyCompleted,
            JourneyFailed,
            ClaimReady,
            Security,
            Degradation,
        }

        impl NotificationClass {
            const fn code(self) -> u8 {
                match self {
                    Self::ApprovalWaiting => 1,
                    Self::MoneyArrived => 2,
                    Self::JourneyCompleted => 3,
                    Self::JourneyFailed => 4,
                    Self::ClaimReady => 5,
                    Self::Security => 6,
                    Self::Degradation => 7,
                }
            }

            fn from_code(value: u8) -> Result<Self, AuditError> {
                match value {
                    1 => Ok(Self::ApprovalWaiting),
                    2 => Ok(Self::MoneyArrived),
                    3 => Ok(Self::JourneyCompleted),
                    4 => Ok(Self::JourneyFailed),
                    5 => Ok(Self::ClaimReady),
                    6 => Ok(Self::Security),
                    7 => Ok(Self::Degradation),
                    _ => Err(AuditError::Corrupt("unknown notification class")),
                }
            }
        }

        /// The channel a notification was dispatched on.
        #[derive(Clone, Copy, Debug, Eq, PartialEq)]
        pub enum NotificationChannel {
            Push,
            Email,
            InApp,
        }

        impl NotificationChannel {
            const fn code(self) -> u8 {
                match self {
                    Self::Push => 1,
                    Self::Email => 2,
                    Self::InApp => 3,
                }
            }

            fn from_code(value: u8) -> Result<Self, AuditError> {
                match value {
                    1 => Ok(Self::Push),
                    2 => Ok(Self::Email),
                    3 => Ok(Self::InApp),
                    _ => Err(AuditError::Corrupt("unknown notification channel")),
                }
            }
        }

        /// The identity-lifecycle milestones recorded with their activating receipt.
        #[derive(Clone, Copy, Debug, Eq, PartialEq)]
        pub enum IdentityEvent {
            DidRegistration,
            KeyActivation,
            RecoveryRegistration,
            WalletBinding,
        }

        impl IdentityEvent {
            const fn code(self) -> u8 {
                match self {
                    Self::DidRegistration => 1,
                    Self::KeyActivation => 2,
                    Self::RecoveryRegistration => 3,
                    Self::WalletBinding => 4,
                }
            }

            fn from_code(value: u8) -> Result<Self, AuditError> {
                match value {
                    1 => Ok(Self::DidRegistration),
                    2 => Ok(Self::KeyActivation),
                    3 => Ok(Self::RecoveryRegistration),
                    4 => Ok(Self::WalletBinding),
                    _ => Err(AuditError::Corrupt("unknown identity event")),
                }
            }
        }

        /// One audited event. Every field is a digest, a bounded label or a closed
        /// enum, so payload secrets, key material, personal data and financial
        /// values are unrepresentable in the audit log.
        #[derive(Clone, Debug, Eq, PartialEq)]
        pub enum AuditEvent {
            Authentication {
                method: AuthMethod,
                outcome: Decision,
            },
            SigningDecision {
                operation: SigningOperation,
                disclosure_digest: [u8; 32],
                step_up: StepUpEvidence,
                outcome: Decision,
            },
            ApprovalDecision {
                hold_digest: [u8; 32],
                step_up: StepUpEvidence,
                outcome: ApprovalOutcome,
            },
            JourneyTransition {
                journey: Label,
                kind: JourneyKind,
                from: JourneyState,
                to: JourneyState,
            },
            SecurityChange {
                change: SecurityChangeKind,
                step_up: StepUpEvidence,
            },
            NotificationDispatch {
                class: NotificationClass,
                channel: NotificationChannel,
            },
            IdentityLifecycle {
                event: IdentityEvent,
                receipt_digest: [u8; 32],
            },
        }

        impl AuditEvent {
            /// Returns the emission label of the event's kind.
            #[must_use]
            pub const fn kind_label(&self) -> &'static str {
                match self {
                    Self::Authentication { .. } => "authentication",
                    Self::SigningDecision { .. } => "signing-decision",
                    Self::ApprovalDecision { .. } => "approval-decision",
                    Self::JourneyTransition { .. } => "journey-transition",
                    Self::SecurityChange { .. } => "security-change",
                    Self::NotificationDispatch { .. } => "notification-dispatch",
                    Self::IdentityLifecycle { .. } => "identity-lifecycle",
                }
            }

            pub(super) fn encode(&self, output: &mut Vec<u8>) -> Result<(), AuditError> {
                match self {
                    Self::Authentication { method, outcome } => {
                        output.push(1);
                        output.push(method.code());
                        output.push(outcome.code());
                    }
                    Self::SigningDecision {
                        operation,
                        disclosure_digest,
                        step_up,
                        outcome,
                    } => {
                        output.push(2);
                        output.push(operation.code());
                        output.extend_from_slice(disclosure_digest);
                        step_up.encode(output);
                        output.push(outcome.code());
                    }
                    Self::ApprovalDecision {
                        hold_digest,
                        step_up,
                        outcome,
                    } => {
                        output.push(3);
                        output.extend_from_slice(hold_digest);
                        step_up.encode(output);
                        output.push(outcome.code());
                    }
                    Self::JourneyTransition {
                        journey,
                        kind,
                        from,
                        to,
                    } => {
                        output.push(4);
                        push_bytes(output, journey.as_str().as_bytes())?;
                        output.push(kind.code());
                        output.push(from.code());
                        output.push(to.code());
                    }
                    Self::SecurityChange { change, step_up } => {
                        output.push(5);
                        output.push(change.code());
                        step_up.encode(output);
                    }
                    Self::NotificationDispatch { class, channel } => {
                        output.push(6);
                        output.push(class.code());
                        output.push(channel.code());
                    }
                    Self::IdentityLifecycle {
                        event,
                        receipt_digest,
                    } => {
                        output.push(7);
                        output.push(event.code());
                        output.extend_from_slice(receipt_digest);
                    }
                }
                Ok(())
            }

            pub(super) fn decode(reader: &mut Reader<'_>) -> Result<Self, AuditError> {
                match reader.byte()? {
                    1 => Ok(Self::Authentication {
                        method: AuthMethod::from_code(reader.byte()?)?,
                        outcome: Decision::from_code(reader.byte()?)?,
                    }),
                    2 => Ok(Self::SigningDecision {
                        operation: SigningOperation::from_code(reader.byte()?)?,
                        disclosure_digest: reader.array()?,
                        step_up: StepUpEvidence::decode(reader)?,
                        outcome: Decision::from_code(reader.byte()?)?,
                    }),
                    3 => Ok(Self::ApprovalDecision {
                        hold_digest: reader.array()?,
                        step_up: StepUpEvidence::decode(reader)?,
                        outcome: ApprovalOutcome::from_code(reader.byte()?)?,
                    }),
                    4 => {
                        let text = std::str::from_utf8(reader.bytes()?)
                            .map_err(|_| AuditError::Corrupt("journey label is not UTF-8"))?;
                        Ok(Self::JourneyTransition {
                            journey: Label::new(text)?,
                            kind: JourneyKind::from_code(reader.byte()?)?,
                            from: JourneyState::from_code(reader.byte()?)?,
                            to: JourneyState::from_code(reader.byte()?)?,
                        })
                    }
                    5 => Ok(Self::SecurityChange {
                        change: SecurityChangeKind::from_code(reader.byte()?)?,
                        step_up: StepUpEvidence::decode(reader)?,
                    }),
                    6 => Ok(Self::NotificationDispatch {
                        class: NotificationClass::from_code(reader.byte()?)?,
                        channel: NotificationChannel::from_code(reader.byte()?)?,
                    }),
                    7 => Ok(Self::IdentityLifecycle {
                        event: IdentityEvent::from_code(reader.byte()?)?,
                        receipt_digest: reader.array()?,
                    }),
                    _ => Err(AuditError::Corrupt("unknown audit event kind")),
                }
            }
        }
    }
    mod export {
        include!(concat!(env!("CARGO_MANIFEST_DIR"), "/src/audit/export.rs"));
    }
    mod wire {
        include!(concat!(env!("CARGO_MANIFEST_DIR"), "/src/audit/wire.rs"));
    }

    pub use event::{
        ApprovalOutcome, AuditEvent, AuthMethod, Decision, IdentityEvent, JourneyKind,
        JourneyState, NotificationChannel, NotificationClass, SecurityChangeKind, SigningOperation,
        StepUpEvidence,
    };
    pub use export::{verify_export, ExportReport};

    use std::fmt::{Display, Formatter};

    use layerx_proof::merkle::leaf_hash;

    use crate::redaction::RedactionError;
    use crate::store::{
        AuditDisposition, EvidenceRef, PrincipalId, PrincipalScope, RowKey, StoreError, Table,
    };
    use crate::trace::{TraceError, TraceId};

    use wire::{push_bytes, push_length, Reader};

    const ENTRY_MAGIC: &[u8; 4] = b"LXAE";
    const ENTRY_VERSION: u8 = 1;
    const HEAD_MAGIC: &[u8; 4] = b"LXAH";
    const GENESIS_DOMAIN: &[u8; 4] = b"LXAG";
    const CHAIN_PREFIX: &str = "chain-";
    const SEQUENCE_DIGITS: usize = 16;

    const fn table_code(table: Table) -> u8 {
        match table {
            Table::Journeys => 1,
            Table::Notifications => 2,
            Table::Support => 6,
            Table::Telemetry => 4,
            Table::Cache => 5,
            Table::Stream => 7,
            Table::EventOutbox => 8,
        }
    }

    fn table_from_code(value: u8) -> Result<Table, AuditError> {
        match value {
            1 => Ok(Table::Journeys),
            2 => Ok(Table::Notifications),
            6 => Ok(Table::Support),
            4 => Ok(Table::Telemetry),
            5 => Ok(Table::Cache),
            7 => Ok(Table::Stream),
            8 => Ok(Table::EventOutbox),
            _ => Err(AuditError::Corrupt("unknown evidence table code")),
        }
    }

    fn chain_key(sequence: u64) -> Result<RowKey, AuditError> {
        Ok(RowKey::new(format!("{CHAIN_PREFIX}{sequence:016x}"))?)
    }

    fn parse_sequence(digits: &str) -> Option<u64> {
        if digits.len() != SEQUENCE_DIGITS
            || !digits
                .bytes()
                .all(|byte| byte.is_ascii_digit() || matches!(byte, b'a'..=b'f'))
        {
            return None;
        }
        u64::from_str_radix(digits, 16).ok()
    }

    fn genesis_link(principal: &PrincipalId) -> Result<[u8; 32], AuditError> {
        let mut bytes = GENESIS_DOMAIN.to_vec();
        bytes.extend_from_slice(principal.as_str().as_bytes());
        leaf_hash(&bytes).map_err(|_| AuditError::Unhashable)
    }

    fn evidence_digest(written_at: u64, bytes: &[u8]) -> Result<[u8; 32], AuditError> {
        let mut buffer = Vec::with_capacity(8_usize.saturating_add(bytes.len()));
        buffer.extend_from_slice(&written_at.to_be_bytes());
        buffer.extend_from_slice(bytes);
        leaf_hash(&buffer).map_err(|_| AuditError::Unhashable)
    }

    /// One evidence reference with the content digest the chain entry binds, so
    /// altering a referenced row after the fact is detectable.
    #[derive(Clone, Debug, Eq, PartialEq)]
    pub struct BoundEvidence {
        table: Table,
        key: RowKey,
        digest: [u8; 32],
    }

    impl BoundEvidence {
        /// Returns the referenced table.
        #[must_use]
        pub const fn table(&self) -> Table {
            self.table
        }

        /// Returns the referenced key.
        #[must_use]
        pub const fn key(&self) -> &RowKey {
            &self.key
        }

        /// Returns the bound content digest.
        #[must_use]
        pub const fn digest(&self) -> [u8; 32] {
            self.digest
        }
    }

    /// The chain head - entry count and tip link - recorded out of band so tail
    /// truncation of an otherwise-consistent chain is detectable.
    #[derive(Clone, Copy, Debug, Eq, PartialEq)]
    pub struct ChainHead {
        length: u64,
        link: [u8; 32],
    }

    impl ChainHead {
        /// Returns the number of chained entries.
        #[must_use]
        pub const fn length(&self) -> u64 {
            self.length
        }

        /// Returns the tip link.
        #[must_use]
        pub const fn link(&self) -> [u8; 32] {
            self.link
        }

        /// Encodes the head for out-of-band recording.
        #[must_use]
        pub fn encode(&self) -> Vec<u8> {
            let mut bytes = Vec::with_capacity(44);
            bytes.extend_from_slice(HEAD_MAGIC);
            bytes.extend_from_slice(&self.length.to_be_bytes());
            bytes.extend_from_slice(&self.link);
            bytes
        }

        /// Reconstructs a head recorded out of band.
        ///
        /// # Errors
        ///
        /// Refuses bytes that are not exactly one encoded head.
        pub fn decode(bytes: &[u8]) -> Result<Self, AuditError> {
            let mut reader = Reader::new(bytes);
            if reader.take(4)? != HEAD_MAGIC {
                return Err(AuditError::Corrupt("invalid chain head header"));
            }
            let length = reader.u64()?;
            let link = reader.array()?;
            if !reader.is_empty() {
                return Err(AuditError::Corrupt("trailing bytes"));
            }
            Ok(Self { length, link })
        }
    }

    /// One verified entry of the audit chain.
    #[derive(Clone, Debug)]
    pub struct ChainEntry {
        sequence: u64,
        recorded_at: u64,
        trace: TraceId,
        event: AuditEvent,
        evidence: Vec<BoundEvidence>,
        prev_link: [u8; 32],
        link: [u8; 32],
        bytes: Vec<u8>,
    }

    impl ChainEntry {
        /// Returns the entry's position in the chain.
        #[must_use]
        pub const fn sequence(&self) -> u64 {
            self.sequence
        }

        /// Returns the caller-injected append timestamp.
        #[must_use]
        pub const fn recorded_at(&self) -> u64 {
            self.recorded_at
        }

        /// Returns the trace the audited action ran under.
        #[must_use]
        pub const fn trace(&self) -> &TraceId {
            &self.trace
        }

        /// Returns the audited event.
        #[must_use]
        pub const fn event(&self) -> &AuditEvent {
            &self.event
        }

        /// Returns the evidence the entry binds.
        #[must_use]
        pub fn evidence(&self) -> &[BoundEvidence] {
            &self.evidence
        }

        /// Returns the predecessor link the entry commits to.
        #[must_use]
        pub const fn prev_link(&self) -> [u8; 32] {
            self.prev_link
        }

        /// Returns the entry's own link.
        #[must_use]
        pub const fn link(&self) -> [u8; 32] {
            self.link
        }

        /// Returns the entry's canonical bytes, the exact preimage of its link.
        #[must_use]
        pub fn bytes(&self) -> &[u8] {
            &self.bytes
        }
    }

    struct EntryBody {
        sequence: u64,
        recorded_at: u64,
        prev_link: [u8; 32],
        trace: TraceId,
        event: AuditEvent,
        evidence: Vec<BoundEvidence>,
    }

    fn encode_entry(
        sequence: u64,
        recorded_at: u64,
        prev_link: &[u8; 32],
        trace: &TraceId,
        event: &AuditEvent,
        evidence: &[BoundEvidence],
    ) -> Result<Vec<u8>, AuditError> {
        let mut output = Vec::new();
        output.extend_from_slice(ENTRY_MAGIC);
        output.push(ENTRY_VERSION);
        output.extend_from_slice(&sequence.to_be_bytes());
        output.extend_from_slice(&recorded_at.to_be_bytes());
        output.extend_from_slice(prev_link);
        push_bytes(&mut output, trace.as_str().as_bytes())?;
        event.encode(&mut output)?;
        push_length(&mut output, evidence.len())?;
        for binding in evidence {
            output.push(table_code(binding.table));
            push_bytes(&mut output, binding.key.as_str().as_bytes())?;
            output.extend_from_slice(&binding.digest);
        }
        Ok(output)
    }

    fn decode_entry(bytes: &[u8]) -> Result<EntryBody, AuditError> {
        let mut reader = Reader::new(bytes);
        if reader.take(4)? != ENTRY_MAGIC {
            return Err(AuditError::Corrupt("invalid audit entry header"));
        }
        if reader.byte()? != ENTRY_VERSION {
            return Err(AuditError::Corrupt("unknown audit entry version"));
        }
        let sequence = reader.u64()?;
        let recorded_at = reader.u64()?;
        let prev_link = reader.array()?;
        let trace_text = std::str::from_utf8(reader.bytes()?)
            .map_err(|_| AuditError::Corrupt("trace is not UTF-8"))?;
        let trace = TraceId::parse(trace_text)?;
        let event = AuditEvent::decode(&mut reader)?;
        let count = reader.length()?;
        let mut evidence = Vec::new();
        for _ in 0..count {
            let table = table_from_code(reader.byte()?)?;
            let key_text = std::str::from_utf8(reader.bytes()?)
                .map_err(|_| AuditError::Corrupt("evidence key is not UTF-8"))?;
            let key =
                RowKey::new(key_text).map_err(|_| AuditError::Corrupt("invalid evidence key"))?;
            let digest = reader.array()?;
            evidence.push(BoundEvidence { table, key, digest });
        }
        if !reader.is_empty() {
            return Err(AuditError::Corrupt("trailing bytes"));
        }
        Ok(EntryBody {
            sequence,
            recorded_at,
            prev_link,
            trace,
            event,
            evidence,
        })
    }

    fn verify_bindings(
        scope: &PrincipalScope<'_>,
        sequence: u64,
        bindings: &[BoundEvidence],
        references: &[EvidenceRef],
    ) -> Result<(), AuditError> {
        if bindings.len() != references.len() {
            return Err(AuditError::EvidenceMismatch { sequence });
        }
        for (binding, reference) in bindings.iter().zip(references) {
            if reference.table() != binding.table || reference.key() != &binding.key {
                return Err(AuditError::EvidenceMismatch { sequence });
            }
            let row = scope
                .get(binding.table, &binding.key)
                .ok_or(AuditError::Corrupt("dangling evidence reference"))?;
            if evidence_digest(row.written_at(), row.bytes())? != binding.digest {
                return Err(AuditError::EvidenceDigestMismatch { sequence });
            }
        }
        Ok(())
    }

    fn load(scope: &PrincipalScope<'_>) -> Result<(Vec<ChainEntry>, ChainHead), AuditError> {
        let mut link = genesis_link(scope.principal())?;
        let mut entries = Vec::new();
        let mut expected: u64 = 0;
        for key in scope.audit_keys() {
            let Some(digits) = key.as_str().strip_prefix(CHAIN_PREFIX) else {
                continue;
            };
            let sequence = parse_sequence(digits).ok_or(AuditError::ForeignChainKey)?;
            if sequence != expected {
                return Err(AuditError::MissingEntry { sequence: expected });
            }
            let entry = scope
                .audit(&key)
                .ok_or(AuditError::Corrupt("audit entry unreadable"))?;
            let AuditDisposition::Exportable { evidence } = entry.disposition() else {
                return Err(AuditError::DispositionMismatch { sequence });
            };
            let body = decode_entry(entry.bytes())?;
            if body.sequence != sequence {
                return Err(AuditError::SequenceMismatch { sequence });
            }
            if body.recorded_at != entry.written_at() {
                return Err(AuditError::TimestampMismatch { sequence });
            }
            if body.prev_link != link {
                return Err(AuditError::LinkMismatch { sequence });
            }
            verify_bindings(scope, sequence, &body.evidence, evidence)?;
            link = leaf_hash(entry.bytes()).map_err(|_| AuditError::Unhashable)?;
            entries.push(ChainEntry {
                sequence,
                recorded_at: body.recorded_at,
                trace: body.trace,
                event: body.event,
                evidence: body.evidence,
                prev_link: body.prev_link,
                link,
                bytes: entry.bytes().to_vec(),
            });
            expected = expected.checked_add(1).ok_or(AuditError::SizeOverflow)?;
        }
        Ok((
            entries,
            ChainHead {
                length: expected,
                link,
            },
        ))
    }

    /// The verified head state of one principal's audit chain. Opening fully
    /// verifies the stored chain; every read re-verifies it; appends extend it
    /// as exportable evidence of record that the store never expires.
    #[derive(Debug)]
    pub struct AuditChain {
        next_sequence: u64,
        head_link: [u8; 32],
    }

    impl AuditChain {
        /// Opens the principal's chain, verifying every entry, link, timestamp,
        /// disposition and evidence binding, and refusing a truncated, reordered
        /// or altered chain.
        ///
        /// # Errors
        ///
        /// Returns the typed refusal naming the first violated entry.
        pub fn open(scope: &PrincipalScope<'_>) -> Result<Self, AuditError> {
            let (_, head) = load(scope)?;
            Ok(Self {
                next_sequence: head.length,
                head_link: head.link,
            })
        }

        /// Opens and verifies the principal's chain against the durable head an
        /// operator recorded outside the principal store. Startup uses this path
        /// after the first anchor exists, making removal of a valid tail entry a
        /// typed refusal instead of a silently shorter history.
        ///
        /// # Errors
        ///
        /// Returns the first chain violation, or [`AuditError::HeadMismatch`] when
        /// the in-store chain no longer reaches the expected durable head.
        pub fn open_anchored(
            scope: &PrincipalScope<'_>,
            expected: ChainHead,
        ) -> Result<Self, AuditError> {
            let chain = Self::open(scope)?;
            chain.verify_head(expected)?;
            Ok(chain)
        }

        /// Returns the chain head to record out of band as the anchor against
        /// tail truncation.
        #[must_use]
        pub const fn head(&self) -> ChainHead {
            ChainHead {
                length: self.next_sequence,
                link: self.head_link,
            }
        }

        /// Compares the verified chain against a head recorded out of band,
        /// catching tail truncation that in-store verification cannot see.
        ///
        /// # Errors
        ///
        /// Returns [`AuditError::HeadMismatch`] when the heads differ.
        pub fn verify_head(&self, expected: ChainHead) -> Result<(), AuditError> {
            if self.head() == expected {
                Ok(())
            } else {
                Err(AuditError::HeadMismatch {
                    expected,
                    found: self.head(),
                })
            }
        }

        /// Appends one audited event, binding the current content of every
        /// referenced row and linking the entry to the chain tip. The entry is
        /// stored exportable, so it never expires and pins its evidence.
        ///
        /// # Errors
        ///
        /// Returns evidence resolution failures, hashing failures and store
        /// persistence failures with nothing appended.
        pub fn append(
            &mut self,
            scope: &mut PrincipalScope<'_>,
            now: u64,
            trace: &TraceId,
            event: &AuditEvent,
            evidence: &[EvidenceRef],
        ) -> Result<ChainHead, AuditError> {
            let mut bound = Vec::with_capacity(evidence.len());
            for reference in evidence {
                let row = scope
                    .get(reference.table(), reference.key())
                    .ok_or(AuditError::Store(StoreError::MissingEvidence))?;
                let digest = evidence_digest(row.written_at(), row.bytes())?;
                bound.push(BoundEvidence {
                    table: reference.table(),
                    key: reference.key().clone(),
                    digest,
                });
            }
            let bytes = encode_entry(
                self.next_sequence,
                now,
                &self.head_link,
                trace,
                event,
                &bound,
            )?;
            let link = leaf_hash(&bytes).map_err(|_| AuditError::Unhashable)?;
            let key = chain_key(self.next_sequence)?;
            scope.append_audit(
                key,
                now,
                bytes,
                AuditDisposition::Exportable {
                    evidence: evidence.to_vec(),
                },
            )?;
            self.next_sequence = self
                .next_sequence
                .checked_add(1)
                .ok_or(AuditError::SizeOverflow)?;
            self.head_link = link;
            Ok(self.head())
        }

        /// Reads the chain, re-verifying every link on the read path and
        /// refusing a store that no longer matches this verified head.
        ///
        /// # Errors
        ///
        /// Returns the typed refusal naming the first violated entry.
        pub fn entries(&self, scope: &PrincipalScope<'_>) -> Result<Vec<ChainEntry>, AuditError> {
            let (entries, head) = load(scope)?;
            if head != self.head() {
                return Err(AuditError::HeadMismatch {
                    expected: self.head(),
                    found: head,
                });
            }
            Ok(entries)
        }

        /// Exports the chain with every referenced evidence row as one bundle
        /// verifiable by [`verify_export`] independently of this plane.
        ///
        /// # Errors
        ///
        /// Returns verification and encoding failures.
        pub fn export(&self, scope: &PrincipalScope<'_>) -> Result<Vec<u8>, AuditError> {
            export::build(scope, self)
        }
    }

    /// Audit chain failures.
    #[derive(Debug)]
    pub enum AuditError {
        Store(StoreError),
        Trace(TraceError),
        Redaction(RedactionError),
        Unhashable,
        ForeignChainKey,
        MissingEntry {
            sequence: u64,
        },
        SequenceMismatch {
            sequence: u64,
        },
        TimestampMismatch {
            sequence: u64,
        },
        LinkMismatch {
            sequence: u64,
        },
        DispositionMismatch {
            sequence: u64,
        },
        EvidenceMismatch {
            sequence: u64,
        },
        EvidenceDigestMismatch {
            sequence: u64,
        },
        HeadMismatch {
            expected: ChainHead,
            found: ChainHead,
        },
        Corrupt(&'static str),
        SizeOverflow,
    }

    impl Display for AuditError {
        fn fmt(&self, formatter: &mut Formatter<'_>) -> std::fmt::Result {
            match self {
                Self::Store(error) => write!(formatter, "audit store failure: {error}"),
                Self::Trace(error) => write!(formatter, "audit trace failure: {error}"),
                Self::Redaction(error) => write!(formatter, "audit redaction failure: {error}"),
                Self::Unhashable => formatter.write_str("audit bytes cannot be hashed"),
                Self::ForeignChainKey => {
                    formatter.write_str("foreign key inside the audit chain namespace")
                }
                Self::MissingEntry { sequence } => {
                    write!(formatter, "audit chain entry {sequence} is missing")
                }
                Self::SequenceMismatch { sequence } => write!(
                    formatter,
                    "audit chain entry at position {sequence} carries another sequence"
                ),
                Self::TimestampMismatch { sequence } => write!(
                    formatter,
                    "audit chain entry {sequence} timestamp does not match its stored write time"
                ),
                Self::LinkMismatch { sequence } => {
                    write!(formatter, "audit chain link broken at entry {sequence}")
                }
                Self::DispositionMismatch { sequence } => write!(
                    formatter,
                    "audit chain entry {sequence} is not exportable evidence"
                ),
                Self::EvidenceMismatch { sequence } => write!(
                    formatter,
                    "audit chain entry {sequence} evidence references do not match its bindings"
                ),
                Self::EvidenceDigestMismatch { sequence } => write!(
                    formatter,
                    "evidence content bound by audit chain entry {sequence} was altered"
                ),
                Self::HeadMismatch { expected, found } => write!(
                    formatter,
                    "audit chain head does not match its anchor: expected {} entries, found {}",
                    expected.length, found.length
                ),
                Self::Corrupt(reason) => write!(formatter, "corrupt audit chain: {reason}"),
                Self::SizeOverflow => formatter.write_str("audit entry exceeds encoding bounds"),
            }
        }
    }

    impl std::error::Error for AuditError {}

    impl From<StoreError> for AuditError {
        fn from(value: StoreError) -> Self {
            Self::Store(value)
        }
    }

    impl From<TraceError> for AuditError {
        fn from(value: TraceError) -> Self {
            Self::Trace(value)
        }
    }

    impl From<RedactionError> for AuditError {
        fn from(value: RedactionError) -> Self {
            Self::Redaction(value)
        }
    }
}
