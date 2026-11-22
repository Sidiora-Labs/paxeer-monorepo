package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/migrate"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := serveTransactions(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serveTransactions() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dsn := os.Getenv("STATS_TEST_POSTGRES_DSN")
	if dsn == "" {
		return errors.New("STATS_TEST_POSTGRES_DSN is required")
	}
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer admin.Close(context.Background())
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		return err
	}
	name := "indexer_sdk_" + hex.EncodeToString(suffix[:])
	identifier := pgx.Identifier{name}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+identifier); err != nil {
		return err
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, err := admin.Exec(cleanup, "DROP DATABASE "+identifier+" WITH (FORCE)"); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	databaseURL, err := url.Parse(dsn)
	if err != nil || (databaseURL.Scheme != "postgres" && databaseURL.Scheme != "postgresql") {
		return errors.New("STATS_TEST_POSTGRES_DSN must be a PostgreSQL URL")
	}
	databaseURL.Path = "/" + name
	query := databaseURL.Query()
	query.Del("dbname")
	databaseURL.RawQuery = query.Encode()
	databaseDSN := databaseURL.String()
	if err = migrate.Up(ctx, databaseDSN); err != nil {
		return err
	}
	db, err := pgxpool.New(ctx, databaseDSN)
	if err != nil {
		return err
	}
	defer db.Close()
	st := store.New(db)
	for i := 0; i < 52; i++ {
		hash := fmt.Sprintf("0x%064x", i+1)
		row := store.TransactionRow{
			ID: hash + "-0", PoolAddress: "0x" + strings.Repeat("a", 40),
			Sender: "0x" + strings.Repeat("b", 40), IsBuy: i%2 == 0,
			AmountIn: "999999999999999999999999999999999999", AmountOut: "1000000000000000001",
			Price: "0.000000000000000001", Fee: "1000001", BlockTimestamp: 1700000000 + int64(i/2), TxHash: hash,
		}
		if err = st.InsertTransaction(ctx, row); err != nil {
			return err
		}
	}
	router := chi.NewRouter()
	httpapi.RegisterPoolTransactions(router, st)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	server := &http.Server{Handler: router, ReadHeaderTimeout: time.Second}
	stopping, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	go func() {
		commands := bufio.NewScanner(os.Stdin)
		for commands.Scan() {
			if commands.Text() != "close database" {
				fmt.Fprintln(os.Stderr, "unknown database command")
				break
			}
			db.Close()
			fmt.Println("database closed")
		}
		if err := commands.Err(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		stop()
	}()
	shutdown := make(chan struct{})
	go func() {
		defer close(shutdown)
		<-stopping.Done()
		deadline, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		if err := server.Shutdown(deadline); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	fmt.Println("http://" + listener.Addr().String())
	err = server.Serve(listener)
	stop()
	<-shutdown
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
