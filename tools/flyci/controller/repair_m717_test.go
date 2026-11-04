package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func repair717Path(r *qualificationRegistry, id string, seq int64) string {
	return filepath.Join(r.root, id, fmt.Sprintf("%020d.json", seq))
}

func repair717Read(t *testing.T, path string) []byte {
	t.Helper()
	b, err := qRead(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func repair717Reopen(t *testing.T, root string) *qualificationRegistry {
	t.Helper()
	r, err := newQualificationRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// Stop at the real writer's filesystem boundaries. Closing descriptors models
// process loss without spawning a binary or running cleanup that a crash omits.
// Payloads come from records emitted by submit/advance in a reference registry.
func repair717Interrupt(t *testing.T, path string, payload []byte, stage string) {
	t.Helper()
	if stage == "before_staging" {
		return
	}
	f, err := qStageWrite(path)
	if err != nil {
		t.Fatal(err)
	}
	if !qStagedWrite(filepath.Base(f.Name())) {
		t.Fatal("writer did not use its recoverable staging namespace")
	}
	switch stage {
	case "before_writing":
		err = f.Close()
	case "during_writing":
		_, err = f.Write(payload[:len(payload)/2])
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
	case "before_publication", "after_publication":
		err = qPrepareWrite(f, payload)
		if err == nil && stage == "after_publication" {
			// Interrupt after rename and before the writer syncs the directory.
			err = qPublishWrite(f.Name(), path)
		}
	default:
		t.Fatalf("unknown interruption boundary %s", stage)
	}
	if err != nil {
		t.Fatal(err)
	}
	if stage != "after_publication" {
		if _, err = os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("uncommitted record became visible: %v", err)
		}
	} else if !bytes.Equal(repair717Read(t, path), payload) {
		t.Fatal("publication changed record bytes")
	}
}

func repair717Status(t *testing.T, r *qualificationRegistry, want qualificationRecord) {
	t.Helper()
	response := qHandle(r, qTestCanonical(t, qualificationControl{Operation: "status", LogicalID: want.Request.LogicalID}))
	if response.Error != "" || response.Record == nil || !reflect.DeepEqual(*response.Record, want) {
		t.Fatalf("recovered control response differs: %+v, want %+v", response, want)
	}
	entries, err := os.ReadDir(filepath.Join(r.root, want.Request.LogicalID))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != int(want.Sequence) {
		t.Fatalf("uncommitted tail remains: %v", entries)
	}
}

func TestRepairM_717_InterruptedInitialPublication(t *testing.T) {
	reference, req := qTestRegistry(t)
	prepared, err := reference.submit(req)
	if err != nil {
		t.Fatal(err)
	}
	payload := repair717Read(t, repair717Path(reference, req.LogicalID, 1))
	for _, stage := range []string{"before_staging", "before_writing", "during_writing", "before_publication", "after_publication"} {
		t.Run(stage, func(t *testing.T) {
			r, _ := qTestRegistry(t)
			other := qTestRequest(t)
			other.Manifest.EnvironmentSHA256 = strings.Repeat("d", 64)
			other.LogicalID = qHash(append([]byte("paxeer-x.logical.v1\x00"), qTestCanonical(t, other.Manifest)...))
			healthy, err := r.submit(other)
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(r.root, req.LogicalID)
			if err = os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err = qSyncDir(r.root); err != nil {
				t.Fatal(err)
			}
			repair717Interrupt(t, repair717Path(r, req.LogicalID, 1), payload, stage)
			root := r.root
			r = nil
			r = repair717Reopen(t, root)
			listed, err := r.list()
			if err != nil {
				t.Fatal("interrupted request blocked other requests:", err)
			}
			wantCount := 1
			if stage == "after_publication" {
				wantCount = 2
				repair717Status(t, r, prepared)
			} else if _, err = os.Lstat(dir); !os.IsNotExist(err) {
				t.Fatalf("pre-record directory was not recovered: %v", err)
			}
			if len(listed) != wantCount {
				t.Fatalf("listed %d records, want %d", len(listed), wantCount)
			}
			repair717Status(t, r, healthy)
			retried, err := r.submit(req)
			if err != nil || retried.Sequence != 1 || retried.State != "prepared" {
				t.Fatalf("submit did not recover: %+v %v", retried, err)
			}
			if !reflect.DeepEqual(retried.Request, req) {
				t.Fatal("retry changed request")
			}
			if stage == "after_publication" && !reflect.DeepEqual(retried, prepared) {
				t.Fatal("published first record was replaced")
			}
			r = nil
			r = repair717Reopen(t, root)
			repair717Status(t, r, retried)
		})
	}
}

func TestRepairM_717_InterruptedTailRecovery(t *testing.T) {
	transitions := []struct {
		state        string
		run, attempt int64
	}{
		{"dispatch_intent", 0, 0},
		{"acknowledgement_unknown", 0, 0},
		{"run_bound", 901, 2},
		{"running", 901, 2},
		{"completed_uncollected", 901, 2},
	}
	for prefix := range transitions {
		for _, stage := range []string{"before_writing", "during_writing", "before_publication", "after_publication"} {
			t.Run(fmt.Sprintf("%s/%s", transitions[prefix].state, stage), func(t *testing.T) {
				reference, req := qTestRegistry(t)
				r, _ := qTestRegistry(t)
				before, err := r.submit(req)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = reference.submit(req); err != nil {
					t.Fatal(err)
				}
				for i := 0; i < prefix; i++ {
					next := transitions[i]
					if _, err = reference.advance(req.LogicalID, before.State, next.state, next.run, next.attempt, "real transition"); err != nil {
						t.Fatal(err)
					}
					before, err = r.advance(req.LogicalID, before.State, next.state, next.run, next.attempt, "real transition")
					if err != nil {
						t.Fatal(err)
					}
				}
				previous := repair717Read(t, repair717Path(r, req.LogicalID, before.Sequence))
				next := transitions[prefix]
				after, err := reference.advance(req.LogicalID, before.State, next.state, next.run, next.attempt, "real transition")
				if err != nil {
					t.Fatal(err)
				}
				// Bind the reference-produced event to this registry's actual bytes.
				after.Previous = qHash(previous)
				payload := qTestCanonical(t, after)
				repair717Interrupt(t, repair717Path(r, req.LogicalID, after.Sequence), payload, stage)
				// Readers see the committed prefix even with an uncommitted tail.
				want := before
				if stage == "after_publication" {
					want = after
				}
				got, err := r.read(req.LogicalID)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("read during interrupted publication: %+v %v", got, err)
				}
				root := r.root
				r = nil
				r = repair717Reopen(t, root)
				repair717Status(t, r, want)
				if !bytes.Equal(repair717Read(t, repair717Path(r, req.LogicalID, before.Sequence)), previous) {
					t.Fatal("recovery rewrote the durable prefix")
				}
				if want.RunID > 0 && !r.protects(want.RunID) {
					t.Fatal("recovery lost assigned-run protection")
				}
				if stage != "after_publication" {
					resumed, err := r.advance(req.LogicalID, before.State, next.state, next.run, next.attempt, "resumed")
					if err != nil || resumed.Sequence != before.Sequence+1 {
						t.Fatalf("tail prevented continuation: %+v %v", resumed, err)
					}
					want = resumed
				}
				r = nil
				r = repair717Reopen(t, root)
				repair717Status(t, r, want)
			})
		}
	}
}

func TestRepairM_717_RetryInitialWriteWithoutRestart(t *testing.T) {
	r, req := qTestRegistry(t)
	dir := filepath.Join(r.root, req.LogicalID)
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	reference, _ := qTestRegistry(t)
	if _, err := reference.submit(req); err != nil {
		t.Fatal(err)
	}
	payload := repair717Read(t, repair717Path(reference, req.LogicalID, 1))
	repair717Interrupt(t, repair717Path(r, req.LogicalID, 1), payload, "during_writing")
	retried, err := r.submit(req)
	if err != nil || retried.Sequence != 1 || retried.State != "prepared" {
		t.Fatalf("interrupted local write prevented retry: %+v %v", retried, err)
	}
	repair717Status(t, r, retried)
}

func TestRepairM_717_PublishedIntentIsNeverRedispatched(t *testing.T) {
	r, req := qTestRegistry(t)
	if _, err := r.submit(req); err != nil {
		t.Fatal(err)
	}
	intent, err := r.advance(req.LogicalID, "prepared", "dispatch_intent", 0, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	key, signer := qTestKeys(t)
	if err = qWrite(filepath.Join(r.root, "artifact-key.der"), x509.MarshalPKCS1PrivateKey(key)); err != nil {
		t.Fatal(err)
	}
	if err = qWrite(filepath.Join(r.root, "signing-key"), signer); err != nil {
		t.Fatal(err)
	}
	reference, _ := qTestRegistry(t)
	if _, err = reference.submit(req); err != nil {
		t.Fatal(err)
	}
	if _, err = reference.advance(req.LogicalID, "prepared", "dispatch_intent", 0, 0, ""); err != nil {
		t.Fatal(err)
	}
	next, err := reference.advance(req.LogicalID, "dispatch_intent", "acknowledgement_unknown", 0, 0, "interrupted acknowledgement")
	if err != nil {
		t.Fatal(err)
	}
	next.Previous = qHash(repair717Read(t, repair717Path(r, req.LogicalID, 2)))
	repair717Interrupt(t, repair717Path(r, req.LogicalID, 3), qTestCanonical(t, next), "during_writing")
	root := r.root
	r = nil
	r = repair717Reopen(t, root)
	repair717Status(t, r, intent)
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls.Add(1)
		http.Error(w, "unexpected network request", http.StatusConflict)
	}))
	defer server.Close()
	client, err := newGitHubClient(config{GitHubAPIURL: server.URL, Owner: "local", Repo: "qualification"}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if err = client.qTick(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 {
		t.Fatal("recovered dispatch intent triggered a source check or dispatch")
	}
	recovered, err := r.read(req.LogicalID)
	if err != nil || recovered.State != "acknowledgement_unknown" || recovered.Sequence != 3 {
		t.Fatalf("intent was not preserved as ambiguous: %+v %v", recovered, err)
	}
	if _, err = r.advance(req.LogicalID, "acknowledgement_unknown", "dispatch_intent", 0, 0, ""); err == nil {
		t.Fatal("ambiguous intent permitted redispatch")
	}
	r = nil
	r = repair717Reopen(t, root)
	repair717Status(t, r, recovered)
}

func TestRepairM_717_AtomicPublicationDoesNotReplace(t *testing.T) {
	r, req := qTestRegistry(t)
	path := filepath.Join(r.root, "publication.json")
	const writers = 8
	payloads := make([][]byte, writers)
	for i := range payloads {
		payloads[i] = qTestCanonical(t, qualificationRecord{Request: req, Sequence: 1, State: "prepared", Detail: fmt.Sprint(i)})
	}
	results := make(chan struct {
		index int
		err   error
	}, writers)
	var wg sync.WaitGroup
	for i := range payloads {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results <- struct {
				index int
				err   error
			}{i, qWrite(path, payloads[i])}
		}(i)
	}
	wg.Wait()
	close(results)
	winner := -1
	for result := range results {
		if result.err == nil {
			if winner != -1 {
				t.Fatal("more than one publisher replaced the final path")
			}
			winner = result.index
		} else if !errors.Is(result.err, os.ErrExist) {
			t.Fatal(result.err)
		}
	}
	if winner < 0 || !bytes.Equal(repair717Read(t, path), payloads[winner]) {
		t.Fatal("published record is not the winning complete payload")
	}
	if err := qWrite(path, []byte("replacement")); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing final path was not protected: %v", err)
	}
	if err := qPrivate(path, false); err != nil {
		t.Fatal("published file lost private single-link ownership:", err)
	}
	link := filepath.Join(r.root, "symlink.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if err := qWrite(link, []byte("replacement")); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing symlink was not protected: %v", err)
	}
	if !bytes.Equal(repair717Read(t, path), payloads[winner]) {
		t.Fatal("publication followed or replaced an existing symlink")
	}
	entries, err := os.ReadDir(r.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if qStagedWrite(entry.Name()) {
			t.Fatal("ordinary publication left a temporary file")
		}
	}
}

func TestRepairM_717_RecoveryRetainsCorruptionChecks(t *testing.T) {
	for _, damage := range []string{"partial_final", "sequence_gap", "unknown_entry", "linked_staging"} {
		t.Run(damage, func(t *testing.T) {
			r, req := qTestRegistry(t)
			if _, err := r.submit(req); err != nil {
				t.Fatal(err)
			}
			path := repair717Path(r, req.LogicalID, 1)
			original := repair717Read(t, path)
			var err error
			switch damage {
			case "partial_final":
				err = os.WriteFile(path, original[:len(original)/2], 0600)
			case "sequence_gap":
				err = os.Rename(path, repair717Path(r, req.LogicalID, 2))
			case "unknown_entry":
				err = os.WriteFile(filepath.Join(filepath.Dir(path), "unknown.tmp"), original, 0600)
			case "linked_staging":
				err = os.Symlink(path, filepath.Join(filepath.Dir(path), ".qualification-123.tmp"))
			}
			if err != nil {
				t.Fatal(err)
			}
			root := r.root
			r = nil
			reopened, err := newQualificationRegistry(root)
			if damage == "linked_staging" {
				if err == nil {
					t.Fatal("recovery accepted a linked staging file")
				}
				if !bytes.Equal(repair717Read(t, path), original) {
					t.Fatal("recovery followed the staging symlink")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = reopened.read(req.LogicalID); err == nil {
				t.Fatal("recovery accepted committed corruption or an unknown entry")
			}
			if _, err = reopened.submit(req); err == nil {
				t.Fatal("recovery recreated a corrupt committed request")
			}
			if _, err = reopened.list(); err == nil {
				t.Fatal("corrupt committed state was hidden from reconciliation")
			}
		})
	}
}
