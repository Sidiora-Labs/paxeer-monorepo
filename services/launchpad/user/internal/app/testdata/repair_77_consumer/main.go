// This executable runs in the notifications module so the user integration
// test can exercise its real client, recipient and worker without changing
// either service's module manifest.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	n "github.com/Sidiora-Technologies/KindleLaunch/core/notifications"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type input struct {
	DSN, Base, CAFile, CertFile, KeyFile        string
	AccountID, DeviceID, Action, NotificationID string
	Target                                      n.PushTarget
}

type output struct {
	Preference     n.DeliveryPreference
	Target         n.PushTarget
	Targets        []n.PushTarget
	Decision       n.Decision
	Resolved       bool
	NotificationID string
	Deliveries     []n.DeliveryAttempt
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var in input
	if err := json.NewDecoder(os.Stdin).Decode(&in); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pair, err := tls.LoadX509KeyPair(in.CertFile, in.KeyFile)
	if err != nil {
		return err
	}
	ca, err := os.ReadFile(in.CAFile)
	if err != nil {
		return err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return errors.New("invalid test CA")
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{pair}}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	credentials, err := n.NewCredentials(ctx, secrets.Options{Provider: secrets.ProviderEnv}, "REPAIR_77_SERVICE_TOKEN")
	if err != nil {
		return err
	}
	authority, err := n.NewAccountClient(in.Base, client, credentials, "REPAIR_77_SERVICE_TOKEN")
	if err != nil {
		return err
	}
	db, err := sql.Open("pgx", in.DSN)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err = db.ExecContext(ctx, n.Schema); err != nil {
		return err
	}
	store := n.Store{DB: db}
	var out output
	if out.Preference, err = authority.CurrentDeliveryPreference(ctx, in.AccountID); err != nil {
		return err
	}
	if out.Target, err = authority.CurrentTarget(ctx, in.AccountID, in.DeviceID); err != nil {
		return err
	}
	if out.Targets, err = authority.Targets(ctx, in.AccountID); err != nil {
		return err
	}
	item := n.InboxItem{Identity: n.Identity{AccountID: in.AccountID}, Category: "chat"}
	if out.Decision, err = n.Decide(out.Preference, item, "push", time.Now()); err != nil {
		return err
	}
	target := out.Target
	if in.Target.AccountID != "" {
		target = in.Target
	}
	endpoint, resolveErr := authority.ResolveEndpoint(ctx, target)
	if resolveErr != nil && !errors.Is(resolveErr, n.ErrForbidden) {
		return resolveErr
	}
	out.Resolved = resolveErr == nil && endpoint.Token == "repair-77-device-token"
	switch in.Action {
	case "enqueue":
		consumer := n.RecipientConsumer{Store: store, Authority: authority}
		created, inserted, err := consumer.EnqueueChat(ctx, in.NotificationID, in.AccountID, "repair-77-chat", time.Now().UTC())
		if err != nil {
			return err
		}
		if !inserted {
			return errors.New("notification was not inserted")
		}
		out.NotificationID = created.ID
	case "work":
		// No provider is installed: the assertions require suppression before
		// provider selection, rather than substituting a provider test double.
		worker := n.Worker{Store: store, Authority: authority, Owner: "repair-77", DeepLinkOrigin: "https://launchpad.test"}
		if err = worker.Once(ctx, 1); err != nil {
			return err
		}
		out.NotificationID = in.NotificationID
	default:
		return errors.New("unknown consumer action")
	}
	var raw []byte
	if err = db.QueryRowContext(ctx, `SELECT deliveries FROM discovery_notifications.outbox WHERE id=$1`, out.NotificationID).Scan(&raw); err != nil {
		return err
	}
	if err = json.Unmarshal(raw, &out.Deliveries); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}
