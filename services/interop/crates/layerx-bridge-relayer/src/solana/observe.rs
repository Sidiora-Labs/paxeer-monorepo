//! The Solana observer: follows the custody program's signatures up to the
//! configured slot depth, reads each deposit's logged record, its receipt PDA
//! and its asset PDA, and turns every deposit into the inbound attestation the
//! Paxeer precompile verifies.
//!
//! A deposit's txHash is keccak256 of the 64-byte signature of the transaction
//! that carried it and its logIndex is the nonce its receipt records. The
//! receipt and the asset record are the program's own state and are
//! authoritative; the logged record must agree with them field for field, and
//! a deposit whose record and receipt disagree is refused, never signed.

use super::journal::{BackfillContinuation, BackfillJournal, SignatureCursor};
use super::rpc::{
    AccountData, Commitment, Instruction, SignatureInfo, SolanaRpc, SolanaTransaction,
};
use super::{base58_fixed, inbound_tx_hash, HANDLE_BYTES};
use crate::attestation::{uint256_from_u64, InboundAttestation};
use crate::hex;
use crate::journal::{Entry, Journal, JournalError, Observation, Position};
use crate::relayer::RelayerError;
use crate::rpc::RpcFault;

/// Every custody instruction starts with this prefix and layout version.
pub const INSTRUCTION_MAGIC: &[u8; 4] = b"PXBR";
pub const INSTRUCTION_VERSION: u16 = 1;
/// The custody program's deposit instruction.
pub const OP_DEPOSIT: u8 = 8;
/// magic, version, op, amount, 32-byte Paxeer recipient.
pub const DEPOSIT_DATA_BYTES: usize = 4 + 2 + 1 + 8 + 32;
/// Positions of the accounts the relayer reads in a deposit instruction.
const DEPOSIT_ASSET_ACCOUNT: usize = 2;
const DEPOSIT_MINT_ACCOUNT: usize = 3;
const DEPOSIT_RECEIPT_ACCOUNT: usize = 6;
const DEPOSIT_ACCOUNTS: usize = 9;

pub const LAYOUT_VERSION: u16 = 1;
pub const RECEIPT_MAGIC: &[u8; 8] = b"PXBRRCP0";
pub const RECEIPT_BYTES: usize = 130;
pub const ASSET_MAGIC: &[u8; 8] = b"PXBRAST0";
pub const ASSET_BYTES: usize = 89;

/// The record a deposit logs, and its prefix in the program's log.
pub const DEPOSIT_RECORD: &str = "PXBR/deposit/v1";
const PROGRAM_LOG: &str = "Program log: ";

/// The page size of every `getSignaturesForAddress` call.
pub const SIGNATURE_PAGE: usize = 1000;

/// Maximum pages (and at most this many times `SIGNATURE_PAGE` transactions) per pass.
pub const MAX_SCAN_PAGES: usize = 2;

/// The reason a deposit whose logged record and receipt disagree is refused.
pub const DISAGREEMENT: &str = "the logged deposit record disagrees with its receipt";

/// One observed Solana custody program.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct SolanaSettings {
    /// [`super::SOLANA_CHAIN_ID`]; the chain the precompile registered.
    pub chain_id: u64,
    /// The handle of the program's vault-authority PDA, as registered.
    pub vault: [u8; HANDLE_BYTES],
    pub program_id: [u8; 32],
    /// How many slots below the head at `commitment` a deposit must be.
    pub finality_depth: u64,
    pub start_slot: u64,
    pub max_slot_range: u64,
    pub commitment: Commitment,
}

/// A deposit's logged record, as the program's `deposit_record` writes it.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct DepositRecord {
    pub nonce: u64,
    pub mint: [u8; 32],
    pub asset: [u8; HANDLE_BYTES],
    pub amount: u64,
    pub recipient: [u8; HANDLE_BYTES],
    pub depositor: [u8; 32],
    pub slot: u64,
}

/// A deposit-receipt PDA, as the program's state layout writes it.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct DepositReceipt {
    pub nonce: u64,
    pub mint: [u8; 32],
    pub amount: u64,
    pub paxeer_recipient: [u8; 32],
    pub depositor: [u8; 32],
    pub slot: u64,
}

/// The fields of an asset PDA the relayer reads.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct AssetRecord {
    pub mint: [u8; 32],
    pub asset_id: [u8; HANDLE_BYTES],
    pub decimals: u8,
}

/// What the observer found for one deposit.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum Finding {
    /// A deposit whose record, receipt and asset agree: attest and submit it.
    Deposit(Observation),
    /// A deposit read from its receipt that must never be attested.
    Refused {
        observation: Observation,
        reason: String,
    },
}

fn malformed() -> RelayerError {
    RelayerError::Rpc(RpcFault::Malformed)
}

fn be_u64(bytes: &[u8]) -> Option<u64> {
    bytes.try_into().ok().map(u64::from_be_bytes)
}

fn be_u16(bytes: &[u8]) -> Option<u16> {
    bytes.try_into().ok().map(u16::from_be_bytes)
}

fn array<const N: usize>(bytes: &[u8]) -> Option<[u8; N]> {
    bytes.try_into().ok()
}

impl DepositReceipt {
    /// Decodes a receipt record; `None` for any other length, magic or
    /// layout version.
    #[must_use]
    pub fn decode(bytes: &[u8]) -> Option<Self> {
        if bytes.len() != RECEIPT_BYTES
            || &bytes[..8] != RECEIPT_MAGIC
            || be_u16(&bytes[8..10])? != LAYOUT_VERSION
        {
            return None;
        }
        Some(Self {
            nonce: be_u64(&bytes[10..18])?,
            mint: array(&bytes[18..50])?,
            amount: be_u64(&bytes[50..58])?,
            paxeer_recipient: array(&bytes[58..90])?,
            depositor: array(&bytes[90..122])?,
            slot: be_u64(&bytes[122..130])?,
        })
    }
}

impl AssetRecord {
    /// Decodes an asset record; `None` for any other length, magic or layout
    /// version.
    #[must_use]
    pub fn decode(bytes: &[u8]) -> Option<Self> {
        if bytes.len() != ASSET_BYTES
            || &bytes[..8] != ASSET_MAGIC
            || be_u16(&bytes[8..10])? != LAYOUT_VERSION
        {
            return None;
        }
        Some(Self {
            mint: array(&bytes[10..42])?,
            asset_id: array(&bytes[42..62])?,
            decimals: bytes[62],
        })
    }
}

/// A 20-byte handle as the program logs it: forty lowercase hex digits, no
/// prefix.
fn logged_handle(text: &str) -> Option<[u8; HANDLE_BYTES]> {
    if !text
        .bytes()
        .all(|digit| digit.is_ascii_digit() || (b'a'..=b'f').contains(&digit))
    {
        return None;
    }
    hex::fixed::<HANDLE_BYTES>(&format!("0x{text}")).ok()
}

impl DepositRecord {
    /// Parses `PXBR/deposit/v1 nonce=… mint=… asset=… amount=… recipient=…
    /// depositor=… slot=…`, every field present once, in that order.
    #[must_use]
    pub fn parse(line: &str) -> Option<Self> {
        let mut words = line.split(' ');
        if words.next()? != DEPOSIT_RECORD {
            return None;
        }
        let mut value = |name: &str| -> Option<&str> {
            words
                .next()?
                .strip_prefix(name)?
                .strip_prefix('=')
                .filter(|value| !value.is_empty())
        };
        let nonce = value("nonce")?.parse().ok()?;
        let mint = base58_fixed::<32>(value("mint")?).ok()?;
        let asset = logged_handle(value("asset")?)?;
        let amount = value("amount")?.parse().ok()?;
        let recipient = logged_handle(value("recipient")?)?;
        let depositor = base58_fixed::<32>(value("depositor")?).ok()?;
        let slot = value("slot")?.parse().ok()?;
        if words.next().is_some() {
            return None;
        }
        Some(Self {
            nonce,
            mint,
            asset,
            amount,
            recipient,
            depositor,
            slot,
        })
    }
}

/// The log lines `program` itself wrote, in order. Solana prefixes every
/// program's `msg!` output identically, so a line is attributed to the program
/// on top of the invocation stack the `invoke` / `success` / `failed` lines
/// describe; another program cannot write a line that counts as the custody
/// program's. A truncated or unbalanced log is malformed.
///
/// # Errors
///
/// Returns `Malformed` for a truncated log or an unbalanced invocation stack.
pub fn program_log_lines<'a>(
    logs: &'a [String],
    program: &[u8; 32],
) -> Result<Vec<&'a str>, RelayerError> {
    let invocations = super::rpc::invocation_logs(logs).map_err(RelayerError::Rpc)?;
    // Return slices of the original log in execution order, after rollback
    // outcomes for the entire invocation tree have been established.
    let mut stack = Vec::new();
    let mut next = 0;
    let mut lines = Vec::new();
    for line in logs {
        if let Some(message) = line.strip_prefix(PROGRAM_LOG) {
            if stack.last().is_some_and(|index: &usize| {
                invocations[*index].program == *program && invocations[*index].committed
            }) {
                lines.push(message);
            }
        } else if let Some((_, event)) = line
            .strip_prefix("Program ")
            .and_then(|rest| rest.split_once(' '))
        {
            if event.starts_with("invoke [") {
                stack.push(next);
                next += 1;
            } else if event == "success" || event.starts_with("failed: ") {
                stack.pop();
            }
        }
    }
    Ok(lines)
}

/// The deposit records `program` logged, in execution order.
///
/// # Errors
///
/// Returns `Malformed` for an unreadable log or a record line that does not
/// parse.
pub fn deposit_records(
    logs: &[String],
    program: &[u8; 32],
) -> Result<Vec<DepositRecord>, RelayerError> {
    program_log_lines(logs, program)?
        .into_iter()
        .filter(|line| line.split(' ').next() == Some(DEPOSIT_RECORD))
        .map(|line| DepositRecord::parse(line).ok_or_else(malformed))
        .collect()
}

/// The amount and 32-byte recipient of a deposit instruction's data, or
/// `None` when the data is not a deposit of this layout version.
#[must_use]
pub fn deposit_instruction(data: &[u8]) -> Option<(u64, [u8; 32])> {
    if data.len() != DEPOSIT_DATA_BYTES
        || &data[..4] != INSTRUCTION_MAGIC
        || be_u16(&data[4..6])? != INSTRUCTION_VERSION
        || data[6] != OP_DEPOSIT
    {
        return None;
    }
    Some((be_u64(&data[7..15])?, array(&data[15..47])?))
}

fn padded(address: &[u8; HANDLE_BYTES]) -> [u8; 32] {
    let mut word = [0_u8; 32];
    word[32 - HANDLE_BYTES..].copy_from_slice(address);
    word
}

fn program_account(
    rpc: &SolanaRpc,
    settings: &SolanaSettings,
    address: &[u8; 32],
) -> Result<AccountData, RelayerError> {
    let account = rpc
        .get_account_info(address, settings.commitment)?
        .ok_or_else(malformed)?;
    if account.owner != settings.program_id {
        return Err(malformed());
    }
    Ok(account)
}

struct DepositCall<'a> {
    instruction: &'a Instruction,
    amount: u64,
    recipient: [u8; 32],
}

/// Whether the record, the receipt, the asset and the instruction describe
/// one deposit in one slot.
fn agrees(
    record: &DepositRecord,
    receipt: &DepositReceipt,
    asset: &AssetRecord,
    call: &DepositCall<'_>,
    slot: u64,
) -> bool {
    let mint = call.instruction.accounts[DEPOSIT_MINT_ACCOUNT];
    record.nonce == receipt.nonce
        && record.mint == receipt.mint
        && record.amount == receipt.amount
        && padded(&record.recipient) == receipt.paxeer_recipient
        && record.depositor == receipt.depositor
        && record.slot == receipt.slot
        && record.asset == asset.asset_id
        && asset.mint == receipt.mint
        && mint == receipt.mint
        && call.amount == receipt.amount
        && call.recipient == receipt.paxeer_recipient
        && receipt.slot == slot
}

/// Every deposit one successful transaction carried.
fn transaction_findings(
    rpc: &SolanaRpc,
    settings: &SolanaSettings,
    info: &SignatureInfo,
    transaction: &SolanaTransaction,
) -> Result<Vec<Finding>, RelayerError> {
    if transaction.slot != info.slot
        || transaction.failed
        || transaction.signatures.first() != Some(&info.signature)
    {
        return Err(malformed());
    }
    let mut calls = Vec::new();
    for (index, instruction) in transaction.instructions.iter().enumerate() {
        if instruction.program != settings.program_id {
            continue;
        }
        let Some((amount, recipient)) = deposit_instruction(&instruction.data) else {
            continue;
        };
        let invocation = transaction
            .invocations
            .iter()
            .find(|invocation| invocation.instruction_index == Some(index))
            .ok_or_else(malformed)?;
        if !invocation.committed {
            continue;
        }
        if instruction.accounts.len() != DEPOSIT_ACCOUNTS {
            return Err(malformed());
        }
        let records = invocation
            .log_messages
            .iter()
            .filter(|line| line.split(' ').next() == Some(DEPOSIT_RECORD))
            .map(|line| DepositRecord::parse(line).ok_or_else(malformed))
            .collect::<Result<Vec<_>, _>>()?;
        // Success requires its own single producer record; a sibling cannot
        // supply evidence for this call.
        if records.len() != 1 {
            return Err(malformed());
        }
        calls.push((
            DepositCall {
                instruction,
                amount,
                recipient,
            },
            records[0],
        ));
    }
    let tx_hash = inbound_tx_hash(&info.signature);
    let mut findings = Vec::with_capacity(calls.len());
    for (call, record) in &calls {
        let receipt = DepositReceipt::decode(
            &program_account(
                rpc,
                settings,
                &call.instruction.accounts[DEPOSIT_RECEIPT_ACCOUNT],
            )?
            .data,
        )
        .ok_or_else(malformed)?;
        let asset = AssetRecord::decode(
            &program_account(
                rpc,
                settings,
                &call.instruction.accounts[DEPOSIT_ASSET_ACCOUNT],
            )?
            .data,
        )
        .ok_or_else(malformed)?;
        let observation = Observation::inbound(
            &InboundAttestation {
                chain_id: settings.chain_id,
                vault: settings.vault,
                tx_hash,
                log_index: receipt.nonce,
                recipient: receipt.paxeer_recipient,
                asset: asset.asset_id,
                amount: uint256_from_u64(receipt.amount),
            },
            // Solana has no block hash at a slot in the transaction answer; the
            // position pins the slot and the blockhash the transaction was
            // built against, which a rescan reads back identically.
            Position {
                block_number: transaction.slot,
                block_hash: transaction.recent_blockhash,
            },
        );
        let consistent = agrees(record, &receipt, &asset, call, transaction.slot);
        findings.push(if consistent {
            Finding::Deposit(observation)
        } else {
            Finding::Refused {
                observation,
                reason: DISAGREEMENT.to_owned(),
            }
        });
    }
    Ok(findings)
}

struct SignaturePage {
    signatures: Vec<SignatureInfo>,
    before: Option<SignatureCursor>,
    complete: bool,
}

fn signature_page(
    rpc: &SolanaRpc,
    settings: &SolanaSettings,
    window: &BackfillContinuation,
) -> Result<SignaturePage, RelayerError> {
    let page = rpc.get_signatures_for_address(
        &settings.program_id,
        window.before.as_ref().map(|before| &before.signature),
        SIGNATURE_PAGE,
        settings.commitment,
    )?;
    let mut previous_slot = window.before.map_or(u64::MAX, |before| before.slot);
    let mut seen = std::collections::BTreeSet::new();
    let mut found = Vec::new();
    let mut reached_start = false;
    for info in &page {
        if info.slot > previous_slot
            || window
                .before
                .is_some_and(|before| before.signature == info.signature)
            || !seen.insert(info.signature)
        {
            return Err(malformed());
        }
        previous_slot = info.slot;
        if info.slot < window.from {
            reached_start = true;
        } else if info.slot <= window.to && !info.failed {
            found.push(*info);
        }
    }
    Ok(SignaturePage {
        signatures: found,
        before: page.last().map(|last| SignatureCursor {
            signature: last.signature,
            slot: last.slot,
        }),
        complete: reached_start || page.len() < SIGNATURE_PAGE,
    })
}

/// Compatibility scan for a complete small window. An incomplete budget is an
/// explicit error: callers must use `observe_deposits_pass` to resume durably.
fn signatures_in(
    rpc: &SolanaRpc,
    settings: &SolanaSettings,
    from: u64,
    to: u64,
) -> Result<Vec<SignatureInfo>, RelayerError> {
    let mut window = BackfillContinuation::new(settings, from, to);
    let mut found = Vec::new();
    for _ in 0..MAX_SCAN_PAGES {
        let page = signature_page(rpc, settings, &window)?;
        found.extend(page.signatures);
        if page.complete {
            found.reverse();
            return Ok(found);
        }
        window.before = page.before;
    }
    Err(RelayerError::Configuration(
        "Solana scan budget exhausted; use journaled observe_deposits_pass".to_owned(),
    ))
}

#[derive(Clone, Copy, Debug, Default, Eq, PartialEq)]
pub struct ScanProgress {
    pub pages: usize,
    pub transactions: usize,
    pub observed: usize,
    pub refused: usize,
    pub complete: bool,
}

fn record_finding(
    journal: &mut Journal,
    progress: &mut ScanProgress,
    finding: Finding,
) -> Result<(), RelayerError> {
    let (observation, refusal) = match finding {
        Finding::Deposit(observation) => (observation, None),
        Finding::Refused {
            observation,
            reason,
        } => (observation, Some(reason)),
    };
    let item = observation.key();
    if let Some(existing) = journal.lookup(&item)? {
        if existing.observation != observation {
            return Err(
                JournalError::Conflict(format!("{item} observed with different content")).into(),
            );
        }
    } else {
        journal.append(&Entry::Observed {
            item: item.clone(),
            observation,
        })?;
        progress.observed += 1;
    }
    if let Some(reason) = refusal {
        if journal
            .lookup(&item)?
            .is_some_and(|record| record.refusal.is_none())
        {
            journal.append(&Entry::Refused { item, reason })?;
            progress.refused += 1;
        }
    }
    Ok(())
}

/// Runs one bounded backfill pass. Findings are synced to the relay journal
/// before the pagination checkpoint moves; the slot cursor moves only when the
/// entire fixed window is covered. Replaying a page after a crash is idempotent.
/// The caller supplies an exclusively owned checkpoint beside its relay journal.
///
/// # Errors
/// Returns RPC, evidence or journal faults without skipping unrecorded history.
pub fn observe_deposits_pass(
    rpc: &SolanaRpc,
    settings: &SolanaSettings,
    journal: &mut Journal,
    backfill: &mut BackfillJournal,
) -> Result<ScanProgress, RelayerError> {
    if settings.max_slot_range == 0 {
        return Err(RelayerError::Configuration(
            "Solana slot range is zero".to_owned(),
        ));
    }
    let stream = format!("in:{}", settings.chain_id);
    let from = journal
        .state()
        .cursors
        .get(&stream)
        .copied()
        .unwrap_or(settings.start_slot);
    if let Some(pending) = backfill.pending() {
        pending.validate(settings)?;
        if from == pending.to + 1 {
            // The cursor synced just before a crash while clearing the checkpoint.
            backfill.save(None)?;
            return Ok(ScanProgress {
                complete: true,
                ..ScanProgress::default()
            });
        }
        if from != pending.from {
            return Err(JournalError::Conflict(
                "Solana cursor disagrees with backfill window".to_owned(),
            )
            .into());
        }
    } else {
        let head = rpc.get_slot(settings.commitment)?;
        let Some(safe) = head.checked_sub(settings.finality_depth) else {
            return Ok(ScanProgress::default());
        };
        if from > safe {
            return Ok(ScanProgress::default());
        }
        let to = from
            .saturating_add(settings.max_slot_range - 1)
            .min(safe)
            .min(u64::MAX - 1);
        if from > to {
            return Err(RelayerError::Configuration(
                "Solana slot cursor overflow".to_owned(),
            ));
        }
        backfill.save(Some(BackfillContinuation::new(settings, from, to)))?;
    }
    let mut progress = ScanProgress::default();
    for _ in 0..MAX_SCAN_PAGES {
        let mut window = backfill.pending().cloned().ok_or_else(|| {
            JournalError::Conflict("Solana backfill window is missing".to_owned())
        })?;
        let page = signature_page(rpc, settings, &window)?;
        progress.pages += 1;
        // Keep only one bounded page of signatures and one transaction's findings.
        for info in page.signatures.iter().rev() {
            let transaction = rpc
                .get_transaction(&info.signature, settings.commitment)?
                .ok_or(RelayerError::Rpc(RpcFault::Unavailable))?;
            progress.transactions += 1;
            for finding in transaction_findings(rpc, settings, info, &transaction)? {
                record_finding(journal, &mut progress, finding)?;
            }
        }
        if page.complete {
            journal.append(&Entry::Cursor {
                stream,
                next_block: window.to + 1,
            })?;
            backfill.save(None)?;
            progress.complete = true;
            break;
        }
        window.before = page.before;
        backfill.save(Some(window))?;
    }
    Ok(progress)
}

/// Observes every deposit into the custody program in slots `from..=to`.
/// The caller bounds `to` by the head at the configured commitment minus the
/// slot finality depth, and a transaction the cluster places above `to` is
/// never read, so only deposits confirmed to that depth are observed.
///
/// # Errors
///
/// Returns the first RPC fault, or `Malformed` for a transaction, receipt or
/// asset record that cannot be read as the program writes it; the scan is
/// retried from the same slot on the next pass. Windows exceeding the page
/// budget require the persisted `observe_deposits_pass` API.
pub fn observe_deposits(
    rpc: &SolanaRpc,
    settings: &SolanaSettings,
    from: u64,
    to: u64,
) -> Result<Vec<Finding>, RelayerError> {
    if from > to {
        return Ok(Vec::new());
    }
    let mut findings = Vec::new();
    for info in signatures_in(rpc, settings, from, to)? {
        let transaction = rpc
            .get_transaction(&info.signature, settings.commitment)?
            .ok_or(RelayerError::Rpc(RpcFault::Unavailable))?;
        findings.extend(transaction_findings(rpc, settings, &info, &transaction)?);
    }
    Ok(findings)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::solana::base58_encode;

    const PROGRAM: &str = "A7SZbByPYuHpunZ9pyMDMrhMYvK44ANT1AVqb8U1FpM9";
    const TOKEN: &str = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA";
    const RECORD: &str = "PXBR/deposit/v1 nonce=7 mint=5w3wVdJaESaJKyLmStM6Hv9UyUkmZ1b9DLQquAqqpump asset=21f7b20a555199fa73a238b1a91fd0f549068fee amount=12345678 recipient=b65aa00b0baa8fe2abc2d188312fb0a6d00e3ed5 depositor=4XijNxiwmXKJFacayVqi3WD66fnzEDgvGpMD57dyQyBb slot=1050";

    fn program() -> [u8; 32] {
        base58_fixed::<32>(PROGRAM).unwrap_or_else(|error| panic!("program: {error}"))
    }

    fn lines(lines: &[&str]) -> Vec<String> {
        lines.iter().map(|line| (*line).to_owned()).collect()
    }

    #[test]
    fn the_record_parses_field_for_field() {
        let record = DepositRecord::parse(RECORD).unwrap_or_else(|| panic!("record"));
        assert_eq!(record.nonce, 7);
        assert_eq!(record.amount, 12_345_678);
        assert_eq!(record.slot, 1050);
        assert_eq!(
            hex::prefixed(&record.asset),
            "0x21f7b20a555199fa73a238b1a91fd0f549068fee"
        );
        assert_eq!(
            hex::prefixed(&record.recipient),
            "0xb65aa00b0baa8fe2abc2d188312fb0a6d00e3ed5"
        );
        assert_eq!(
            base58_encode(&record.mint),
            "5w3wVdJaESaJKyLmStM6Hv9UyUkmZ1b9DLQquAqqpump"
        );
    }

    #[test]
    fn a_record_with_a_missing_extra_or_reordered_field_does_not_parse() {
        assert_eq!(
            DepositRecord::parse(&RECORD.replace(" slot=1050", "")),
            None
        );
        assert_eq!(DepositRecord::parse(&format!("{RECORD} extra=1")), None);
        assert_eq!(
            DepositRecord::parse(&RECORD.replace("nonce=7 mint=", "mint=")),
            None
        );
        assert_eq!(
            DepositRecord::parse(&RECORD.replace("amount=12345678", "amount=")),
            None
        );
        assert_eq!(
            DepositRecord::parse(&RECORD.replace("PXBR/deposit/v1", "PXBR/deposit/v2")),
            None
        );
        assert_eq!(
            DepositRecord::parse(&RECORD.replace("asset=21f7", "asset=21F7")),
            None
        );
        assert_eq!(
            DepositRecord::parse(&RECORD.replace("asset=21f7", "asset=0x21f7")),
            None
        );
    }

    #[test]
    fn only_lines_the_program_itself_wrote_are_its_records() {
        let spoof = format!("Program log: {RECORD}");
        let logs = lines(&[
            &format!("Program {TOKEN} invoke [1]"),
            &spoof,
            &format!("Program {TOKEN} success"),
            &format!("Program {PROGRAM} invoke [1]"),
            &format!("Program {TOKEN} invoke [2]"),
            &spoof,
            &format!("Program {TOKEN} success"),
            &spoof,
            &format!("Program {PROGRAM} consumed 41000 of 200000 compute units"),
            &format!("Program {PROGRAM} success"),
        ]);
        let records =
            deposit_records(&logs, &program()).unwrap_or_else(|error| panic!("records: {error}"));
        assert_eq!(records.len(), 1);
        assert_eq!(records[0].nonce, 7);
    }

    #[test]
    fn a_truncated_or_unbalanced_log_is_malformed() {
        let truncated = lines(&[&format!("Program {PROGRAM} invoke [1]"), "Log truncated"]);
        assert_eq!(
            program_log_lines(&truncated, &program()),
            Err(RelayerError::Rpc(RpcFault::Malformed))
        );
        let unbalanced = lines(&[
            &format!("Program {PROGRAM} invoke [1]"),
            &format!("Program {TOKEN} success"),
        ]);
        assert_eq!(
            program_log_lines(&unbalanced, &program()),
            Err(RelayerError::Rpc(RpcFault::Malformed))
        );
        let open = lines(&[&format!("Program {PROGRAM} invoke [1]")]);
        assert_eq!(
            program_log_lines(&open, &program()),
            Err(RelayerError::Rpc(RpcFault::Malformed))
        );
    }

    #[test]
    fn deposit_instruction_data_is_read_only_in_its_own_layout() {
        let mut data = b"PXBR".to_vec();
        data.extend_from_slice(&[0, 1, OP_DEPOSIT]);
        data.extend_from_slice(&600_u64.to_be_bytes());
        data.extend_from_slice(&[0x55; 32]);
        assert_eq!(deposit_instruction(&data), Some((600, [0x55; 32])));
        let mut other = data.clone();
        other[6] = 9;
        assert_eq!(deposit_instruction(&other), None);
        let mut version = data.clone();
        version[5] = 2;
        assert_eq!(deposit_instruction(&version), None);
        assert_eq!(deposit_instruction(&data[..46]), None);
    }

    #[test]
    fn receipts_and_assets_decode_only_their_own_layout() {
        let mut receipt = RECEIPT_MAGIC.to_vec();
        receipt.extend_from_slice(&[0, 1]);
        receipt.extend_from_slice(&7_u64.to_be_bytes());
        receipt.extend_from_slice(&[0x11; 32]);
        receipt.extend_from_slice(&12_345_678_u64.to_be_bytes());
        receipt.extend_from_slice(&[0x22; 32]);
        receipt.extend_from_slice(&[0x33; 32]);
        receipt.extend_from_slice(&1050_u64.to_be_bytes());
        let decoded = DepositReceipt::decode(&receipt).unwrap_or_else(|| panic!("receipt"));
        assert_eq!(
            decoded,
            DepositReceipt {
                nonce: 7,
                mint: [0x11; 32],
                amount: 12_345_678,
                paxeer_recipient: [0x22; 32],
                depositor: [0x33; 32],
                slot: 1050,
            }
        );
        assert_eq!(AssetRecord::decode(&receipt), None);
        let mut version = receipt.clone();
        version[9] = 2;
        assert_eq!(DepositReceipt::decode(&version), None);
        assert_eq!(DepositReceipt::decode(&receipt[..129]), None);

        let mut asset = ASSET_MAGIC.to_vec();
        asset.extend_from_slice(&[0, 1]);
        asset.extend_from_slice(&[0x11; 32]);
        asset.extend_from_slice(&[0x21; 20]);
        asset.extend_from_slice(&[6, 1, 254]);
        asset.extend_from_slice(&[0; 24]);
        assert_eq!(
            AssetRecord::decode(&asset),
            Some(AssetRecord {
                mint: [0x11; 32],
                asset_id: [0x21; 20],
                decimals: 6,
            })
        );
        assert_eq!(DepositReceipt::decode(&asset), None);
    }
}
