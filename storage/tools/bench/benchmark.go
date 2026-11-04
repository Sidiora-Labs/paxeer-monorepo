package bench

import (
	"fmt"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/tools/utils"
)

func requirePositiveBenchmarkDimension(name string, value int64) {
	if value <= 0 {
		panic(fmt.Errorf("benchmark: %s must be positive (got %d)", name, value))
	}
}

func validateBenchmarkDimensions(numVersions, concurrency int) {
	requirePositiveBenchmarkDimension("concurrency", int64(concurrency))
	requirePositiveBenchmarkDimension("num-versions", int64(numVersions))
}

func requireBenchmarkData(allKVs []utils.KeyValuePair) {
	if len(allKVs) == 0 {
		panic(fmt.Errorf("benchmark: input dataset is empty"))
	}
}

func requireBenchmarkLatencies(latencies []time.Duration) {
	if len(latencies) == 0 {
		panic(fmt.Errorf("benchmark: no operation latencies to summarize"))
	}
}

func iterateBenchmarkOperation(it dbm.Iterator, numIterationSteps int) (steps int) {
	defer func() {
		if err := it.Close(); err != nil {
			panic(err)
		}
	}()
	for steps < numIterationSteps && it.Valid() {
		steps++
		it.Next()
	}
	if err := it.Error(); err != nil {
		panic(err)
	}
	return steps
}

// writeToDBConcurrently generates random write load against the db
// Given kv pairs (randomly shuffled), the version, batch size, it will spin up `concurrency` goroutines
// each of which is assigned to a portion of the kv data and writes to db in `batchSize` batches.
// It maintains a `latencies` channel which aggregates all the latencies
func writeToDBConcurrently(db types.StateStore, allKVs []utils.KeyValuePair, concurrency int, version int64, batchSize int) []time.Duration {
	requirePositiveBenchmarkDimension("concurrency", int64(concurrency))
	requirePositiveBenchmarkDimension("version", version)
	requirePositiveBenchmarkDimension("batch-size", int64(batchSize))
	requireBenchmarkData(allKVs)
	allKVsLen := len(allKVs)
	allLatencies := make([]time.Duration, 0, allKVsLen)
	latencies := make(chan time.Duration, allKVsLen)

	kvsPerRoutine := allKVsLen / concurrency
	remainder := allKVsLen % concurrency

	wg := &sync.WaitGroup{}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			start := i * kvsPerRoutine
			end := start + kvsPerRoutine

			if i == concurrency-1 {
				end += remainder
			}

			for j := start; j < end; j += batchSize {
				ncs := &proto.NamedChangeSet{}
				cs := &proto.ChangeSet{}
				cs.Pairs = []*proto.KVPair{}

				batchEnd := j + batchSize
				if batchEnd > end {
					batchEnd = end
				}

				// Add key-value pairs to the batch up to batchSize
				for k := j; k < batchEnd; k++ {
					kv := allKVs[k]
					// No store key for benchmarks
					cs.Pairs = append(cs.Pairs, &proto.KVPair{
						Key:   kv.Key,
						Value: kv.Value,
					})
				}
				ncs.Changeset = *cs
				startTime := time.Now()
				err := db.ApplyChangesetSync(version, []*proto.NamedChangeSet{ncs})
				latency := time.Since(startTime)

				if err == nil {
					latencies <- latency
				} else {
					panic(err)
				}
			}
		}(i)
	}

	wg.Wait()
	close(latencies)

	for l := range latencies {
		allLatencies = append(allLatencies, l)
	}

	return allLatencies
}

// BenchmarkDBWrite measures random write performance of the db
// Given an input dir containing all the raw kv data, it writes to the db one version after another
func BenchmarkDBWrite(db types.StateStore, inputKVDir string, numVersions int, concurrency int, batchSize int) {
	validateBenchmarkDimensions(numVersions, concurrency)
	requirePositiveBenchmarkDimension("batch-size", int64(batchSize))
	startLoad := time.Now()
	kvData, err := utils.LoadAndShuffleKV(inputKVDir, concurrency)
	if err != nil {
		panic(err)
	}
	requireBenchmarkData(kvData)
	endLoad := time.Now()
	fmt.Printf("Finishing loading %+v kv pairs into memory %+v\n", len(kvData), endLoad.Sub(startLoad).String())

	// Write each version sequentially
	totalTime := time.Duration(0)
	writeCount := 0
	for v := 1; v < numVersions+1; v++ {
		// Write shuffled entries to RocksDB concurrently
		fmt.Printf("On Version %+v\n", v)
		startTime := time.Now()
		latencies := writeToDBConcurrently(db, kvData, concurrency, int64(v), batchSize)
		endTime := time.Now()
		totalTime += endTime.Sub(startTime)
		writeCount += len(latencies)
		requireBenchmarkLatencies(latencies)

		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		// Latencies per version
		fmt.Printf("P50 Latency: %v\n", utils.CalculatePercentile(latencies, 50))
		fmt.Printf("P75 Latency: %v\n", utils.CalculatePercentile(latencies, 75))
		fmt.Printf("P99 Latency: %v\n", utils.CalculatePercentile(latencies, 99))
		fmt.Printf("Total time: %v\n", totalTime)
		fmt.Printf("Total Successfully Written %d\n", writeCount)
		runtime.GC()
	}

	// Log throughput
	fmt.Printf("Total Successfully Written %d\n", writeCount)
	fmt.Printf("Total Time taken: %v\n", totalTime)
	fmt.Printf("Throughput: %f writes/sec\n", float64(writeCount)/totalTime.Seconds())
	fmt.Printf("Total records written %d\n", writeCount)
}

// readFromDBConcurrently generates random read load against the db
// Given kv pairs (randomly shuffled), numVersions, it will spin up `concurrency` goroutines
// that randomly select a version, key and query the db.
// It only performs `maxOps“ random reads and maintains a `latencies` channel which aggregates all the latencies.
func readFromDBConcurrently(db types.StateStore, allKVs []utils.KeyValuePair, numVersions int, concurrency int, maxOps int64) []time.Duration {
	validateBenchmarkDimensions(numVersions, concurrency)
	requirePositiveBenchmarkDimension("max-operations", maxOps)
	requireBenchmarkData(allKVs)
	allLatencies := make([]time.Duration, 0, maxOps)
	latencies := make(chan time.Duration, maxOps)

	var opCounter int64
	wg := &sync.WaitGroup{}

	// Each goroutine will handle reading a subset of kv pairs
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				currentOps := atomic.AddInt64(&opCounter, 1)
				if currentOps > maxOps {
					break
				}
				// Randomly pick a version and retrieve its column family handle
				version := int64(rand.Intn(numVersions)) + 1

				// Randomly pick a key-value pair to read
				kv := allKVs[rand.Intn(len(allKVs))]

				startTime := time.Now()
				// No store key for benchmarks
				_, err := db.Get("", version, kv.Key)
				latency := time.Since(startTime)
				if err == nil {
					latencies <- latency
				} else {
					panic(err)
				}

			}
		}()
	}
	wg.Wait()
	close(latencies)

	for l := range latencies {
		allLatencies = append(allLatencies, l)
	}

	return allLatencies
}

// BenchmarkDBRead measures random read performance of the db
// Given an input dir containing all the raw kv data, it generates random read load and measures performance.
func BenchmarkDBRead(db types.StateStore, inputKVDir string, numVersions int, concurrency int, maxOps int64) {
	validateBenchmarkDimensions(numVersions, concurrency)
	requirePositiveBenchmarkDimension("max-operations", maxOps)
	kvData, err := utils.LoadAndShuffleKV(inputKVDir, concurrency)
	if err != nil {
		panic(err)
	}
	requireBenchmarkData(kvData)
	startTime := time.Now()
	latencies := readFromDBConcurrently(db, kvData, numVersions, concurrency, maxOps)
	endTime := time.Now()

	totalTime := endTime.Sub(startTime)

	// Log throughput
	fmt.Printf("Total Successfully Read %d\n", len(latencies))
	fmt.Printf("Total Time taken: %v\n", totalTime)
	fmt.Printf("Throughput: %f reads/sec\n", float64(len(latencies))/totalTime.Seconds())
	fmt.Printf("Total records read %d\n", len(latencies))

	requireBenchmarkLatencies(latencies)
	// Sort latencies for percentile calculations
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	// Calculate average latency
	var totalLatency time.Duration
	for _, l := range latencies {
		totalLatency += l
	}
	avgLatency := totalLatency / time.Duration(len(latencies))

	fmt.Printf("Average Latency: %v\n", avgLatency)
	fmt.Printf("P50 Latency: %v\n", utils.CalculatePercentile(latencies, 50))
	fmt.Printf("P75 Latency: %v\n", utils.CalculatePercentile(latencies, 75))
	fmt.Printf("P99 Latency: %v\n", utils.CalculatePercentile(latencies, 99))
}

// forwardIterateDBConcurrently generates forward iteration load against the db
// Given kv pairs (randomly shuffled), numVersions, it will spin up `concurrency` goroutines
// that randomly select a version, key, seeks to that key and starts a forward iteration for at most `numIterationSteps` steps.
// It only performs `maxOps“ forward iterations and maintains a `latencies` channel which aggregates all the latencies.
func forwardIterateDBConcurrently(db types.StateStore, allKVs []utils.KeyValuePair, numVersions int, concurrency int, numIterationSteps int, maxOps int64) ([]time.Duration, int) {
	validateBenchmarkDimensions(numVersions, concurrency)
	requirePositiveBenchmarkDimension("max-operations", maxOps)
	requirePositiveBenchmarkDimension("iteration-steps", int64(numIterationSteps))
	requireBenchmarkData(allKVs)
	allLatencies := make([]time.Duration, 0, maxOps)
	var totalSteps int
	latencies := make(chan time.Duration, maxOps)
	steps := make(chan int, maxOps)

	var opCounter int64
	wg := &sync.WaitGroup{}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				currentOps := atomic.AddInt64(&opCounter, 1)
				if currentOps > maxOps {
					break
				}
				// Randomly pick a version and retrieve its column family handle
				version := int64(rand.Intn(numVersions)) + 1

				// Randomly pick a key-value pair to seek to
				kv := allKVs[rand.Intn(len(allKVs))]

				startTime := time.Now()
				// No end key since we iterate for fixed numIterationSteps steps
				it, err := db.Iterator("", version, kv.Key, nil)
				if err != nil {
					panic(err)
				}

				step := iterateBenchmarkOperation(it, numIterationSteps)
				latency := time.Since(startTime)

				latencies <- latency
				steps <- step
			}
		}()
	}

	wg.Wait()
	close(latencies)
	close(steps)

	for l := range latencies {
		allLatencies = append(allLatencies, l)
	}

	for s := range steps {
		totalSteps += s
	}

	return allLatencies, totalSteps
}

// BenchmarkDBForwardIteration measures forward iteration performance of the db
// Given an input dir containing all the raw kv data, it selects a random key, forward iterates and measures performance.
func BenchmarkDBForwardIteration(db types.StateStore, inputKVDir string, numVersions int, concurrency int, maxOps int64, iterationSteps int) {
	validateBenchmarkDimensions(numVersions, concurrency)
	requirePositiveBenchmarkDimension("max-operations", maxOps)
	requirePositiveBenchmarkDimension("iteration-steps", int64(iterationSteps))
	kvData, err := utils.LoadAndShuffleKV(inputKVDir, concurrency)
	if err != nil {
		panic(err)
	}
	requireBenchmarkData(kvData)
	startTime := time.Now()
	latencies, totalCountIteration := forwardIterateDBConcurrently(db, kvData, numVersions, concurrency, iterationSteps, maxOps)
	endTime := time.Now()

	totalTime := endTime.Sub(startTime)

	// Log throughput
	fmt.Printf("Total Prefixes Iterated: %d\n", totalCountIteration)
	fmt.Printf("Total Time taken: %v\n", totalTime)
	fmt.Printf("Throughput: %f iterations/sec\n", float64(totalCountIteration)/totalTime.Seconds())

	if totalCountIteration == 0 {
		fmt.Println("Average Per-Key Latency: unavailable (no keys iterated)")
		return
	}

	// Calculate average latency
	var totalLatency time.Duration
	for _, l := range latencies {
		totalLatency += l
	}
	avgLatency := time.Duration(int64(totalLatency) / int64(totalCountIteration))
	fmt.Printf("Average Per-Key Latency: %v\n", avgLatency)
}

// reverseIterateDBConcurrently generates reverse iteration load against the db
// Given kv pairs (randomly shuffled), numVersions, it will spin up `concurrency` goroutines
// that randomly select a version, key, seeks to that key and starts a reverse iteration for at most `numIterationSteps` steps.
// It only performs `maxOps“ reverse iterations and maintains a `latencies` channel which aggregates all the latencies.
func reverseIterateDBConcurrently(db types.StateStore, allKVs []utils.KeyValuePair, numVersions int, concurrency int, numIterationSteps int, maxOps int64) ([]time.Duration, int) {
	validateBenchmarkDimensions(numVersions, concurrency)
	requirePositiveBenchmarkDimension("max-operations", maxOps)
	requirePositiveBenchmarkDimension("iteration-steps", int64(numIterationSteps))
	requireBenchmarkData(allKVs)
	allLatencies := make([]time.Duration, 0, maxOps)
	var totalSteps int
	latencies := make(chan time.Duration, maxOps)
	steps := make(chan int, maxOps)

	var opCounter int64
	wg := &sync.WaitGroup{}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				currentOps := atomic.AddInt64(&opCounter, 1)
				if currentOps > maxOps {
					break
				}
				// Randomly pick a version and retrieve its column family handle
				version := int64(rand.Intn(numVersions)) + 1

				// Randomly pick a key-value pair to seek to
				kv := allKVs[rand.Intn(len(allKVs))]

				startTime := time.Now()
				// No start key since we iterate for fixed numIterationSteps steps
				it, err := db.ReverseIterator("", version, nil, kv.Key)
				if err != nil {
					panic(err)
				}

				step := iterateBenchmarkOperation(it, numIterationSteps)
				latency := time.Since(startTime)

				latencies <- latency
				steps <- step

			}
		}()
	}

	wg.Wait()
	close(latencies)
	close(steps)

	for l := range latencies {
		allLatencies = append(allLatencies, l)
	}

	for s := range steps {
		totalSteps += s
	}

	return allLatencies, totalSteps
}

// BenchmarkDBReverseIteration measures reverse iteration performance of the db
// Given an input dir containing all the raw kv data, it selects a random key, reverse iterates and measures performance.
func BenchmarkDBReverseIteration(db types.StateStore, inputKVDir string, numVersions int, concurrency int, maxOps int64, iterationSteps int) {
	validateBenchmarkDimensions(numVersions, concurrency)
	requirePositiveBenchmarkDimension("max-operations", maxOps)
	requirePositiveBenchmarkDimension("iteration-steps", int64(iterationSteps))
	kvData, err := utils.LoadAndShuffleKV(inputKVDir, concurrency)
	if err != nil {
		panic(err)
	}
	requireBenchmarkData(kvData)

	startTime := time.Now()
	latencies, totalCountIteration := reverseIterateDBConcurrently(db, kvData, numVersions, concurrency, iterationSteps, maxOps)
	endTime := time.Now()

	totalTime := endTime.Sub(startTime)

	// Log throughput
	fmt.Printf("Total Prefixes Reverse-Iterated: %d\n", totalCountIteration)
	fmt.Printf("Total Time taken: %v\n", totalTime)
	fmt.Printf("Throughput: %f iterations/sec\n", float64(totalCountIteration)/totalTime.Seconds())

	if totalCountIteration == 0 {
		fmt.Println("Average Per-Key Latency: unavailable (no keys iterated)")
		return
	}

	// Calculate average latency
	var totalLatency time.Duration
	for _, l := range latencies {
		totalLatency += l
	}
	avgLatency := time.Duration(int64(totalLatency) / int64(totalCountIteration))
	fmt.Printf("Average Per-Key Latency: %v\n", avgLatency)
}
