package utils

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	dbm "github.com/tendermint/tm-db"
)

type KeyValuePair struct {
	Key   []byte `json:"key"`
	Value []byte `json:"value"`
}

// Binary KV files support keys up to 1 MiB and values up to 64 MiB.
// Both bounds are checked before allocating a payload buffer.
const (
	MaxKVKeySize   uint32 = 1 << 20
	MaxKVValueSize uint32 = 64 << 20
)

var ErrKVRecordTooLarge = errors.New("KV record exceeds supported size")

// KVRecordLengthError describes an invalid length prefix or truncated payload.
// Remaining is -1 when the available payload size is unknown.
// Unwrap supports errors.Is with ErrKVRecordTooLarge and io.ErrUnexpectedEOF.
type KVRecordLengthError struct {
	Length    uint32
	Limit     uint32
	Remaining int64
	Err       error
}

func (e *KVRecordLengthError) Error() string {
	return fmt.Sprintf("invalid KV record length %d (limit %d, remaining %d): %v", e.Length, e.Limit, e.Remaining, e.Err)
}

func (e *KVRecordLengthError) Unwrap() error {
	return e.Err
}

// Opens application db
func OpenDB(dir string) (dbm.DB, error) {
	switch {
	case strings.HasSuffix(dir, ".db"):
		dir = dir[:len(dir)-3]
	case strings.HasSuffix(dir, ".db/"):
		dir = dir[:len(dir)-4]
	default:
		return nil, fmt.Errorf("database directory must end with .db")
	}
	// TODO: doesn't work on windows!
	cut := strings.LastIndex(dir, "/")
	if cut == -1 {
		return nil, fmt.Errorf("cannot cut paths on %s", dir)
	}
	name := dir[cut+1:]
	db, err := dbm.NewGoLevelDB(name, dir[:cut])
	if err != nil {
		return nil, err
	}
	return db, nil
}

// Reads raw keys / values from a file
func ReadKVEntriesFromFile(filename string) ([]KeyValuePair, error) {
	f, err := os.Open(filepath.Clean(filename))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var kvPairs []KeyValuePair
	for {
		key, err := readByteSlice(f, MaxKVKeySize)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read key: %w", err)
		}

		value, err := readByteSlice(f, MaxKVValueSize)
		if err != nil {
			if err == io.EOF {
				err = &KVRecordLengthError{Limit: MaxKVValueSize, Remaining: 0, Err: io.ErrUnexpectedEOF}
			}
			return nil, fmt.Errorf("read value: %w", err)
		}

		kvPairs = append(kvPairs, KeyValuePair{Key: key, Value: value})
	}

	return kvPairs, nil
}

func readByteSlice(r io.Reader, limit uint32) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		if err == io.ErrUnexpectedEOF {
			return nil, &KVRecordLengthError{Limit: limit, Remaining: -1, Err: err}
		}
		return nil, err
	}

	if length > limit {
		return nil, &KVRecordLengthError{Length: length, Limit: limit, Remaining: -1, Err: ErrKVRecordTooLarge}
	}
	if f, ok := r.(*os.File); ok {
		info, err := f.Stat()
		if err != nil {
			return nil, err
		}
		if info.Mode().IsRegular() {
			offset, err := f.Seek(0, io.SeekCurrent)
			if err != nil {
				return nil, err
			}
			remaining := info.Size() - offset
			if int64(length) > remaining {
				return nil, &KVRecordLengthError{Length: length, Limit: limit, Remaining: remaining, Err: io.ErrUnexpectedEOF}
			}
		}
	}

	data := make([]byte, length)
	n, err := io.ReadFull(r, data)
	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, &KVRecordLengthError{Length: length, Limit: limit, Remaining: int64(n), Err: io.ErrUnexpectedEOF}
		}
		return nil, err
	}
	return data, nil
}

// NOTE: Assumes latencies is sorted
// CalculatePercentile calculates latencies percentile
func CalculatePercentile(latencies []time.Duration, percentile float64) time.Duration {
	if percentile < 0 || percentile > 100 {
		panic(fmt.Sprintf("Invalid percentile: %f", percentile))
	}
	index := int(float64(len(latencies)-1) * percentile / 100.0)
	return latencies[index]
}

// Picks random file from input kv dir and updates processedFiles Map with it
func PickRandomKVFile(inputKVDir string, processedFiles *sync.Map) string {
	files, _ := os.ReadDir(inputKVDir)
	var availableFiles []string

	for _, file := range files {
		if _, processed := processedFiles.Load(file.Name()); !processed && strings.HasSuffix(file.Name(), ".kv") {
			availableFiles = append(availableFiles, file.Name())
		}
	}

	if len(availableFiles) == 0 {
		return ""
	}

	selected := availableFiles[rand.Intn(len(availableFiles))]
	processedFiles.Store(selected, true)
	return selected
}

func ListAllFiles(dir string) ([]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return []string{}, err
	}
	// Extract file nams from input KV dir
	fileNames := make([]string, 0, len(files))
	for _, file := range files {
		fileNames = append(fileNames, file.Name())
	}

	return fileNames, nil
}

func LoadAndShuffleKV(inputDir string, concurrency int) ([]KeyValuePair, error) {
	var allKVs []KeyValuePair
	mu := &sync.Mutex{}

	allFiles, err := ListAllFiles(inputDir)
	if err != nil {
		log.Fatalf("Failed to list all files: %v", err)
	}

	filesChan := make(chan string)
	wg := &sync.WaitGroup{}

	// Start worker goroutines
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for selectedFile := range filesChan {
				kvEntries, err := ReadKVEntriesFromFile(filepath.Join(inputDir, selectedFile))
				if err != nil {
					panic(err)
				}

				// Safely append the kvEntries to allKVs
				mu.Lock()
				allKVs = append(allKVs, kvEntries...)
				fmt.Printf("Done processing file %+v\n", filepath.Join(inputDir, selectedFile))
				mu.Unlock()
			}
		}()
	}

	// Send file names to filesChan
	for _, file := range allFiles {
		filesChan <- file
	}
	close(filesChan)

	// Wait for all workers to finish
	wg.Wait()

	rand.Shuffle(len(allKVs), func(i, j int) {
		allKVs[i], allKVs[j] = allKVs[j], allKVs[i]
	})

	return allKVs, nil
}

func CreateFile(outputDir string, fileName string) (*os.File, error) {
	outputDir = filepath.Clean(outputDir)
	fileName = filepath.Clean(fileName)
	err := os.MkdirAll(outputDir, fs.ModePerm)
	if err != nil {
		return nil, err
	}
	filename := filepath.Clean(filepath.Join(outputDir, fileName))

	currentFile, err := os.Create(filename)
	if err != nil {
		return nil, err
	}

	return currentFile, nil
}
