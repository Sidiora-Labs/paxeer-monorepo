package brokerprovision

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

func TestProvisionApplyCause(t *testing.T) {
	for _, tc := range []struct {
		name   string
		kind   string
		update bool
	}{
		{name: "create stream", kind: kindStream},
		{name: "create durable", kind: kindDurable},
		{name: "update stream", kind: kindStream, update: true},
		{name: "update durable", kind: kindDurable, update: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nc, js, _ := realBroker(t)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			d := smallDocument(t)
			stream, durable := d.Streams[0], d.Durables[0]
			updates := Updates{}
			failed := Difference{Kind: tc.kind, Stream: stream.Name, State: stateAbsent}
			subject := "$JS.API.STREAM.CREATE." + stream.Name
			if tc.update {
				if _, err := Provision(ctx, js, d, Updates{}); err != nil {
					t.Fatal(err)
				}
				failed.State = stateDiffers
				if tc.kind == kindStream {
					d.Streams[0].Limits.MaxMessages++
					updates.Streams = []string{stream.Name}
					subject = "$JS.API.STREAM.UPDATE." + stream.Name
				} else {
					d.Durables[0].AckWait += time.Second
					updates.Durables = []string{durable.Stream + "/" + durable.Durable}
				}
			}
			if tc.kind == kindDurable {
				failed.Durable = durable.Durable
				subject = "$JS.API.CONSUMER.CREATE." + durable.Stream + "." + durable.Durable + "." + durable.Subject
			}
			first := stream
			first.Name, first.Subjects = "APPLY_FIRST", []string{"apply.first"}
			d.Streams = append([]Stream{first}, d.Streams...)
			wantApplied := []Difference{{Kind: kindStream, Stream: first.Name, State: stateAbsent}}
			if tc.kind == kindDurable && !tc.update {
				wantApplied = append(wantApplied, Difference{Kind: kindStream, Stream: stream.Name, State: stateAbsent})
			}
			applyCtx, stop := context.WithCancel(ctx)
			defer stop()
			traced, err := jetstream.New(nc, jetstream.WithClientTrace(&jetstream.ClientTrace{
				RequestSent: func(requestSubject string, _ []byte) {
					if requestSubject == subject {
						stop()
					}
				},
			}))
			if err != nil {
				t.Fatal(err)
			}
			applied, err := Provision(applyCtx, traced, d, updates)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("apply cause: %v", err)
			}
			wantError := "brokerprovision: cannot apply " + failed.String() + ": " + context.Canceled.Error()
			if err.Error() != wantError {
				t.Fatalf("apply error: got %q want %q", err.Error(), wantError)
			}
			if !reflect.DeepEqual(applied, wantApplied) {
				t.Fatalf("applied differences: got %v want %v", applied, wantApplied)
			}
			for _, difference := range applied {
				if _, err := js.Stream(ctx, difference.Stream); err != nil {
					t.Fatalf("applied stream %s: %v", difference.Stream, err)
				}
			}
			remaining, err := Check(ctx, js, d)
			if err != nil {
				t.Fatal(err)
			}
			if len(remaining) == 0 || remaining[0] != failed {
				t.Fatalf("unapplied difference: got %v want %v", remaining, failed)
			}
		})
	}
}
