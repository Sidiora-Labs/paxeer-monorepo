package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func TestPublicationBarrierRetainsBrokerAcknowledgement(t *testing.T) {
	endpoint, _ := status1695Broker(t)
	nc, err := nats.Connect(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = js.CreateStream(ctx, jetstream.StreamConfig{Name: "BARRIER", Subjects: []string{"canonical.creation.v1"}, Storage: jetstream.FileStorage}); err != nil {
		t.Fatal(err)
	}
	publisher, err := natsdriver.NewPublisher(nc, "BARRIER")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"release", "cancel", "invalid release", "marker collision", "broker rejection"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "publication")
			barrier := fixturePublishBarrier{Publisher: publisher, Path: path}
			switch mode {
			case "release":
				if err := os.WriteFile(path+".release", nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "invalid release":
				if err := os.Mkdir(path+".release", 0700); err != nil {
					t.Fatal(err)
				}
			case "marker collision":
				if err := os.WriteFile(path+".reached", nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			bounded, stop := context.WithTimeout(ctx, 30*time.Millisecond)
			defer stop()
			subject := "canonical.creation.v1"
			if mode == "broker rejection" {
				subject = "unavailable.subject"
			}
			eventID := strings.ReplaceAll(mode, " ", "-")
			ack, err := barrier.Publish(bounded, subject, eventID, []byte(`{"token":"LAUNCH"}`))
			if mode == "broker rejection" {
				if err == nil {
					t.Fatal("unbound stream acknowledged")
				}
				return
			}
			if ack.Stream != "BARRIER" || ack.Sequence == 0 {
				t.Fatalf("broker acknowledgement %+v", ack)
			}
			if mode == "release" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("barrier released without valid marker")
			}
			if mode == "cancel" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("cancelled barrier %v", err)
			}
			if mode != "marker collision" {
				marker, e := os.ReadFile(path + ".reached")
				if e != nil || !strings.Contains(string(marker), eventID+" BARRIER ") {
					t.Fatalf("persisted acknowledgement %q %v", marker, e)
				}
			}
		})
	}
	digest, err := runtimeBinaryDigest()
	if err != nil || len(digest) != 64 || digest != strings.ToLower(digest) {
		t.Fatalf("runtime digest %q %v", digest, err)
	}
	repeated, err := runtimeBinaryDigest()
	if err != nil || repeated != digest {
		t.Fatalf("runtime digest changed %q/%q %v", digest, repeated, err)
	}
}
