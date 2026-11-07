package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"strconv"
	"strings"

	raster "github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/image"
	metadata "github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/metadata"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/repair51/objects"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
)

type request struct {
	Operation, DSN, Metadata, Safety, ObjectRoot string
	TargetType, TargetID, Status                 string
	Revision                                     int64
}
type response struct {
	Error                      string
	AssetID, Hash, Key, Status string
	Revision                   int64
	Body                       []byte
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var input request
		var output response
		if err := json.Unmarshal(scanner.Bytes(), &input); err != nil {
			output.Error = err.Error()
		} else {
			var err error
			output, err = run(input)
			if err != nil {
				output.Error = err.Error()
			}
		}
		if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
			panic(err)
		}
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
}

type queryLog struct{}

func (queryLog) Log(_ context.Context, _ tracelog.LogLevel, message string, data map[string]any) {
	if err, ok := data["err"]; ok {
		fmt.Fprintf(os.Stderr, "%s: %v; SQL=%v\n", message, err, data["sql"])
	}
}

func run(in request) (result response, resultErr error) {
	stage := "connect"
	defer func() {
		if resultErr != nil {
			resultErr = fmt.Errorf("%s: %w", stage, resultErr)
		}
	}()
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(in.DSN)
	if err != nil {
		return response{}, err
	}
	config.ConnConfig.Tracer = &tracelog.TraceLog{Logger: queryLog{}, LogLevel: tracelog.LogLevelError}
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return response{}, err
	}
	defer db.Close()
	store, err := metadata.NewProductionStore(db, in.Metadata, nil)
	if err != nil {
		return response{}, err
	}
	visibility := metadata.PostgresVisibilityAuthority{Pool: db, Schema: in.Safety}
	pool := "0x" + strings.Repeat("1", 40)
	id := "51515151-5151-4151-8151-515151515151"
	switch in.Operation {
	case "promote":
		var original bytes.Buffer
		if err := png.Encode(&original, image.NewNRGBA(image.Rect(0, 0, 4, 4))); err != nil {
			return response{}, err
		}
		hash := sha256.Sum256(original.Bytes())
		record := metadata.AssetRecord{ID: id, Pool: pool, Hash: hex.EncodeToString(hash[:]), ObjectKey: "quarantine/" + id, Size: int64(original.Len())}
		table := func(name string) string { return pgx.Identifier{in.Metadata, name}.Sanitize() }
		if _, err = db.Exec(ctx, "INSERT INTO "+table("pools")+"(pool,creator,token) VALUES($1,$1,$1)", pool); err != nil {
			return response{}, err
		}
		stage = "initial visibility read"
		v, err := visibility.Read(ctx, pool)
		if err != nil {
			return response{}, err
		}
		if v.Status != "allowed" || v.Revision != 0 {
			return response{}, fmt.Errorf("initial metadata visibility: %+v", v)
		}
		stage = "initial metadata restriction"
		if err = store.Restrict(ctx, pool, strconv.FormatInt(v.Revision, 10), false); err != nil {
			return response{}, err
		}
		stage = "raster decode"
		worker := raster.InProcessWorker{}
		decoded, err := worker.Decode(ctx, original.Bytes())
		if err != nil {
			return response{}, err
		}
		store.RasterWorker = worker
		stage = "create connected asset"
		if err = store.CreateConnectedAsset(ctx, record, "image/png", decoded.Width, decoded.Height); err != nil {
			return response{}, err
		}
		stage = "asset transitions"
		for _, transition := range [][2]string{{"CREATED", "UPLOADING"}, {"UPLOADING", "QUARANTINED"}, {"QUARANTINED", "VALIDATING"}} {
			if err = store.TransitionAsset(ctx, id, transition[0], transition[1]); err != nil {
				return response{}, err
			}
		}
		publisher := objects.Store{Root: in.ObjectRoot}
		// The clean scan is input to the real metadata producer; no gateway
		// approval, lineage, variant key or visibility response is fabricated.
		scan := sharedhttp.ScanResult{Clean: true, Verdict: sharedhttp.ScanClean, Engine: "qualification-input", SignatureRevision: "1", ContentHash: record.Hash, Size: record.Size}
		stage = "metadata promotion"
		if err = store.Promote(ctx, id, original.Bytes(), decoded.Variants, scan, publisher); err != nil {
			return response{}, err
		}
		stage = "read promoted variants"
		var raw []byte
		if err = db.QueryRow(ctx, "SELECT variants FROM "+table("asset_details_v2")+" WHERE asset_id=$1", id).Scan(&raw); err != nil {
			return response{}, err
		}
		var keys map[string]string
		if err = json.Unmarshal(raw, &keys); err != nil {
			return response{}, err
		}
		return response{AssetID: id, Hash: record.Hash, Key: keys["256"], Body: decoded.Variants[256], Status: v.Status, Revision: v.Revision}, nil
	case "visibility", "sync":
		stage = "moderation visibility read"
		v, err := visibility.ReadTarget(ctx, in.TargetType, in.TargetID)
		if err != nil {
			return response{}, err
		}
		if in.Operation == "sync" {
			stage = "sync metadata moderation"
			if err = store.Restrict(ctx, in.TargetID, strconv.FormatInt(v.Revision, 10), v.Status == "removed"); err != nil {
				return response{}, err
			}
		}
		return response{Status: v.Status, Revision: v.Revision}, nil
	default:
		return response{}, fmt.Errorf("unknown operation %q", in.Operation)
	}
}
