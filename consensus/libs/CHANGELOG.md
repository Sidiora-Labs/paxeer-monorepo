# Consensus library packages

These packages belong to the repository's root [Go module](../../go.mod).

| Package | Contents |
| --- | --- |
| [`bits`](bits/) | Bit arrays |
| [`bytes`](bytes/) | Byte helpers and hexadecimal `HexBytes` encoding |
| [`cli`](cli/) | Cobra commands and Viper configuration binding |
| [`json`](json/doc.go) | Concrete JSON values with decimal-string integers and base64 byte data |
| [`math`](math/) | Integer arithmetic and fraction helpers |
| [`net`](net/) | Protocol/address parsing and network helpers |
| [`os`](os/) | File, directory and signal helpers |
| [`rand`](rand/) | Random values and random-source helpers |
| [`service`](service/) | Context-bound service lifecycle |
| [`strings`](strings/) | Splitting, trimming and string-slice comparison |
| [`sync`](sync/) | Mutex wrappers |
| [`time`](time/) | Canonical UTC time and time-source abstractions |
| [`utils`](utils/) | Optional values, synchronization, watches and utility types |
| [`utils/scope`](utils/scope/) | Structured concurrency |

`json.Marshal` encodes a concrete value without a type wrapper. The separate
[`internal/jsontypes`](../internal/jsontypes/) package supplies registered
`type`/`value` wrappers for callers within the consensus tree.
