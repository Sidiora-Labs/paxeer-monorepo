package fanout_test

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

// The production store uses pgx against this committed, transaction-aware wire
// peer. Only COMMIT publishes state; unknown SQL fails. No store method, codec,
// identity check or socket handler under repair is replaced.
type chatAdmissionConversation struct {
	Kind          string
	Next, Version int64
	Members       map[string]bool
}
type chatAdmissionData struct {
	Conversations map[string]*chatAdmissionConversation
	Messages      map[string]fanout.DurableMessage
	Outbox        map[string][]byte
	Channels      map[string][]string
	Delivered     map[string]bool
}

func chatAdmissionEmptyData() *chatAdmissionData {
	return &chatAdmissionData{Conversations: map[string]*chatAdmissionConversation{}, Messages: map[string]fanout.DurableMessage{}, Outbox: map[string][]byte{}, Channels: map[string][]string{}, Delivered: map[string]bool{}}
}
func (d *chatAdmissionData) clone() *chatAdmissionData {
	raw, err := json.Marshal(d)
	if err != nil {
		panic(err)
	}
	var result chatAdmissionData
	if err := json.Unmarshal(raw, &result); err != nil {
		panic(err)
	}
	return &result
}

type chatAdmissionPeer struct {
	mu   sync.Mutex
	data *chatAdmissionData
}
type chatAdmissionResult struct {
	rows [][]any
	tag  string
	err  error
}

func chatAdmissionRows(rows ...[]any) chatAdmissionResult {
	return chatAdmissionResult{rows: rows, tag: fmt.Sprintf("SELECT %d", len(rows))}
}
func chatAdmissionWrite(tag string) chatAdmissionResult { return chatAdmissionResult{tag: tag} }
func chatAdmissionFailure(message string) chatAdmissionResult {
	return chatAdmissionResult{err: errors.New(message)}
}

func newChatAdmissionDatabase(t *testing.T) (*chatAdmissionPeer, func() *pgxpool.Pool) {
	t.Helper()
	peer := &chatAdmissionPeer{data: chatAdmissionEmptyData()}
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
	return peer, open
}

// pgx's Exec mode uses text parameters without a prepared-statement cache.
// Values returned by the peer carry their real PostgreSQL scalar OIDs, so pgx
// performs the same scanning/JSON conversions as against the service database.
func (p *chatAdmissionPeer) serve(conn net.Conn) {
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
	var result *chatAdmissionResult
	var transaction *chatAdmissionData
	dirty := false
	failed := false
	evaluate := func(query string, parameters []string) chatAdmissionResult {
		p.mu.Lock()
		defer p.mu.Unlock()
		normalized := strings.TrimSpace(query)
		switch {
		case strings.HasPrefix(strings.ToLower(normalized), "begin"):
			if transaction != nil {
				return chatAdmissionFailure("nested transaction")
			}
			transaction = p.data.clone()
			dirty = false
			return chatAdmissionWrite("BEGIN")
		case strings.EqualFold(normalized, "commit"):
			if transaction == nil {
				return chatAdmissionFailure("commit without transaction")
			}
			if dirty {
				p.data = transaction
			}
			transaction = nil
			dirty = false
			return chatAdmissionWrite("COMMIT")
		case strings.EqualFold(normalized, "rollback"):
			transaction = nil
			dirty = false
			return chatAdmissionWrite("ROLLBACK")
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
	describe := func(r chatAdmissionResult) {
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
	emit := func(r chatAdmissionResult) {
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
func chatAdmissionNumber(raw string) int64 {
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		panic(err)
	}
	return n
}
func chatAdmissionTime(raw string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02 15:04:05.999999999-07:00:00", "2006-01-02 15:04:05.999999999-07"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	panic("invalid fixture time: " + raw)
}
func chatAdmissionBytes(raw string) []byte {
	if strings.HasPrefix(raw, `\x`) {
		b, err := hex.DecodeString(raw[2:])
		if err != nil {
			panic(err)
		}
		return b
	}
	return []byte(raw)
}
func chatAdmissionMessageRow(m fanout.DurableMessage) []any {
	return []any{m.ID, m.ConversationID, m.Sender, m.Content, m.ReplyTo, m.Sequence, m.Revision, m.CreatedAt, m.Deleted, m.ClientID}
}
func (d *chatAdmissionData) execute(q string, a []string, user string) chatAdmissionResult {
	// sqlc includes a name comment; strip comments without changing its SQL.
	for strings.HasPrefix(q, "--") {
		i := strings.IndexByte(q, '\n')
		if i < 0 {
			return chatAdmissionFailure("SQL comment without query")
		}
		q = strings.TrimSpace(q[i+1:])
	}
	q = strings.Join(strings.Fields(q), " ")
	get := func(i int) string {
		if i >= len(a) {
			panic(fmt.Sprintf("missing parameter %d: %s", i, q))
		}
		return a[i]
	}
	exists := func(value bool) chatAdmissionResult { return chatAdmissionRows([]any{value}) }
	member := func(conv, wallet string) bool { c := d.Conversations[conv]; return c != nil && c.Members[wallet] }
	switch {
	case strings.HasPrefix(q, "SELECT pg_advisory_xact_lock"):
		return chatAdmissionRows([]any{nil})
	case strings.HasPrefix(q, "SELECT EXISTS(SELECT 1 FROM chat.chat_bans"), strings.HasPrefix(q, "SELECT EXISTS(SELECT 1 FROM chat.interaction_erased_wallets"):
		return exists(false)
	case q == "SELECT lower(token),lower(creator) FROM indexer.indexer_canonical_creations_v1 WHERE pool=lower($1)":
		if get(0) != addr(1) {
			return chatAdmissionRows()
		}
		return chatAdmissionRows([]any{addr(2), addr(0xbb)})
	case q == "SELECT EXISTS(SELECT 1 FROM chat_conversations WHERE id=$1)":
		return exists(d.Conversations[get(0)] != nil)
	case strings.HasPrefix(q, "INSERT INTO chat_conversations"):
		id := get(0)
		kind := "pool"
		if len(a) > 1 {
			kind = get(1)
		}
		if d.Conversations[id] != nil {
			return chatAdmissionWrite("INSERT 0 0")
		}
		d.Conversations[id] = &chatAdmissionConversation{Kind: kind, Version: 1, Members: map[string]bool{}}
		if strings.Contains(q, "RETURNING") {
			return chatAdmissionRows([]any{id})
		}
		return chatAdmissionWrite("INSERT 0 1")
	case strings.HasPrefix(q, "INSERT INTO chat_members"):
		c := d.Conversations[get(0)]
		if c == nil {
			return chatAdmissionFailure("conversation foreign key")
		}
		for _, w := range a[1:] {
			c.Members[w] = true
		}
		return chatAdmissionWrite("INSERT 0 1")
	case strings.HasPrefix(q, "SELECT EXISTS(SELECT 1 FROM chat_members"):
		return exists(member(get(0), get(1)))
	case strings.HasPrefix(q, "SELECT wallet FROM chat_members"):
		c := d.Conversations[get(0)]
		if c == nil {
			return chatAdmissionRows()
		}
		if strings.Contains(q, "wallet=$2") {
			if c.Members[get(1)] {
				return chatAdmissionRows([]any{get(1)})
			}
			return chatAdmissionRows()
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
		return chatAdmissionRows(rows...)
	case strings.HasPrefix(q, "UPDATE chat_conversations SET next_sequence=next_sequence+1"):
		c := d.Conversations[get(0)]
		if c == nil {
			return chatAdmissionRows()
		}
		c.Next++
		return chatAdmissionRows([]any{c.Next})
	case strings.HasPrefix(q, "UPDATE chat_conversations SET history_version=history_version+1"):
		c := d.Conversations[get(0)]
		if c == nil {
			return chatAdmissionFailure("conversation absent")
		}
		c.Version++
		return chatAdmissionWrite("UPDATE 1")
	case strings.HasPrefix(q, "SELECT kind FROM chat_conversations"):
		c := d.Conversations[get(0)]
		if c == nil {
			return chatAdmissionRows()
		}
		return chatAdmissionRows([]any{c.Kind})
	case strings.HasPrefix(q, "SELECT retained_from,kind,history_version,m.read_sequence"):
		c := d.Conversations[get(0)]
		if c == nil || !member(get(0), get(1)) {
			return chatAdmissionRows()
		}
		return chatAdmissionRows([]any{int64(1), c.Kind, c.Version, int64(0)})
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
			if int64(len(rows)) == chatAdmissionNumber(get(1)) {
				break
			}
			rows = append(rows, []any{id})
		}
		return chatAdmissionRows(rows...)
	case strings.HasPrefix(q, "SELECT peer.wallet,extract(epoch from max(msg.created_at))"):
		c := d.Conversations[get(0)]
		if c == nil || !member(get(0), get(1)) {
			return chatAdmissionRows()
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
			return chatAdmissionRows()
		}
		return chatAdmissionRows([]any{peer, last})
	case strings.HasPrefix(q, "INSERT INTO chat_messages"):
		m := fanout.DurableMessage{ID: get(0), ConversationID: get(1), Sender: get(2), ClientID: get(3), Content: get(4), ReplyTo: get(5), Sequence: chatAdmissionNumber(get(6)), CreatedAt: chatAdmissionTime(get(7)).Unix(), Revision: 1}
		if _, ok := d.Messages[m.ID]; ok {
			return chatAdmissionFailure("duplicate message")
		}
		d.Messages[m.ID] = m
		return chatAdmissionWrite("INSERT 0 1")
	case strings.HasPrefix(q, "UPDATE chat_messages SET content=$2"):
		m, ok := d.Messages[get(0)]
		if !ok {
			return chatAdmissionWrite("UPDATE 0")
		}
		m.Content = get(1)
		m.Revision = chatAdmissionNumber(get(2))
		m.Deleted = get(4) == "true" || get(4) == "t"
		if m.Deleted {
			m.ReplyTo = ""
		}
		d.Messages[m.ID] = m
		return chatAdmissionWrite("UPDATE 1")
	case strings.HasPrefix(q, "SELECT EXISTS(SELECT 1 FROM chat_messages"):
		m, ok := d.Messages[get(0)]
		return exists(ok && m.ConversationID == get(1))
	case strings.HasPrefix(q, "SELECT id,conversation_id,sender,content,reply_to,sequence,revision,extract(epoch from created_at)::bigint,deleted_at IS NOT NULL,client_id FROM chat_messages"):
		rows := [][]any{}
		switch {
		case strings.Contains(q, "WHERE sender=$1 AND client_id=$2"):
			for _, m := range d.Messages {
				if m.Sender == get(0) && m.ClientID == get(1) {
					rows = append(rows, chatAdmissionMessageRow(m))
				}
			}
		case strings.Contains(q, "WHERE conversation_id=$1 AND sequence>$2"):
			messages := []fanout.DurableMessage{}
			for _, m := range d.Messages {
				if m.ConversationID == get(0) && m.Sequence > chatAdmissionNumber(get(1)) && member(get(0), get(2)) {
					messages = append(messages, m)
				}
			}
			sort.Slice(messages, func(i, j int) bool { return messages[i].Sequence < messages[j].Sequence })
			for _, m := range messages {
				if int64(len(rows)) == chatAdmissionNumber(get(3)) {
					break
				}
				rows = append(rows, chatAdmissionMessageRow(m))
			}
		case strings.Contains(q, "WHERE id=$1"):
			m, ok := d.Messages[get(0)]
			if ok && (!strings.Contains(q, "sender=$2") || m.Sender == get(1) && member(m.ConversationID, get(1))) {
				rows = append(rows, chatAdmissionMessageRow(m))
			}
		default:
			return chatAdmissionFailure("unsupported message selection: " + q)
		}
		return chatAdmissionRows(rows...)
	case strings.HasPrefix(q, "INSERT INTO chat_outbox"):
		if _, ok := d.Outbox[get(0)]; ok {
			return chatAdmissionFailure("duplicate outbox event")
		}
		d.Outbox[get(0)] = chatAdmissionBytes(get(3))
		var channels []string
		if err := json.Unmarshal(chatAdmissionBytes(get(4)), &channels); err != nil {
			return chatAdmissionFailure(err.Error())
		}
		d.Channels[get(0)] = channels
		return chatAdmissionWrite("INSERT 0 1")
	case strings.HasPrefix(q, "SELECT payload FROM chat_outbox WHERE message_id=$1 AND revision=1"):
		raw, ok := d.Outbox[get(0)+":1"]
		if !ok {
			return chatAdmissionRows()
		}
		return chatAdmissionRows([]any{raw})
	case strings.HasPrefix(q, "SELECT event_id,payload,channels FROM chat_outbox WHERE delivered_at IS NULL"):
		ids := []string{}
		for id := range d.Outbox {
			if !d.Delivered[id] {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		if len(ids) == 0 {
			return chatAdmissionRows()
		}
		id := ids[0]
		ch, err := json.Marshal(d.Channels[id])
		if err != nil {
			return chatAdmissionFailure(err.Error())
		}
		return chatAdmissionRows([]any{id, d.Outbox[id], ch})
	case strings.HasPrefix(q, "UPDATE chat_outbox SET delivered_at=now(),attempts=attempts+1 WHERE event_id=$1"):
		d.Delivered[get(0)] = true
		return chatAdmissionWrite("UPDATE 1")
	default:
		return chatAdmissionFailure("unsupported fixture SQL: " + q)
	}
}

// This RESP2 peer retains counters and deadlines across client/hub restarts.
// MULTI queues commands and EXEC applies them under one lock, including Redis's
// per-command errors (a failed command does not roll back earlier commands).
// The safety script has a pinned transcript identity and token-bucket state.
type chatAdmissionBucket struct{ tokens, at float64 }
type chatAdmissionRedis struct {
	mu                       sync.Mutex
	now                      time.Time
	counters                 map[string]int64
	expires                  map[string]time.Time
	buckets                  map[string]chatAdmissionBucket
	peers                    map[*chatAdmissionRedisConn]bool
	addr                     string
	interrupt                string
	interruptKey             string
	refuseExpiry             bool
	unavailable              bool
	abuseUnavailable         bool
	transactions, abuseCalls int
	scripts                  map[string]string
}
type chatAdmissionRedisConn struct {
	net.Conn
	writeMu sync.Mutex
	topics  map[string]bool // guarded by the peer's mutex
}

func (c *chatAdmissionRedisConn) write(raw string) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, _ = io.WriteString(c.Conn, raw)
}
func newChatAdmissionRedis(t *testing.T) *chatAdmissionRedis {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &chatAdmissionRedis{now: time.Now(), counters: map[string]int64{}, expires: map[string]time.Time{}, buckets: map[string]chatAdmissionBucket{}, peers: map[*chatAdmissionRedisConn]bool{}, scripts: map[string]string{}, addr: listener.Addr().String()}
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
			c := &chatAdmissionRedisConn{Conn: conn, topics: map[string]bool{}}
			p.mu.Lock()
			p.peers[c] = true
			p.mu.Unlock()
			go p.serve(c)
		}
	}()
	return p
}
func (p *chatAdmissionRedis) client(t *testing.T) *goredis.Client {
	t.Helper()
	c := goredis.NewClient(&goredis.Options{Addr: p.addr, Protocol: 2, DisableIdentity: true, MaxRetries: -1, DialTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second})
	t.Cleanup(func() { _ = c.Close() })
	return c
}
func chatAdmissionRESP(args ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(arg), arg)
	}
	return b.String()
}
func chatAdmissionReadRESP(r *bufio.Reader) ([]string, error) {
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
func (p *chatAdmissionRedis) serve(c *chatAdmissionRedisConn) {
	defer c.Close()
	defer func() { p.mu.Lock(); delete(p.peers, c); p.mu.Unlock() }()
	reader := bufio.NewReader(c.Conn)
	var queued [][]string
	multi := false
	for {
		args, err := chatAdmissionReadRESP(reader)
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
					subscriber.write(chatAdmissionRESP("message", args[1], args[2]))
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
func (p *chatAdmissionRedis) expire(key string) {
	if until, ok := p.expires[key]; ok && !until.After(p.now) {
		delete(p.counters, key)
		delete(p.expires, key)
	}
}
func (p *chatAdmissionRedis) execute(a []string) string {
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
		p.expires[a[1]] = p.now.Add(time.Duration(chatAdmissionNumber(a[2])) * time.Millisecond)
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
		if hex.EncodeToString(digest[:]) != chatAdmissionAbuseSHA {
			return "-ERR unrecorded script\r\n"
		}
		p.scripts[hex.EncodeToString(digest[:])] = script
		p.abuseCalls++
		if p.abuseUnavailable {
			return "-ERR abuse authority unavailable\r\n"
		}
		n := int(chatAdmissionNumber(a[2]))
		if len(a) != 3+n*3 {
			return "-ERR script argument mismatch\r\n"
		}
		now := float64(p.now.UnixMilli())
		retry := float64(0)
		next := make(map[string]chatAdmissionBucket, n)
		for i, key := range a[3 : 3+n] {
			limit, span := float64(chatAdmissionNumber(a[3+n+i*2])), float64(chatAdmissionNumber(a[4+n+i*2]))
			bucket, ok := p.buckets[key]
			if !ok {
				bucket = chatAdmissionBucket{tokens: limit, at: now}
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

const chatAdmissionAbuseSHA = "d73c7abe1c7d25d5be8eda955b68668efd4bcf91"
