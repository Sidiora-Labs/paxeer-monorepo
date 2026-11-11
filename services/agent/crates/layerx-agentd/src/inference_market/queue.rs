use std::collections::{BTreeMap, VecDeque};
use std::io::{Read, Write};
use std::sync::{Arc, Condvar, Mutex, MutexGuard};
use std::time::{Duration, Instant};

use layerx_client::ai_market::market_program::{policy::DEFAULT_QUEUE_DEPTH, workers};

use super::serve::{AskGate, CompletionBinding, CompletionRequest, ServeError, ServeRoutes};
use super::{DealFrame, PlannedLayers};
use layerx_agentd::node_rpc::NodeRpc;

pub const MAX_SPECULATIVE_SLOTS: usize = 4;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct DraftPeer {
    room: [u8; 32],
    ask: [u8; 32],
    deal: [u8; 32],
    peer_key: [u8; 32],
    manifest_digest: [u8; 32],
}
impl DraftPeer {
    /// Checks the advertised runtime kind against the peer assigned by the live deal.
    ///
    /// # Errors
    /// Returns an error for an invalid manifest, runtime kind or deal binding.
    pub fn from_deal_manifest(
        binding: &CompletionBinding,
        frame: &DealFrame,
        split: &PlannedLayers,
        manifest: &[u8],
        draft_runtime_kind: u8,
    ) -> Result<Option<Self>, ServeError> {
        if draft_runtime_kind > 2 {
            return Err(unavailable());
        }
        let summary = workers::decode_manifest(manifest).map_err(|_| unavailable())?;
        let recorded = frame.request().map_err(|_| unavailable())?;
        if recorded.room_id != binding.room.room_id
            || recorded.deal_id != binding.deal.deal_id
            || recorded.ask_id != binding.deal.ask_id
            || recorded.roster_digest != binding.deal.roster_digest
            || recorded.layers != binding.deal.layers
            || recorded.agreed_price_per_thousand_tokens
                != binding.deal.agreed_price_per_thousand_tokens
            || frame.model_digest != binding.room.model_digest.bytes()
            || frame.runtime_digest != binding.room.runtime_digest.bytes()
            || !frame.plan.layers.contains(split)
            || summary.owner.bytes() != split.owner
            || summary.digest.bytes() != split.manifest_digest
        {
            return Err(unavailable());
        }
        Ok(summary
            .capabilities
            .filter(|fields| fields.runtime_kind == draft_runtime_kind)
            .map(|_| Self {
                room: binding.room.room_id.bytes(),
                ask: binding.deal.ask_id.bytes(),
                deal: binding.deal.deal_id.bytes(),
                peer_key: split.peer_key,
                manifest_digest: summary.digest.bytes(),
            }))
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct SpeculativeSlot {
    pub index: usize,
    pub peer: DraftPeer,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum GenerationSlot {
    Generation,
    Speculative(SpeculativeSlot),
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum QueueError {
    Overflow,
    Unavailable,
    Timeout,
}
impl QueueError {
    /// Writes the queue rejection as an HTTP response.
    ///
    /// # Errors
    /// Returns an error if writing or flushing the response fails.
    pub fn respond(self, stream: &mut impl Write) -> Result<(), ServeError> {
        if self != Self::Overflow {
            return super::serve::write_error(stream, &unavailable());
        }
        let body = br#"{"error":{"type":"rate_limit_error","message":"room_queue_full"}}"#;
        write!(stream, "HTTP/1.1 429 Too Many Requests\r\nContent-Type: application/json\r\nCache-Control: no-store\r\nRetry-After: 1\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).map_err(|_| unavailable())?;
        stream.write_all(body).map_err(|_| unavailable())?;
        stream.flush().map_err(|_| unavailable())
    }
}

fn unavailable() -> ServeError {
    ServeError {
        status: 503,
        code: "completion_queue_unavailable",
    }
}

#[derive(Default)]
struct RoomQueue {
    depth: usize,
    next: u64,
    entries: BTreeMap<u64, Entry>,
    asks: BTreeMap<[u8; 32], VecDeque<u64>>,
    turns: VecDeque<[u8; 32]>,
    last_ask: Option<[u8; 32]>,
    generation: Option<u64>,
    speculative: [Option<u64>; MAX_SPECULATIVE_SLOTS],
}
struct Entry {
    ask: [u8; 32],
    draft: Option<DraftPeer>,
    slot: Option<GenerationSlot>,
}
impl RoomQueue {
    fn pending(&self) -> usize {
        self.asks.values().map(VecDeque::len).sum()
    }

    fn select(&mut self, speculative: bool) -> Option<u64> {
        if self.turns.len() > 1 && self.turns.front().copied() == self.last_ask {
            self.turns.rotate_left(1);
        }
        for _ in 0..self.turns.len() {
            let ask = self.turns.pop_front()?;
            let pending = self.asks.get_mut(&ask)?;
            let id = *pending.front()?;
            let eligible = (!speculative || self.entries.get(&id)?.draft.is_some())
                && !self
                    .entries
                    .values()
                    .any(|entry| entry.ask == ask && entry.slot.is_some());
            if !eligible {
                self.turns.push_back(ask);
                continue;
            }
            pending.pop_front();
            if pending.is_empty() {
                self.asks.remove(&ask);
            } else {
                self.turns.push_back(ask);
            }
            self.last_ask = Some(ask);
            return Some(id);
        }
        None
    }

    fn schedule(&mut self) {
        if self.generation.is_none() {
            if let Some(id) = self.select(false) {
                self.generation = Some(id);
                if let Some(entry) = self.entries.get_mut(&id) {
                    entry.slot = Some(GenerationSlot::Generation);
                }
            }
        }
        if self.generation.is_none() {
            return;
        }
        for index in 0..MAX_SPECULATIVE_SLOTS {
            if self.speculative[index].is_none() {
                if let Some(id) = self.select(true) {
                    if let Some(entry) = self.entries.get_mut(&id) {
                        if let Some(peer) = entry.draft {
                            self.speculative[index] = Some(id);
                            entry.slot =
                                Some(GenerationSlot::Speculative(SpeculativeSlot { index, peer }));
                        }
                    }
                }
            }
        }
    }

    fn remove(&mut self, id: u64) {
        let Some(entry) = self.entries.remove(&id) else {
            return;
        };
        match entry.slot {
            Some(GenerationSlot::Generation) => self.generation = None,
            Some(GenerationSlot::Speculative(slot)) => self.speculative[slot.index] = None,
            None => {
                if let Some(pending) = self.asks.get_mut(&entry.ask) {
                    pending.retain(|queued| *queued != id);
                    if pending.is_empty() {
                        self.asks.remove(&entry.ask);
                        self.turns.retain(|ask| *ask != entry.ask);
                    }
                }
            }
        }
        self.schedule();
    }
}

type Rooms = BTreeMap<[u8; 32], RoomQueue>;

#[derive(Default)]
struct Inner {
    rooms: Mutex<Rooms>,
    changed: Condvar,
}
#[derive(Clone, Default)]
pub struct RequestQueue {
    inner: Arc<Inner>,
}
impl RequestQueue {
    /// Admits a request at the room profile's waiting depth, defaulting to 32.
    ///
    /// # Errors
    /// Returns overflow for a full queue, or unavailable for invalid bindings,
    /// inconsistent room depths, exhausted identifiers or a poisoned queue.
    pub fn enqueue(
        &self,
        room: [u8; 32],
        ask: [u8; 32],
        depth: Option<u16>,
        draft: Option<DraftPeer>,
    ) -> Result<QueueTicket, QueueError> {
        let depth = depth.unwrap_or(DEFAULT_QUEUE_DEPTH);
        if room == [0; 32]
            || ask == [0; 32]
            || depth == 0
            || draft.is_some_and(|peer| peer.room != room || peer.ask != ask)
        {
            return Err(QueueError::Unavailable);
        }
        let mut rooms = self.lock()?;
        let state = rooms.entry(room).or_insert_with(|| RoomQueue {
            depth: usize::from(depth),
            ..RoomQueue::default()
        });
        if state.depth != usize::from(depth) {
            return Err(QueueError::Unavailable);
        }
        let id = state.next;
        state.next = state.next.checked_add(1).ok_or(QueueError::Unavailable)?;
        state.entries.insert(
            id,
            Entry {
                ask,
                draft,
                slot: None,
            },
        );
        if !state.asks.contains_key(&ask) {
            state.turns.push_back(ask);
        }
        state.asks.entry(ask).or_default().push_back(id);
        state.schedule();
        if state.pending() > state.depth {
            state.remove(id);
            return Err(QueueError::Overflow);
        }
        self.inner.changed.notify_all();
        Ok(QueueTicket {
            queue: self.clone(),
            room,
            id,
        })
    }

    fn lock(&self) -> Result<MutexGuard<'_, Rooms>, QueueError> {
        self.inner.rooms.lock().map_err(|_| QueueError::Unavailable)
    }
}

pub struct QueueTicket {
    queue: RequestQueue,
    room: [u8; 32],
    id: u64,
}
impl QueueTicket {
    /// Reads the current assignment without waiting.
    ///
    /// # Errors
    /// Returns unavailable if the queue cannot be read or the ticket is absent.
    #[cfg(test)]
    pub fn slot(&self) -> Result<Option<GenerationSlot>, QueueError> {
        self.queue
            .lock()?
            .get(&self.room)
            .and_then(|room| room.entries.get(&self.id))
            .map(|entry| entry.slot)
            .ok_or(QueueError::Unavailable)
    }

    /// Waits for an assignment, releasing the request if the wait fails.
    ///
    /// # Errors
    /// Returns timeout when the deadline elapses, or unavailable for a poisoned
    /// queue, absent ticket or unrepresentable deadline.
    pub fn wait(self, timeout: Duration) -> Result<GenerationPermit, QueueError> {
        let deadline = Instant::now()
            .checked_add(timeout)
            .ok_or(QueueError::Unavailable)?;
        let slot = {
            let mut rooms = self.queue.lock()?;
            loop {
                let entry = rooms
                    .get(&self.room)
                    .and_then(|room| room.entries.get(&self.id))
                    .ok_or(QueueError::Unavailable)?;
                if let Some(slot) = entry.slot {
                    break slot;
                }
                let remaining = deadline
                    .checked_duration_since(Instant::now())
                    .ok_or(QueueError::Timeout)?;
                let (guard, _) = self
                    .queue
                    .inner
                    .changed
                    .wait_timeout(rooms, remaining)
                    .map_err(|_| QueueError::Unavailable)?;
                rooms = guard;
            }
        };
        Ok(GenerationPermit { ticket: self, slot })
    }
}
impl Drop for QueueTicket {
    fn drop(&mut self) {
        if let Ok(mut rooms) = self.queue.lock() {
            if let Some(room) = rooms.get_mut(&self.room) {
                room.remove(self.id);
                if room.entries.is_empty() {
                    rooms.remove(&self.room);
                }
            }
            self.queue.inner.changed.notify_all();
        }
    }
}

pub struct GenerationPermit {
    ticket: QueueTicket,
    pub slot: GenerationSlot,
}
impl GenerationPermit {
    #[must_use]
    pub fn room(&self) -> [u8; 32] {
        self.ticket.room
    }
}

impl super::DealEngine {
    /// Resolves the live completion deal and its optional draft-capable peer.
    ///
    /// # Errors
    /// Returns an error when the binding, manifest or peer admission is invalid.
    pub fn queue_binding(
        &self,
        ask: &[u8; 32],
        draft_runtime_kind: Option<u8>,
    ) -> Result<(CompletionBinding, Option<DraftPeer>), ServeError> {
        let binding = self.completion_binding(ask)?;
        let mut draft = None;
        if let Some(kind) = draft_runtime_kind {
            let negotiation = self
                .session
                .live
                .iter()
                .find(|deal| deal.frame.ask_id == *ask)
                .ok_or_else(unavailable)?;
            let snapshot = self.snapshot.as_ref().ok_or_else(unavailable)?;
            for split in &negotiation.frame.plan.layers {
                if self.session.excluded.contains(&split.peer_key) {
                    continue;
                }
                let Some(peer) = self.peers.get(&split.peer_key) else {
                    continue;
                };
                self.admitted_peer(snapshot, &split.peer_key, &peer.bytes)
                    .map_err(|_| unavailable())?;
                if let Some(capability) = DraftPeer::from_deal_manifest(
                    &binding,
                    &negotiation.frame,
                    split,
                    &peer.bytes,
                    kind,
                )? {
                    draft = Some(capability);
                    break;
                }
            }
        }
        Ok((binding, draft))
    }
}

pub struct QueuedRoutes {
    queue: RequestQueue,
    routes: Arc<ServeRoutes>,
    node: Arc<NodeRpc>,
    deals: Arc<Mutex<super::DealEngine>>,
    draft_runtime_kind: Option<u8>,
    timeout: Duration,
}
impl QueuedRoutes {
    /// Configures per-room scheduling for completion routes.
    ///
    /// # Errors
    /// Returns an error for an unsupported draft runtime kind or zero timeout.
    pub fn new(
        routes: ServeRoutes,
        node: NodeRpc,
        deals: Arc<Mutex<super::DealEngine>>,
        draft_runtime_kind: Option<u8>,
        timeout: Duration,
    ) -> Result<Self, ServeError> {
        if draft_runtime_kind.is_some_and(|kind| kind > 2) || timeout.is_zero() {
            return Err(unavailable());
        }
        Ok(Self {
            queue: RequestQueue::default(),
            routes: Arc::new(routes),
            node: Arc::new(node),
            deals,
            draft_runtime_kind,
            timeout,
        })
    }

    /// Parses a request using the completion routes' existing HTTP decoder.
    ///
    /// # Errors
    /// Returns the decoder's error for a malformed request or transport failure.
    pub fn read_request(
        &self,
        stream: &mut (impl Read + Write),
    ) -> Result<Option<CompletionRequest>, ServeError> {
        self.routes.read_request(stream)
    }

    /// Queues a funded request and revalidates its live deal before serving it.
    ///
    /// # Errors
    /// Returns an error if a rejection cannot be written or worker handoff fails.
    pub fn enqueue_completion<S: Read + Write + Send + 'static>(
        &self,
        mut stream: S,
        request: CompletionRequest,
    ) -> Result<(), ServeError> {
        if let Err(error) = AskGate::read(&self.node, &request.ask) {
            return super::serve::write_error(&mut stream, &error);
        }
        let admission = self
            .deals
            .lock()
            .map_err(|_| unavailable())
            .and_then(|deals| deals.queue_binding(&request.ask, self.draft_runtime_kind));
        let (binding, draft) = match admission {
            Ok(admission) => admission,
            Err(error) => return super::serve::write_error(&mut stream, &error),
        };
        let ticket = match self.queue.enqueue(
            binding.room.room_id.bytes(),
            request.ask,
            Some(binding.room.profile.queue_depth),
            draft,
        ) {
            Ok(ticket) => ticket,
            Err(error) => return error.respond(&mut stream),
        };
        let node = Arc::clone(&self.node);
        let routes = Arc::clone(&self.routes);
        let deals = Arc::clone(&self.deals);
        let kind = self.draft_runtime_kind;
        let timeout = self.timeout;
        let (send, receive) =
            std::sync::mpsc::sync_channel::<(S, QueueTicket, CompletionRequest)>(1);
        let worker = std::thread::Builder::new().name("inference-completion".to_owned()).spawn(move || {
            let Ok((mut stream, ticket, request)) = receive.recv() else { return; };
            let permit = match ticket.wait(timeout) {
                Ok(permit) => permit,
                Err(error) => { let _ = error.respond(&mut stream); return; }
            };
            let admitted = AskGate::read(&node, &request.ask).and_then(|gate| {
                let (binding, draft) = deals.lock().map_err(|_| unavailable())?.queue_binding(&request.ask, kind)?;
                if binding.room.room_id.bytes() != permit.room()
                    || matches!(permit.slot, GenerationSlot::Speculative(slot) if Some(slot.peer) != draft)
                { return Err(unavailable()); }
                Ok((gate, binding))
            });
            match admitted {
                Ok((gate, binding)) => { let _ = routes.respond(&mut stream, request, gate, binding); }
                Err(error) => { let _ = super::serve::write_error(&mut stream, &error); }
            }
        });
        if worker.is_err() {
            return super::serve::write_error(&mut stream, &unavailable());
        }
        send.send((stream, ticket, request))
            .map_err(|_| unavailable())
    }
}
