;; Small executable contract for querying the real host before IBC metadata exists.
;; Instantiate stores the complete host response; query forwards the supplied request.
(module
  (import "env" "query_chain" (func $query_chain (param i32) (result i32)))
  (import "env" "db_write" (func $db_write (param i32 i32)))
  (memory (export "memory") 2)
  (global $heap (mut i32) (i32.const 4096))
  (func (export "interface_version_8"))
  (func (export "requires_iterator"))
  (func (export "allocate") (param $size i32) (result i32) (local $r i32)
    global.get $heap local.set $r
    local.get $r local.get $r i32.const 12 i32.add i32.store
    local.get $r local.get $size i32.store offset=4
    local.get $r local.get $size i32.store offset=8
    local.get $r local.get $size i32.add i32.const 12 i32.add global.set $heap
    local.get $r)
  (func (export "deallocate") (param i32))
  ;; Static regions: key at 1024 and an empty successful contract response at 1100.
  (data (i32.const 1024) "\0c\04\00\00\0a\00\00\00\0a\00\00\00init_query")
  (data (i32.const 1100) "\58\04\00\00\3e\00\00\00\3e\00\00\00{\22ok\22:{\22messages\22:[],\22attributes\22:[],\22events\22:[],\22data\22:null}}")
  (data (i32.const 1300) "\20\05\00\00\1a\00\00\00\1a\00\00\00\7b\22\6f\6b\22\3a\7b\22\76\65\72\73\69\6f\6e\22\3a\22\6d\31\32\38\38\22\7d\7d")
  (data (i32.const 1400) "\84\05\00\00\4b\00\00\00\4b\00\00\00\7b\22\6f\6b\22\3a\7b\22\6d\65\73\73\61\67\65\73\22\3a\5b\5d\2c\22\61\74\74\72\69\62\75\74\65\73\22\3a\5b\5d\2c\22\65\76\65\6e\74\73\22\3a\5b\5d\2c\22\61\63\6b\6e\6f\77\6c\65\64\67\65\6d\65\6e\74\22\3a\22\41\51\3d\3d\22\7d\7d")
  (func (export "instantiate") (param i32 i32 i32) (result i32) (local $answer i32)
    local.get 2 call $query_chain local.set $answer
    i32.const 1024 local.get $answer call $db_write
    i32.const 1100)
  (func (export "execute") (param i32 i32 i32) (result i32) i32.const 1100)
  (func (export "query") (param i32 i32) (result i32) (local $answer i32)
    local.get 1 call $query_chain local.set $answer
    ;; Remove the outer SystemResult wrapper, preserving ContractResult<Binary>.
    local.get $answer local.get $answer i32.load i32.const 6 i32.add i32.store
    local.get $answer local.get $answer i32.load offset=8 i32.const 7 i32.sub i32.store offset=8
    local.get $answer)
  (func (export "ibc_channel_open") (param i32 i32) (result i32) i32.const 1300)
  (func (export "ibc_channel_connect") (param i32 i32) (result i32) i32.const 1100)
  (func (export "ibc_channel_close") (param i32 i32) (result i32) i32.const 1100)
  (func (export "ibc_packet_receive") (param i32 i32) (result i32) i32.const 1400)
  (func (export "ibc_packet_ack") (param i32 i32) (result i32) i32.const 1100)
  (func (export "ibc_packet_timeout") (param i32 i32) (result i32) i32.const 1100)
)
