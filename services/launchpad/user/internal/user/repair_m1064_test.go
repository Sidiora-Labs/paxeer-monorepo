package user

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	goredis "github.com/redis/go-redis/v9"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/pkg/types"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/storage"
)

// The handler is the production component under test. Its peers below are
// committed in-memory row/object stores and loopback RESP/clamd transcripts.
type repairRows1064 struct {
	mu       sync.Mutex
	profiles map[string]sqlcdb.UsersUserProfile
	images   map[string]sqlcdb.UsersUserImage
	watch    map[string]map[string]int64
	writes   int
	reads    int
}

func (s *repairRows1064) UpsertUserProfile(_ context.Context, p sqlcdb.UpsertUserProfileParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, exists := s.profiles[p.WalletAddress]
	created := p.CreatedAt
	if exists {
		created = old.CreatedAt
	}
	s.profiles[p.WalletAddress] = sqlcdb.UsersUserProfile{WalletAddress: p.WalletAddress, DisplayName: p.DisplayName, Bio: p.Bio, Twitter: p.Twitter, Telegram: p.Telegram, Discord: p.Discord, Website: p.Website, CreatedAt: created, UpdatedAt: p.UpdatedAt}
	s.writes++
	return nil
}
func (s *repairRows1064) GetUserProfile(_ context.Context, addr string) (sqlcdb.UsersUserProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads++
	p, ok := s.profiles[addr]
	if !ok {
		return p, pgx.ErrNoRows
	}
	return p, nil
}
func (s *repairRows1064) UpsertUserImage(_ context.Context, p sqlcdb.UpsertUserImageParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.images[p.WalletAddress+":"+p.ImageType] = sqlcdb.UsersUserImage{ID: p.ID, WalletAddress: p.WalletAddress, ImageType: p.ImageType, StorageKey: p.StorageKey, MimeType: p.MimeType, SizeBytes: p.SizeBytes, UploadedAt: p.UploadedAt}
	s.writes++
	return nil
}
func (s *repairRows1064) GetUserImages(_ context.Context, addr string) ([]sqlcdb.UsersUserImage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []sqlcdb.UsersUserImage{}
	for _, v := range s.images {
		if v.WalletAddress == addr {
			out = append(out, v)
		}
	}
	return out, nil
}
func (s *repairRows1064) GetImageByType(_ context.Context, p sqlcdb.GetImageByTypeParams) (sqlcdb.UsersUserImage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.images[p.WalletAddress+":"+p.ImageType]
	if !ok {
		return v, pgx.ErrNoRows
	}
	return v, nil
}
func (s *repairRows1064) ListCreatedPools(context.Context, string) ([]sqlcdb.ListCreatedPoolsRow, error) {
	return []sqlcdb.ListCreatedPoolsRow{}, nil
}
func (s *repairRows1064) AddWatchlist(_ context.Context, p sqlcdb.AddWatchlistParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.watch[p.WalletAddress] == nil {
		s.watch[p.WalletAddress] = map[string]int64{}
	}
	if _, ok := s.watch[p.WalletAddress][p.PoolAddress]; !ok {
		s.watch[p.WalletAddress][p.PoolAddress] = p.AddedAt
	}
	s.writes++
	return nil
}
func (s *repairRows1064) RemoveWatchlist(_ context.Context, p sqlcdb.RemoveWatchlistParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.watch[p.WalletAddress], p.PoolAddress)
	s.writes++
	return nil
}
func (s *repairRows1064) GetWatchlist(_ context.Context, addr string) ([]sqlcdb.GetWatchlistRow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []sqlcdb.GetWatchlistRow{}
	for pool, ts := range s.watch[addr] {
		out = append(out, sqlcdb.GetWatchlistRow{PoolAddress: pool, AddedAt: ts})
	}
	return out, nil
}

type repairObjects1064 struct {
	mu      sync.Mutex
	objects map[string][]byte
	puts    int
}

func (s *repairObjects1064) Put(_ context.Context, key string, body io.Reader, size int64, _ string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if int64(len(data)) != size {
		return fmt.Errorf("object size mismatch")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = bytes.Clone(data)
	s.puts++
	return nil
}
func (s *repairObjects1064) Get(_ context.Context, key string) (*storage.Object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.objects[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return &storage.Object{Body: io.NopCloser(bytes.NewReader(bytes.Clone(data))), Size: int64(len(data))}, nil
}

type repairRedisValue1064 struct {
	data    string
	expires time.Time
}
type repairRedis1064 struct {
	mu     sync.Mutex
	values map[string]repairRedisValue1064
	hits   int
	claims int
}

func repairRedisClient1064(t *testing.T) (*goredis.Client, *repairRedis1064) {
	t.Helper()
	state := &repairRedis1064{values: map[string]repairRedisValue1064{}}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go state.serve(conn)
		}
	}()
	client := goredis.NewClient(&goredis.Options{Addr: ln.Addr().String(), Protocol: 2, DisableIdentity: true, MaxRetries: -1})
	t.Cleanup(func() { client.Close() })
	return client, state
}
func (s *repairRedis1064) serve(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		count, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "*")))
		if err != nil || count < 1 || count > 20 {
			return
		}
		args := make([]string, count)
		for i := range args {
			line, err = r.ReadString('\n')
			if err != nil {
				return
			}
			n, e := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "$")))
			if e != nil || n < 0 || n > 2<<20 {
				return
			}
			b := make([]byte, n+2)
			if _, err = io.ReadFull(r, b); err != nil {
				return
			}
			args[i] = string(b[:n])
		}
		s.mu.Lock()
		now := time.Now()
		for k, v := range s.values {
			if !v.expires.IsZero() && !v.expires.After(now) {
				delete(s.values, k)
			}
		}
		response := ""
		switch strings.ToUpper(args[0]) {
		case "HELLO":
			response = "-ERR unknown command 'hello'\r\n"
		case "CLIENT":
			response = "+OK\r\n"
		case "PING":
			response = "+PONG\r\n"
		case "GET":
			v, ok := s.values[args[1]]
			if ok {
				s.hits++
				response = fmt.Sprintf("$%d\r\n%s\r\n", len(v.data), v.data)
			} else {
				response = "$-1\r\n"
			}
		case "SET":
			nx := false
			expires := time.Time{}
			for i := 3; i < len(args); i++ {
				switch strings.ToUpper(args[i]) {
				case "NX":
					nx = true
				case "EX", "PX":
					unit := time.Second
					if strings.EqualFold(args[i], "PX") {
						unit = time.Millisecond
					}
					i++
					n, _ := strconv.ParseInt(args[i], 10, 64)
					expires = now.Add(time.Duration(n) * unit)
				}
			}
			_, exists := s.values[args[1]]
			if nx && exists {
				response = "$-1\r\n"
			} else {
				s.values[args[1]] = repairRedisValue1064{args[2], expires}
				response = "+OK\r\n"
				if nx {
					s.claims++
				}
			}
		case "DEL":
			deleted := 0
			for _, key := range args[1:] {
				if _, ok := s.values[key]; ok {
					delete(s.values, key)
					deleted++
				}
			}
			response = fmt.Sprintf(":%d\r\n", deleted)
		default:
			response = "-ERR unsupported fixture command\r\n"
		}
		s.mu.Unlock()
		if _, err := io.WriteString(conn, response); err != nil {
			return
		}
	}
}

type repairFixture1064 struct {
	t         *testing.T
	key       *ecdsa.PrivateKey
	wallet    string
	deps      Deps
	rows      *repairRows1064
	objects   *repairObjects1064
	redis     *repairRedis1064
	scanned   atomic.Int64
	scanBytes atomic.Int64
	server    *httptest.Server
}

func newRepairFixture1064(t *testing.T) *repairFixture1064 {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	f := &repairFixture1064{t: t, key: key, wallet: strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex()), rows: &repairRows1064{profiles: map[string]sqlcdb.UsersUserProfile{}, images: map[string]sqlcdb.UsersUserImage{}, watch: map[string]map[string]int64{}}, objects: &repairObjects1064{objects: map[string][]byte{}}}
	client, state := repairRedisClient1064(t)
	f.redis = state
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(3 * time.Second))
				header := make([]byte, len("zINSTREAM\x00"))
				if _, err := io.ReadFull(conn, header); err != nil || string(header) != "zINSTREAM\x00" {
					return
				}
				var total int64
				for {
					var size [4]byte
					if _, err := io.ReadFull(conn, size[:]); err != nil {
						return
					}
					n := binary.BigEndian.Uint32(size[:])
					if n == 0 {
						break
					}
					if n > 65536 {
						return
					}
					if _, err := io.CopyN(io.Discard, conn, int64(n)); err != nil {
						return
					}
					total += int64(n)
				}
				f.scanned.Add(1)
				f.scanBytes.Add(total)
				io.WriteString(conn, "stream: OK\x00")
			}()
		}
	}()
	f.deps = Deps{Queries: f.rows, Redis: client, Store: f.objects, MaxAvatar: 1024, MaxBanner: 2048, Scan: func(ctx context.Context, data []byte) sharedhttp.ScanResult {
		return sharedhttp.ScanBuffer(ctx, data, sharedhttp.ScanOptions{Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port, Timeout: time.Second})
	}}
	f.reopen()
	return f
}
func (f *repairFixture1064) reopen() {
	if f.server != nil {
		f.server.Close()
	}
	router := chi.NewRouter()
	New(f.deps).RegisterRoutes(router)
	f.server = httptest.NewServer(router)
	f.t.Cleanup(f.server.Close)
}
func (f *repairFixture1064) headers(method, path, action, contentType string, body []byte, change func(*sharedauth.OperationEnvelope)) http.Header {
	f.t.Helper()
	intent := body
	if contentType != "" {
		intent = repairUploadIntent1064(contentType, body)
	}
	digest, err := sharedauth.BodyDigest(intent)
	if err != nil {
		f.t.Fatal(err)
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		f.t.Fatal(err)
	}
	now := time.Now().Unix()
	e := sharedauth.OperationEnvelope{Version: 2, ChainID: "125", Environment: "launchpad-user-legacy", Actor: f.wallet, Action: action, Method: method, Path: path, BodyHash: digest, Nonce: hex.EncodeToString(nonce), IssuedAt: now, ExpiresAt: now + 300}
	if change != nil {
		change(&e)
	}
	// Serialize the public wire statement directly so negative chain/envelope
	// cases can carry a genuine wallet signature over invalid consent as well.
	raw, err := json.Marshal(e)
	if err != nil {
		f.t.Fatal(err)
	}
	statement := fmt.Sprintf("KindleLaunch signed operation v%d\n%s", e.Version, raw)
	sig, err := crypto.Sign(accounts.TextHash([]byte(statement)), f.key)
	if err != nil {
		f.t.Fatal(err)
	}
	headers := http.Header{}
	headers.Set(sharedauth.OperationEnvelopeHeader, base64.RawURLEncoding.EncodeToString(raw))
	headers.Set(sharedauth.OperationSignatureHeader, hexutil.Encode(sig))
	if contentType != "" {
		headers.Set("Content-Type", contentType)
	} else {
		headers.Set("Content-Type", "application/json")
	}
	return headers
}

// This client-side producer deliberately computes the documented upload
// manifest independently of the production uploadIntent implementation.
func repairUploadIntent1064(contentType string, body []byte) []byte {
	sum := sha256.Sum256(body)
	data, _ := json.Marshal(struct {
		ContentType string `json:"content_type"`
		SHA256      string `json:"sha256"`
	}{contentType, hex.EncodeToString(sum[:])})
	return data
}
func (f *repairFixture1064) request(method, path string, body []byte, headers http.Header) (int, http.Header, []byte) {
	f.t.Helper()
	req, err := http.NewRequest(method, f.server.URL+path, bytes.NewReader(body))
	if err != nil {
		f.t.Fatal(err)
	}
	req.Header = headers.Clone()
	resp, err := f.server.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		f.t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, data
}
func repairWant1064(t *testing.T, code, want int, body []byte) {
	t.Helper()
	if code != want {
		t.Fatalf("status = %d, want %d: %s", code, want, body)
	}
}
func repairMultipart1064(t *testing.T, field, mimeType string, data []byte, extra int) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for i := 0; i <= extra; i++ {
		header := textproto.MIMEHeader{}
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename="image.png"`, field))
		header.Set("Content-Type", mimeType)
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), writer.FormDataContentType()
}

func TestRepairM_1064_LegacyWalletReplay(t *testing.T) {
	f := newRepairFixture1064(t)
	path := "/users/" + f.wallet
	body := []byte(`{"data":{"display_name":"first"}}`)
	headers := f.headers("POST", path, "profile.update", "", body, nil)
	code, _, out := f.request("POST", path, body, headers)
	repairWant1064(t, code, 200, out)
	f.reopen()
	code, _, out = f.request("POST", path, body, headers)
	repairWant1064(t, code, 403, out)
	for name, change := range map[string]func(*sharedauth.OperationEnvelope){
		"chain":       func(e *sharedauth.OperationEnvelope) { e.ChainID = "1" },
		"expired":     func(e *sharedauth.OperationEnvelope) { e.IssuedAt -= 400; e.ExpiresAt -= 400 },
		"future":      func(e *sharedauth.OperationEnvelope) { e.IssuedAt += 60; e.ExpiresAt += 60 },
		"environment": func(e *sharedauth.OperationEnvelope) { e.Environment = "other-service" },
		"actor":       func(e *sharedauth.OperationEnvelope) { e.Actor = "0x1111111111111111111111111111111111111111" },
	} {
		t.Run(name, func(t *testing.T) {
			h := f.headers("POST", path, "profile.update", "", body, change)
			code, _, out := f.request("POST", path, body, h)
			repairWant1064(t, code, 403, out)
		})
	}
	fresh := f.headers("POST", path, "profile.update", "", body, nil)
	code, _, out = f.request("POST", path, []byte(`{"data":{"display_name":"tampered"}}`), fresh)
	repairWant1064(t, code, 403, out)
	pool := "0x2222222222222222222222222222222222222222"
	watch := path + "/watchlist/" + pool
	code, _, out = f.request("PUT", watch, body, fresh)
	repairWant1064(t, code, 403, out)
	code, _, out = f.request("POST", path, body, fresh)
	repairWant1064(t, code, 200, out)
	legacy := []byte(`{"data":{"display_name":"stolen"},"message":"sign-in"}`)
	sig, err := crypto.Sign(accounts.TextHash([]byte("sign-in")), f.key)
	if err != nil {
		t.Fatal(err)
	}
	legacy = []byte(fmt.Sprintf(`{"data":{"display_name":"stolen"},"message":"sign-in","signature":%q}`, hexutil.Encode(sig)))
	code, _, out = f.request("POST", path, legacy, nil)
	repairWant1064(t, code, 400, out)
	code, _, out = f.request("GET", path, nil, nil)
	repairWant1064(t, code, 200, out)
	var profile types.PublicProfile
	if err := json.Unmarshal(out, &profile); err != nil {
		t.Fatal(err)
	}
	if profile.DisplayName == nil || *profile.DisplayName != "first" {
		t.Fatalf("profile changed after denied writes: %s", out)
	}
	empty := []byte(`{}`)
	add := f.headers("PUT", watch, "watchlist.update", "", empty, nil)
	code, _, out = f.request("PUT", watch, empty, add)
	repairWant1064(t, code, 200, out)
	code, _, out = f.request("PUT", watch, empty, add)
	repairWant1064(t, code, 403, out)
	code, _, out = f.request("DELETE", watch, empty, add)
	repairWant1064(t, code, 403, out)
	code, _, out = f.request("GET", path+"/watchlist", nil, nil)
	repairWant1064(t, code, 200, out)
	var list types.WatchlistResponse
	if err := json.Unmarshal(out, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Pools) != 1 || list.Pools[0].PoolAddress != pool {
		t.Fatalf("watchlist mutation escaped binding: %s", out)
	}
	remove := f.headers("DELETE", watch, "watchlist.remove", "", empty, nil)
	code, _, out = f.request("DELETE", watch, empty, remove)
	repairWant1064(t, code, 200, out)
	code, _, out = f.request("GET", path+"/watchlist", nil, nil)
	repairWant1064(t, code, 200, out)
	if err := json.Unmarshal(out, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Pools) != 0 {
		t.Fatalf("watchlist removal not visible: %s", out)
	}
	// A nonce remains single-use even when the wallet signs another action with it.
	var used sharedauth.OperationEnvelope
	raw, _ := base64.RawURLEncoding.DecodeString(add.Get(sharedauth.OperationEnvelopeHeader))
	if err := json.Unmarshal(raw, &used); err != nil {
		t.Fatal(err)
	}
	reused := f.headers("POST", path, "profile.update", "", body, func(e *sharedauth.OperationEnvelope) { e.Nonce = used.Nonce })
	code, _, out = f.request("POST", path, body, reused)
	repairWant1064(t, code, 403, out)
	f.deps.Redis = nil
	f.reopen()
	unavailable := f.headers("POST", path, "profile.update", "", body, nil)
	code, _, out = f.request("POST", path, body, unavailable)
	repairWant1064(t, code, 503, out)
}

func TestRepairM_1064_ConcurrentNonce(t *testing.T) {
	f := newRepairFixture1064(t)
	path := "/users/" + f.wallet
	body := []byte(`{"data":{"display_name":"once"}}`)
	h := f.headers("POST", path, "profile.update", "", body, nil)
	router := chi.NewRouter()
	New(f.deps).RegisterRoutes(router)
	second := httptest.NewServer(router)
	defer second.Close()
	var wg sync.WaitGroup
	codes := make(chan int, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			base := f.server.URL
			if i%2 == 0 {
				base = second.URL
			}
			req, err := http.NewRequest("POST", base+path, bytes.NewReader(body))
			if err != nil {
				codes <- 0
				return
			}
			req.Header = h.Clone()
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				codes <- 0
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			codes <- resp.StatusCode
		}(i)
	}
	wg.Wait()
	close(codes)
	accepted := 0
	for code := range codes {
		if code == 200 {
			accepted++
		} else if code != 403 {
			t.Fatalf("concurrent request status %d", code)
		}
	}
	if accepted != 1 || f.rows.writes != 1 {
		t.Fatalf("accepted=%d mutations=%d, require one", accepted, f.rows.writes)
	}
}

type repairCountingBody1064 struct {
	r io.Reader
	n int
}

func (b *repairCountingBody1064) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	b.n += n
	return n, err
}
func (b *repairCountingBody1064) Close() error { return nil }

func TestRepairM_1064_LegacyMultipartUnbounded(t *testing.T) {
	f := newRepairFixture1064(t)
	path := "/users/" + f.wallet + "/avatar"
	limit := int(f.deps.MaxAvatar) + multipartOverhead
	large, ct := repairMultipart1064(t, "avatar", "image/png", bytes.Repeat([]byte("a"), limit*2), 0)
	h := f.headers("POST", path, "image.avatar.update", ct, large, nil)
	router := chi.NewRouter()
	New(f.deps).RegisterRoutes(router)
	for _, known := range []bool{true, false} {
		t.Run(fmt.Sprintf("known-length-%t", known), func(t *testing.T) {
			counter := &repairCountingBody1064{r: bytes.NewReader(large)}
			req := httptest.NewRequest("POST", path, nil)
			req.Header = h.Clone()
			req.Body = counter
			req.ContentLength = -1
			if known {
				req.ContentLength = int64(len(large))
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			repairWant1064(t, rec.Code, 413, rec.Body.Bytes())
			if known && counter.n != 0 {
				t.Fatalf("known oversized request read %d bytes", counter.n)
			}
			if counter.n > limit+1 {
				t.Fatalf("read %d bytes exceeds budget %d", counter.n, limit+1)
			}
		})
	}
	// Bounded authorization is checked without consuming even a valid body.
	small, ct := repairMultipart1064(t, "avatar", "image/png", []byte("png"), 0)
	for name, headers := range map[string]http.Header{"missing": nil, "oversized": {sharedauth.OperationEnvelopeHeader: []string{strings.Repeat("a", 8193)}, sharedauth.OperationSignatureHeader: []string{strings.Repeat("a", 132)}}, "forged": f.headers("POST", path, "image.avatar.update", ct, small, func(e *sharedauth.OperationEnvelope) { e.Actor = "0x1111111111111111111111111111111111111111" })} {
		t.Run(name, func(t *testing.T) {
			counter := &repairCountingBody1064{r: bytes.NewReader(small)}
			req := httptest.NewRequest("POST", path, nil)
			req.Header = headers.Clone()
			req.Body = counter
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			want := 403
			if name == "missing" {
				want = 400
			}
			repairWant1064(t, rec.Code, want, rec.Body.Bytes())
			if counter.n != 0 {
				t.Fatalf("unauthenticated body read: %d", counter.n)
			}
		})
	}
	for name, tc := range map[string]struct {
		size, extra int
		field       string
		want        int
	}{
		"extra-part": {3, 1, "avatar", 400}, "many-parts": {1, 20, "file", 400}, "wrong-part": {3, 0, "unrelated", 400}, "selected-file-limit": {1025, 0, "avatar", 413},
	} {
		t.Run(name, func(t *testing.T) {
			body, ct := repairMultipart1064(t, tc.field, "image/png", bytes.Repeat([]byte("a"), tc.size), tc.extra)
			h := f.headers("POST", path, "image.avatar.update", ct, body, nil)
			code, _, out := f.request("POST", path, body, h)
			repairWant1064(t, code, tc.want, out)
		})
	}
	if f.scanned.Load() != 0 || f.objects.puts != 0 || f.rows.writes != 0 {
		t.Fatalf("rejected upload performed work: scans=%d puts=%d writes=%d", f.scanned.Load(), f.objects.puts, f.rows.writes)
	}
	// Rejection does not damage subsequent production scanner/storage use.
	h = f.headers("POST", path, "image.avatar.update", ct, small, nil)
	code, _, out := f.request("POST", path, small, h)
	repairWant1064(t, code, 200, out)
	if f.scanned.Load() != 1 || f.objects.puts != 1 || f.rows.writes != 1 {
		t.Fatal("bounded upload did not traverse scanner, object and row stores")
	}
}

func TestRepairM_1064_LegacyImageCacheStale(t *testing.T) {
	f := newRepairFixture1064(t)
	for _, kind := range []string{"avatar", "banner"} {
		t.Run(kind, func(t *testing.T) {
			path := "/users/" + f.wallet + "/" + kind
			var stable string
			for i, data := range [][]byte{[]byte("first-png"), []byte("replacement-png")} {
				body, ct := repairMultipart1064(t, kind, "image/png", data, 0)
				h := f.headers("POST", path, "image."+kind+".update", ct, body, nil)
				code, _, out := f.request("POST", path, body, h)
				repairWant1064(t, code, 200, out)
				var uploaded types.UploadResponse
				if err := json.Unmarshal(out, &uploaded); err != nil {
					t.Fatal(err)
				}
				if !uploaded.Success || uploaded.URL != path {
					t.Fatalf("upload response: %s", out)
				}
				if i == 0 {
					stable = uploaded.URL
				} else if uploaded.URL != stable {
					t.Fatal("expected stable revalidated URL")
				}
				code, _, out = f.request("GET", "/users/"+f.wallet, nil, nil)
				repairWant1064(t, code, 200, out)
				var profile types.PublicProfile
				if err := json.Unmarshal(out, &profile); err != nil {
					t.Fatal(err)
				}
				url := profile.Images.Avatar
				if kind == "banner" {
					url = profile.Images.Banner
				}
				if url == nil || *url != stable {
					t.Fatalf("profile does not expose public upload URL: %s", out)
				}
				readHeaders := http.Header{"If-None-Match": []string{`"old-object"`}, "If-Modified-Since": []string{time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)}}
				code, headers, out := f.request("GET", *url, nil, readHeaders)
				repairWant1064(t, code, 200, out)
				if !bytes.Equal(out, data) {
					t.Fatalf("replacement %d public bytes = %q, want %q", i, out, data)
				}
				for _, name := range []string{"Cache-Control", "CDN-Cache-Control"} {
					cc := headers.Get(name)
					if !strings.Contains(cc, "no-cache") || !strings.Contains(cc, "must-revalidate") || !strings.Contains(cc, "max-age=0") || strings.Contains(cc, "immutable") {
						t.Fatalf("%s allows stale bytes: %q", name, cc)
					}
				}
				if headers.Get("Content-Type") != "image/png" {
					t.Fatalf("public content type: %s", headers.Get("Content-Type"))
				}
				scans, puts, writes := f.scanned.Load(), f.objects.puts, f.rows.writes
				code, _, out = f.request("POST", path, body, h)
				repairWant1064(t, code, 403, out)
				other := "banner"
				if kind == "banner" {
					other = "avatar"
				}
				code, _, out = f.request("POST", "/users/"+f.wallet+"/"+other, body, h)
				repairWant1064(t, code, 403, out)
				changed := bytes.Replace(body, data, []byte("tampered-image"), 1)
				code, _, out = f.request("POST", path, changed, f.headers("POST", path, "image."+kind+".update", ct, body, nil))
				repairWant1064(t, code, 403, out)
				if f.scanned.Load() != scans || f.objects.puts != puts || f.rows.writes != writes {
					t.Fatal("replayed or rebound image reached scanner/storage")
				}
			}
		})
	}
	if f.scanned.Load() != 4 || f.scanBytes.Load() != int64(2*(len("first-png")+len("replacement-png"))) {
		t.Fatal("uploads did not traverse real ScanBuffer INSTREAM transport")
	}
}

func TestRepairM_1064_ProfileHostCachePoisoning(t *testing.T) {
	for _, canonical := range []string{"", "https://canonical.example/media"} {
		t.Run(canonical, func(t *testing.T) {
			f := newRepairFixture1064(t)
			f.deps.PublicURL = canonical
			f.reopen()
			path := "/users/" + f.wallet
			// Seed via the real signed upload route, then share the wallet cache across hosts.
			body, ct := repairMultipart1064(t, "avatar", "image/png", []byte("profile-image"), 0)
			code, _, out := f.request("POST", path+"/avatar", body, f.headers("POST", path+"/avatar", "image.avatar.update", ct, body, nil))
			repairWant1064(t, code, 200, out)
			// Prior-version host-derived cache entries must not survive the migration.
			poisoned := types.PublicProfile{WalletAddress: f.wallet, Images: types.Images{Avatar: strPtr("https://poison.example/image")}}
			raw, err := json.Marshal(poisoned)
			if err != nil {
				t.Fatal(err)
			}
			if err = f.deps.Redis.Set(context.Background(), "user:json:"+f.wallet, raw, time.Minute).Err(); err != nil {
				t.Fatal(err)
			}
			router := chi.NewRouter()
			New(f.deps).RegisterRoutes(router)
			var first []byte
			for i, host := range []string{"attacker.example", "viewer.example"} {
				req := httptest.NewRequest("GET", path, nil)
				req.Host = host
				req.Header.Set("X-Forwarded-Proto", "https")
				req.Header.Set("X-Forwarded-Host", "poison.example")
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				repairWant1064(t, rec.Code, 200, rec.Body.Bytes())
				var profile types.PublicProfile
				if err := json.Unmarshal(rec.Body.Bytes(), &profile); err != nil {
					t.Fatal(err)
				}
				if profile.Images.Avatar == nil || *profile.Images.Avatar != canonical+path+"/avatar" {
					t.Fatalf("host %s poisoned shared profile: %s", host, rec.Body.Bytes())
				}
				if i == 0 {
					first = bytes.Clone(rec.Body.Bytes())
				} else if !bytes.Equal(first, rec.Body.Bytes()) {
					t.Fatal("same wallet cache varies with host")
				}
			}
			f.redis.mu.Lock()
			hits := f.redis.hits
			f.redis.mu.Unlock()
			if hits != 1 || f.rows.reads != 1 {
				t.Fatalf("did not exercise shared wallet cache: hits=%d row reads=%d", hits, f.rows.reads)
			}
		})
	}
}
