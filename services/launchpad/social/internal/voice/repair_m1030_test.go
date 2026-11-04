package voice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/jackc/pgx/v5"
)

// This committed store fixture implements the same transaction port as Postgres.
// Reopening reads only committed data, never the old controller's scan state.
type repairM1030Participant struct {
	Wallet, Grant  string
	Revision       int64
	Muted, Revoked bool
}
type repairM1030Room struct {
	Room         Room
	Updated      time.Time
	Participants []repairM1030Participant
}
type repairM1030Recording struct {
	ID, Room, Provider, State string
	Revision                  int64
}
type repairM1030Data struct {
	Rooms      map[string]repairM1030Room
	Recordings map[string]repairM1030Recording
	Events     []string
}
type repairM1030Store struct {
	sync.Mutex
	path   string
	closed bool
	data   repairM1030Data
}

func repairM1030Open(t *testing.T, path string) *repairM1030Store {
	t.Helper()
	s := &repairM1030Store{path: path}
	raw, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		s.data = repairM1030Data{Rooms: make(map[string]repairM1030Room), Recordings: make(map[string]repairM1030Recording)}
	} else if err = json.Unmarshal(raw, &s.data); err != nil {
		t.Fatal(err)
	}
	return s
}
func (s *repairM1030Store) persist(data repairM1030Data) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err = os.WriteFile(s.path+".next", raw, 0600); err != nil {
		return err
	}
	return os.Rename(s.path+".next", s.path)
}
func (s *repairM1030Store) close() error {
	s.Lock()
	defer s.Unlock()
	if err := s.persist(s.data); err != nil {
		return err
	}
	s.closed = true
	return nil
}
func (s *repairM1030Store) roomsAfter(ctx context.Context, after string) ([]string, error) {
	s.Lock()
	defer s.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.closed {
		return nil, errors.New("store closed")
	}
	var ids []string
	for id, r := range s.data.Rooms {
		if id > after && (r.Room.State == "draft" || r.Room.State == "live" || r.Room.State == "ending") {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) > 100 {
		ids = ids[:100]
	}
	return ids, nil
}
func (s *repairM1030Store) beginRoom(ctx context.Context, id string) (reconciliationRoomTx, error) {
	s.Lock()
	if err := ctx.Err(); err != nil {
		s.Unlock()
		return nil, err
	}
	if s.closed {
		s.Unlock()
		return nil, errors.New("store closed")
	}
	r, exists := s.data.Rooms[id]
	r.Participants = append([]repairM1030Participant(nil), r.Participants...)
	return &repairM1030Tx{store: s, id: id, value: r, exists: exists}, nil
}

type repairM1030Tx struct {
	store        *repairM1030Store
	id           string
	value        repairM1030Room
	exists, done bool
}

func (tx *repairM1030Tx) room(ctx context.Context) (Room, time.Time, error) {
	if err := ctx.Err(); err != nil {
		return Room{}, time.Time{}, err
	}
	if !tx.exists {
		return Room{}, time.Time{}, pgx.ErrNoRows
	}
	return tx.value.Room, tx.value.Updated, nil
}
func (tx *repairM1030Tx) adopt(_ context.Context, session string, at time.Time) error {
	tx.value.Room.State = "live"
	tx.value.Room.ProviderSession = session
	tx.value.Room.Revision++
	tx.value.Updated = at
	return nil
}
func (tx *repairM1030Tx) end(_ context.Context, at time.Time) error {
	for i := range tx.value.Participants {
		p := &tx.value.Participants[i]
		if !p.Revoked || p.Grant != "revoked" {
			p.Revoked, p.Muted, p.Grant = true, true, "revoked"
			p.Revision++
		}
	}
	tx.value.Room.State = "ended"
	tx.value.Room.Revision++
	tx.value.Updated = at
	return nil
}
func (tx *repairM1030Tx) pendingGrants(context.Context) ([]reconciliationGrant, error) {
	var grants []reconciliationGrant
	for _, p := range tx.value.Participants {
		if p.Grant == "granting" || p.Grant == "revoking" {
			grants = append(grants, reconciliationGrant{p.Wallet, p.Revision})
		}
	}
	sort.Slice(grants, func(i, j int) bool { return grants[i].wallet < grants[j].wallet })
	if len(grants) > 100 {
		grants = grants[:100]
	}
	return grants, nil
}
func (tx *repairM1030Tx) revokeGrant(_ context.Context, g reconciliationGrant) error {
	for i := range tx.value.Participants {
		p := &tx.value.Participants[i]
		if p.Wallet == g.wallet && p.Revision == g.revision && (p.Grant == "granting" || p.Grant == "revoking") {
			p.Muted, p.Grant = true, "revoked"
		}
	}
	return nil
}
func (tx *repairM1030Tx) Commit(ctx context.Context) error {
	if tx.done {
		return pgx.ErrTxClosed
	}
	defer func() { tx.done = true; tx.store.Unlock() }()
	if err := ctx.Err(); err != nil {
		return err
	}
	previous := tx.store.data.Rooms[tx.id]
	tx.store.data.Rooms[tx.id] = tx.value
	if err := tx.store.persist(tx.store.data); err != nil {
		tx.store.data.Rooms[tx.id] = previous
		return err
	}
	return nil
}
func (tx *repairM1030Tx) Rollback(context.Context) error {
	if tx.done {
		return pgx.ErrTxClosed
	}
	tx.done = true
	tx.store.Unlock()
	return nil
}
func (s *repairM1030Store) recordingsAfter(ctx context.Context, after string) ([]reconciliationRecording, error) {
	s.Lock()
	defer s.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.closed {
		return nil, errors.New("store closed")
	}
	var recordings []reconciliationRecording
	for id, r := range s.data.Recordings {
		room, exists := s.data.Rooms[r.Room]
		if !exists || id <= after {
			continue
		}
		if r.State == "starting" || r.State == "stopping" || (r.State == "recording" && (room.Room.State == "ending" || room.Room.State == "ended")) {
			recordings = append(recordings, reconciliationRecording{id: id, room: r.Room, provider: r.Provider, session: room.Room.ProviderSession, state: r.State, revision: r.Revision, roomState: room.Room.State})
		}
	}
	sort.Slice(recordings, func(i, j int) bool { return recordings[i].id < recordings[j].id })
	if len(recordings) > 100 {
		recordings = recordings[:100]
	}
	return recordings, nil
}
func (s *repairM1030Store) transitionRecording(ctx context.Context, r reconciliationRecording, to, provider string, actor safety.Principal) error {
	s.Lock()
	defer s.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.closed {
		return errors.New("store closed")
	}
	current, exists := s.data.Recordings[r.id]
	if !exists || current.State != r.state || current.Revision != r.revision {
		return safety.ErrConflict
	}
	interrupted := to == "recording" && s.data.Rooms[r.room].Room.State != "live"
	if interrupted {
		to = "stopping"
	}
	next := current
	next.State, next.Provider, next.Revision = to, provider, current.Revision+1
	s.data.Recordings[r.id] = next
	oldEvents := len(s.data.Events)
	s.data.Events = append(s.data.Events, actor.Wallet+":"+r.id+":recording_"+to)
	if err := s.persist(s.data); err != nil {
		s.data.Recordings[r.id] = current
		s.data.Events = s.data.Events[:oldEvents]
		return err
	}
	if interrupted {
		return safety.ErrConflict
	}
	return nil
}

// The loopback peer consumes actual LiveKit requests and keeps provider state
// across controller restarts. It never implements Provider or reconciliation.
type repairM1030LiveKit struct {
	sync.Mutex
	rooms          map[string]bool
	participants   map[string]map[string]bool
	recordings     map[string]string
	recordingRooms map[string]string
	recordingPaths map[string]string
	failedRooms    map[string]bool
	calls          map[string]int
	server         *httptest.Server
}

func repairM1030Provider(t *testing.T) (*LiveKit, *repairM1030LiveKit) {
	t.Helper()
	peer := &repairM1030LiveKit{rooms: map[string]bool{}, participants: map[string]map[string]bool{}, recordings: map[string]string{}, recordingRooms: map[string]string{}, recordingPaths: map[string]string{}, failedRooms: map[string]bool{}, calls: map[string]int{}}
	peer.server = httptest.NewServer(http.HandlerFunc(peer.serveHTTP))
	t.Cleanup(peer.server.Close)
	return &LiveKit{Endpoint: peer.server.URL, APIKey: "committed-loopback-key", APISecret: strings.Repeat("s", 32), Client: peer.server.Client(), RecordingOutput: S3RecordingOutput{Bucket: "restricted", Region: "fixture", AccessKey: "fixture", Secret: "fixture"}}, peer
}
func (p *repairM1030LiveKit) serveHTTP(w http.ResponseWriter, r *http.Request) {
	p.Lock()
	defer p.Unlock()
	if r.Method != http.MethodPost || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	var input struct {
		Names    []string `json:"names"`
		Name     string   `json:"name"`
		Room     string   `json:"room"`
		RoomName string   `json:"room_name"`
		Identity string   `json:"identity"`
		EgressID string   `json:"egress_id"`
		Files    []struct {
			Path string `json:"filepath"`
		} `json:"file_outputs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	room := input.Room
	if input.RoomName != "" {
		room = input.RoomName
	}
	if len(input.Names) > 0 {
		room = input.Names[0]
	}
	if input.EgressID != "" {
		room = p.recordingRooms[input.EgressID]
	}
	method := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	p.calls[method+":"+room]++
	w.Header().Set("Content-Type", "application/json")
	write := func(v any) {
		if err := json.NewEncoder(w).Encode(v); err != nil {
			panic(err)
		}
	}
	if p.failedRooms[room] {
		w.WriteHeader(503)
		write(map[string]string{"code": "unavailable"})
		return
	}
	switch method {
	case "ListRooms":
		rooms := []map[string]string{}
		for _, name := range input.Names {
			if p.rooms[name] {
				rooms = append(rooms, map[string]string{"name": name})
			}
		}
		write(map[string]any{"rooms": rooms})
	case "CreateRoom":
		p.rooms[input.Name] = true
		write(map[string]string{"name": input.Name})
	case "ListParticipants":
		participants := []map[string]string{}
		for wallet := range p.participants[room] {
			participants = append(participants, map[string]string{"identity": wallet})
		}
		write(map[string]any{"participants": participants})
	case "RemoveParticipant":
		delete(p.participants[room], input.Identity)
		write(map[string]any{})
	case "DeleteRoom":
		delete(p.rooms, room)
		write(map[string]any{})
	case "ListEgress":
		items := []map[string]any{}
		for id, status := range p.recordings {
			if (input.EgressID != "" && id == input.EgressID) || (input.EgressID == "" && p.recordingRooms[id] == room) {
				items = append(items, map[string]any{"egress_id": id, "status": status, "room_composite": map[string]any{"file_outputs": []map[string]string{{"filepath": p.recordingPaths[id]}}}})
			}
		}
		write(map[string]any{"items": items})
	case "StartRoomCompositeEgress":
		if len(input.Files) != 1 {
			http.Error(w, "file output required", 400)
			return
		}
		id := "egress-" + strings.TrimSuffix(strings.TrimPrefix(input.Files[0].Path, "restricted/voice/"), ".ogg")
		p.recordings[id] = "EGRESS_ACTIVE"
		p.recordingRooms[id] = room
		p.recordingPaths[id] = input.Files[0].Path
		write(map[string]string{"egress_id": id})
	case "StopEgress":
		if _, ok := p.recordings[input.EgressID]; !ok {
			w.WriteHeader(404)
			write(map[string]string{"code": "not_found"})
			return
		}
		p.recordings[input.EgressID] = "EGRESS_COMPLETE"
		write(map[string]string{"status": "EGRESS_COMPLETE"})
	default:
		http.Error(w, "unexpected provider operation", 400)
	}
}
func (p *repairM1030LiveKit) setFailed(room string, failed bool) {
	p.Lock()
	defer p.Unlock()
	p.failedRooms[room] = failed
}
func (p *repairM1030LiveKit) addParticipant(room, wallet string) {
	p.Lock()
	defer p.Unlock()
	if p.participants[room] == nil {
		p.participants[room] = map[string]bool{}
	}
	p.participants[room][wallet] = true
}
func (p *repairM1030LiveKit) hasRoom(room string) bool {
	p.Lock()
	defer p.Unlock()
	return p.rooms[room]
}
func (p *repairM1030LiveKit) hasParticipant(room, wallet string) bool {
	p.Lock()
	defer p.Unlock()
	return p.participants[room][wallet]
}
func (p *repairM1030LiveKit) recordingStatus(id string) string {
	p.Lock()
	defer p.Unlock()
	return p.recordings[id]
}
func (p *repairM1030LiveKit) callCount(method, room string) int {
	p.Lock()
	defer p.Unlock()
	return p.calls[method+":"+room]
}
func repairM1030ID(n int) string { return fmt.Sprintf("00000000-0000-0000-0000-%012d", n) }
func repairM1030SeedRoom(t *testing.T, s *repairM1030Store, p *LiveKit, n int, state string, at time.Time) string {
	t.Helper()
	id := repairM1030ID(n)
	room := Room{ID: id, StartKey: id, State: state, Revision: 1, HostWallet: host, PoolID: "known-pool"}
	if state != "draft" {
		session, err := p.Start(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		room.ProviderSession = session
	}
	s.data.Rooms[id] = repairM1030Room{Room: room, Updated: at}
	return id
}
func repairM1030SeedRecording(t *testing.T, s *repairM1030Store, p *LiveKit, n int, room string) string {
	t.Helper()
	id := repairM1030ID(n)
	provider, err := p.Record(context.Background(), s.data.Rooms[room].Room.ProviderSession, id)
	if err != nil {
		t.Fatal(err)
	}
	s.data.Recordings[id] = repairM1030Recording{ID: id, Room: room, Provider: provider, State: "stopping", Revision: 1}
	return id
}
func repairM1030Controller(p *LiveKit, now *time.Time) *Control {
	return &Control{Provider: p, Clock: func() time.Time { return *now }, Policy: allowPolicy{}, Gates: Gates{true, true, true, true}}
}

func TestRepairM_1030_RestartFairCleanup(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	old := now.Add(-time.Hour)
	store := repairM1030Open(t, filepath.Join(t.TempDir(), "committed.json"))
	provider, peer := repairM1030Provider(t)
	for n := 1; n <= 120; n++ {
		state := "live"
		if n > 110 {
			state = "draft"
		}
		repairM1030SeedRoom(t, store, provider, n, state, old)
	}
	failed := repairM1030SeedRoom(t, store, provider, 121, "ending", now.Add(-10*time.Minute))
	ending := repairM1030SeedRoom(t, store, provider, 122, "ending", now.Add(-10*time.Minute))
	revoking := repairM1030SeedRoom(t, store, provider, 123, "live", now.Add(-time.Minute))
	r := store.data.Rooms[revoking]
	r.Participants = []repairM1030Participant{{Wallet: host, Grant: "revoking", Revision: 2, Revoked: true, Muted: true}}
	store.data.Rooms[revoking] = r
	peer.addParticipant(r.Room.ProviderSession, host)
	failedRecording := repairM1030SeedRecording(t, store, provider, 1, failed)
	goodRecording := repairM1030SeedRecording(t, store, provider, 2, ending)
	peer.setFailed(store.data.Rooms[failed].Room.ProviderSession, true)
	controller := repairM1030Controller(provider, &now)
	ctx, cancel := context.WithCancel(context.Background())
	first, err := controller.reconcileWithStore(ctx, store)
	if err == nil || first.Examined != 100 || first.Failed != 1 {
		t.Fatalf("first bounded scan: %+v %v", first, err)
	}
	if store.data.Recordings[goodRecording].State != "stopped" {
		t.Fatal("room scan or failed recording blocked recording stop")
	}
	cancel()
	if err = store.close(); err != nil {
		t.Fatal(err)
	}
	controller = nil
	// Open the durable state with a fresh controller and a fresh scan cursor.
	store = repairM1030Open(t, store.path)
	controller = repairM1030Controller(provider, &now)
	first, err = controller.reconcileWithStore(context.Background(), store)
	if first.Examined != 100 || err == nil {
		t.Fatalf("restart first page: %+v %v", first, err)
	}
	second, err := controller.reconcileWithStore(context.Background(), store)
	if err == nil || second.Examined != 23 || second.Failed != 1 || second.Terminated != 1 || second.Revoked != 1 || second.Orphans != 2 {
		t.Fatalf("later cleanup not reached: %+v %v", second, err)
	}
	if !strings.Contains(err.Error(), failed) {
		t.Fatal("failed room identity missing", err)
	}
	if store.data.Rooms[ending].Room.State != "ended" || peer.hasRoom("kindle-"+ending) {
		t.Fatal("end intent/provider acknowledgement not committed")
	}
	if p := store.data.Rooms[revoking].Participants[0]; p.Grant != "revoked" || !p.Muted || peer.hasParticipant("kindle-"+revoking, host) {
		t.Fatal("provider ejection not recovered", p)
	}
	if store.data.Rooms[failed].Room.State != "ending" || store.data.Recordings[failedRecording].State != "stopping" {
		t.Fatal("failed operations lost durable retry intent")
	}
	for n := 1; n <= 120; n++ {
		r := store.data.Rooms[repairM1030ID(n)]
		if !r.Updated.Equal(old) || r.Room.Revision != 1 {
			t.Fatal("scan modified lifecycle timestamp/revision", r)
		}
	}
	// The failed session becomes available; its pending intents survive another restart.
	peer.setFailed("kindle-"+failed, false)
	if err = store.close(); err != nil {
		t.Fatal(err)
	}
	store = repairM1030Open(t, store.path)
	controller = repairM1030Controller(provider, &now)
	for batch := 0; batch < 2; batch++ {
		if _, err = controller.reconcileWithStore(context.Background(), store); err != nil {
			t.Fatal(err)
		}
	}
	if store.data.Rooms[failed].Room.State != "ended" || store.data.Recordings[failedRecording].State != "stopped" {
		t.Fatal("reopened cleanup did not finish")
	}
	if peer.recordingStatus(store.data.Recordings[failedRecording].Provider) != "EGRESS_COMPLETE" {
		t.Fatal("provider recording still active")
	}
	if err = store.close(); err != nil {
		t.Fatal(err)
	}
	recovered := repairM1030Open(t, store.path)
	if recovered.data.Rooms[failed].Room.State != "ended" || len(recovered.data.Events) != 2 {
		t.Fatal("acknowledged cleanup/audit not durable", recovered.data)
	}
}

func TestRepairM_1030_PerRoomRetryAccounting(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	store := repairM1030Open(t, filepath.Join(t.TempDir(), "committed.json"))
	provider, peer := repairM1030Provider(t)
	failed := repairM1030SeedRoom(t, store, provider, 1, "ending", now.Add(-time.Hour))
	healthy := repairM1030SeedRoom(t, store, provider, 2, "live", now.Add(-time.Hour))
	session := store.data.Rooms[failed].Room.ProviderSession
	peer.setFailed(session, true)
	controller := repairM1030Controller(provider, &now)
	ctx := context.Background()
	first, err := controller.reconcileWithStore(ctx, store)
	if err == nil || first.Failed != 1 || first.Retried != 0 || first.Examined != 2 {
		t.Fatal(first, err)
	}
	retry := controller.reconciliation.retries["room:"+failed]
	if retry.attempts != 1 || !retry.next.Equal(now.Add(time.Minute)) {
		t.Fatal("first retry accounting", retry)
	}
	second, err := controller.reconcileWithStore(ctx, store)
	if err == nil || second.Failed != 0 || second.Examined != 1 || peer.callCount("ListParticipants", session) != 1 {
		t.Fatal("backoff did not preserve fair progress", second, err)
	}
	if peer.callCount("ListRooms", "kindle-"+healthy) != 3 {
		t.Fatal("healthy room blocked by retry")
	}
	now = now.Add(time.Minute)
	third, err := controller.reconcileWithStore(ctx, store)
	retry = controller.reconciliation.retries["room:"+failed]
	if err == nil || third.Retried != 1 || third.Failed != 1 || retry.attempts != 2 || !retry.next.Equal(now.Add(2*time.Minute)) {
		t.Fatal("second retry accounting", third, retry, err)
	}
	// Repeated failures cap delay, without touching the lifecycle age used for orphan alarms.
	for attempt := 3; attempt <= 8; attempt++ {
		now = controller.reconciliation.retries["room:"+failed].next
		result, err := controller.reconcileWithStore(ctx, store)
		retry = controller.reconciliation.retries["room:"+failed]
		delay := time.Minute * time.Duration(1<<min(attempt-1, 5))
		if err == nil || result.Retried != 1 || retry.attempts != attempt || !retry.next.Equal(now.Add(delay)) {
			t.Fatal("retry accounting or cap", result, retry, err)
		}
	}
	peer.setFailed(session, false)
	now = retry.next
	recovered, err := controller.reconcileWithStore(ctx, store)
	if err != nil || recovered.Terminated != 1 || recovered.Retried != 1 {
		t.Fatal(recovered, err)
	}
	if len(controller.reconciliation.retries) != 0 {
		t.Fatal("successful retry retained failure state")
	}
	if !store.data.Rooms[healthy].Updated.Equal(time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC)) {
		t.Fatal("scan progress rewrote lifecycle time")
	}
}

func TestRepairM_1030_RecordingCursorRecovery(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	store := repairM1030Open(t, filepath.Join(t.TempDir(), "committed.json"))
	provider, peer := repairM1030Provider(t)
	for n := 1; n <= 100; n++ {
		failed := repairM1030SeedRoom(t, store, provider, n, "live", now.Add(-time.Hour))
		repairM1030SeedRecording(t, store, provider, n, failed)
		peer.setFailed(store.data.Rooms[failed].Room.ProviderSession, true)
	}
	healthy := repairM1030SeedRoom(t, store, provider, 101, "live", now.Add(-time.Hour))
	later := repairM1030SeedRecording(t, store, provider, 101, healthy)
	controller := repairM1030Controller(provider, &now)
	result, err := controller.reconcileWithStore(context.Background(), store)
	if err == nil || result.Failed != 200 || store.data.Recordings[later].State != "stopping" {
		t.Fatal("first bounded recording page", result, err)
	}
	if err = store.close(); err != nil {
		t.Fatal(err)
	}
	store = repairM1030Open(t, store.path)
	controller = repairM1030Controller(provider, &now)
	if result, err = controller.reconcileWithStore(context.Background(), store); err == nil || result.Failed != 200 {
		t.Fatal(result, err)
	}
	if result, err = controller.reconcileWithStore(context.Background(), store); err != nil || result.Failed != 0 {
		t.Fatal("failed page blocked later work", result, err)
	}
	if store.data.Recordings[later].State != "stopped" || peer.recordingStatus(store.data.Recordings[later].Provider) != "EGRESS_COMPLETE" {
		t.Fatal("failed recording page starved newer stop")
	}
	if len(store.data.Events) != 1 {
		t.Fatal("missing or duplicate recording transition", store.data.Events)
	}
	// Removing completed work must not prevent wraparound and retry of earlier IDs.
	for n := 1; n <= 100; n++ {
		peer.setFailed(store.data.Rooms[repairM1030ID(n)].Room.ProviderSession, false)
	}
	now = now.Add(time.Minute)
	result, err = controller.reconcileWithStore(context.Background(), store)
	if err != nil || result.Retried != 200 {
		t.Fatal("cursor wrap failed to retry pending work", result, err)
	}
	for _, recording := range store.data.Recordings {
		if recording.State != "stopped" {
			t.Fatal("recording still pending", recording)
		}
	}
	if err = store.close(); err != nil {
		t.Fatal(err)
	}
	reopened := repairM1030Open(t, store.path)
	if len(reopened.data.Events) != 101 {
		t.Fatal("recording recovery was not committed")
	}
}
