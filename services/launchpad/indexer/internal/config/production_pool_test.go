package config

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
)

func TestProductionPoolUsesCoordinatorSchemaAndDeclaredConnectionBudget(t *testing.T) {
	ctx := context.Background()
	database := coordinatortest.Bootstrap(t, os.Getenv("L06_POSTGRES_DSN"), "indexer_runtime")
	runtime := database.Runtimes["indexer_runtime"]
	parsed, err := url.Parse(runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.ParseUint(parsed.Port(), 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	budget := production.ConnectionBudget{Identity: "indexer_pool", Targets: []production.PostgresTarget{{ID: "primary", Member: "primary", Host: parsed.Hostname(), Port: uint16(port), MaxConnections: 1000}}, Operational: []production.OperationalBudget{{ID: "migrations", Target: "primary", Category: "migration", Connections: 1}}}
	for i, role := range []string{"apid:rest", "apid:sockets", "indexerd", "chartsd", "statsd", "pnld", "rankingd", "mediagatewayd", "userd", "sociald", "metadatad", "livestreamd"} {
		user := fmt.Sprintf("budget_user_%d", i)
		if role == "indexerd" {
			user = runtime.Role
		}
		budget.Clients = append(budget.Clients, production.ClientBudget{Role: role, PoolID: "batch", Target: "primary", Transport: runtimedb.TransportDirect, Alias: database.Name, User: user, QueryClass: runtimedb.Batch, Processes: production.LiveProcesses{Replicas: 1, Maximum: 1}, MaxConnections: 3})
	}
	body, err := json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{NodeEnv: "production", RuntimeProfile: "owned_local", BindAddress: "127.0.0.1", RuntimePoolID: "batch", RuntimeInstanceID: "indexer_pool", RuntimeBudgetDocument: base64.StdEncoding.EncodeToString(body), RuntimeBudgetSHA256: production.Checksum(body)}
	cfg.DatabaseURL = runtime.DSN
	cfg.ExpectedSchemaFile, cfg.ExpectedSchemaSHA256 = database.ExpectedSchemaFile(t, "indexer_runtime")
	pool, err := cfg.OpenRuntimePool(ctx, "indexerd", runtimedb.PoolOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if pool.Config().MaxConns != 3 {
		t.Fatalf("declared connections %d", pool.Config().MaxConns)
	}
	if err := cfg.CheckRuntimeSchema(ctx, pool); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"role", "pool", "database", "schema"} {
		t.Run(mode, func(t *testing.T) {
			changed := cfg
			role := "indexerd"
			switch mode {
			case "role":
				role = "undeclared"
			case "pool":
				changed.RuntimePoolID = "undeclared"
			case "database":
				changed.DatabaseURL = ":"
			case "schema":
				changed.ExpectedSchemaFile += ".absent"
			}
			bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			got, err := changed.OpenRuntimePool(bounded, role, runtimedb.PoolOptions{})
			if err == nil {
				got.Close()
				t.Fatal("unbound production pool opened")
			}
			if strings.TrimSpace(err.Error()) == "" {
				t.Fatal("pool rejection lacks diagnostic")
			}
		})
	}
}
