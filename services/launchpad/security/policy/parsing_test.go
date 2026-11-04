package policy

import (
	"bufio"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestProductionParsingExactBounds(t *testing.T) {
	var dst struct {
		Values []int `json:"values"`
	}
	schema := MutationSchema{BodyBytes: 65536, Arrays: map[string]int{"/values": 2}}
	for _, body := range []string{`{"values":[1,2,3]}`, `{"values":[],"unknown":1}`} {
		if DecodeMutationSchema(httptest.NewRequest("POST", "/", strings.NewReader(body)), &dst, schema) == nil {
			t.Fatal("invalid schema accepted")
		}
	}
	err := DecodeMutationSchema(httptest.NewRequest("POST", "/", strings.NewReader(strings.Repeat(" ", 65537))), &dst, schema)
	if err == nil || MutationStatus(err) != 413 {
		t.Fatal("oversized mutation", err)
	}
	if err = DecodeMutationSchema(httptest.NewRequest("POST", "/", strings.NewReader(`{"values":[1,2]}`)), &dst, schema); err != nil {
		t.Fatal(err)
	}
	server := ProductionServer("127.0.0.1:0", http.NotFoundHandler())
	if server.ReadHeaderTimeout != 5*time.Second || server.IdleTimeout != 60*time.Second || server.MaxHeaderBytes != 16384 {
		t.Fatal("server bounds")
	}
	record := httptest.NewRecorder()
	RequestBounds(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("oversized query reached business handler") })).ServeHTTP(record, httptest.NewRequest("GET", "/?q="+strings.Repeat("a", 8193), nil))
	if record.Code != 414 {
		t.Fatal(record.Code)
	}
}

func TestRejectedMutationLeavesDestinationUnchanged(t *testing.T) {
	for _, body := range []string{`{"values":[1,2,3]}`, `{"values":[1],"unknown":true}`, `{"values":["invalid"]}`} {
		dst := struct {
			Values []int `json:"values"`
		}{Values: []int{99}}
		request := httptest.NewRequest("POST", "/", strings.NewReader(body))
		if DecodeMutationSchema(request, &dst, MutationSchema{BodyBytes: 65536, Arrays: map[string]int{"/values": 2}}) == nil {
			t.Fatal("invalid mutation accepted")
		}
		if len(dst.Values) != 1 || dst.Values[0] != 99 {
			t.Fatal("failed decode changed caller state", dst)
		}
	}
	var dst struct{ Name string }
	dst.Name = "preserved"
	if DecodeMutation(httptest.NewRequest("POST", "/", strings.NewReader(`{"Name":"changed","unknown":1}`)), &dst, 65536) == nil || dst.Name != "preserved" {
		t.Fatal("unknown field altered destination", dst)
	}
}

func TestOversizeHTTPMutationHasZeroDatabaseWrites(t *testing.T) {
	pool, schema := realSecurityDatabase(t)
	ctx := context.Background()
	table := pgx.Identifier{schema, "body_writes"}.Sanitize()
	if _, err := pool.Exec(ctx, "CREATE TABLE "+table+"(payload text NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var value struct {
			Payload string `json:"payload"`
		}
		if err := DecodeMutationSchema(r, &value, MutationSchema{BodyBytes: 65536}); err != nil {
			http.Error(w, "invalid body", MutationStatus(err))
			return
		}
		if _, err := pool.Exec(r.Context(), "INSERT INTO "+table+" VALUES($1)", value.Payload); err != nil {
			http.Error(w, "storage unavailable", 503)
			return
		}
		w.WriteHeader(204)
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	for _, size := range []int{65536, 65537} {
		body := `{"payload":"` + strings.Repeat("x", size-len(`{"payload":""}`)) + `"}`
		response, err := server.Client().Post(server.URL, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		want := 204
		if size > 65536 {
			want = 413
		}
		if response.StatusCode != want {
			t.Fatal(response.StatusCode, want)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil || count != 1 {
		t.Fatal("oversized request mutated database", count, err)
	}
}

func TestAcceptedMutationPreservesAbsentDefaults(t *testing.T) {
	dst := struct {
		Name   string `json:"name"`
		Values []int  `json:"values"`
	}{Name: "default", Values: []int{1}}
	if err := DecodeMutationSchema(httptest.NewRequest("PATCH", "/", strings.NewReader(`{"values":[2]}`)), &dst, MutationSchema{BodyBytes: 65536, Arrays: map[string]int{"/values": 2}}); err != nil {
		t.Fatal(err)
	}
	if dst.Name != "default" || len(dst.Values) != 1 || dst.Values[0] != 2 {
		t.Fatal("accepted patch changed absent defaults", dst)
	}
}

func TestRepairNestedMutationCaseAliases(t *testing.T) {
	var callbacks atomic.Int64
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dst := struct {
			Items []struct {
				Role string `json:"role"`
			} `json:"items"`
		}{}
		if err := DecodeMutationSchema(r, &dst, MutationSchema{BodyBytes: 65536, Arrays: map[string]int{"/items": 2}}); err != nil {
			w.WriteHeader(MutationStatus(err))
			return
		}
		callbacks.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	for _, body := range []string{
		`{"items":[{"role":"reader","Role":"admin"}]}`,
		`{"items":[{"role":"reader","\u0052ole":"admin"}]}`,
		`{"items":[{"role":"reader","\u0072ole":"admin"}]}`,
		`{"items":[],"Items":[{"role":"admin"}]}`,
	} {
		response, err := server.Client().Post(server.URL, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusBadRequest || callbacks.Load() != 0 {
			t.Fatal("case/escaped alias reached mutation callback", body, response.StatusCode, callbacks.Load())
		}
	}
	response, err := server.Client().Post(server.URL, "application/json", strings.NewReader(`{"items":[{"role":"reader"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent || callbacks.Load() != 1 {
		t.Fatal("canonical mutation did not reach callback exactly once", response.StatusCode, callbacks.Load())
	}
	type Embedded struct {
		Scope string `json:"scope"`
	}
	var canonical struct {
		Embedded
		Labels map[string]string `json:"labels"`
	}
	if err := DecodeMutation(httptest.NewRequest("POST", "/", strings.NewReader(`{"scope":"read","labels":{"role":"reader","Role":"label"}}`)), &canonical, 1024); err != nil || canonical.Scope != "read" || len(canonical.Labels) != 2 {
		t.Fatal("canonical embedded field or distinct map keys rejected", canonical, err)
	}
	for _, body := range []string{`{"Scope":"write"}`, `{"\u017fcope":"write"}`, `{"scope":"read","\u0053cope":"write"}`} {
		if err := DecodeMutation(httptest.NewRequest("POST", "/", strings.NewReader(body)), &canonical, 1024); err == nil || canonical.Scope != "read" {
			t.Fatal("noncanonical destination alias accepted or changed state", body, canonical, err)
		}
	}
}

func TestRepairActiveBodyDeadlinePreservesReadStreams(t *testing.T) {
	var callbacks atomic.Int64
	released := make(chan struct{}, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: open\n\n")
			w.(http.Flusher).Flush()
			time.Sleep(MutationRequestTimeout + 200*time.Millisecond)
			fmt.Fprint(w, "data: alive\n\n")
			return
		}
		defer func() { released <- struct{}{} }()
		var dst struct{ Role string }
		if err := DecodeMutation(r, &dst, 1024); err != nil {
			w.WriteHeader(http.StatusRequestTimeout)
			return
		}
		callbacks.Add(1)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := ProductionServer(listener.Addr().String(), handler)
	defer server.Close()
	go server.Serve(listener)
	client := &http.Client{Timeout: MutationRequestTimeout + 4*time.Second}
	stream, err := client.Get("http://" + listener.Addr().String() + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	connection, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	started := time.Now()
	if _, err := fmt.Fprint(connection, "POST /mutate HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: 100\r\n\r\n{\"Role\":"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-released:
	case <-time.After(MutationRequestTimeout + 2*time.Second):
		t.Fatal("slow body retained its handler past the mutation deadline")
	}
	if callbacks.Load() != 0 || time.Since(started) > MutationRequestTimeout+2*time.Second {
		t.Fatal("slow body reached mutation or exceeded request bound")
	}
	if err := connection.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: http.MethodPost})
	if err != nil {
		t.Fatal("bounded timeout response absent", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusRequestTimeout && response.StatusCode != http.StatusServiceUnavailable {
		t.Fatal("slow body did not receive a timeout", response.StatusCode)
	}
	body, err := io.ReadAll(stream.Body)
	if err != nil || string(body) != "data: open\n\ndata: alive\n\n" {
		t.Fatal("read stream was cut off by the mutation deadline", string(body), err)
	}
}
