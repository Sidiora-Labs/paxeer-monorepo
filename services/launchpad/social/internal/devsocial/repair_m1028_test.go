package devsocial

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/social"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

const repairMA = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const repairMB = "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
const repairMC = "0xcccccccccccccccccccccccccccccccccccccccc"

// The database peer is a transactional in-memory relation store on the pgwire
// boundary. Production pgx, SQL producers, policies, codecs, handlers and socket
// pumps run unchanged. Unsupported SQL fails rather than returning canned rows.
type repairMPreference struct {
	Inbox    string
	Revision int64
}
type repairMBan struct {
	Wallet, Scope, Resource string
	Expires                 int64
	Revoked                 bool
}
type repairMConversation struct {
	Kind    string
	Members []string
	Next    int64
}
type repairMReport struct {
	Report                     v2.Report
	Hash, ReasonText, Evidence string
}
type repairMCase struct {
	ID, Kind, Target, Pool, Affected, Reporter, Category, Description, State string
	Revision                                                                 int64
}
type repairMState struct {
	Preferences      map[string]repairMPreference
	Active, Erased   map[string]bool
	Sessions         map[string]string
	Blocks, Follows  map[string]bool
	Bans             []repairMBan
	Conversations    map[string]repairMConversation
	Messages         map[string]fanout.DurableMessage
	ChatOutbox       map[string]string
	Reports          map[string]repairMReport
	Cases            map[string]repairMCase
	Comments, Tokens map[string][]string
	Actions          map[string][]string
	SafetyOutbox     map[string]string
}
type repairMDatabase struct {
	mu           sync.Mutex
	state        *repairMState
	listener     net.Listener
	failures     []string
	failRelation string
	redis        *repairMRedis
}

func repairMNewDatabase(t *testing.T) *repairMDatabase {
	t.Helper()
	s := &repairMState{
		Preferences: map[string]repairMPreference{}, Active: map[string]bool{}, Erased: map[string]bool{}, Sessions: map[string]string{},
		Blocks: map[string]bool{}, Follows: map[string]bool{}, Conversations: map[string]repairMConversation{}, Messages: map[string]fanout.DurableMessage{},
		ChatOutbox: map[string]string{}, Reports: map[string]repairMReport{}, Cases: map[string]repairMCase{}, Comments: map[string][]string{}, Tokens: map[string][]string{}, Actions: map[string][]string{}, SafetyOutbox: map[string]string{},
	}
	for _, wallet := range []string{repairMA, repairMB, repairMC} {
		s.Preferences[wallet] = repairMPreference{"everyone", 1}
		s.Active[wallet] = true
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	db := &repairMDatabase{state: s, listener: listener, redis: newRepairMRedis(t)}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go db.serve(conn)
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		db.mu.Lock()
		defer db.mu.Unlock()
		if len(db.failures) != 0 {
			t.Errorf("database peer rejected SQL: %v", db.failures)
		}
	})
	return db
}
func repairMSession(i int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1) }
func repairMClone(s *repairMState) *repairMState {
	raw, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	var out repairMState
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	return &out
}
func (db *repairMDatabase) snapshot() *repairMState {
	db.mu.Lock()
	defer db.mu.Unlock()
	return repairMClone(db.state)
}
func (db *repairMDatabase) change(f func(*repairMState)) {
	db.mu.Lock()
	defer db.mu.Unlock()
	f(db.state)
}
func (db *repairMDatabase) open(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig("postgres://fixture@" + db.listener.Addr().String() + "/fixture?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	cfg.AfterConnect = func(_ context.Context, conn *pgx.Conn) error {
		// Every byte-slice parameter in these paths is JSON. Match the JSONB
		// parameter type PostgreSQL infers for the production prepared queries.
		conn.TypeMap().RegisterDefaultPgType([]byte{}, "jsonb")
		return nil
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	return pool
}

type repairMRows struct {
	types  string
	values [][]string
	tag    string
}

func repairMResult(types string, rows ...[]any) repairMRows {
	r := repairMRows{types: types, tag: fmt.Sprintf("SELECT %d", len(rows))}
	for _, row := range rows {
		if len(row) != len(types) {
			panic("fixture column count")
		}
		values := make([]string, len(row))
		for i, v := range row {
			values[i] = fmt.Sprint(v)
			if r.types[i] == 'b' {
				if v == true {
					values[i] = "t"
				} else {
					values[i] = "f"
				}
			}
		}
		r.values = append(r.values, values)
	}
	return r
}
func repairMCommand(tag string) repairMRows { return repairMRows{tag: tag} }
func repairMMemoryArgs(p *pgproto3.Bind) []string {
	args := make([]string, len(p.Parameters))
	for i, raw := range p.Parameters {
		if raw == nil {
			continue
		}
		format := int16(0)
		if len(p.ParameterFormatCodes) == 1 {
			format = p.ParameterFormatCodes[0]
		} else if len(p.ParameterFormatCodes) > i {
			format = p.ParameterFormatCodes[i]
		}
		if format == 1 {
			switch len(raw) {
			case 8:
				args[i] = strconv.FormatInt(int64(binary.BigEndian.Uint64(raw)), 10)
			case 4:
				args[i] = strconv.FormatInt(int64(int32(binary.BigEndian.Uint32(raw))), 10)
			case 2:
				args[i] = strconv.FormatInt(int64(int16(binary.BigEndian.Uint16(raw))), 10)
			case 1:
				args[i] = strconv.FormatBool(raw[0] != 0)
			default:
				args[i] = string(raw)
			}
		} else {
			args[i] = string(raw)
		}
	}
	return args
}
func repairMDescribe(b *pgproto3.Backend, r repairMRows) {
	if r.types == "" {
		b.Send(&pgproto3.NoData{})
		return
	}
	fields := make([]pgproto3.FieldDescription, len(r.types))
	for i, kind := range r.types {
		oid, size := uint32(25), int16(-1)
		if kind == 'b' {
			oid, size = 16, 1
		}
		if kind == 'i' {
			oid, size = 20, 8
		}
		if kind == 'j' {
			oid = 3802
		}
		fields[i] = pgproto3.FieldDescription{Name: []byte(fmt.Sprintf("c%d", i)), DataTypeOID: oid, DataTypeSize: size, TypeModifier: -1}
	}
	b.Send(&pgproto3.RowDescription{Fields: fields})
}
func repairMEmit(b *pgproto3.Backend, r repairMRows) {
	for _, row := range r.values {
		values := make([][]byte, len(row))
		for i, v := range row {
			values[i] = []byte(v)
		}
		b.Send(&pgproto3.DataRow{Values: values})
	}
	b.Send(&pgproto3.CommandComplete{CommandTag: []byte(r.tag)})
}
func (db *repairMDatabase) serve(conn net.Conn) {
	defer conn.Close()
	b := pgproto3.NewBackend(conn, conn)
	if _, err := b.ReceiveStartupMessage(); err != nil {
		return
	}
	b.Send(&pgproto3.AuthenticationOk{})
	b.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "16.0"})
	b.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
	b.Send(&pgproto3.ParameterStatus{Name: "standard_conforming_strings", Value: "on"})
	b.Send(&pgproto3.BackendKeyData{ProcessID: 1, SecretKey: []byte{0, 0, 0, 1}})
	b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if b.Flush() != nil {
		return
	}
	statements := map[string]string{}
	var query string
	var args []string
	var result repairMRows
	var evaluated, failed bool
	var tx *repairMState
	run := func() (repairMRows, error) {
		db.mu.Lock()
		defer db.mu.Unlock()
		lower := strings.ToLower(strings.TrimSpace(query))
		if strings.HasPrefix(lower, "begin") {
			tx = repairMClone(db.state)
			return repairMCommand("BEGIN"), nil
		}
		if lower == "rollback" {
			tx = nil
			return repairMCommand("ROLLBACK"), nil
		}
		if lower == "commit" {
			if tx != nil {
				db.state = tx
			}
			tx = nil
			return repairMCommand("COMMIT"), nil
		}
		state := db.state
		if tx != nil {
			state = tx
		}
		return db.execute(state, query, args)
	}
	sendError := func(err error) {
		b.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: err.Error()})
		failed = true
	}
	for {
		message, err := b.Receive()
		if err != nil {
			return
		}
		switch m := message.(type) {
		case *pgproto3.Terminate:
			return
		case *pgproto3.Parse:
			statements[m.Name] = m.Query
			b.Send(&pgproto3.ParseComplete{})
		case *pgproto3.Bind:
			query = statements[m.PreparedStatement]
			args = repairMMemoryArgs(m)
			evaluated = false
			b.Send(&pgproto3.BindComplete{})
		case *pgproto3.Describe:
			if failed {
				break
			}
			if m.ObjectType != 'P' {
				sendError(errors.New("fixture requires portal describe"))
				break
			}
			result, err = run()
			evaluated = true
			if err != nil {
				sendError(err)
			} else {
				repairMDescribe(b, result)
			}
		case *pgproto3.Execute:
			if failed {
				break
			}
			if !evaluated {
				result, err = run()
				evaluated = true
			}
			if err != nil {
				sendError(err)
			} else {
				repairMEmit(b, result)
			}
		case *pgproto3.Query:
			query, args = m.String, nil
			result, err = run()
			if err != nil {
				sendError(err)
			} else {
				if result.types != "" {
					repairMDescribe(b, result)
				}
				repairMEmit(b, result)
			}
			status := byte('I')
			if tx != nil {
				status = 'T'
			}
			b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
			failed = false
		case *pgproto3.Sync:
			status := byte('I')
			if tx != nil {
				status = 'T'
			}
			b.Send(&pgproto3.ReadyForQuery{TxStatus: status})
			failed = false
		case *pgproto3.Close:
			b.Send(&pgproto3.CloseComplete{})
		case *pgproto3.Flush:
		default:
			sendError(fmt.Errorf("unsupported frontend message %T", m))
		}
		if b.Flush() != nil {
			return
		}
	}
}

func (db *repairMDatabase) execute(s *repairMState, query string, a []string) (repairMRows, error) {
	q := strings.ToLower(strings.Join(strings.Fields(query), " "))
	if db.failRelation != "" && strings.Contains(q, "insert into "+db.failRelation) {
		return repairMRows{}, errors.New("injected relation write failure")
	}
	one := func(v bool) repairMRows { return repairMResult("b", []any{v}) }
	empty := func(types string) repairMRows { return repairMResult(types) }
	pair := func(x, y string) string { return x + ":" + y }
	switch {
	case strings.Contains(q, "pg_advisory_xact_lock"):
		return repairMResult("s", []any{""}), nil
	case strings.Contains(q, "interaction_session_valid"):
		return one(s.Sessions[a[0]] == a[1]), nil
	case strings.Contains(q, "interaction_account_active"):
		return one(s.Active[a[0]]), nil
	case strings.Contains(q, "from chat.chat_bans"):
		denied := s.Erased[a[0]]
		for _, ban := range s.Bans {
			if ban.Wallet == a[0] && !ban.Revoked && (ban.Expires == 0 || ban.Expires > time.Now().Unix()) && (ban.Scope == "global" || ban.Scope == "pool" && ban.Resource == a[1]) {
				denied = true
			}
		}
		return one(denied), nil
	case strings.Contains(q, "from interaction_erased_wallets") && strings.Contains(q, "chat_members"):
		c := s.Conversations[a[1]]
		return one(s.Erased[a[0]] || !repairMContains(c.Members, a[0])), nil
	case strings.Contains(q, "from interaction_erased_wallets"):
		return one(s.Erased[a[0]]), nil
	case strings.HasPrefix(q, "select ") && strings.Contains(q, "from interaction_blocks"):
		return one(s.Blocks[pair(a[0], a[1])] || s.Blocks[pair(a[1], a[0])]), nil
	case strings.Contains(q, "from safety_bans"):
		denied := false
		for _, ban := range s.Bans {
			if (ban.Wallet == a[0] || ban.Wallet == a[1]) && !ban.Revoked && (ban.Expires == 0 || ban.Expires > time.Now().Unix()) && (ban.Scope == "global" || ban.Scope == "dm" && ban.Resource == a[2]) {
				denied = true
			}
		}
		return one(denied), nil
	case strings.HasPrefix(q, "select inbox from interaction_preferences"):
		if p, ok := s.Preferences[a[0]]; ok {
			return repairMResult("s", []any{p.Inbox}), nil
		}
		return empty("s"), nil
	case strings.HasPrefix(q, "select ") && strings.Contains(q, "from interaction_follows"):
		return one(s.Follows[pair(a[0], a[1])]), nil
	case strings.HasPrefix(q, "update interaction_preferences set inbox"):
		p := s.Preferences[a[0]]
		expected, _ := strconv.ParseInt(a[2], 10, 64)
		if p.Revision != expected {
			return repairMCommand("UPDATE 0"), nil
		}
		p.Inbox = a[1]
		p.Revision++
		s.Preferences[a[0]] = p
		return repairMCommand("UPDATE 1"), nil
	case strings.HasPrefix(q, "update interaction_preferences set revision"):
		for _, wallet := range a {
			p := s.Preferences[wallet]
			p.Revision++
			s.Preferences[wallet] = p
		}
		return repairMCommand("UPDATE 2"), nil
	case strings.HasPrefix(q, "insert into interaction_blocks"):
		s.Blocks[pair(a[0], a[1])] = true
		return repairMCommand("INSERT 0 1"), nil
	case strings.HasPrefix(q, "delete from interaction_blocks"):
		delete(s.Blocks, pair(a[0], a[1]))
		return repairMCommand("DELETE 1"), nil
	case strings.HasPrefix(q, "delete from interaction_follows"):
		key := pair(a[0], a[1])
		if s.Follows[key] {
			return repairMRows{}, errors.New("fixture follow publication path not seeded")
		}
		return repairMCommand("DELETE 0"), nil
	case strings.Contains(q, "from chat_messages where sender="):
		for _, m := range s.Messages {
			if m.Sender == a[0] && m.ClientID == a[1] {
				return repairMResult("sssssiiibs", repairMMessageRow(m)), nil
			}
		}
		return empty("sssssiiibs"), nil
	case strings.HasPrefix(q, "select exists(select 1 from chat_conversations"):
		_, exists := s.Conversations[a[0]]
		return one(exists), nil
	case strings.HasPrefix(q, "insert into chat_conversations"):
		if _, ok := s.Conversations[a[0]]; ok {
			return empty("s"), nil
		}
		s.Conversations[a[0]] = repairMConversation{Kind: a[1]}
		return repairMResult("s", []any{a[0]}), nil
	case strings.HasPrefix(q, "insert into chat_members"):
		c := s.Conversations[a[0]]
		c.Members = append(c.Members, a[1:]...)
		s.Conversations[a[0]] = c
		return repairMCommand("INSERT 0 2"), nil
	case strings.HasPrefix(q, "select wallet from chat_members"):
		c := s.Conversations[a[0]]
		if len(a) > 1 {
			if repairMContains(c.Members, a[1]) {
				return repairMResult("s", []any{a[1]}), nil
			}
			return empty("s"), nil
		}
		result := empty("s")
		for _, w := range c.Members {
			result.values = append(result.values, []string{w})
		}
		result.tag = fmt.Sprintf("SELECT %d", len(result.values))
		return result, nil
	case strings.HasPrefix(q, "select exists(select 1 from chat_members"):
		return one(repairMContains(s.Conversations[a[0]].Members, a[1])), nil
	case strings.HasPrefix(q, "update chat_conversations set next_sequence"):
		c := s.Conversations[a[0]]
		c.Next++
		s.Conversations[a[0]] = c
		return repairMResult("i", []any{c.Next}), nil
	case strings.HasPrefix(q, "insert into chat_messages"):
		seq, _ := strconv.ParseInt(a[6], 10, 64)
		s.Messages[a[0]] = fanout.DurableMessage{ID: a[0], ConversationID: a[1], Sender: a[2], ClientID: a[3], Content: a[4], ReplyTo: a[5], Sequence: seq, Revision: 1, CreatedAt: time.Now().Unix(), Type: "dm"}
		return repairMCommand("INSERT 0 1"), nil
	case strings.HasPrefix(q, "insert into chat_outbox"):
		if !json.Valid([]byte(a[3])) {
			return repairMRows{}, fmt.Errorf("non-JSON outbox payload %q", a[3])
		}
		s.ChatOutbox[a[0]] = a[3]
		return repairMCommand("INSERT 0 1"), nil
	case strings.HasPrefix(q, "select retained_from,kind,history_version"):
		c, ok := s.Conversations[a[0]]
		if !ok || !repairMContains(c.Members, a[1]) {
			return empty("isii"), nil
		}
		return repairMResult("isii", []any{1, c.Kind, 1, 0}), nil
	case strings.Contains(q, "from chat_messages where conversation_id="):
		after, _ := strconv.ParseInt(a[1], 10, 64)
		result := empty("sssssiiibs")
		var messages []fanout.DurableMessage
		for _, m := range s.Messages {
			if m.ConversationID == a[0] && m.Sequence > after && repairMContains(s.Conversations[a[0]].Members, a[2]) {
				messages = append(messages, m)
			}
		}
		sort.Slice(messages, func(i, j int) bool { return messages[i].Sequence < messages[j].Sequence })
		for _, m := range messages {
			row := repairMResult("sssssiiibs", repairMMessageRow(m))
			result.values = append(result.values, row.values...)
		}
		result.tag = fmt.Sprintf("SELECT %d", len(result.values))
		return result, nil
	case strings.HasPrefix(q, "select report_id::text"):
		r, ok := s.Reports[pair(a[0], a[1])]
		if !ok {
			return empty("sssssss"), nil
		}
		return repairMResult("sssssss", append(repairMReportRow(r.Report), r.Hash)), nil
	case strings.HasPrefix(q, "insert into chat.v2_reports"):
		out := v2.Report{ReportId: a[0], ReporterWallet: a[1], TargetType: a[4], TargetId: a[5], Reason: a[6], State: "open"}
		s.Reports[pair(a[1], a[2])] = repairMReport{Report: out, Hash: a[3], ReasonText: a[7], Evidence: a[8]}
		return repairMResult("ssssss", repairMReportRow(out)), nil
	case strings.HasPrefix(q, "select exists(select 1 from chat.chat_messages"):
		_, ok := s.Messages[a[0]]
		return one(ok), nil
	case strings.HasPrefix(q, "select case when c.kind="):
		m, ok := s.Messages[a[0]]
		if !ok {
			return empty("ss"), nil
		}
		pool := ""
		if s.Conversations[m.ConversationID].Kind == "pool" {
			pool = strings.TrimPrefix(m.ConversationID, "pool:")
		}
		return repairMResult("ss", []any{pool, m.Sender}), nil
	case strings.HasPrefix(q, "select exists(select 1 from chat.interaction_comments"):
		_, ok := s.Comments[a[0]]
		return one(ok), nil
	case strings.HasPrefix(q, "select pool,author from chat.interaction_comments"):
		if row, ok := s.Comments[a[0]]; ok {
			return repairMResult("ss", []any{row[0], row[1]}), nil
		}
		return empty("ss"), nil
	case strings.HasPrefix(q, "select pool,creator from chat.social_canonical_pools()"):
		if row, ok := s.Tokens[a[0]]; ok {
			return repairMResult("ss", []any{row[0], row[1]}), nil
		}
		return empty("ss"), nil
	case strings.HasPrefix(q, "insert into safety_bans"):
		s.Bans = append(s.Bans, repairMBan{Wallet: a[1], Scope: a[2], Resource: a[3]})
		return repairMCommand("INSERT 0 1"), nil
	case strings.HasPrefix(q, "insert into safety_actions"):
		s.Actions[a[0]] = append([]string(nil), a...)
		return repairMCommand("INSERT 0 1"), nil
	case strings.HasPrefix(q, "insert into safety_outbox"):
		s.SafetyOutbox[a[0]] = a[4]
		return repairMCommand("INSERT 0 1"), nil
	case strings.HasPrefix(q, "insert into chat.safety_cases"):
		key := a[5] + ":" + a[1] + ":" + a[2]
		c, ok := s.Cases[key]
		if !ok {
			c = repairMCase{ID: a[0], Kind: a[1], Target: a[2], Pool: a[3], Affected: a[4], Reporter: a[5], Category: a[6], Description: a[7], State: "open", Revision: 1}
			s.Cases[key] = c
		}
		return repairMResult("si", []any{c.ID, c.Revision}), nil
	case strings.HasPrefix(q, "select exists(select 1 from chat.safety_outbox"):
		_, ok := s.SafetyOutbox[a[0]]
		return one(ok), nil
	case strings.HasPrefix(q, "insert into chat.safety_outbox"):
		if !json.Valid([]byte(a[3])) {
			return repairMRows{}, errors.New("invalid safety event")
		}
		if _, ok := s.SafetyOutbox[a[0]]; !ok {
			s.SafetyOutbox[a[0]] = a[3]
		}
		return repairMCommand("INSERT 0 1"), nil
	case strings.HasPrefix(q, "select id,target_type,target_id,pool_id,state,revision from safety_cases"):
		result := empty("sssssi")
		var cases []repairMCase
		for _, c := range s.Cases {
			if (a[0] == "" || c.State == a[0]) && c.ID > a[1] {
				cases = append(cases, c)
			}
		}
		sort.Slice(cases, func(i, j int) bool { return cases[i].ID < cases[j].ID })
		for _, c := range cases {
			row := repairMResult("sssssi", []any{c.ID, c.Kind, c.Target, c.Pool, c.State, c.Revision})
			result.values = append(result.values, row.values...)
		}
		result.tag = fmt.Sprintf("SELECT %d", len(result.values))
		return result, nil
	}
	db.failures = append(db.failures, query)
	return repairMRows{}, fmt.Errorf("unimplemented fixture SQL: %s", query)
}
func repairMContains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func repairMMessageRow(m fanout.DurableMessage) []any {
	return []any{m.ID, m.ConversationID, m.Sender, m.Content, m.ReplyTo, m.Sequence, m.Revision, m.CreatedAt, m.Deleted, m.ClientID}
}
func repairMReportRow(r v2.Report) []any {
	return []any{r.ReportId, r.ReporterWallet, r.TargetType, r.TargetId, r.Reason, r.State}
}

// The identity peer serves the committed platform request/ticket/session HTTP
// contracts. Tickets are single use and scoped by the real shared validator.
type repairMIdentity struct {
	mu         sync.Mutex
	principals map[string]sharedauth.IdentityPrincipal
	tickets    map[string]sharedauth.StreamPrincipal
	server     *httptest.Server
}

func repairMNewIdentity(t *testing.T, db *repairMDatabase) *repairMIdentity {
	t.Helper()
	peer := &repairMIdentity{principals: map[string]sharedauth.IdentityPrincipal{}, tickets: map[string]sharedauth.StreamPrincipal{}}
	for _, wallet := range []string{repairMA, repairMB, repairMC} {
		principal := peer.session(wallet)
		db.change(func(s *repairMState) { s.Sessions[principal.SessionID] = wallet })
	}
	peer.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer.mu.Lock()
		defer peer.mu.Unlock()
		if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer "+strings.Repeat("fixture", 8) {
			http.Error(w, "unauthorized", 401)
			return
		}
		switch r.URL.Path {
		case "/internal/v2/identity/request:authenticate":
			var in sharedauth.IdentityAuthenticationRequest
			if json.NewDecoder(r.Body).Decode(&in) != nil {
				http.Error(w, "invalid", 400)
				return
			}
			p, ok := peer.principals[strings.TrimPrefix(in.Authorization, "Bearer ")]
			if !ok || p.Validate(time.Now()) != nil {
				http.Error(w, "unknown session", 401)
				return
			}
			_ = json.NewEncoder(w).Encode(p)
		case "/internal/v2/identity/chat-ticket:consume":
			var in struct {
				Ticket  string                         `json:"ticket"`
				Binding sharedauth.StreamTicketBinding `json:"binding"`
			}
			if json.NewDecoder(r.Body).Decode(&in) != nil {
				http.Error(w, "invalid", 400)
				return
			}
			p, ok := peer.tickets[in.Ticket]
			if !ok || sharedauth.ValidateChatStreamBinding(p, in.Binding) != nil {
				http.Error(w, "invalid ticket", 401)
				return
			}
			delete(peer.tickets, in.Ticket)
			_ = json.NewEncoder(w).Encode(p)
		case "/internal/v2/identity/session:check":
			var in sharedauth.StreamPrincipal
			if json.NewDecoder(r.Body).Decode(&in) != nil {
				http.Error(w, "invalid", 400)
				return
			}
			p, ok := peer.principals[in.Wallet]
			if !ok || p.StreamPrincipal() != in || p.Validate(time.Now()) != nil {
				http.Error(w, "revoked", 401)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "unknown identity contract", 404)
		}
	}))
	t.Cleanup(peer.server.Close)
	for wallet, issued := range peer.principals {
		ctx := peer.context(wallet)
		principal, ok := fanout.PrincipalFromContext(ctx)
		if !ok || principal.Wallet != issued.Wallet || principal.SessionID != issued.SessionID {
			t.Fatalf("issued session failed request/session verification: wallet=%s principal=%+v", wallet, principal)
		}
		if _, err := interactions.Identity(ctx); err != nil {
			t.Fatalf("issued session failed interaction verification: %v", err)
		}
	}
	return peer
}
func (p *repairMIdentity) session(wallet string) sharedauth.IdentityPrincipal {
	principal := sharedauth.IdentityPrincipal{AccountID: uuid.NewString(), SessionID: uuid.NewString(), Wallet: wallet, AuthEpoch: 1, ProofAt: time.Now().Add(-time.Minute).UTC(), AccessExpiresAt: time.Now().Add(time.Hour).UTC()}
	p.mu.Lock()
	p.principals[wallet] = principal
	p.mu.Unlock()
	return principal
}
func (p *repairMIdentity) authority() sharedauth.HTTPIdentityAuthority {
	return sharedauth.HTTPIdentityAuthority{Endpoint: p.server.URL, ServiceToken: strings.Repeat("fixture", 8), Client: p.server.Client()}
}
func (p *repairMIdentity) ticket(wallet string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	a, b := uuid.New(), uuid.New()
	ticket := hex.EncodeToString(append(a[:], b[:]...))
	p.tickets[ticket] = p.principals[wallet].StreamPrincipal()
	return ticket
}
func (p *repairMIdentity) context(wallet string) context.Context {
	req := httptest.NewRequest("GET", "http://social.example", nil)
	req.Header.Set("Authorization", "Bearer "+wallet)
	principal, err := fanout.VerifiedRequestAuthenticator(p.authority())(req)
	if err != nil {
		panic(err)
	}
	return interactions.WithIdentity(fanout.WithPrincipal(context.Background(), principal), principal)
}

type repairMHarness struct {
	runtime         *Runtime
	pool            *pgxpool.Pool
	redis           *goredis.Client
	social, gateway *httptest.Server
	identity        *repairMIdentity
}

func repairMStart(t *testing.T, db *repairMDatabase, identity *repairMIdentity) *repairMHarness {
	t.Helper()
	pool := db.open(t)
	// A committed RESP peer backs the production Redis client and admission
	// policies without an external service; it survives runtime restarts.
	redis := db.redis.client(t)
	request := fanout.VerifiedRequestAuthenticator(identity.authority())
	rt, err := New(context.Background(), Deps{Pool: pool, Redis: redis, Authenticate: request, CursorKey: []byte(strings.Repeat("k", 32)), Pools: CanonicalPools})
	if err != nil {
		t.Fatal(err)
	}
	if rt.Chat.ReadPolicy == nil {
		t.Fatal("platform runtime did not wire read authorization")
	}
	authority := identity.authority()
	hub := fanout.New(fanout.Deps{ProtocolVersion: 2, Durable: rt.Chat, Pub: redis, Sub: redis, Authenticate: fanout.TicketAuthenticator(sharedauth.HTTPStreamAuthority(authority), authority)})
	router := chi.NewRouter()
	if err = rt.Register(router); err != nil {
		t.Fatal(err)
	}
	router.Get("/v2/stream", hub.ServeWS)
	router.Group(func(gr chi.Router) {
		gr.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := safety.WithPrincipal(r.Context(), safety.Principal{Wallet: repairMC, SessionID: repairMSession(2), Role: "global_administrator", Expires: time.Now().Add(time.Hour)})
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		social.RegisterSafetyRoutes(gr, &safety.Store{DB: pool}, nil)
	})
	social := httptest.NewServer(router)
	upstream, err := url.Parse(social.URL)
	if err != nil {
		t.Fatal(err)
	}
	gateway := httptest.NewServer(httputil.NewSingleHostReverseProxy(upstream))
	h := &repairMHarness{runtime: rt, pool: pool, redis: redis, social: social, gateway: gateway, identity: identity}
	return h
}
func (h *repairMHarness) close() {
	h.gateway.Close()
	h.social.Close()
	h.pool.Close()
	_ = h.redis.Close()
}
func (h *repairMHarness) socket(t *testing.T, wallet string) *websocket.Conn {
	t.Helper()
	ticket := h.identity.ticket(wallet)
	dialer := websocket.Dialer{HandshakeTimeout: 3 * time.Second, Subprotocols: []string{"sidiora.v2", "ticket." + ticket}}
	endpoint := "ws" + strings.TrimPrefix(h.gateway.URL, "http") + "/v2/stream?topics=private.chat." + wallet
	conn, response, err := dialer.Dial(endpoint, http.Header{"Origin": []string{"https://web.example"}})
	if err != nil {
		if response != nil {
			body, _ := io.ReadAll(response.Body)
			t.Fatalf("upgrade: %v: %s", err, body)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var accepted map[string]any
	if err = conn.ReadJSON(&accepted); err != nil || accepted["type"] != "session_accepted" {
		t.Fatalf("socket admission: %v %v", accepted, err)
	}
	return conn
}
func repairMSend(t *testing.T, conn *websocket.Conn, target, clientID string) map[string]any {
	t.Helper()
	if err := conn.WriteJSON(map[string]string{"type": "dm", "to": target, "content": "private message", "client_message_id": clientID}); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var out map[string]any
	if err := conn.ReadJSON(&out); err != nil {
		t.Fatal(err)
	}
	if out["client_message_id"] != clientID {
		t.Fatalf("receipt identity: %v", out)
	}
	return out
}
func (h *repairMHarness) request(t *testing.T, method, path, wallet, key string, body any) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = strings.NewReader(string(raw))
	}
	req, err := http.NewRequest(method, h.gateway.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if wallet != "" {
		req.Header.Set("Authorization", "Bearer "+wallet)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := h.gateway.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, raw
}
func repairMAssertCounts(t *testing.T, db *repairMDatabase, messages, outbox int) {
	t.Helper()
	state := db.snapshot()
	db.redis.mu.Lock()
	transactions, abuseCalls := db.redis.transactions, db.redis.abuseCalls
	db.redis.mu.Unlock()
	if transactions < messages || abuseCalls < messages {
		t.Fatalf("committed sends bypassed admission: transactions=%d abuse=%d messages=%d", transactions, abuseCalls, messages)
	}
	if len(state.Messages) != messages || len(state.ChatOutbox) != outbox {
		t.Fatalf("committed messages/intents = %d/%d, want %d/%d", len(state.Messages), len(state.ChatOutbox), messages, outbox)
	}
}

func TestRepairM_1028_PlatformDMPermissions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*testing.T, *repairMDatabase, *repairMHarness)
		allow   bool
	}{
		{"everyone", nil, true},
		{"sender_blocks_peer", func(t *testing.T, db *repairMDatabase, h *repairMHarness) {
			if err := h.runtime.policy.chatAuthority().SetBlock(h.identity.context(repairMA), repairMB, false); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"peer_blocks_sender", func(t *testing.T, db *repairMDatabase, h *repairMHarness) {
			if err := h.runtime.policy.chatAuthority().SetBlock(h.identity.context(repairMB), repairMA, false); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"nobody", func(t *testing.T, db *repairMDatabase, h *repairMHarness) {
			if err := h.runtime.policy.chatAuthority().SetPrivacy(h.identity.context(repairMB), "nobody", 1); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"followed_only_denied", func(t *testing.T, db *repairMDatabase, h *repairMHarness) {
			if err := h.runtime.policy.chatAuthority().SetPrivacy(h.identity.context(repairMB), "followed_only", 1); err != nil {
				t.Fatal(err)
			}
		}, false},
		{"followed_only_wrong_direction", func(t *testing.T, db *repairMDatabase, h *repairMHarness) {
			if err := h.runtime.policy.chatAuthority().SetPrivacy(h.identity.context(repairMB), "followed_only", 1); err != nil {
				t.Fatal(err)
			}
			db.change(func(s *repairMState) { s.Follows[repairMA+":"+repairMB] = true })
		}, false},
		{"followed_only_allowed", func(t *testing.T, db *repairMDatabase, h *repairMHarness) {
			if err := h.runtime.policy.chatAuthority().SetPrivacy(h.identity.context(repairMB), "followed_only", 1); err != nil {
				t.Fatal(err)
			}
			db.change(func(s *repairMState) { s.Follows[repairMB+":"+repairMA] = true })
		}, true},
		{"missing_inbox", func(t *testing.T, db *repairMDatabase, h *repairMHarness) {
			db.change(func(s *repairMState) { delete(s.Preferences, repairMB) })
		}, false},
		{"sender_dm_ban", func(t *testing.T, db *repairMDatabase, h *repairMHarness) {
			repairMCreateBan(t, h, repairMA, "dm", repairMConversationID())
		}, false},
		{"recipient_dm_ban", func(t *testing.T, db *repairMDatabase, h *repairMHarness) {
			repairMCreateBan(t, h, repairMB, "dm", repairMConversationID())
		}, false},
		{"recipient_global_ban", func(t *testing.T, db *repairMDatabase, h *repairMHarness) {
			repairMCreateBan(t, h, repairMB, "global", "")
		}, false},
		{"expired_dm_ban", func(t *testing.T, db *repairMDatabase, h *repairMHarness) {
			db.change(func(s *repairMState) {
				s.Bans = append(s.Bans, repairMBan{Wallet: repairMA, Scope: "dm", Resource: repairMConversationID(), Expires: time.Now().Add(-time.Second).Unix()})
			})
		}, true},
		{"revoked_dm_ban", func(t *testing.T, db *repairMDatabase, h *repairMHarness) {
			db.change(func(s *repairMState) {
				s.Bans = append(s.Bans, repairMBan{Wallet: repairMA, Scope: "dm", Resource: repairMConversationID(), Revoked: true})
			})
		}, true},
		{"unrelated_dm_ban", func(t *testing.T, db *repairMDatabase, h *repairMHarness) {
			db.change(func(s *repairMState) {
				s.Bans = append(s.Bans, repairMBan{Wallet: repairMA, Scope: "dm", Resource: uuid.NewString()})
			})
		}, true},
		{"inactive_peer", func(t *testing.T, db *repairMDatabase, h *repairMHarness) {
			db.change(func(s *repairMState) { s.Active[repairMB] = false })
		}, false},
		{"erased_sender", func(t *testing.T, db *repairMDatabase, h *repairMHarness) {
			db.change(func(s *repairMState) { s.Erased[repairMA] = true })
		}, false},
		{"session_guard_revoked", func(t *testing.T, db *repairMDatabase, h *repairMHarness) {
			db.change(func(s *repairMState) { delete(s.Sessions, h.identity.principals[repairMA].SessionID) })
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := repairMNewDatabase(t)
			identity := repairMNewIdentity(t, db)
			h := repairMStart(t, db, identity)
			defer func() { h.close() }()
			if tc.prepare != nil {
				tc.prepare(t, db, h)
			}
			for round := 0; round < 2; round++ {
				conn := h.socket(t, repairMA)
				ack := repairMSend(t, conn, repairMB, fmt.Sprintf("permission-%d", round))
				want := "error"
				if tc.allow {
					want = "message_persisted"
				}
				if ack["type"] != want {
					t.Fatalf("round %d response %v, want %s", round, ack, want)
				}
				if !tc.allow && (ack["message"] != interactions.ErrDenied.Error() && ack["message"] != errChatDenied.Error()) {
					t.Fatalf("unrelated refusal cannot prove policy: %v", ack)
				}
				_ = conn.Close()
				count := 0
				if tc.allow {
					count = round + 1
				}
				repairMAssertCounts(t, db, count, count)
				h.close()
				if round == 0 {
					h = repairMStart(t, db, identity)
				}
			}
		})
	}
}
func repairMConversationID() string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(repairMA+":"+repairMB)).String()
}

func TestRepairM_1028_ConnectedPolicyChangesAndReadAuthorization(t *testing.T) {
	db := repairMNewDatabase(t)
	identity := repairMNewIdentity(t, db)
	h := repairMStart(t, db, identity)
	conn := h.socket(t, repairMA)
	receipt := repairMSend(t, conn, repairMB, "before-block")
	if receipt["type"] != "message_persisted" {
		t.Fatal(receipt)
	}
	authority := h.runtime.policy.chatAuthority()
	if err := authority.SetBlock(identity.context(repairMB), repairMA, false); err != nil {
		t.Fatal(err)
	}
	refused := repairMSend(t, conn, repairMB, "after-block")
	if refused["type"] != "error" || refused["message"] != interactions.ErrDenied.Error() {
		t.Fatal(refused)
	}
	repairMAssertCounts(t, db, 1, 1)
	_ = conn.Close()
	h.close()
	h = repairMStart(t, db, identity)
	defer h.close()
	conn = h.socket(t, repairMA)
	refused = repairMSend(t, conn, repairMB, "restart-block")
	if refused["type"] != "error" || refused["message"] != interactions.ErrDenied.Error() {
		t.Fatal(refused)
	}
	path := "/v2/chat/history?conversation_id=" + repairMConversationID()
	status, raw := h.request(t, "GET", path, repairMB, "", nil)
	var history fanout.HistoryPage
	if err := json.Unmarshal(raw, &history); err != nil || status != 200 || len(history.Messages) != 1 || history.Messages[0].Content != "private message" {
		t.Fatalf("recovered authorized history: %d %s %v", status, raw, err)
	}
	status, raw = h.request(t, "GET", path, repairMC, "", nil)
	if status != 404 || strings.Contains(string(raw), "private message") {
		t.Fatalf("nonmember read: %d %s", status, raw)
	}
	tx, err := h.pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	err = h.runtime.Chat.ReadPolicy(identity.context(repairMA), tx, repairMB, repairMConversationID())
	_ = tx.Rollback(context.Background())
	if !errors.Is(err, interactions.ErrDenied) {
		t.Fatalf("identity substitution: %v", err)
	}
	db.change(func(s *repairMState) { delete(s.Sessions, identity.principals[repairMB].SessionID) })
	status, raw = h.request(t, "GET", path, repairMB, "", nil)
	if status != 503 || strings.Contains(string(raw), "private message") {
		t.Fatalf("revoked database session read: %d %s", status, raw)
	}
	db.change(func(s *repairMState) {
		s.Sessions[identity.principals[repairMB].SessionID] = repairMB
		s.Erased[repairMB] = true
	})
	status, raw = h.request(t, "GET", path, repairMB, "", nil)
	if status != 503 || strings.Contains(string(raw), "private message") {
		t.Fatalf("erased member read: %d %s", status, raw)
	}
	_ = conn.Close()
}

func repairMReportRequest(t *testing.T, h *repairMHarness, key string, body v2.ReportCreateBody) (int, v2.Report, []byte) {
	t.Helper()
	status, raw := h.request(t, "POST", "/v2/reports", repairMA, key, body)
	var envelope v2.ReportCreateResponse
	if status == 201 {
		var err error
		envelope, err = v2.DecodeReportCreateResponse(raw)
		if err != nil {
			t.Fatalf("frozen response codec: %v %s", err, raw)
		}
	}
	return status, envelope.Data.Report, raw
}
func TestRepairM_1028_ReportSafetyIntakeAndRestart(t *testing.T) {
	db := repairMNewDatabase(t)
	identity := repairMNewIdentity(t, db)
	h := repairMStart(t, db, identity)
	conn := h.socket(t, repairMA)
	message := repairMSend(t, conn, repairMB, "reported-message")
	_ = conn.Close()
	if message["type"] != "message_persisted" {
		t.Fatal(message)
	}
	body := v2.ReportCreateBody{TargetType: "message", TargetId: message["id"].(string), Reason: "abuse", Evidence: "recorded evidence: exact UTF-8 café"}
	key := uuid.NewString()
	status, first, raw := repairMReportRequest(t, h, key, body)
	if status != 201 {
		t.Fatalf("report: %d %s", status, raw)
	}
	status, duplicate, raw := repairMReportRequest(t, h, key, body)
	if status != 201 || duplicate != first {
		t.Fatalf("duplicate result: %d %+v %+v %s", status, duplicate, first, raw)
	}
	state := db.snapshot()
	if len(state.Reports) != 1 || len(state.Cases) != 1 || len(state.SafetyOutbox) != 1 {
		t.Fatalf("duplicate intake cardinality: %+v", state)
	}
	var reportCase repairMCase
	for _, c := range state.Cases {
		reportCase = c
	}
	if reportCase.ID != first.ReportId || reportCase.Target != body.TargetId || reportCase.Affected != repairMA || reportCase.Category != body.Reason || reportCase.Description != body.Evidence || reportCase.State != "open" {
		t.Fatalf("case mapping: %+v", reportCase)
	}
	for _, payload := range state.SafetyOutbox {
		var event map[string]any
		if json.Unmarshal([]byte(payload), &event) != nil || event["entityId"] != reportCase.ID || event["reportId"] != first.ReportId || strings.Contains(payload, body.Evidence) {
			t.Fatalf("durable report/case link: %s", payload)
		}
	}
	conflict := body
	conflict.Evidence = "different evidence"
	status, _, raw = repairMReportRequest(t, h, key, conflict)
	if status != 409 {
		t.Fatalf("idempotency conflict: %d %s", status, raw)
	}
	h.close()
	h = repairMStart(t, db, identity)
	defer h.close()
	// Intake already committed with the receipt; the first moderator delivery
	// occurs only after the consumer and its connection pool have been reopened.
	status, raw = h.request(t, "GET", "/v2/safety/cases", "", "", nil)
	var listed []safety.Case
	if err := json.Unmarshal(raw, &listed); err != nil || status != 200 || len(listed) != 1 || listed[0].ID != first.ReportId || listed[0].TargetID != body.TargetId {
		t.Fatalf("moderator delivery after restart: %d %s %v", status, raw, err)
	}
	status, duplicate, raw = repairMReportRequest(t, h, key, body)
	if status != 201 || duplicate != first {
		t.Fatalf("recovered idempotency: %d %s", status, raw)
	}
	// A distinct key is a distinct receipt, linked durably to the same canonical
	// safety case. Both evidence submissions survive; neither is overwritten.
	status, second, raw := repairMReportRequest(t, h, uuid.NewString(), conflict)
	if status != 201 || second.ReportId == first.ReportId {
		t.Fatalf("second receipt: %d %s", status, raw)
	}
	state = db.snapshot()
	if len(state.Reports) != 2 || len(state.Cases) != 1 || len(state.SafetyOutbox) != 2 {
		t.Fatalf("canonical case/receipt cardinality: %+v", state)
	}
	evidence := map[string]bool{}
	for _, r := range state.Reports {
		evidence[r.Evidence] = true
	}
	if !evidence[body.Evidence] || !evidence[conflict.Evidence] {
		t.Fatalf("lost evidence: %v", evidence)
	}
	for _, payload := range state.SafetyOutbox {
		var event map[string]any
		_ = json.Unmarshal([]byte(payload), &event)
		if event["entityId"] != first.ReportId {
			t.Fatalf("case identity diverged: %s", payload)
		}
	}
}

func TestRepairM_1028_ReportAtomicityAndReasonMapping(t *testing.T) {
	for _, relation := range []string{"chat.safety_cases", "chat.safety_outbox"} {
		t.Run(relation, func(t *testing.T) {
			db := repairMNewDatabase(t)
			identity := repairMNewIdentity(t, db)
			h := repairMStart(t, db, identity)
			defer h.close()
			db.mu.Lock()
			db.failRelation = relation
			db.mu.Unlock()
			body := v2.ReportCreateBody{TargetType: "profile", TargetId: repairMB, Reason: "deceptive promotion", Evidence: strings.Repeat("e", 4000)}
			key := uuid.NewString()
			status, _, raw := repairMReportRequest(t, h, key, body)
			if status != 503 {
				t.Fatalf("failed intake must refuse receipt: %d %s", status, raw)
			}
			state := db.snapshot()
			if len(state.Reports) != 0 || len(state.Cases) != 0 || len(state.SafetyOutbox) != 0 {
				t.Fatalf("partial transaction survived: %+v", state)
			}
			db.mu.Lock()
			db.failRelation = ""
			db.mu.Unlock()
			status, report, raw := repairMReportRequest(t, h, key, body)
			if status != 201 || report.Reason != "other" {
				t.Fatalf("retry after rollback: %d %s", status, raw)
			}
			state = db.snapshot()
			for _, c := range state.Cases {
				if c.Category != body.Reason || c.Description != body.Evidence || c.Affected != repairMB || c.ID != report.ReportId {
					t.Fatalf("raw reason/evidence mapping: %+v", c)
				}
			}
			for _, r := range state.Reports {
				if r.Report.Reason != "other" || r.ReasonText != body.Reason || r.Evidence != body.Evidence {
					t.Fatalf("frozen reason/evidence lost: %+v", r)
				}
			}
		})
	}
}

func repairMCreateBan(t *testing.T, h *repairMHarness, wallet, scope, resource string) {
	t.Helper()
	ctx := safety.WithPrincipal(context.Background(), safety.Principal{Wallet: repairMC, SessionID: repairMSession(2), Role: "global_administrator", Expires: time.Now().Add(time.Hour), StepUp: time.Now()})
	if _, err := (&safety.Store{DB: h.pool}).Ban(ctx, safety.BanCommand{Wallet: wallet, Scope: scope, Resource: resource, Reason: "persisted moderation restriction", PolicyVersion: "fixture-v1"}); err != nil {
		t.Fatal(err)
	}
}

func TestRepairM_1028_ReportTargetMapping(t *testing.T) {
	db := repairMNewDatabase(t)
	identity := repairMNewIdentity(t, db)
	h := repairMStart(t, db, identity)
	defer h.close()
	pool := "0xdddddddddddddddddddddddddddddddddddddddd"
	db.change(func(s *repairMState) {
		s.Comments["comment-fixture"] = []string{pool, repairMB}
		s.Tokens[repairMC] = []string{pool, repairMC}
	})
	for _, tc := range []struct{ kind, target, pool, affected string }{
		{"comment", "comment-fixture", pool, repairMB},
		{"token", repairMC, pool, repairMC},
		{"profile", repairMB, "", repairMB},
		{"stream", uuid.NewString(), "", ""},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			body := v2.ReportCreateBody{TargetType: tc.kind, TargetId: tc.target, Reason: "fraud", Evidence: "target evidence"}
			status, receipt, raw := repairMReportRequest(t, h, uuid.NewString(), body)
			if status != 201 {
				t.Fatalf("target intake: %d %s", status, raw)
			}
			state := db.snapshot()
			found := false
			for _, c := range state.Cases {
				if c.ID == receipt.ReportId {
					found = true
					if c.Kind != tc.kind || c.Target != tc.target || c.Pool != tc.pool || c.Affected != tc.affected || c.Category != body.Reason || c.Description != body.Evidence {
						t.Fatalf("target mapping: %+v", c)
					}
				}
			}
			if !found {
				t.Fatal("accepted report absent from safety intake")
			}
		})
	}
}

func TestRepairM_1028_ReportRetryAfterTargetRemoval(t *testing.T) {
	db := repairMNewDatabase(t)
	identity := repairMNewIdentity(t, db)
	h := repairMStart(t, db, identity)
	defer h.close()
	conn := h.socket(t, repairMA)
	sent := repairMSend(t, conn, repairMB, "report-before-erasure")
	_ = conn.Close()
	if sent["type"] != "message_persisted" {
		t.Fatal(sent)
	}
	body := v2.ReportCreateBody{TargetType: "message", TargetId: sent["id"].(string), Reason: "spam", Evidence: "retained private evidence"}
	key := uuid.NewString()
	status, first, raw := repairMReportRequest(t, h, key, body)
	if status != 201 {
		t.Fatalf("initial report: %d %s", status, raw)
	}
	db.change(func(s *repairMState) { delete(s.Messages, body.TargetId) })
	status, retry, raw := repairMReportRequest(t, h, key, body)
	if status != 201 || retry != first {
		t.Fatalf("accepted identity lost after target removal: %d %s", status, raw)
	}
	state := db.snapshot()
	if len(state.Reports) != 1 || len(state.Cases) != 1 || len(state.SafetyOutbox) != 1 {
		t.Fatalf("retry duplicated intake: %+v", state)
	}
}

func TestRepairM_1028_IdentityGuards(t *testing.T) {
	db := repairMNewDatabase(t)
	identity := repairMNewIdentity(t, db)
	h := repairMStart(t, db, identity)
	defer h.close()
	input := fanout.SendInput{Actor: repairMA, Kind: "dm", Target: repairMB, ClientID: "untrusted", Content: "private"}
	if err := h.runtime.policy.SendChat(context.Background(), nil, input); !errors.Is(err, interactions.ErrDenied) {
		t.Fatalf("missing principal: %v", err)
	}
	input.Actor = repairMB
	if err := h.runtime.policy.SendChat(identity.context(repairMA), nil, input); !errors.Is(err, interactions.ErrDenied) {
		t.Fatalf("substituted actor: %v", err)
	}
	dialer := websocket.Dialer{HandshakeTimeout: 3 * time.Second, Subprotocols: []string{"sidiora.v2"}}
	conn, response, err := dialer.Dial("ws"+strings.TrimPrefix(h.gateway.URL, "http")+"/v2/stream?topics=private.chat."+repairMA, http.Header{ActorHeader: []string{repairMA}, "Origin": []string{"https://web.example"}})
	if conn != nil {
		_ = conn.Close()
	}
	if err == nil || response == nil || response.StatusCode != 401 {
		t.Fatalf("actor header bypassed ticket identity: %v %v", response, err)
	}
	if response.Body != nil {
		_ = response.Body.Close()
	}
	conn = h.socket(t, repairMA)
	identity.mu.Lock()
	delete(identity.principals, repairMA)
	identity.mu.Unlock()
	if err = conn.WriteJSON(map[string]string{"type": "dm", "to": repairMB, "content": "revoked", "client_message_id": "revoked-session"}); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err = conn.ReadMessage()
	var closed *websocket.CloseError
	if !errors.As(err, &closed) || closed.Code != 4001 {
		t.Fatalf("revoked connected session: %v", err)
	}
	repairMAssertCounts(t, db, 0, 0)
	_ = conn.Close()
}

func repairMNumber(raw string) int64 {
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		panic(err)
	}
	return n
}

// This RESP2 peer retains counters and deadlines across client/hub restarts.
// MULTI queues commands and EXEC applies them under one lock, including Redis's
// per-command errors (a failed command does not roll back earlier commands).
// The safety script has a pinned transcript identity and token-bucket state.
type repairMBucket struct{ tokens, at float64 }
type repairMRedis struct {
	mu                       sync.Mutex
	now                      time.Time
	counters                 map[string]int64
	expires                  map[string]time.Time
	buckets                  map[string]repairMBucket
	peers                    map[*repairMRedisConn]bool
	addr                     string
	interrupt                string
	interruptKey             string
	refuseExpiry             bool
	unavailable              bool
	abuseUnavailable         bool
	transactions, abuseCalls int
	scripts                  map[string]string
}
type repairMRedisConn struct {
	net.Conn
	writeMu sync.Mutex
	topics  map[string]bool // guarded by the peer's mutex
}

func (c *repairMRedisConn) write(raw string) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, _ = io.WriteString(c.Conn, raw)
}
func newRepairMRedis(t *testing.T) *repairMRedis {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &repairMRedis{now: time.Now(), counters: map[string]int64{}, expires: map[string]time.Time{}, buckets: map[string]repairMBucket{}, peers: map[*repairMRedisConn]bool{}, scripts: map[string]string{}, addr: listener.Addr().String()}
	t.Cleanup(func() {
		_ = listener.Close()
		p.mu.Lock()
		defer p.mu.Unlock()
		for c := range p.peers {
			_ = c.Close()
		}
	})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			c := &repairMRedisConn{Conn: conn, topics: map[string]bool{}}
			p.mu.Lock()
			p.peers[c] = true
			p.mu.Unlock()
			go p.serve(c)
		}
	}()
	return p
}
func (p *repairMRedis) client(t *testing.T) *goredis.Client {
	t.Helper()
	c := goredis.NewClient(&goredis.Options{Addr: p.addr, Protocol: 2, DisableIdentity: true, MaxRetries: -1, DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second})
	t.Cleanup(func() { _ = c.Close() })
	return c
}
func repairMRESP(args ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(arg), arg)
	}
	return b.String()
}
func repairMReadRESP(r *bufio.Reader) ([]string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(line, "*") {
		return nil, errors.New("expected RESP array")
	}
	n, err := strconv.Atoi(strings.TrimSpace(line[1:]))
	if err != nil || n < 1 || n > 100 {
		return nil, errors.New("invalid RESP array")
	}
	args := make([]string, n)
	for i := range args {
		line, err = r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(line, "$") {
			return nil, errors.New("expected RESP bulk string")
		}
		size, err := strconv.Atoi(strings.TrimSpace(line[1:]))
		if err != nil || size < 0 || size > 1<<20 {
			return nil, errors.New("invalid RESP bulk length")
		}
		raw := make([]byte, size+2)
		if _, err = io.ReadFull(r, raw); err != nil {
			return nil, err
		}
		args[i] = string(raw[:size])
	}
	return args, nil
}
func (p *repairMRedis) serve(c *repairMRedisConn) {
	defer c.Close()
	defer func() { p.mu.Lock(); delete(p.peers, c); p.mu.Unlock() }()
	reader := bufio.NewReader(c.Conn)
	var queued [][]string
	multi := false
	for {
		args, err := repairMReadRESP(reader)
		if err != nil {
			return
		}
		command := strings.ToLower(args[0])
		if multi && command != "exec" && command != "discard" {
			queued = append(queued, args)
			c.write("+QUEUED\r\n")
			continue
		}
		switch command {
		case "hello":
			c.write("-ERR unknown command 'hello'\r\n")
		case "ping":
			c.write("+PONG\r\n")
		case "multi":
			multi = true
			c.write("+OK\r\n")
		case "discard":
			multi = false
			queued = nil
			c.write("+OK\r\n")
		case "exec":
			p.mu.Lock()
			key := ""
			if len(queued) > 0 && len(queued[0]) > 1 {
				key = queued[0][1]
			}
			interrupt := ""
			if key == p.interruptKey {
				interrupt = p.interrupt
				p.interrupt = ""
			}
			if interrupt == "before" {
				p.mu.Unlock()
				return
			}
			if p.unavailable {
				p.mu.Unlock()
				c.write("-ERR dependency unavailable\r\n")
				multi = false
				queued = nil
				continue
			}
			p.transactions++
			raw := fmt.Sprintf("*%d\r\n", len(queued))
			for _, q := range queued {
				raw += p.execute(q)
			}
			p.mu.Unlock()
			if interrupt == "after" {
				return
			}
			c.write(raw)
			multi = false
			queued = nil
		case "subscribe", "unsubscribe":
			p.mu.Lock()
			for _, topic := range args[1:] {
				if command == "subscribe" {
					c.topics[topic] = true
				} else {
					delete(c.topics, topic)
				}
				c.write(fmt.Sprintf("*3\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n:%d\r\n", len(command), command, len(topic), topic, len(c.topics)))
			}
			p.mu.Unlock()
		case "publish":
			p.mu.Lock()
			count := 0
			for subscriber := range p.peers {
				if subscriber.topics[args[1]] {
					subscriber.write(repairMRESP("message", args[1], args[2]))
					count++
				}
			}
			p.mu.Unlock()
			c.write(fmt.Sprintf(":%d\r\n", count))
		default:
			p.mu.Lock()
			raw := p.execute(args)
			p.mu.Unlock()
			c.write(raw)
		}
	}
}
func (p *repairMRedis) expire(key string) {
	if until, ok := p.expires[key]; ok && !until.After(p.now) {
		delete(p.counters, key)
		delete(p.expires, key)
	}
}
func (p *repairMRedis) execute(a []string) string {
	integer := func(n int64) string { return fmt.Sprintf(":%d\r\n", n) }
	switch strings.ToLower(a[0]) {
	case "incr":
		p.expire(a[1])
		p.counters[a[1]]++
		return integer(p.counters[a[1]])
	case "pexpire":
		if len(a) != 4 || a[3] != "NX" {
			return "-ERR expected PEXPIRE NX\r\n"
		}
		if p.refuseExpiry {
			return "-NOPERM expiry refused\r\n"
		}
		p.expire(a[1])
		if _, ok := p.counters[a[1]]; !ok {
			return integer(0)
		}
		if _, ok := p.expires[a[1]]; ok {
			return integer(0)
		}
		p.expires[a[1]] = p.now.Add(time.Duration(repairMNumber(a[2])) * time.Millisecond)
		return integer(1)
	case "pttl":
		p.expire(a[1])
		if _, ok := p.counters[a[1]]; !ok {
			return integer(-2)
		}
		if until, ok := p.expires[a[1]]; ok {
			return integer(until.Sub(p.now).Milliseconds())
		}
		return integer(-1)
	case "evalsha", "eval":
		script := a[1]
		if strings.EqualFold(a[0], "evalsha") {
			var ok bool
			script, ok = p.scripts[a[1]]
			if !ok {
				return "-NOSCRIPT No matching script. Please use EVAL.\r\n"
			}
		}
		digest := sha1.Sum([]byte(script))
		// Pin the real safety.AbuseLimiter script's recorded wire contract. This
		// peer never claims to be a Lua interpreter or to qualify Redis itself.
		if hex.EncodeToString(digest[:]) != repairMAbuseSHA {
			return "-ERR unrecorded script\r\n"
		}
		p.scripts[hex.EncodeToString(digest[:])] = script
		p.abuseCalls++
		if p.abuseUnavailable {
			return "-ERR abuse authority unavailable\r\n"
		}
		n := int(repairMNumber(a[2]))
		if len(a) != 3+n*3 {
			return "-ERR script argument mismatch\r\n"
		}
		now := float64(p.now.UnixMilli())
		retry := float64(0)
		next := make(map[string]repairMBucket, n)
		for i, key := range a[3 : 3+n] {
			limit, span := float64(repairMNumber(a[3+n+i*2])), float64(repairMNumber(a[4+n+i*2]))
			bucket, ok := p.buckets[key]
			if !ok {
				bucket = repairMBucket{tokens: limit, at: now}
			}
			bucket.tokens = math.Min(limit, bucket.tokens+math.Max(0, now-bucket.at)*limit/span)
			bucket.at = now
			if bucket.tokens < 1 {
				retry = math.Max(retry, math.Ceil((1-bucket.tokens)*span/limit))
			}
			bucket.tokens--
			next[key] = bucket
		}
		if retry > 0 {
			return integer(int64(math.Max(1, math.Ceil(retry/1000))))
		}
		for key, bucket := range next {
			p.buckets[key] = bucket
		}
		return integer(0)
	default:
		return "-ERR unsupported fixture command\r\n"
	}
}

const repairMAbuseSHA = "d73c7abe1c7d25d5be8eda955b68668efd4bcf91"
