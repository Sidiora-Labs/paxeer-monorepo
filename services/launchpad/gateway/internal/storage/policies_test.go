package storage

import (
	"context"
	"errors"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"net"
	"strings"
	"testing"
	"time"
)

func TestServePolicyReadOnlyApprovedPrefix(t *testing.T) {
	p, e := ScopedPolicy("owned-media-bucket", "serve")
	if e != nil {
		t.Fatal(e)
	}
	s := p.Statement[0]
	if len(s.Action) != 1 || s.Action[0] != "s3:GetObject" || len(s.Resource) != 1 || s.Resource[0] != "arn:aws:s3:::owned-media-bucket/approved/*" {
		t.Fatal(p)
	}
	for _, v := range []string{"*", "other/bucket", "BUCKET"} {
		if _, e := ScopedPolicy(v, "serve"); e == nil {
			t.Fatal(v)
		}
	}
	if _, e := ScopedPolicy("owned-media-bucket", "admin"); e == nil {
		t.Fatal("admin")
	}
}

func TestScopedPolicyLimitsEachObjectRole(t *testing.T) {
	for _, tc := range []struct {
		role, action, prefix string
		resources            int
	}{
		{"validate-read", "s3:GetObject", "quarantine/*", 1},
		{"validate-write", "s3:PutObject", approvedPattern, 1},
		{"delete", "s3:DeleteObject", approvedPattern, 4},
	} {
		t.Run(tc.role, func(t *testing.T) {
			policy, err := ScopedPolicy("owned-media-bucket", tc.role)
			if err != nil || len(policy.Statement) != 1 {
				t.Fatalf("role policy: policy=%+v err=%v", policy, err)
			}
			statement := policy.Statement[0]
			if len(statement.Action) != 1 || statement.Action[0] != tc.action || len(statement.Resource) != tc.resources || statement.Resource[0] != "arn:aws:s3:::owned-media-bucket/"+tc.prefix {
				t.Fatalf("role policy grants: %+v", statement)
			}
			for _, resource := range statement.Resource {
				if !strings.HasPrefix(resource, "arn:aws:s3:::owned-media-bucket/") {
					t.Fatalf("resource outside selected bucket: %q", resource)
				}
			}
		})
	}
}

func TestScannerRealConnectionFailureRetainsAlertTimeline(t *testing.T) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if e = listener.Close(); e != nil {
		t.Fatal(e)
	}
	observer := NewMediaObserver()
	now := time.Now().UTC()
	observer.ScannerResult(true, 0, now)
	if e = observer.ProbeScanner(context.Background(), sharedhttp.ScanOptions{Host: "127.0.0.1", Port: port, Timeout: time.Second}, now); !errors.Is(e, ErrUnavailable) {
		t.Fatal(e)
	}
	health, e := observer.Health(context.Background(), nil, now.Add(61*time.Second), true)
	if !errors.Is(e, ErrUnavailable) || !health.ScanAlert || health.ScannerUnavailableSince != now || health.WriteReady {
		t.Fatal(health, e)
	}
	if e = observer.RecordUpload("quarantined"); e != nil {
		t.Fatal(e)
	}
	observer.RecordRead(false)
	outcomes := observer.Snapshot()["uploadOutcomes"].(map[string]uint64)
	if outcomes["quarantined"] != 1 {
		t.Fatal(outcomes)
	}
}
