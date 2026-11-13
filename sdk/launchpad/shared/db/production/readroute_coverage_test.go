package production

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestPrimaryReadRouterPreservesCommittedPosition(t *testing.T) {
	dsn := os.Getenv("COORDINATOR_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("coordinator PostgreSQL fixture is required")
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := parsed.User.Password()
	primary := phase2AdmittedPool(t, dsn, parsed.User.Username(), password, "primary", 2)
	router, err := NewReadRouter(primary, AdmittedReadPool{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	position, err := router.CommittedPosition(ctx)
	if err != nil || position == 0 {
		t.Fatalf("committed primary position: %s, %v", position, err)
	}
	for _, request := range []ReadRequest{{Class: AuthoritativeRead}, {Class: ReadYourWrite, RequiredPosition: position}, {Class: BoundedStaleRead}} {
		if err := router.WithRead(ctx, request, func(tx pgx.Tx, observation ReadObservation) error {
			var database string
			if err := tx.QueryRow(ctx, "SELECT current_database()").Scan(&database); err != nil {
				return err
			}
			watermark, err := ParseWALPosition(observation.Watermark)
			if err != nil || database != parsed.Path[1:] || observation.Class != request.Class || observation.Target != "primary" || observation.Fallback || observation.Stale || watermark < position {
				t.Fatalf("primary read observation: db=%s observation=%+v watermark=%s err=%v", database, observation, watermark, err)
			}
			return nil
		}); err != nil {
			t.Fatalf("primary %s read: %v", request.Class, err)
		}
	}
	if err := router.WithPrimaryMutation(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "CREATE TEMP TABLE readroute_fixture(value integer)"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO readroute_fixture VALUES(7)"); err != nil {
			return err
		}
		var actual int
		if err := tx.QueryRow(ctx, "SELECT value FROM readroute_fixture").Scan(&actual); err != nil {
			return err
		}
		if actual != 7 {
			t.Fatalf("primary transaction changed value: %d", actual)
		}
		return nil
	}); err != nil {
		t.Fatalf("primary transaction: %v", err)
	}
	metrics := router.Metrics()
	for _, class := range []ReadClass{AuthoritativeRead, ReadYourWrite, BoundedStaleRead} {
		if metrics[class].Reads == 0 || metrics[class].Fallbacks != 0 || metrics[class].Unavailable != 0 {
			t.Fatalf("primary read metric %s: %+v", class, metrics[class])
		}
	}
	if err := router.WithRead(ctx, ReadRequest{Class: ReadYourWrite}, func(pgx.Tx, ReadObservation) error { return nil }); err == nil {
		t.Fatal("read-your-write without position admitted")
	}
	if err := router.WithRead(ctx, ReadRequest{Class: "unregistered"}, func(pgx.Tx, ReadObservation) error { return nil }); err == nil {
		t.Fatal("unregistered read class admitted")
	}
	if err := router.WithRead(ctx, ReadRequest{Class: AuthoritativeRead}, nil); err == nil {
		t.Fatal("read without consumer callback admitted")
	}
	future := WALPosition(^uint64(0))
	if err := router.WithRead(ctx, ReadRequest{Class: ReadYourWrite, RequiredPosition: future}, func(pgx.Tx, ReadObservation) error { t.Fatal("future position was read"); return nil }); !errors.Is(err, ErrReadUnavailable) {
		t.Fatalf("future primary history admitted: %v", err)
	}
	if metrics := router.Metrics(); metrics[ReadYourWrite].Unavailable != 1 {
		t.Fatalf("unavailable read not counted: %+v", metrics[ReadYourWrite])
	}
}

func TestReadRouterChecksPoolAdmissionAndDatabaseAuthority(t *testing.T) {
	dsn := os.Getenv("COORDINATOR_TEST_POSTGRES_DSN")
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	password, _ := parsed.User.Password()
	primary := phase2AdmittedPool(t, dsn, parsed.User.Username(), password, "authority_primary", 1)
	second := phase2AdmittedPool(t, dsn, parsed.User.Username(), password, "authority_second", 1)
	for _, mode := range []string{"missing", "invalid options", "unadmitted primary", "same member", "replica budget", "replica options", "unadmitted replica"} {
		t.Run(mode, func(t *testing.T) {
			p, r := primary, second
			switch mode {
			case "missing":
				p.Pool = nil
			case "invalid options":
				p.Options.MaxConns = 0
			case "unadmitted primary":
				p.Options.Instance = "different"
			case "same member":
				r = p
			case "replica budget":
				r.Options.BudgetIdentity = "different"
			case "replica options":
				r.Options.MaxConns = 0
			case "unadmitted replica":
				r.Options.Instance = "different"
			}
			if _, err := NewReadRouter(p, r, time.Second); err == nil {
				t.Fatal("unadmitted route accepted")
			}
		})
	}
	router, err := NewReadRouter(primary, second, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	if err := router.WithRead(context.Background(), ReadRequest{Class: BoundedStaleRead}, func(pgx.Tx, ReadObservation) error { called = true; return nil }); !errors.Is(err, ErrReadUnavailable) || called {
		t.Fatalf("primary configured as standby: %t %v", called, err)
	}
	if router.Metrics()[BoundedStaleRead].Unavailable != 1 {
		t.Fatal("authority mismatch not counted")
	}
	router, err = NewReadRouter(primary, AdmittedReadPool{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, read := range []func(context.Context, func(pgx.Tx, ReadObservation) error) error{router.WithOwnershipRead, router.WithIdempotencyRead} {
		if err := read(context.Background(), func(tx pgx.Tx, o ReadObservation) error {
			if o.Target != "primary" || o.Class != AuthoritativeRead {
				return errors.New("authoritative read left primary")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := router.WithRead(cancelled, ReadRequest{Class: AuthoritativeRead}, func(pgx.Tx, ReadObservation) error { t.Fatal("cancelled read invoked"); return nil }); !errors.Is(err, ErrReadUnavailable) {
		t.Fatal(err)
	}
	var absent *ReadRouter
	if len(absent.Metrics()) != 0 {
		t.Fatal("absent router has observations")
	}
	if err := absent.WithRead(context.Background(), ReadRequest{}, func(pgx.Tx, ReadObservation) error { return nil }); err == nil {
		t.Fatal("absent router admitted read")
	}
	if err := absent.WithPrimaryMutation(context.Background(), func(pgx.Tx) error { return nil }); err == nil {
		t.Fatal("absent router admitted mutation")
	}
}
