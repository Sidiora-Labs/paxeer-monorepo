"""Capture the retained scenarios once against an owned local Paxeer node.

Run from bindings: python3 testdata/recorded_events/record_retained.py ws://127.0.0.1:8546
Only the implementation-time recorder needs Go and a node; the gate replays JSON.
"""
from pathlib import Path
import subprocess
import sys
import tempfile

here = Path(__file__).resolve().parent
bindings = here.parents[1]
source = (bindings / 'recorded_event_bindings_test.go').read_text()
scenarios = source[source.index('type recordedEventChain'):source.index('func TestRecordedRoleMetadata')]
scenarios = scenarios.replace('*testing.T', '*recordRun').replace('TestRecorded', 'record')
artifact = (bindings / 'event_bindings_integration_test.go').read_text()
artifact = artifact[artifact.index('func humanizeEventValue'):artifact.index('func TestEventBindingsRetainedEmitterV2')]
artifact = artifact.replace('*testing.T', '*recordRun').replace('assertProofABI(t, canonical, *exported)', 'if len(canonical.Methods) != len(exported.Methods) { t.Fatal("artifact method inventory differs") }')
imports = '''package main
import (
 "bytes"
 "context"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "fmt"
 "math/big"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "reflect"
 "strings"
 "sync"
 "time"
 "github.com/Sidiora-Technologies/KindleLaunch/protocol"
 b "github.com/Sidiora-Technologies/KindleLaunch/protocol/bindings"
 "github.com/ethereum/go-ethereum/accounts/abi"
 "github.com/ethereum/go-ethereum/accounts/abi/bind"
 "github.com/ethereum/go-ethereum/common"
 "github.com/ethereum/go-ethereum/core/types"
 "github.com/ethereum/go-ethereum/crypto"
 "github.com/ethereum/go-ethereum/ethclient"
 "github.com/gorilla/websocket"
)
'''
with tempfile.TemporaryDirectory(prefix='repair-m948-record-') as tmp:
    main = Path(tmp) / 'main.go'
    main.write_text(imports + scenarios + artifact + (here / 'retained_recorder.go.txt').read_text())
    subprocess.run(['go', 'run', str(main), *sys.argv[1:]], cwd=bindings, check=True)
