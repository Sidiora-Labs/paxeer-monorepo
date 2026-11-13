package social

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/voice"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These peers implement the database/broker wire boundary in process. The
// components being repaired (HTTP adapters) and their durable producer, SQL
// codec, transaction handling, authorization and cursor signing are unchanged.
// The database fixture supports only the queries used by this proof: unknown
// SQL is an error, never a fabricated successful result. Each connection has a
// private transaction snapshot; COMMIT alone publishes its writes. Closing and
// reopening pgx pools/HTTP servers retains only committed peer state.
type durableRoutesConversation struct {
	Kind          string
	Next, Version int64
	Members       map[string]bool
}
type durableRoutesSession struct {
	Wallet  string
	Expires time.Time
	Revoked bool
}
type durableRoutesBan struct {
	Wallet, Scope, Resource string
	Revoked                 bool
}
type durableRoutesParticipant struct {
	Role    string
	Revoked bool
}
type durableRoutesData struct {
	Moderation1029   *moderationRecoveryData
	Conversations    map[string]*durableRoutesConversation
	Messages         map[string]fanout.DurableMessage
	Outbox           map[string][]byte
	Sessions         map[string]durableRoutesSession
	Accounts         map[string]string
	IdentityAccounts map[string]bool
	WalletLinks      map[string]string
	Pools            map[string]string
	Rooms            map[string]voice.Room
	Metadata         map[string][]byte
	Recordings       map[string]voice.Recording
	Participants     map[string]map[string]durableRoutesParticipant
	Bans             map[string]durableRoutesBan
	Follows          map[string]bool
	Inbox            map[string]string
	BansUnavailable  bool
	FollowCountGrant bool
}

func durableRoutesEmptyData() *durableRoutesData {
	return &durableRoutesData{Conversations: map[string]*durableRoutesConversation{}, Messages: map[string]fanout.DurableMessage{}, Outbox: map[string][]byte{}, Sessions: map[string]durableRoutesSession{}, Accounts: map[string]string{}, IdentityAccounts: map[string]bool{}, WalletLinks: map[string]string{}, Pools: map[string]string{}, Rooms: map[string]voice.Room{}, Metadata: map[string][]byte{}, Recordings: map[string]voice.Recording{}, Participants: map[string]map[string]durableRoutesParticipant{}, Bans: map[string]durableRoutesBan{}, Follows: map[string]bool{}, Inbox: map[string]string{}}
}
func (d *durableRoutesData) clone() *durableRoutesData {
	raw, err := json.Marshal(d)
	if err != nil {
		panic(err)
	}
	var copy durableRoutesData
	if err := json.Unmarshal(raw, &copy); err != nil {
		panic(err)
	}
	return &copy
}

type durableRoutesPeer struct {
	mu   sync.Mutex
	data *durableRoutesData
}
type durableRoutesResult struct {
	rows [][]any
	tag  string
	err  error
}

func durableRoutesRows(rows ...[]any) durableRoutesResult {
	return durableRoutesResult{rows: rows, tag: fmt.Sprintf("SELECT %d", len(rows))}
}
func durableRoutesWrite(tag string) durableRoutesResult { return durableRoutesResult{tag: tag} }
func durableRoutesFailure(message string) durableRoutesResult {
	return durableRoutesResult{err: errors.New(message)}
}

func newDurableRoutesDatabase(t *testing.T) (*pgxpool.Pool, func() *pgxpool.Pool) {
	t.Helper()
	return newDurableRoutesDatabaseWithData(t, durableRoutesEmptyData())
}

func newDurableRoutesDatabaseWithData(t *testing.T, data *durableRoutesData) (*pgxpool.Pool, func() *pgxpool.Pool) {
	t.Helper()
	peer := &durableRoutesPeer{data: data}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go peer.serve(conn)
		}
	}()
	open := func() *pgxpool.Pool {
		t.Helper()
		cfg, err := pgxpool.ParseConfig("postgres://repair@" + listener.Addr().String() + "/repair?sslmode=disable")
		if err != nil {
			t.Fatal(err)
		}
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
		cfg.MaxConns = 8
		db, err := pgxpool.NewWithConfig(context.Background(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(db.Close)
		return db
	}
	return open(), open
}

// pgx's Exec mode uses text parameters without a prepared-statement cache.
// Values returned by the peer carry their real PostgreSQL scalar OIDs, so pgx
// performs the same scanning/JSON conversions as against the service database.
func (p *durableRoutesPeer) serve(conn net.Conn) {
	defer conn.Close()
	backend := pgproto3.NewBackend(conn, conn)
	startup, err := backend.ReceiveStartupMessage()
	if err != nil {
		return
	}
	msg, ok := startup.(*pgproto3.StartupMessage)
	if !ok {
		return
	}
	user := msg.Parameters["user"]
	backend.Send(&pgproto3.AuthenticationOk{})
	backend.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "16.0"})
	backend.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
	backend.Send(&pgproto3.ParameterStatus{Name: "standard_conforming_strings", Value: "on"})
	backend.Send(&pgproto3.BackendKeyData{ProcessID: 1, SecretKey: []byte{0, 0, 0, 1}})
	backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if backend.Flush() != nil {
		return
	}
	statements := map[string]string{}
	var sql string
	var args []string
	var result *durableRoutesResult
	var transaction *durableRoutesData
	dirty := false
	failed := false
	evaluate := func(query string, parameters []string) durableRoutesResult {
		p.mu.Lock()
		defer p.mu.Unlock()
		normalized := strings.TrimSpace(query)
		switch {
		case strings.HasPrefix(strings.ToLower(normalized), "begin"):
			if transaction != nil {
				return durableRoutesFailure("nested transaction")
			}
			transaction = p.data.clone()
			dirty = false
			return durableRoutesWrite("BEGIN")
		case strings.EqualFold(normalized, "commit"):
			if transaction == nil {
				return durableRoutesFailure("commit without transaction")
			}
			if dirty {
				p.data = transaction
			}
			transaction = nil
			dirty = false
			return durableRoutesWrite("COMMIT")
		case strings.EqualFold(normalized, "rollback"):
			transaction = nil
			dirty = false
			return durableRoutesWrite("ROLLBACK")
		}
		data := p.data
		if transaction != nil {
			data = transaction
		}
		r := data.execute(normalized, parameters, user)
		if r.err == nil && transaction != nil && (strings.HasPrefix(normalized, "INSERT") || strings.HasPrefix(normalized, "UPDATE") || strings.HasPrefix(normalized, "DELETE")) {
			dirty = true
		}
		return r
	}
	describe := func(r durableRoutesResult) {
		if len(r.rows) == 0 {
			backend.Send(&pgproto3.NoData{})
			return
		}
		fields := make([]pgproto3.FieldDescription, len(r.rows[0]))
		for i, v := range r.rows[0] {
			oid := uint32(25)
			switch v.(type) {
			case bool:
				oid = 16
			case int64, int:
				oid = 20
			case []byte:
				oid = 17
			case time.Time:
				oid = 1184
			}
			fields[i] = pgproto3.FieldDescription{Name: []byte(fmt.Sprintf("column%d", i)), DataTypeOID: oid, DataTypeSize: -1, TypeModifier: -1}
		}
		backend.Send(&pgproto3.RowDescription{Fields: fields})
	}
	emit := func(r durableRoutesResult) {
		if r.err != nil {
			backend.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "42501", Message: r.err.Error()})
			failed = true
			return
		}
		for _, row := range r.rows {
			values := make([][]byte, len(row))
			for i, v := range row {
				if v == nil {
					continue
				}
				switch x := v.(type) {
				case []byte:
					values[i] = []byte(`\x` + hex.EncodeToString(x))
				case bool:
					if x {
						values[i] = []byte("t")
					} else {
						values[i] = []byte("f")
					}
				case time.Time:
					values[i] = []byte(x.UTC().Format("2006-01-02 15:04:05.999999999Z07:00"))
				default:
					values[i] = []byte(fmt.Sprint(x))
				}
			}
			backend.Send(&pgproto3.DataRow{Values: values})
		}
		backend.Send(&pgproto3.CommandComplete{CommandTag: []byte(r.tag)})
	}
	for {
		request, err := backend.Receive()
		if err != nil {
			return
		}
		switch m := request.(type) {
		case *pgproto3.Parse:
			if failed {
				continue
			}
			statements[m.Name] = m.Query
			backend.Send(&pgproto3.ParseComplete{})
		case *pgproto3.Bind:
			if failed {
				continue
			}
			sql = statements[m.PreparedStatement]
			args = make([]string, len(m.Parameters))
			result = nil
			for i, value := range m.Parameters {
				args[i] = string(value)
			}
			backend.Send(&pgproto3.BindComplete{})
		case *pgproto3.Describe:
			if failed {
				continue
			}
			r := evaluate(sql, args)
			result = &r
			if r.err != nil {
				emit(r)
			} else {
				describe(r)
			}
		case *pgproto3.Execute:
			if failed {
				continue
			}
			if result == nil {
				r := evaluate(sql, args)
				result = &r
			}
			emit(*result)
		case *pgproto3.Close:
			backend.Send(&pgproto3.CloseComplete{})
		case *pgproto3.Flush:
			if backend.Flush() != nil {
				return
			}
		case *pgproto3.Sync:
			failed = false
			status := byte('I')
			if transaction != nil {
				status = 'T'
			}
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: status})
			if backend.Flush() != nil {
				return
			}
		case *pgproto3.Query:
			r := evaluate(m.String, nil)
			if r.err == nil {
				describe(r)
			}
			emit(r)
			failed = false
			status := byte('I')
			if transaction != nil {
				status = 'T'
			}
			backend.Send(&pgproto3.ReadyForQuery{TxStatus: status})
			if backend.Flush() != nil {
				return
			}
		case *pgproto3.Terminate:
			return
		default:
			return
		}
	}
}
func durableRoutesNumber(raw string) int64 {
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		panic(err)
	}
	return n
}
func durableRoutesTime(raw string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02 15:04:05.999999999-07:00:00", "2006-01-02 15:04:05.999999999-07"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	panic("invalid fixture time: " + raw)
}
func durableRoutesBytes(raw string) []byte {
	if strings.HasPrefix(raw, `\x`) {
		b, err := hex.DecodeString(raw[2:])
		if err != nil {
			panic(err)
		}
		return b
	}
	return []byte(raw)
}
func durableRoutesMessageRow(m fanout.DurableMessage) []any {
	return []any{m.ID, m.ConversationID, m.Sender, m.Content, m.ReplyTo, m.Sequence, m.Revision, m.CreatedAt, m.Deleted, m.ClientID}
}
func (d *durableRoutesData) execute(q string, a []string, user string) durableRoutesResult {
	// sqlc includes a name comment; strip comments without changing its SQL.
	for strings.HasPrefix(q, "--") {
		i := strings.IndexByte(q, '\n')
		if i < 0 {
			return durableRoutesFailure("SQL comment without query")
		}
		q = strings.TrimSpace(q[i+1:])
	}
	q = strings.Join(strings.Fields(q), " ")
	if d.Moderation1029 != nil {
		if result, handled := d.executeModeration(q, a); handled {
			return result
		}
	}
	get := func(i int) string {
		if i >= len(a) {
			panic(fmt.Sprintf("missing parameter %d: %s", i, q))
		}
		return a[i]
	}
	exists := func(value bool) durableRoutesResult { return durableRoutesRows([]any{value}) }
	member := func(conv, wallet string) bool { c := d.Conversations[conv]; return c != nil && c.Members[wallet] }
	switch {
	case strings.HasPrefix(q, "CREATE ROLE repair_follow_reader"):
		return durableRoutesWrite("CREATE ROLE")
	case q == "GRANT SELECT(followee) ON chat.follows TO repair_follow_reader":
		d.FollowCountGrant = true
		return durableRoutesWrite("GRANT")
	case strings.Contains(q, "FROM chat.follows") || q == "SELECT count(*) FROM follows" || strings.HasPrefix(q, "INSERT INTO chat.follows") || strings.HasPrefix(q, "DELETE FROM chat.follows"):
		if user == "repair_follow_reader" && !(d.FollowCountGrant && strings.Contains(q, "count(*)") && strings.Contains(q, "followee = $1")) {
			return durableRoutesFailure("permission denied for table follows")
		}
		if strings.HasPrefix(q, "INSERT") {
			d.Follows[get(0)+":"+get(1)] = true
			return durableRoutesWrite("INSERT 0 1")
		}
		if strings.HasPrefix(q, "DELETE") {
			delete(d.Follows, get(0)+":"+get(1))
			return durableRoutesWrite("DELETE 1")
		}
		var count int64
		for pair := range d.Follows {
			parts := strings.Split(pair, ":")
			if len(a) == 0 || strings.Contains(q, "followee = $1") && parts[1] == get(0) || strings.Contains(q, "follower = $1") && parts[0] == get(0) {
				count++
			}
		}
		if strings.Contains(q, "EXISTS") {
			return exists(d.Follows[get(0)+":"+get(1)])
		}
		return durableRoutesRows([]any{count})
	case strings.HasPrefix(q, "INSERT INTO account_state_v2"), strings.HasPrefix(q, "INSERT INTO account_state("):
		d.Accounts[get(1)] = get(0)
		return durableRoutesWrite("INSERT 0 1")
	case strings.HasPrefix(q, "INSERT INTO identity_wallet_links_v2"):
		d.WalletLinks[get(1)] = get(0)
		return durableRoutesWrite("INSERT 0 1")
	case strings.HasPrefix(q, "INSERT INTO identity_wallet_links"):
		d.WalletLinks[get(2)] = get(1)
		return durableRoutesWrite("INSERT 0 1")
	case strings.HasPrefix(q, "INSERT INTO identity_accounts"):
		d.IdentityAccounts[get(0)] = true
		return durableRoutesWrite("INSERT 0 1")
	case strings.HasPrefix(q, "INSERT INTO indexer_canonical_creations_v1"):
		d.Pools[get(0)] = get(2)
		return durableRoutesWrite("INSERT 0 1")
	case strings.HasPrefix(q, "INSERT INTO pools VALUES"):
		d.Pools[get(0)] = get(2)
		d.Pools[get(1)] = get(2)
		return durableRoutesWrite("INSERT 0 2")
	case strings.HasPrefix(q, "INSERT INTO identity_sessions"):
		d.Sessions[get(0)] = durableRoutesSession{Wallet: get(2), Expires: durableRoutesTime(get(5))}
		return durableRoutesWrite("INSERT 0 1")
	case strings.HasPrefix(q, "UPDATE identity_sessions SET revoked_at"):
		s := d.Sessions[get(0)]
		s.Revoked = true
		d.Sessions[get(0)] = s
		return durableRoutesWrite("UPDATE 1")
	case strings.HasPrefix(q, "SELECT wallet,session_id::text,access_expires_at FROM identity_sessions"):
		s, ok := d.Sessions[get(0)]
		if !ok {
			return durableRoutesRows()
		}
		return durableRoutesRows([]any{s.Wallet, get(0), s.Expires})
	case strings.HasPrefix(q, `SELECT "chat"."repair_session_live"`):
		s, ok := d.Sessions[get(0)]
		return exists(ok && s.Wallet == get(1) && !s.Revoked && s.Expires.After(time.Now()) && d.Accounts[s.Wallet] != "" && d.WalletLinks[s.Wallet] == d.Accounts[s.Wallet] && d.IdentityAccounts[d.Accounts[s.Wallet]])
	case strings.HasPrefix(q, `SELECT "chat"."repair_account_active"`):
		return exists(d.Accounts[get(0)] != "")
	case strings.HasPrefix(q, `SELECT "chat"."repair_pool_creator"`):
		return durableRoutesRows([]any{d.Pools[get(0)]})
	case strings.HasPrefix(q, "SELECT social_account_for_wallet"):
		id, ok := d.Accounts[get(0)]
		if !ok || d.WalletLinks[get(0)] != id {
			return durableRoutesRows()
		}
		return durableRoutesRows([]any{id})
	case strings.HasPrefix(q, "SELECT EXISTS(SELECT 1 FROM indexer_canonical_creations_v1"):
		return exists(d.Pools[get(0)] != "")
	case strings.HasPrefix(q, "INSERT INTO interaction_preferences"):
		d.Inbox[get(0)] = "everyone"
		return durableRoutesWrite("INSERT 0 1")
	case strings.HasPrefix(q, "SELECT inbox FROM interaction_preferences"):
		v, ok := d.Inbox[get(0)]
		if !ok {
			return durableRoutesRows()
		}
		return durableRoutesRows([]any{v})
	case strings.Contains(q, "FROM interaction_erased_wallets"):
		if strings.Contains(q, "OR NOT EXISTS") {
			return exists(!member(get(1), get(0)))
		}
		return exists(false)
	case strings.Contains(q, "FROM interaction_blocks"):
		return exists(false)
	case strings.HasPrefix(q, "SELECT pg_advisory_xact_lock"):
		return durableRoutesRows([]any{nil})
	case strings.HasPrefix(q, "ALTER TABLE safety_bans RENAME TO unavailable_bans"):
		d.BansUnavailable = true
		return durableRoutesWrite("ALTER TABLE")
	case strings.HasPrefix(q, "INSERT INTO safety_bans"):
		d.Bans[get(0)] = durableRoutesBan{Wallet: get(1), Scope: get(2), Resource: get(3)}
		return durableRoutesWrite("INSERT 0 1")
	case strings.HasPrefix(q, "UPDATE safety_bans SET revoked_at"):
		b := d.Bans[get(0)]
		b.Revoked = true
		d.Bans[get(0)] = b
		return durableRoutesWrite("UPDATE 1")
	case strings.Contains(q, "FROM safety_bans"):
		if d.BansUnavailable {
			return durableRoutesFailure("relation safety_bans does not exist")
		}
		banned := false
		for _, b := range d.Bans {
			if b.Revoked {
				continue
			}
			if strings.Contains(q, "wallet IN") {
				banned = banned || ((b.Wallet == get(0) || b.Wallet == get(1)) && (b.Scope == "global" || b.Scope == "dm" && b.Resource == get(2)))
				continue
			}
			if b.Wallet != get(0) {
				continue
			}
			switch {
			case strings.Contains(q, "scope=$2"):
				banned = banned || b.Scope == "global" || b.Scope == get(1) && b.Resource == get(2)
			case strings.Contains(q, "scope='pool'"):
				banned = banned || b.Scope == "global" || b.Scope == "pool" && b.Resource == get(1)
			default:
				banned = banned || b.Scope == "global"
			}
		}
		return exists(banned)
	case strings.HasPrefix(q, "INSERT INTO voice_rooms"):
		d.Rooms[get(0)] = voice.Room{ID: get(0), PoolID: get(1), HostWallet: get(2), StartKey: get(3), State: "draft", Recording: "off", Revision: 1}
		return durableRoutesWrite("INSERT 0 1")
	case strings.HasPrefix(q, "SELECT id,pool_id,host_wallet,state,provider_session,start_key,recording,revision FROM voice_rooms"):
		r, ok := d.Rooms[get(0)]
		if !ok {
			return durableRoutesRows()
		}
		return durableRoutesRows([]any{r.ID, r.PoolID, r.HostWallet, r.State, r.ProviderSession, r.StartKey, r.Recording, r.Revision})
	case strings.HasPrefix(q, "INSERT INTO safety_outbox") && strings.Contains(q, "'voice_room_created_v2'"):
		d.Metadata[get(1)] = durableRoutesBytes(get(3))
		return durableRoutesWrite("INSERT 0 1")
	case strings.HasPrefix(q, "SELECT payload FROM safety_outbox"):
		b, ok := d.Metadata[get(0)]
		if !ok {
			return durableRoutesRows()
		}
		return durableRoutesRows([]any{b})
	case strings.HasPrefix(q, "INSERT INTO voice_recordings"):
		d.Recordings[get(1)] = voice.Recording{ID: get(0), RoomID: get(1), State: "consent_pending", Purpose: "Private recording purpose", PolicyVersion: "proof-policy", Revision: 1, DeleteAt: time.Now().Add(24 * time.Hour)}
		return durableRoutesWrite("INSERT 0 1")
	case strings.HasPrefix(q, "SELECT id,room_id,state,purpose,policy_version,revision,delete_at FROM voice_recordings"):
		r, ok := d.Recordings[get(0)]
		if !ok {
			return durableRoutesRows()
		}
		return durableRoutesRows([]any{r.ID, r.RoomID, r.State, r.Purpose, r.PolicyVersion, r.Revision, r.DeleteAt})
	case strings.HasPrefix(q, "INSERT INTO voice_participants"):
		if d.Participants[get(0)] == nil {
			d.Participants[get(0)] = map[string]durableRoutesParticipant{}
		}
		d.Participants[get(0)][get(1)] = durableRoutesParticipant{Role: "listener"}
		return durableRoutesWrite("INSERT 0 1")
	case strings.HasPrefix(q, "UPDATE voice_participants SET revoked=true"):
		p := d.Participants[get(0)][get(1)]
		p.Revoked = true
		d.Participants[get(0)][get(1)] = p
		return durableRoutesWrite("UPDATE 1")
	case strings.HasPrefix(q, "SELECT EXISTS(SELECT 1 FROM voice_participants"):
		p, ok := d.Participants[get(0)][get(1)]
		return exists(ok && !p.Revoked)
	case strings.HasPrefix(q, "SELECT wallet,role,muted,revision FROM voice_participants"):
		wallets := []string{}
		for w, p := range d.Participants[get(0)] {
			if !p.Revoked {
				wallets = append(wallets, w)
			}
		}
		sort.Strings(wallets)
		rows := [][]any{}
		for _, w := range wallets {
			if len(rows) == 50 {
				break
			}
			rows = append(rows, []any{w, d.Participants[get(0)][w].Role, false, int64(1)})
		}
		return durableRoutesRows(rows...)
	case strings.HasPrefix(q, "INSERT INTO chat_conversations"):
		id := get(0)
		kind := "pool"
		if len(a) > 1 {
			kind = get(1)
		}
		if d.Conversations[id] != nil {
			return durableRoutesWrite("INSERT 0 0")
		}
		d.Conversations[id] = &durableRoutesConversation{Kind: kind, Version: 1, Members: map[string]bool{}}
		if strings.Contains(q, "RETURNING") {
			return durableRoutesRows([]any{id})
		}
		return durableRoutesWrite("INSERT 0 1")
	case strings.HasPrefix(q, "INSERT INTO chat_members"):
		c := d.Conversations[get(0)]
		if c == nil {
			return durableRoutesFailure("conversation foreign key")
		}
		for _, w := range a[1:] {
			c.Members[w] = true
		}
		return durableRoutesWrite("INSERT 0 1")
	case strings.HasPrefix(q, "SELECT EXISTS(SELECT 1 FROM chat_members"):
		return exists(member(get(0), get(1)))
	case strings.HasPrefix(q, "SELECT wallet FROM chat_members"):
		c := d.Conversations[get(0)]
		if c == nil {
			return durableRoutesRows()
		}
		if strings.Contains(q, "wallet=$2") {
			if c.Members[get(1)] {
				return durableRoutesRows([]any{get(1)})
			}
			return durableRoutesRows()
		}
		wallets := []string{}
		for w := range c.Members {
			if len(a) == 1 || w != get(1) {
				wallets = append(wallets, w)
			}
		}
		sort.Strings(wallets)
		rows := [][]any{}
		for _, w := range wallets {
			rows = append(rows, []any{w})
			if strings.Contains(q, "LIMIT 1") {
				break
			}
		}
		return durableRoutesRows(rows...)
	case strings.HasPrefix(q, "UPDATE chat_conversations SET next_sequence=next_sequence+1"):
		c := d.Conversations[get(0)]
		if c == nil {
			return durableRoutesRows()
		}
		c.Next++
		return durableRoutesRows([]any{c.Next})
	case strings.HasPrefix(q, "UPDATE chat_conversations SET history_version=history_version+1"):
		c := d.Conversations[get(0)]
		if c == nil {
			return durableRoutesFailure("conversation absent")
		}
		c.Version++
		return durableRoutesWrite("UPDATE 1")
	case strings.HasPrefix(q, "SELECT kind FROM chat_conversations"):
		c := d.Conversations[get(0)]
		if c == nil {
			return durableRoutesRows()
		}
		return durableRoutesRows([]any{c.Kind})
	case strings.HasPrefix(q, "SELECT retained_from,kind,history_version,m.read_sequence"):
		c := d.Conversations[get(0)]
		if c == nil || !member(get(0), get(1)) {
			return durableRoutesRows()
		}
		return durableRoutesRows([]any{int64(1), c.Kind, c.Version, int64(0)})
	case strings.HasPrefix(q, "SELECT c.id FROM chat_conversations"):
		ids := []string{}
		for id, c := range d.Conversations {
			if c.Kind == "dm" && c.Members[get(0)] {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		rows := [][]any{}
		for _, id := range ids {
			if int64(len(rows)) == durableRoutesNumber(get(1)) {
				break
			}
			rows = append(rows, []any{id})
		}
		return durableRoutesRows(rows...)
	case strings.HasPrefix(q, "SELECT peer.wallet,extract(epoch from max(msg.created_at))"):
		c := d.Conversations[get(0)]
		if c == nil || !member(get(0), get(1)) {
			return durableRoutesRows()
		}
		peer := ""
		for w := range c.Members {
			if w != get(1) {
				peer = w
			}
		}
		var last any
		var latest int64
		for _, m := range d.Messages {
			if m.ConversationID == get(0) && m.CreatedAt > latest {
				latest = m.CreatedAt
				last = latest
			}
		}
		if peer == "" {
			return durableRoutesRows()
		}
		return durableRoutesRows([]any{peer, last})
	case strings.HasPrefix(q, "INSERT INTO chat_messages"):
		m := fanout.DurableMessage{ID: get(0), ConversationID: get(1), Sender: get(2), ClientID: get(3), Content: get(4), ReplyTo: get(5), Sequence: durableRoutesNumber(get(6)), CreatedAt: durableRoutesTime(get(7)).Unix(), Revision: 1}
		if _, ok := d.Messages[m.ID]; ok {
			return durableRoutesFailure("duplicate message")
		}
		d.Messages[m.ID] = m
		return durableRoutesWrite("INSERT 0 1")
	case strings.HasPrefix(q, "UPDATE chat_messages SET content=$2"):
		m, ok := d.Messages[get(0)]
		if !ok {
			return durableRoutesWrite("UPDATE 0")
		}
		m.Content = get(1)
		m.Revision = durableRoutesNumber(get(2))
		m.Deleted = get(4) == "true" || get(4) == "t"
		if m.Deleted {
			m.ReplyTo = ""
		}
		d.Messages[m.ID] = m
		return durableRoutesWrite("UPDATE 1")
	case strings.HasPrefix(q, "SELECT EXISTS(SELECT 1 FROM chat_messages"):
		m, ok := d.Messages[get(0)]
		return exists(ok && m.ConversationID == get(1))
	case strings.HasPrefix(q, "SELECT id,conversation_id,sender,content,reply_to,sequence,revision,extract(epoch from created_at)::bigint,deleted_at IS NOT NULL,client_id FROM chat_messages"):
		rows := [][]any{}
		switch {
		case strings.Contains(q, "WHERE sender=$1 AND client_id=$2"):
			for _, m := range d.Messages {
				if m.Sender == get(0) && m.ClientID == get(1) {
					rows = append(rows, durableRoutesMessageRow(m))
				}
			}
		case strings.Contains(q, "WHERE conversation_id=$1 AND sequence>$2"):
			messages := []fanout.DurableMessage{}
			for _, m := range d.Messages {
				if m.ConversationID == get(0) && m.Sequence > durableRoutesNumber(get(1)) && member(get(0), get(2)) {
					messages = append(messages, m)
				}
			}
			sort.Slice(messages, func(i, j int) bool { return messages[i].Sequence < messages[j].Sequence })
			for _, m := range messages {
				if int64(len(rows)) == durableRoutesNumber(get(3)) {
					break
				}
				rows = append(rows, durableRoutesMessageRow(m))
			}
		case strings.Contains(q, "WHERE id=$1"):
			m, ok := d.Messages[get(0)]
			if ok && (!strings.Contains(q, "sender=$2") || m.Sender == get(1) && member(m.ConversationID, get(1))) {
				rows = append(rows, durableRoutesMessageRow(m))
			}
		default:
			return durableRoutesFailure("unsupported message selection: " + q)
		}
		return durableRoutesRows(rows...)
	case strings.HasPrefix(q, "INSERT INTO chat_outbox"):
		if _, ok := d.Outbox[get(0)]; ok {
			return durableRoutesFailure("duplicate outbox event")
		}
		d.Outbox[get(0)] = durableRoutesBytes(get(3))
		return durableRoutesWrite("INSERT 0 1")
	case strings.HasPrefix(q, "SELECT payload FROM chat_outbox WHERE message_id=$1 AND revision=1"):
		raw, ok := d.Outbox[get(0)+":1"]
		if !ok {
			return durableRoutesRows()
		}
		return durableRoutesRows([]any{raw})
	case q == "SELECT (SELECT count(*) FROM chat_messages),(SELECT count(*) FROM chat_outbox),(SELECT count(*) FROM pool_messages)+(SELECT count(*) FROM dm_messages)":
		return durableRoutesRows([]any{int64(len(d.Messages)), int64(len(d.Outbox)), int64(0)})
	default:
		return durableRoutesFailure("unsupported fixture SQL: " + q)
	}
}

// The broker peer provides RESP2 subscriptions and bounded-window counters.
// It is deliberately separate from the durable database: a persisted socket
// acknowledgement must round-trip through REST without a live fanout echo.
func newDurableRoutesBroker(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	counters := map[string]int64{}
	expires := map[string]time.Time{}
	connections := map[net.Conn]bool{}
	t.Cleanup(func() {
		listener.Close()
		mu.Lock()
		defer mu.Unlock()
		for conn := range connections {
			conn.Close()
		}
	})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			connections[conn] = true
			mu.Unlock()
			go func() {
				defer conn.Close()
				defer func() { mu.Lock(); delete(connections, conn); mu.Unlock() }()
				reader := bufio.NewReader(conn)
				subscriptions := map[string]bool{}
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					if !strings.HasPrefix(line, "*") {
						return
					}
					count, err := strconv.Atoi(strings.TrimSpace(line[1:]))
					if err != nil {
						return
					}
					args := make([]string, count)
					for i := range args {
						line, err = reader.ReadString('\n')
						if err != nil {
							return
						}
						if !strings.HasPrefix(line, "$") {
							return
						}
						size, err := strconv.Atoi(strings.TrimSpace(line[1:]))
						if err != nil || size < 0 {
							return
						}
						raw := make([]byte, size+2)
						if _, err = io.ReadFull(reader, raw); err != nil {
							return
						}
						args[i] = string(raw[:size])
					}
					if len(args) == 0 {
						return
					}
					switch strings.ToLower(args[0]) {
					case "ping":
						fmt.Fprint(conn, "+PONG\r\n")
					case "incr":
						mu.Lock()
						if until, ok := expires[args[1]]; ok && !until.After(time.Now()) {
							delete(counters, args[1])
							delete(expires, args[1])
						}
						counters[args[1]]++
						n := counters[args[1]]
						mu.Unlock()
						fmt.Fprintf(conn, ":%d\r\n", n)
					case "expire":
						mu.Lock()
						expires[args[1]] = time.Now().Add(time.Duration(durableRoutesNumber(args[2])) * time.Second)
						mu.Unlock()
						fmt.Fprint(conn, ":1\r\n")
					case "subscribe", "unsubscribe":
						operation := strings.ToLower(args[0])
						for _, channel := range args[1:] {
							if operation == "subscribe" {
								subscriptions[channel] = true
							} else {
								delete(subscriptions, channel)
							}
							fmt.Fprintf(conn, "*3\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n:%d\r\n", len(operation), operation, len(channel), channel, len(subscriptions))
						}
					default:
						fmt.Fprint(conn, "-ERR unsupported fixture command\r\n")
					}
				}
			}()
		}
	}()
	return listener.Addr().String()
}

func durableRoutesPolicy(db *pgxpool.Pool) *interactions.PGPolicy {
	return &interactions.PGPolicy{DB: db, Accounts: pgx.Identifier{"chat", "account_state"}, Pools: pgx.Identifier{"chat", "pools"}, Sessions: pgx.Identifier{"chat", "identity_sessions"}, IdentityAccounts: pgx.Identifier{"chat", "identity_accounts"}, WalletLinks: pgx.Identifier{"chat", "identity_wallet_links"}, SessionGuard: pgx.Identifier{"chat", "repair_session_live"}, AccountGuard: pgx.Identifier{"chat", "repair_account_active"}, PoolGuard: pgx.Identifier{"chat", "repair_pool_creator"}}
}

// NewDurableChatFixture shares the production store/policy setup with the
// external-package legacy handler tests, so their new assertions exercise the
// durable producer rather than seeding the retired tables.
func NewDurableChatFixture(t *testing.T, wallets []string, pools map[string]string) (*fanout.DurableStore, fanout.AuthenticateUpgrade, map[string]string) {
	t.Helper()
	db, _ := newDurableRoutesDatabase(t)
	now := time.Now().UTC()
	sessions := map[string]string{}
	for _, wallet := range wallets {
		account, session := uuid.NewString(), uuid.NewString()
		durableRoutesExec(t, db, `INSERT INTO identity_accounts VALUES($1,0,$2)`, account, now)
		durableRoutesExec(t, db, `INSERT INTO account_state(account_id,wallet,profile,preferences) VALUES($1,$2,'{}','{}')`, account, wallet)
		durableRoutesExec(t, db, `INSERT INTO identity_wallet_links VALUES($1,$2,125,$3,'private','verified',$4)`, uuid.NewString(), account, wallet, now)
		durableRoutesExec(t, db, `INSERT INTO identity_sessions(session_id,account_id,wallet,auth_epoch,access_hash,csrf_hash,issued_at,access_expires_at,refresh_expires_at,reauthenticated_at,last_seen_at,device_label) VALUES($1,$2,$3,0,$4,$4,$5,$6,$6,$5,$5,'repair-proof')`, session, account, wallet, []byte("fixture-hash"), now, now.Add(time.Hour))
		durableRoutesExec(t, db, `INSERT INTO interaction_preferences(wallet,inbox) VALUES($1,'everyone')`, wallet)
		sessions[wallet] = session
	}
	for pool, creator := range pools {
		durableRoutesExec(t, db, `INSERT INTO indexer_canonical_creations_v1 VALUES($1,$2,$3)`, pool, "fixture-token", creator)
	}
	policy := durableRoutesPolicy(db)
	authenticate := func(r *http.Request) (fanout.Principal, error) {
		var principal fanout.Principal
		err := db.QueryRow(r.Context(), `SELECT wallet,session_id::text,access_expires_at FROM identity_sessions WHERE session_id=$1::uuid`, r.Header.Get("X-Fixture-Session")).Scan(&principal.Wallet, &principal.SessionID, &principal.ExpiresAt)
		principal.Validate = func(ctx context.Context) error {
			return policy.ValidateSession(ctx, principal.Wallet, principal.SessionID)
		}
		return principal, err
	}
	return &fanout.DurableStore{Pool: db, Policy: policy.SendPolicy, ReadPolicy: policy.ReadChatPolicy, CursorKey: []byte("repair-68-private-cursor-signing-key")}, authenticate, sessions
}
