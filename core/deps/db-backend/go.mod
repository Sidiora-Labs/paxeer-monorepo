module github.com/tendermint/tm-db

go 1.25.0

require (
	github.com/cosmos/gorocksdb v1.2.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/btree v1.0.1
	github.com/jmhodges/levigo v1.0.0
	github.com/stretchr/testify v1.7.1
	github.com/syndtr/goleveldb v1.0.1-0.20200815110645-5c35d600f0ca
	go.etcd.io/bbolt v1.3.6
	google.golang.org/grpc v1.83.2
)

require github.com/dgraph-io/badger/v3 v3.2103.2

require (
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgraph-io/ristretto v0.1.0 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/golang/glog v1.2.5 // indirect
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6 // indirect
	github.com/golang/snappy v0.0.3 // indirect
	github.com/google/flatbuffers v1.12.1 // indirect
	github.com/klauspost/compress v1.18.7 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opencensus.io v0.22.5 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto v0.0.0-20200526211855-cb27e3aa2013 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// Breaking changes were released with the wrong tag (use v0.6.6 or later).
retract v0.6.5
