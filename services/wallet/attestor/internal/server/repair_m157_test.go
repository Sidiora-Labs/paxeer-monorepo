package server

import (
	"bufio"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/lxwire"
	"github.com/ethereum/go-ethereum/common"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRepairM_157_RejectedKeyLocksAreReclaimed(t *testing.T) {
	c := newTestCluster(t, 5, true)
	node := c.nodes[0]
	for i := 0; i < 1024; i++ {
		result := c.call(t, node, PathEVMReview, EVMReviewRequest{
			SessionID: fmt.Sprintf("rejected-%d", i), KeyID: fmt.Sprintf("unknown-%d", i), Transaction: "00",
		}, "untrusted-token")
		expectError(t, "unknown review key", result, CodeKeyNotFound)
	}
	node.server.keyMu.Lock()
	remaining := len(node.server.keyLocks)
	node.server.keyMu.Unlock()
	if remaining != 0 {
		t.Fatalf("%d rejected key locks remain resident", remaining)
	}
}

func TestRepairM_157_SameKeyHoldersAndWaitersShareLock(t *testing.T) {
	c := newTestCluster(t, 5, true)
	const keyID = "repair-m157-lock"
	importEd25519(t, c, keyID, "repair m157 held key lock")
	s := c.nodes[0].server
	unlock := s.lockKey(keyID)
	const waiters = 32
	var wg sync.WaitGroup
	var active, overlaps atomic.Int32
	for i := 0; i < waiters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release := s.lockKey(keyID)
			defer release()
			if active.Add(1) != 1 {
				overlaps.Add(1)
			}
			defer active.Add(-1)
			if _, err := s.opts.Store.Get(keyID); err != nil {
				t.Error(err)
			}
		}()
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.keyMu.Lock()
		refs := s.keyLocks[keyID].refs
		s.keyMu.Unlock()
		if refs == waiters+1 {
			break
		}
		if time.Now().After(deadline) {
			unlock()
			wg.Wait()
			t.Fatalf("only %d references registered for %d queued waiters", refs, waiters)
		}
		time.Sleep(time.Millisecond)
	}
	unlock()
	wg.Wait()
	if overlaps.Load() != 0 {
		t.Fatalf("same-key critical sections overlapped %d times", overlaps.Load())
	}
	s.keyMu.Lock()
	remaining := len(s.keyLocks)
	s.keyMu.Unlock()
	if remaining != 0 {
		t.Fatalf("%d locks remain after every holder and waiter released", remaining)
	}
}

func TestRepairM_157_BodyReadDeadlineAndProtocolBudget(t *testing.T) {
	const bound = 250 * time.Millisecond
	c := newTestClusterWith(t, 5, true, testPolicy(), func(o *Options) {
		o.APIReadTimeout = bound
		o.RoundTimeout = 5 * time.Second
		o.ProtocolTimeout = 10 * time.Second
	})
	for _, drip := range []bool{false, true} {
		t.Run(fmt.Sprintf("drip=%t", drip), func(t *testing.T) {
			config := c.client.Transport.(*http.Transport).TLSClientConfig.Clone()
			conn, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", c.nodes[0].apiAddr, config)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(bound + time.Second)); err != nil {
				t.Fatal(err)
			}
			started := time.Now()
			if _, err := fmt.Fprintf(conn, "POST %s HTTP/1.1\r\nHost: localhost\r\nContent-Length: 128\r\nContent-Type: application/json\r\n\r\n{", PathSign); err != nil {
				t.Fatal(err)
			}
			stop := make(chan struct{})
			var writer sync.WaitGroup
			if drip {
				writer.Add(1)
				go func() {
					defer writer.Done()
					ticker := time.NewTicker(25 * time.Millisecond)
					defer ticker.Stop()
					for {
						select {
						case <-stop:
							return
						case <-ticker.C:
							if _, err := conn.Write([]byte(" ")); err != nil {
								return
							}
						}
					}
				}()
			}
			defer func() { close(stop); writer.Wait() }()
			reader := bufio.NewReader(conn)
			response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodPost})
			if err != nil {
				t.Fatalf("stalled body did not receive a bounded refusal: %v", err)
			}
			defer response.Body.Close()
			var failure errorBody
			if err := json.NewDecoder(response.Body).Decode(&failure); err != nil {
				t.Fatal(err)
			}
			if failure.Error == nil || failure.Error.Code != CodeSessionBadRequest || !response.Close {
				t.Fatalf("stalled body response: status=%d close=%t error=%+v", response.StatusCode, response.Close, failure.Error)
			}
			if _, err := io.Copy(io.Discard, response.Body); err != nil {
				t.Fatal(err)
			}
			if _, err := reader.ReadByte(); err != io.EOF {
				t.Fatalf("server retained the stalled connection: %v", err)
			}
			if elapsed := time.Since(started); elapsed > bound+time.Second {
				t.Fatalf("body connection survived its read budget: %s", elapsed)
			}
		})
	}
	pub, account := importEd25519(t, c, "repair-m157-budget", "repair m157 independent protocol budget")
	signers := []string{"node-1", "node-2", "node-3"}
	bind := lxwire.BindMessage(testChainID, common.HexToAddress(account), 1)
	request := SignRequest{SessionID: "repair-m157-delayed-protocol", KeyID: "repair-m157-budget", Kind: KindLXBind, Signers: signers, Message: hex.EncodeToString(bind)}
	token := c.idp.mint(t, c.idp.key, testOwner)
	started := time.Now()
	answers := make(chan []apiResult, 1)
	go func() {
		answers <- c.callAll(t, c.byID("node-1", "node-2"), PathSign, func(*testNode) any { return request }, token)
	}()
	// The third real participant joins after the first two body deadlines.
	time.Sleep(2 * bound)
	third := c.call(t, c.byID("node-3")[0], PathSign, request, token)
	results := append(<-answers, third)
	verifyBind(t, "sign after the body deadline", pub, account, results)
	if time.Since(started) <= bound {
		t.Fatal("the protocol did not outlive the body budget")
	}
	if c.nodes[0].server.opts.ProtocolTimeout <= c.nodes[0].server.opts.APIReadTimeout {
		t.Fatal("the protocol budget was shortened to the body budget")
	}
}

func TestRepairM_157_ChangedTransactionRetainsSessionIdentity(t *testing.T) {
	testChangedTransactionRetainsSessionIdentity(t)
}
