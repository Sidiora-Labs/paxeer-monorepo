# Recorded producer vectors

These JSON vectors are copied unchanged from `testdata/{tendermint,iavl}`
in https://github.com/cosmos/ics23/tree/806345b7a81d9be3375a28517c7fcb76cab71419/testdata.
They record producer-emitted protobuf proofs, keys, values and independently
recorded roots for membership and non-membership at each boundary and interior.
The repair tests decode these bytes with the retained production protobuf codec,
combine/compress them, encode/decode the resulting batch and verify against the
recorded root. No network access or external producer is needed by the tests.
