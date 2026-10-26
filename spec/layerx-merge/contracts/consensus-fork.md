# Consensus fork contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## autobahn-events

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.15.evt-autobahn-executed-block-events-j57</code> | <code>event</code> | <code>new</code> | <code>consensus/internal/p2p/giga_router.go</code> (planned) | <code>tm.event='NewBlockHeader' and tm.event='Tx' published by GigaRouter.executeBlock</code> |
| <code>interface.15.go-consensus-internal-p2p-block-event-publisher-j57</code> | <code>go_exported</code> | <code>new</code> | <code>consensus/internal/p2p/giga_router.go</code> (planned) | <code>type BlockEventPublisher interface { PublishEventNewBlockHeader(types.EventDataNewBlockHeader) error; PublishEventTx(types.EventDataTx) error }</code> |
| <code>interface.15.go-consensus-internal-p2p-giga-router-set-event-bus-j57</code> | <code>go_exported</code> | <code>new</code> | <code>consensus/internal/p2p/giga_router.go</code> (planned) | <code>func (r *GigaRouter) SetEventBus(bus BlockEventPublisher)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.15.evt-autobahn-executed-block-events-j57</code> | notes | planned: under Autobahn the indexer service and the events route receive the same event types the classic block executor publishes |
| <code>interface.15.go-consensus-internal-p2p-block-event-publisher-j57</code> | notes | planned interface naming the two eventbus.EventBus methods (consensus/internal/eventbus/event_bus.go:94, :129) the router calls, so p2p does not import the eventbus package |
| <code>interface.15.go-consensus-internal-p2p-giga-router-set-event-bus-j57</code> | notes | planned method; executeBlock publishes NewBlockHeader and Tx events to the bus after SaveFinalizeBlockResponses |

Contract context <code>contract_context.15.autobahn-events-j57</code>:

| Field | Frozen rule |
|---|---|
| interface | GigaRouter.SetEventBus(BlockEventPublisher) planned in consensus/internal/p2p/giga_router.go and its publication from executeBlock at consensus/internal/p2p/giga_router.go:296 |
| storage | No storage; events feed the indexer service and its kv EventSink |
| errors | Publish errors are logged and never returned from executeBlock |
| events | tm.event='NewBlockHeader' with EventDataNewBlockHeader and tm.event='Tx' with EventDataTx, the types of consensus/types/events.go:242 |
| gas | Not metered |
| activation | Active once the node calls SetEventBus (wiring.J57.1) |
| invariants | Events of height h are published after SaveFinalizeBlockResponses(h) and before PushAppHash(h); one Tx event per tx in block order |
| pause | No pause |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.15.273.1</code> | req | 15273.1 |
| <code>acceptance.15.273.1</code> | scenario | Given a GigaRouter with SetEventBus called on a started eventbus.EventBus and a subscriber on tm.event='NewBlockHeader', when the router finalizes and executes blocks 1..3, then the subscriber receives three EventDataNewBlockHeader with heights 1, 2, 3 and AppHash equal to the FinalizeBlock response of each height |
| <code>acceptance.15.273.1</code> | test | consensus/internal/p2p/giga_router_events_test.go::TestGigaRouterPublishesNewBlockHeaderEvents (planned) |
| <code>acceptance.15.273.1</code> | qualification | UNRUN |
| <code>req.15273</code> | ac_1 | Under Autobahn every executed block publishes one NewBlockHeader event carrying its FinalizeBlock response; at HEAD executeBlock at consensus/internal/p2p/giga_router.go:296-346 publishes nothing. |
| <code>acceptance.15.273.2</code> | req | 15273.2 |
| <code>acceptance.15.273.2</code> | scenario | Given the same router and a subscriber on tm.event='Tx', when the router executes a block holding two txs, then the subscriber receives two EventDataTx with indexes 0 and 1, the block height and the tx bytes |
| <code>acceptance.15.273.2</code> | test | consensus/internal/p2p/giga_router_events_test.go::TestGigaRouterPublishesTxEvents (planned) |
| <code>acceptance.15.273.2</code> | qualification | UNRUN |
| <code>req.15273</code> | ac_2 | Under Autobahn every executed tx publishes one Tx event with its height, index and result, the input the tx indexer consumes. |
| <code>acceptance.15.276.1</code> | req | 15276.1 |
| <code>acceptance.15.276.1</code> | scenario | Given a GigaRouter publishing to a started EventBus and an indexer.Service writing a kv EventSink, when the router executes a block holding one tx, then TxSearch with tx.height=&lt;h&gt; returns TotalCount 1 with that tx hash and TxResult.Code 0 |
| <code>acceptance.15.276.1</code> | test | consensus/internal/rpc/core/autobahn_indexing_test.go::TestAutobahnTxSearchFindsFinalizedTx (planned) |
| <code>acceptance.15.276.1</code> | qualification | UNRUN |
| <code>req.15276</code> | ac_1 | Under Autobahn tx_search finds executed txs through the indexer fed by the router events; at HEAD consensus/internal/p2p/giga_router.go:296-346 publishes no event, so the index stays empty. |
| <code>acceptance.15.276.2</code> | req | 15276.2 |
| <code>acceptance.15.276.2</code> | scenario | Given the same set-up, when the router executes blocks 1 and 2, then BlockSearch with block.height=2 reports TotalCount 1 |
| <code>acceptance.15.276.2</code> | test | consensus/internal/rpc/core/autobahn_indexing_test.go::TestAutobahnBlockSearchCountsFinalizedBlock (planned) |
| <code>acceptance.15.276.2</code> | qualification | UNRUN |
| <code>req.15276</code> | ac_2 | Under Autobahn block_search indexes executed blocks through the NewBlockHeader events of the router. |
| <code>capability.15.27.1</code> | title | Publish NewBlockHeader and Tx events from Autobahn block execution |
| <code>capability.15.27.1</code> | reqs | 15273.1<br>15273.2 |
| <code>capability.15.27.4</code> | title | Index Autobahn blocks so tx_search and block_search find them |
| <code>capability.15.27.4</code> | reqs | 15276.1<br>15276.2 |
| <code>resolution.17.971</code> | title | decision.1527 binds: Router event publication never fails block execution |
| <code>resolution.17.971</code> | applies_to | decision.1527<br>task.5.67.1 |
| <code>resolution.17.971</code> | source | decision.1527 in fragments/J57/decision.kvx; tasks from affects |

## cli

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.15.cli-consensus-cmd-priv-val-server-flag-addr-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/priv_val_server/main.go:41</code> | <code>addr = flag.String("addr"</code> |
| <code>interface.15.cli-consensus-cmd-priv-val-server-flag-certfile-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/priv_val_server/main.go:46</code> | <code>certFile = flag.String("certfile"</code> |
| <code>interface.15.cli-consensus-cmd-priv-val-server-flag-chain-id-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/priv_val_server/main.go:42</code> | <code>chainID = flag.String("chain-id"</code> |
| <code>interface.15.cli-consensus-cmd-priv-val-server-flag-insecure-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/priv_val_server/main.go:45</code> | <code>insecure = flag.Bool("insecure"</code> |
| <code>interface.15.cli-consensus-cmd-priv-val-server-flag-keyfile-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/priv_val_server/main.go:47</code> | <code>keyFile = flag.String("keyfile"</code> |
| <code>interface.15.cli-consensus-cmd-priv-val-server-flag-priv-key-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/priv_val_server/main.go:43</code> | <code>privValKeyPath = flag.String("priv-key"</code> |
| <code>interface.15.cli-consensus-cmd-priv-val-server-flag-priv-state-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/priv_val_server/main.go:44</code> | <code>privValStatePath = flag.String("priv-state"</code> |
| <code>interface.15.cli-consensus-cmd-priv-val-server-flag-prometheus-addr-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/priv_val_server/main.go:49</code> | <code>prometheusAddr = flag.String("prometheus-addr"</code> |
| <code>interface.15.cli-consensus-cmd-priv-val-server-flag-rootcafile-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/priv_val_server/main.go:48</code> | <code>rootCA = flag.String("rootcafile"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-blockchain-j57</code> | <code>cli_command</code> | <code>change</code> | <code>consensus/cmd/tendermint/commands/reset.go:28</code> | <code>Use: "blockchain"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-completion-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/completion.go:15</code> | <code>Use: "completion"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-debug-debug-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/debug/debug.go:15</code> | <code>Use: "debug"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-debug-dump-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/debug/dump.go:24</code> | <code>Use: "dump [output-directory]"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-debug-flag-flag-frequency-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/debug/dump.go:94</code> | <code>cmd.Flags().Uint(</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-debug-flag-flag-node-rpc-addr-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/debug/debug.go:19</code> | <code>cmd.PersistentFlags().String(</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-debug-flag-flag-prof-addr-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/debug/dump.go:100</code> | <code>cmd.Flags().String(</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-debug-kill-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/debug/kill.go:23</code> | <code>Use: "kill [pid] [compressed-output-file]"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-consensus-create-empty-blocks-interval-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:65</code> | <code>cmd.Flags().String( "consensus.create-empty-blocks-interval"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-consensus-create-empty-blocks-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:61</code> | <code>cmd.Flags().Bool( "consensus.create-empty-blocks"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-consensus-double-sign-check-height-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:35</code> | <code>cmd.Flags().Int64("consensus.double-sign-check-height"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-consensus-gossip-tx-key-only-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:69</code> | <code>cmd.Flags().Bool( "consensus.gossip-tx-key-only"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-db-backend-2-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:85</code> | <code>cmd.Flags().String( "db-backend"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-db-backend-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/inspect.go:48</code> | <code>cmd.Flags().String("db-backend"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-db-dir-2-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:89</code> | <code>cmd.Flags().String( "db-dir"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-db-dir-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/inspect.go:50</code> | <code>cmd.Flags().String("db-dir"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-dir-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/light.go:207</code> | <code>cmd.Flags().StringVarP(&amp;dir, "dir"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-end-height-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/reindex_event.go:95</code> | <code>cmd.Flags().Int64Var(&amp;endHeight, "end-height"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-flag-zsh-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/completion.go:43</code> | <code>cmd.Flags().Bool(flagZsh, false</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-hash-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/light.go:217</code> | <code>cmd.Flags().BytesHexVar(&amp;trustedHash, "hash"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-height-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/light.go:216</code> | <code>cmd.Flags().Int64Var(&amp;trustedHeight, "height"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-key-2-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/reset.go:72</code> | <code>resetSignerCmd.Flags().StringVar(&amp;keyType, "key"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-key-3-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/reset.go:75</code> | <code>resetAllCmd.Flags().StringVar(&amp;keyType, "key"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-key-4-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/reset.go:217</code> | <code>resetAllCmd.Flags().StringVar(&amp;keyType, "key"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-key-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/gen_validator.go:37</code> | <code>cmd.Flags().StringVar(&amp;keyType, "key"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-laddr-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/light.go:201</code> | <code>cmd.Flags().StringVar(&amp;listenAddr, "laddr"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-log-format-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/light.go:219</code> | <code>cmd.Flags().StringVar(&amp;logFormat, "log-format"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-log-level-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/light.go:218</code> | <code>cmd.Flags().StringVar(&amp;logLevel, "log-level"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-mode-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:20</code> | <code>cmd.Flags().String("mode"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-moniker-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:17</code> | <code>cmd.Flags().String("moniker"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-name-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:78</code> | <code>cmd.Flags().String(name, ""</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-output-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/gen_autobahn_config.go:112</code> | <code>cmd.Flags().StringP("output"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-p2p-laddr-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:49</code> | <code>cmd.Flags().String( "p2p.laddr"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-p2p-persistent-peers-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:53</code> | <code>cmd.Flags().String("p2p.persistent-peers"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-p2p-pex-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:55</code> | <code>cmd.Flags().Bool("p2p.pex"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-p2p-private-peer-ids-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:56</code> | <code>cmd.Flags().String("p2p.private-peer-ids"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-p2p-unconditional-peer-ids-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:57</code> | <code>cmd.Flags().String("p2p.unconditional_peer_ids"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-p2p-upnp-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:54</code> | <code>cmd.Flags().Bool("p2p.upnp"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-persistent-state-dir-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/gen_autobahn_config.go:113</code> | <code>cmd.Flags().String("persistent-state-dir"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-primary-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/light.go:203</code> | <code>cmd.Flags().StringVarP(&amp;primaryAddr, "primary"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-priv-validator-laddr-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:23</code> | <code>cmd.Flags().String( "priv-validator-laddr"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-rpc-laddr-2-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:44</code> | <code>cmd.Flags().String("rpc.laddr"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-rpc-laddr-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/inspect.go:46</code> | <code>cmd.Flags().String("rpc.laddr"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-rpc-pprof-laddr-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:46</code> | <code>cmd.Flags().String("rpc.pprof-laddr"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-rpc-unsafe-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:45</code> | <code>cmd.Flags().Bool("rpc.unsafe"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-sequential-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/light.go:223</code> | <code>cmd.Flags().BoolVar(&amp;sequential, "sequential"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-start-height-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/reindex_event.go:94</code> | <code>cmd.Flags().Int64Var(&amp;startHeight, "start-height"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-trust-level-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/light.go:220</code> | <code>cmd.Flags().StringVar(&amp;trustLevelStr, "trust-level"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-trusting-period-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/light.go:214</code> | <code>cmd.Flags().DurationVar(&amp;trustingPeriod, "trusting-period"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-witnesses-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/light.go:205</code> | <code>cmd.Flags().StringVarP(&amp;witnessAddrsJoined, "witnesses"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-gen-autobahn-config-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/gen_autobahn_config.go:25</code> | <code>Use: "gen-autobahn-config [node-dirs...]"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-gen-node-key-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/gen_node_key.go:15</code> | <code>Use: "gen-node-key"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-gen-validator-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/gen_validator.go:18</code> | <code>Use: "gen-validator"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-inspect-j57</code> | <code>cli_command</code> | <code>change</code> | <code>consensus/cmd/tendermint/commands/inspect.go:19</code> | <code>Use: "inspect"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-key-migrate-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/key_migrate.go:16</code> | <code>Use: "key-migrate"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-light-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/light.go:77</code> | <code>Use: "light [chainID]"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-peers-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/reset.go:36</code> | <code>Use: "peers"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-reindex-event-j57</code> | <code>cli_command</code> | <code>change</code> | <code>consensus/cmd/tendermint/commands/reindex_event.go:38</code> | <code>Use: "reindex-event"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-reset-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/reset.go:23</code> | <code>Use: "reset"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-unsafe-all-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/reset.go:54</code> | <code>Use: "unsafe-all"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-unsafe-reset-all-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/reset.go:196</code> | <code>Use: "unsafe-reset-all"</code> |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-unsafe-signer-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/reset.go:44</code> | <code>Use: "unsafe-signer"</code> |
| <code>interface.15.cli-consensus-libs-cli-completion-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/libs/cli/helper.go:80</code> | <code>Use: "completion"</code> |
| <code>interface.15.cli-consensus-libs-cli-flag-flag-zsh-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/libs/cli/helper.go:108</code> | <code>cmd.Flags().Bool(flagZsh, false</code> |
| <code>interface.15.cli-consensus-libs-cli-flag-home-flag-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/libs/cli/setup.go:30</code> | <code>cmd.PersistentFlags().StringP(HomeFlag, ""</code> |
| <code>interface.15.cli-consensus-libs-cli-flag-trace-flag-j57</code> | <code>cli_command</code> | <code>stable</code> | <code>consensus/libs/cli/setup.go:31</code> | <code>cmd.PersistentFlags().Bool(TraceFlag, false</code> |
| <code>interface.15.go-consensus-cmd-tendermint-commands-err-autobahn-offline-store-j57</code> | <code>go_exported</code> | <code>new</code> | <code>consensus/cmd/tendermint/commands/reindex_event.go</code> (planned) | <code>ErrAutobahnOfflineStore = errors.New("offline block and state store tooling is not supported when autobahn-config-file is set")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.15.cli-consensus-cmd-priv-val-server-flag-addr-j57</code> | notes | flag --addr registered in main; read by the same command |
| <code>interface.15.cli-consensus-cmd-priv-val-server-flag-certfile-j57</code> | notes | flag --certfile registered in main; read by the same command |
| <code>interface.15.cli-consensus-cmd-priv-val-server-flag-chain-id-j57</code> | notes | flag --chain-id registered in main; read by the same command |
| <code>interface.15.cli-consensus-cmd-priv-val-server-flag-insecure-j57</code> | notes | flag --insecure registered in main; read by the same command |
| <code>interface.15.cli-consensus-cmd-priv-val-server-flag-keyfile-j57</code> | notes | flag --keyfile registered in main; read by the same command |
| <code>interface.15.cli-consensus-cmd-priv-val-server-flag-priv-key-j57</code> | notes | flag --priv-key registered in main; read by the same command |
| <code>interface.15.cli-consensus-cmd-priv-val-server-flag-priv-state-j57</code> | notes | flag --priv-state registered in main; read by the same command |
| <code>interface.15.cli-consensus-cmd-priv-val-server-flag-prometheus-addr-j57</code> | notes | flag --prometheus-addr registered in main; read by the same command |
| <code>interface.15.cli-consensus-cmd-priv-val-server-flag-rootcafile-j57</code> | notes | flag --rootcafile registered in main; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-blockchain-j57</code> | notes | constructed by MakeResetCommand; mounted by the node binary |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-completion-j57</code> | notes | constructed by NewCompletionCmd; mounted by the node binary |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-debug-debug-j57</code> | notes | constructed by GetDebugCommand; not mounted by any in-repo binary at HEAD |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-debug-dump-j57</code> | notes | constructed by getDumpCmd; not mounted by any in-repo binary at HEAD |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-debug-flag-flag-frequency-j57</code> | notes | flag --flagFrequency registered in getDumpCmd; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-debug-flag-flag-node-rpc-addr-j57</code> | notes | flag --flagNodeRPCAddr registered in GetDebugCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-debug-flag-flag-prof-addr-j57</code> | notes | flag --flagProfAddr registered in getDumpCmd; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-debug-kill-j57</code> | notes | constructed by getKillCmd; not mounted by any in-repo binary at HEAD |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-consensus-create-empty-blocks-interval-j57</code> | notes | flag --consensus.create-empty-blocks-interval registered in AddNodeFlags; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-consensus-create-empty-blocks-j57</code> | notes | flag --consensus.create-empty-blocks registered in AddNodeFlags; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-consensus-double-sign-check-height-j57</code> | notes | flag --consensus.double-sign-check-height registered in AddNodeFlags; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-consensus-gossip-tx-key-only-j57</code> | notes | flag --consensus.gossip-tx-key-only registered in AddNodeFlags; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-db-backend-2-j57</code> | notes | flag --db-backend registered in addDBFlags; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-db-backend-j57</code> | notes | flag --db-backend registered in MakeInspectCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-db-dir-2-j57</code> | notes | flag --db-dir registered in addDBFlags; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-db-dir-j57</code> | notes | flag --db-dir registered in MakeInspectCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-dir-j57</code> | notes | flag --dir registered in MakeLightCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-end-height-j57</code> | notes | flag --end-height registered in MakeReindexEventCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-flag-zsh-j57</code> | notes | flag --flagZsh registered in NewCompletionCmd; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-hash-j57</code> | notes | flag --hash registered in MakeLightCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-height-j57</code> | notes | flag --height registered in MakeLightCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-key-2-j57</code> | notes | flag --key registered in MakeResetCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-key-3-j57</code> | notes | flag --key registered in MakeResetCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-key-4-j57</code> | notes | flag --key registered in MakeUnsafeResetAllCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-key-j57</code> | notes | flag --key registered in MakeGenValidatorCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-laddr-j57</code> | notes | flag --laddr registered in MakeLightCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-log-format-j57</code> | notes | flag --log-format registered in MakeLightCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-log-level-j57</code> | notes | flag --log-level registered in MakeLightCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-mode-j57</code> | notes | flag --mode registered in AddNodeFlags; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-moniker-j57</code> | notes | flag --moniker registered in AddNodeFlags; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-name-j57</code> | notes | flag --name registered in mustMarkDeprecated; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-output-j57</code> | notes | flag --output registered in MakeGenAutobahnConfigCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-p2p-laddr-j57</code> | notes | flag --p2p.laddr registered in AddNodeFlags; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-p2p-persistent-peers-j57</code> | notes | flag --p2p.persistent-peers registered in AddNodeFlags; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-p2p-pex-j57</code> | notes | flag --p2p.pex registered in AddNodeFlags; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-p2p-private-peer-ids-j57</code> | notes | flag --p2p.private-peer-ids registered in AddNodeFlags; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-p2p-unconditional-peer-ids-j57</code> | notes | flag --p2p.unconditional_peer_ids registered in AddNodeFlags; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-p2p-upnp-j57</code> | notes | flag --p2p.upnp registered in AddNodeFlags; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-persistent-state-dir-j57</code> | notes | flag --persistent-state-dir registered in MakeGenAutobahnConfigCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-primary-j57</code> | notes | flag --primary registered in MakeLightCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-priv-validator-laddr-j57</code> | notes | flag --priv-validator-laddr registered in AddNodeFlags; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-rpc-laddr-2-j57</code> | notes | flag --rpc.laddr registered in AddNodeFlags; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-rpc-laddr-j57</code> | notes | flag --rpc.laddr registered in MakeInspectCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-rpc-pprof-laddr-j57</code> | notes | flag --rpc.pprof-laddr registered in AddNodeFlags; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-rpc-unsafe-j57</code> | notes | flag --rpc.unsafe registered in AddNodeFlags; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-sequential-j57</code> | notes | flag --sequential registered in MakeLightCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-start-height-j57</code> | notes | flag --start-height registered in MakeReindexEventCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-trust-level-j57</code> | notes | flag --trust-level registered in MakeLightCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-trusting-period-j57</code> | notes | flag --trusting-period registered in MakeLightCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-flag-witnesses-j57</code> | notes | flag --witnesses registered in MakeLightCommand; read by the same command |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-gen-autobahn-config-j57</code> | notes | constructed by MakeGenAutobahnConfigCommand; mounted by the node binary |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-gen-node-key-j57</code> | notes | constructed by GenNodeKeyCmd; mounted by the node binary |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-gen-validator-j57</code> | notes | constructed by MakeGenValidatorCommand; mounted by the node binary |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-inspect-j57</code> | notes | constructed by MakeInspectCommand; mounted by the node binary |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-key-migrate-j57</code> | notes | constructed by MakeKeyMigrateCommand; mounted by the node binary |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-light-j57</code> | notes | constructed by MakeLightCommand; mounted by the node binary |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-peers-j57</code> | notes | constructed by MakeResetCommand; mounted by the node binary |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-reindex-event-j57</code> | notes | constructed by MakeReindexEventCommand; mounted by the node binary |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-reset-j57</code> | notes | constructed by MakeResetCommand; mounted by the node binary |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-unsafe-all-j57</code> | notes | constructed by MakeResetCommand; mounted by the node binary |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-unsafe-reset-all-j57</code> | notes | constructed by MakeUnsafeResetAllCommand; mounted by the node binary |
| <code>interface.15.cli-consensus-cmd-tendermint-commands-unsafe-signer-j57</code> | notes | constructed by MakeResetCommand; mounted by the node binary |
| <code>interface.15.cli-consensus-libs-cli-completion-j57</code> | notes | constructed by NewCompletionCmd; mounted by the node binary |
| <code>interface.15.cli-consensus-libs-cli-flag-flag-zsh-j57</code> | notes | flag --flagZsh registered in NewCompletionCmd; read by the same command |
| <code>interface.15.cli-consensus-libs-cli-flag-home-flag-j57</code> | notes | flag --HomeFlag registered in PrepareBaseCmd; read by the same command |
| <code>interface.15.cli-consensus-libs-cli-flag-trace-flag-j57</code> | notes | flag --TraceFlag registered in PrepareBaseCmd; read by the same command |
| <code>interface.15.go-consensus-cmd-tendermint-commands-err-autobahn-offline-store-j57</code> | notes | planned sentinel returned by loadStateAndBlockStore, MakeInspectCommand RunE and RollbackStateToTargetHeight when Config.AutobahnConfigFile is non-empty |

Contract context <code>contract_context.15.offline-cli-j57</code>:

| Field | Frozen rule |
|---|---|
| interface | Offline consensus CLI commands of consensus/cmd/tendermint/commands: reset blockchain, reindex-event, inspect, and the LoadTendermintState and RollbackStateToTargetHeight functions of consensus/cmd/tendermint/commands/rollback.go:12 and :32 |
| storage | data/autobahn is the default Autobahn persistent state directory (consensus/cmd/tendermint/commands/gen_autobahn_config.go:113); CometBFT stores under data/ and data/tendermint/ |
| errors | ErrAutobahnOfflineStore (planned) when autobahn-config-file is set |
| events | None |
| gas | Not applicable |
| activation | Refusal is decided by Config.AutobahnConfigFile at consensus/config/config.go:80 |
| invariants | No offline tool opens CometBFT stores of an Autobahn node; reset blockchain removes data/autobahn |
| pause | No pause |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.15.268.1</code> | req | 15268.1 |
| <code>acceptance.15.268.1</code> | scenario | Given a data directory holding autobahn/wal/0.log and tendermint/blockstore.db, when the test calls ResetState on it, then neither data/autobahn nor data/tendermint/blockstore.db exists and the data directory itself exists |
| <code>acceptance.15.268.1</code> | test | consensus/cmd/tendermint/commands/reset_test.go::TestResetState_RemovesAutobahnState (planned) |
| <code>acceptance.15.268.1</code> | qualification | UNRUN |
| <code>req.15268</code> | ac_1 | reset blockchain removes the Autobahn persistent state directory data/autobahn; at HEAD ResetState at consensus/cmd/tendermint/commands/reset.go:126-146 leaves it, so a reset node replays stale Autobahn blocks. |
| <code>acceptance.15.269.1</code> | req | 15269.1 |
| <code>acceptance.15.269.1</code> | scenario | Given a config whose AutobahnConfigFile is autobahn.json and whose data directory holds blockstore.db and state.db, when the test calls loadStateAndBlockStore, then it returns nil stores and an error matching ErrAutobahnOfflineStore |
| <code>acceptance.15.269.1</code> | test | consensus/cmd/tendermint/commands/reindex_event_test.go::TestLoadBlockStoreRefusesUnderAutobahn (planned) |
| <code>acceptance.15.269.1</code> | qualification | UNRUN |
| <code>req.15269</code> | ac_1 | loadStateAndBlockStore refuses with ErrAutobahnOfflineStore when autobahn-config-file is set; at HEAD consensus/cmd/tendermint/commands/reindex_event.go:149-175 opens the CometBFT stores, which Autobahn does not feed. |
| <code>acceptance.15.269.2</code> | req | 15269.2 |
| <code>acceptance.15.269.2</code> | scenario | Given the same config, when the test executes the reindex-event command with --start-height 1 --end-height 2, then the command returns an error matching ErrAutobahnOfflineStore and writes no event sink entry |
| <code>acceptance.15.269.2</code> | test | consensus/cmd/tendermint/commands/reindex_event_test.go::TestReIndexEventRefusesUnderAutobahn (planned) |
| <code>acceptance.15.269.2</code> | qualification | UNRUN |
| <code>req.15269</code> | ac_2 | reindex-event under an Autobahn config fails with ErrAutobahnOfflineStore before reading any store. |
| <code>acceptance.15.270.1</code> | req | 15270.1 |
| <code>acceptance.15.270.1</code> | scenario | Given a config whose AutobahnConfigFile is set and whose data directory holds blockstore.db and state.db, when the test calls RollbackStateToTargetHeight(cfg, false, 1), then it returns height -1, a nil hash and an error matching ErrAutobahnOfflineStore that starts with rollback:, and state.db keeps its size |
| <code>acceptance.15.270.1</code> | test | consensus/cmd/tendermint/commands/rollback_test.go::TestRollbackRefusesUnderAutobahn (planned) |
| <code>acceptance.15.270.1</code> | qualification | UNRUN |
| <code>req.15270</code> | ac_1 | RollbackStateToTargetHeight refuses under an Autobahn config with an error naming the rollback; its caller at sdk/server/rollback.go:16 surfaces it. |
| <code>acceptance.15.270.2</code> | req | 15270.2 |
| <code>acceptance.15.270.2</code> | scenario | Given the same config, when the test calls LoadTendermintState, then it returns an empty state and an error matching ErrAutobahnOfflineStore |
| <code>acceptance.15.270.2</code> | test | consensus/cmd/tendermint/commands/rollback_test.go::TestLoadTendermintStateRefusesUnderAutobahn (planned) |
| <code>acceptance.15.270.2</code> | qualification | UNRUN |
| <code>req.15270</code> | ac_2 | LoadTendermintState refuses under an Autobahn config; its caller at sdk/server/rollback.go:68 surfaces the error. |
| <code>acceptance.15.271.1</code> | req | 15271.1 |
| <code>acceptance.15.271.1</code> | scenario | Given a config whose AutobahnConfigFile is set, when the test executes the command of MakeInspectCommand with a cancelled context, then it returns an error matching ErrAutobahnOfflineStore and opens no RPC listener |
| <code>acceptance.15.271.1</code> | test | consensus/cmd/tendermint/commands/inspect_test.go::TestInspectRefusesUnderAutobahn (planned) |
| <code>acceptance.15.271.1</code> | qualification | UNRUN |
| <code>req.15271</code> | ac_1 | inspect refuses under an Autobahn config; at HEAD consensus/cmd/tendermint/commands/inspect.go:30-44 serves the unfed CometBFT stores. |
| <code>capability.15.25.1</code> | title | Remove the Autobahn state directory in reset blockchain |
| <code>capability.15.25.1</code> | reqs | 15268.1 |
| <code>capability.15.25.2</code> | title | Refuse offline block and state store tooling when an Autobahn config is set |
| <code>capability.15.25.2</code> | reqs | 15269.1<br>15269.2 |
| <code>capability.15.25.3</code> | title | Name the rollback refusal under Autobahn |
| <code>capability.15.25.3</code> | reqs | 15270.1<br>15270.2 |
| <code>capability.15.25.4</code> | title | Refuse inspect under Autobahn |
| <code>capability.15.25.4</code> | reqs | 15271.1 |

## env

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.15.env-consensus-cli-env-prefix-j57</code> | <code>env_var</code> | <code>stable</code> | <code>consensus/libs/cli/setup.go:54</code> | <code>viper.SetEnvPrefix(prefix)</code> |
| <code>interface.15.env-consensus-tmhome-j57</code> | <code>env_var</code> | <code>stable</code> | <code>consensus/test/e2e/node/main.go:271</code> | <code>home := os.Getenv("TMHOME")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.15.env-consensus-cli-env-prefix-j57</code> | notes | every flag of a command prepared by libs/cli is also read from &lt;PREFIX&gt;_&lt;FLAG&gt; environment variables via AutomaticEnv |
| <code>interface.15.env-consensus-tmhome-j57</code> | notes | e2e node home directory |

## go-api

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.15.go-consensus-autobahn-types-app-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/app_proposal.go:13</code> | <code>type AppHash []byte</code> |
| <code>interface.15.go-consensus-autobahn-types-app-proposal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/app_proposal.go:16</code> | <code>type AppProposal struct</code> |
| <code>interface.15.go-consensus-autobahn-types-app-qc-conv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/app_qc.go:44</code> | <code>var AppQCConv = protoutils.Conv[*AppQC, *pb.AppQC]</code> |
| <code>interface.15.go-consensus-autobahn-types-app-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/app_qc.go:12</code> | <code>type AppQC struct</code> |
| <code>interface.15.go-consensus-autobahn-types-app-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/app_vote.go:12</code> | <code>type AppVote struct</code> |
| <code>interface.15.go-consensus-autobahn-types-block-conv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/block.go:274</code> | <code>var BlockConv = protoutils.Conv[*Block, *pb.Block]</code> |
| <code>interface.15.go-consensus-autobahn-types-block-header-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/block.go:32</code> | <code>type BlockHeaderHash hashable.Hash[*pb.BlockHeader]</code> |
| <code>interface.15.go-consensus-autobahn-types-block-header-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/block.go:44</code> | <code>type BlockHeader struct</code> |
| <code>interface.15.go-consensus-autobahn-types-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/block.go:100</code> | <code>type Block struct</code> |
| <code>interface.15.go-consensus-autobahn-types-block-number-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/block.go:26</code> | <code>type BlockNumber uint64</code> |
| <code>interface.15.go-consensus-autobahn-types-commit-qc-conv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/commit_qc.go:122</code> | <code>var CommitQCConv = protoutils.Conv[*CommitQC, *pb.CommitQC]</code> |
| <code>interface.15.go-consensus-autobahn-types-commit-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/commit_qc.go:13</code> | <code>type CommitQC struct</code> |
| <code>interface.15.go-consensus-autobahn-types-commit-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/commit_vote.go:12</code> | <code>type CommitVote struct</code> |
| <code>interface.15.go-consensus-autobahn-types-committee-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/committee.go:26</code> | <code>type Committee struct</code> |
| <code>interface.15.go-consensus-autobahn-types-consensus-req-commit-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/consensus.go:21</code> | <code>type ConsensusReqCommitVote struct{ *Signed[*CommitVote] }</code> |
| <code>interface.15.go-consensus-autobahn-types-consensus-req-conv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/consensus.go:36</code> | <code>var ConsensusReqConv = protoutils.Conv[ConsensusReq, *pb.ConsensusReq]</code> |
| <code>interface.15.go-consensus-autobahn-types-consensus-req-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/consensus.go:12</code> | <code>type ConsensusReq interface</code> |
| <code>interface.15.go-consensus-autobahn-types-consensus-req-prepare-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/consensus.go:18</code> | <code>type ConsensusReqPrepareVote struct{ *Signed[*PrepareVote] }</code> |
| <code>interface.15.go-consensus-autobahn-types-full-commit-qc-conv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/commit_qc.go:143</code> | <code>var FullCommitQCConv = protoutils.Conv[*FullCommitQC, *pb.FullCommitQC]</code> |
| <code>interface.15.go-consensus-autobahn-types-full-commit-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/commit_qc.go:63</code> | <code>type FullCommitQC struct</code> |
| <code>interface.15.go-consensus-autobahn-types-full-proposal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/proposal.go:273</code> | <code>type FullProposal struct</code> |
| <code>interface.15.go-consensus-autobahn-types-full-timeout-vote-conv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/timeout.go:241</code> | <code>var FullTimeoutVoteConv = protoutils.Conv[*FullTimeoutVote, *pb.FullTimeoutVote]</code> |
| <code>interface.15.go-consensus-autobahn-types-full-timeout-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/timeout.go:40</code> | <code>type FullTimeoutVote struct</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-app-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:197</code> | <code>func GenAppHash(rng utils.Rng) AppHash</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-app-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:212</code> | <code>func GenAppQC(rng utils.Rng) *AppQC</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-app-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:207</code> | <code>func GenAppVote(rng utils.Rng) *AppVote</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-block-header-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:94</code> | <code>func GenBlockHeaderHash(rng utils.Rng) BlockHeaderHash</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:123</code> | <code>func GenBlock(rng utils.Rng) *Block</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-block-number-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:84</code> | <code>func GenBlockNumber(rng utils.Rng) BlockNumber</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-commit-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:268</code> | <code>func GenCommitQC(rng utils.Rng) *CommitQC</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-commit-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:263</code> | <code>func GenCommitVote(rng utils.Rng) *CommitVote</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-committee-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:32</code> | <code>func GenCommittee(rng utils.Rng, size int) (*Committee, []SecretKey)</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-full-timeout-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:290</code> | <code>func GenFullTimeoutVote(rng utils.Rng) *FullTimeoutVote</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-global-block-number-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:235</code> | <code>func GenGlobalBlockNumber(rng utils.Rng) GlobalBlockNumber</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-lane-proposal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:141</code> | <code>func GenLaneProposal(rng utils.Rng) *LaneProposal</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-lane-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:146</code> | <code>func GenLaneVote(rng utils.Rng) *LaneVote</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-payload-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:113</code> | <code>func GenPayload(rng utils.Rng) *Payload</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-prepare-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:254</code> | <code>func GenPrepareQC(rng utils.Rng) *PrepareQC</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-prepare-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:249</code> | <code>func GenPrepareVote(rng utils.Rng) *PrepareVote</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-proposal-for-at-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:188</code> | <code>func GenProposalForAt(rng utils.Rng, c *Committee, view View) *Proposal</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-secret-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:26</code> | <code>func GenSecretKey(rng utils.Rng) SecretKey</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-signed-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:133</code> | <code>func GenSigned[T Msg](rng utils.Rng, msg T) *Signed[T]</code> |
| <code>interface.15.go-consensus-autobahn-types-gen-timeout-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:298</code> | <code>func GenTimeoutQC(rng utils.Rng) *TimeoutQC</code> |
| <code>interface.15.go-consensus-autobahn-types-global-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/block.go:107</code> | <code>type GlobalBlock struct</code> |
| <code>interface.15.go-consensus-autobahn-types-global-block-number-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/block.go:29</code> | <code>type GlobalBlockNumber uint64</code> |
| <code>interface.15.go-consensus-autobahn-types-global-range-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/proposal.go:64</code> | <code>type GlobalRange struct</code> |
| <code>interface.15.go-consensus-autobahn-types-global-range-opt-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/opt.go:50</code> | <code>func GlobalRangeOpt[T interface</code> |
| <code>interface.15.go-consensus-autobahn-types-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/msg.go:50</code> | <code>type Hash[T Msg] hashable.Hash[*pb.Msg]</code> |
| <code>interface.15.go-consensus-autobahn-types-lane-id-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/block.go:20</code> | <code>type LaneID = PublicKey</code> |
| <code>interface.15.go-consensus-autobahn-types-lane-proposal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/lane_proposal.go:12</code> | <code>type LaneProposal struct</code> |
| <code>interface.15.go-consensus-autobahn-types-lane-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/lane_qc.go:12</code> | <code>type LaneQC struct</code> |
| <code>interface.15.go-consensus-autobahn-types-lane-range-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/proposal.go:17</code> | <code>type LaneRange struct</code> |
| <code>interface.15.go-consensus-autobahn-types-lane-range-opt-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/opt.go:40</code> | <code>func LaneRangeOpt[T interface</code> |
| <code>interface.15.go-consensus-autobahn-types-lane-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/lane_vote.go:12</code> | <code>type LaneVote struct</code> |
| <code>interface.15.go-consensus-autobahn-types-max-block-proto-size-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/block.go:86</code> | <code>var MaxBlockProtoSize = func() uint64</code> |
| <code>interface.15.go-consensus-autobahn-types-max-txs-bytes-per-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/block.go:83</code> | <code>const MaxTxsBytesPerBlock = MaxTxsPerBlock * standardTxBytes</code> |
| <code>interface.15.go-consensus-autobahn-types-max-txs-per-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/block.go:78</code> | <code>const MaxTxsPerBlock uint64 = 2000</code> |
| <code>interface.15.go-consensus-autobahn-types-new-app-proposal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/app_proposal.go:24</code> | <code>func NewAppProposal(globalNumber GlobalBlockNumber, roadIndex RoadIndex, appHash AppHash) *AppProposal</code> |
| <code>interface.15.go-consensus-autobahn-types-new-app-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/app_qc.go:19</code> | <code>func NewAppQC(votes []*Signed[*AppVote]) *AppQC</code> |
| <code>interface.15.go-consensus-autobahn-types-new-app-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/app_vote.go:18</code> | <code>func NewAppVote(proposal *AppProposal) *AppVote</code> |
| <code>interface.15.go-consensus-autobahn-types-new-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/block.go:117</code> | <code>func NewBlock( lane LaneID, blockNumber BlockNumber, parentHash BlockHeaderHash, payload *Payload, ) *Block</code> |
| <code>interface.15.go-consensus-autobahn-types-new-commit-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/commit_qc.go:20</code> | <code>func NewCommitQC(votes []*Signed[*CommitVote]) *CommitQC</code> |
| <code>interface.15.go-consensus-autobahn-types-new-commit-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/commit_vote.go:18</code> | <code>func NewCommitVote(proposal *Proposal) *CommitVote</code> |
| <code>interface.15.go-consensus-autobahn-types-new-full-commit-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/commit_qc.go:70</code> | <code>func NewFullCommitQC(qc *CommitQC, headers []*BlockHeader) *FullCommitQC</code> |
| <code>interface.15.go-consensus-autobahn-types-new-full-timeout-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/timeout.go:47</code> | <code>func NewFullTimeoutVote(key SecretKey, view View, latestPrepareQC utils.Option[*PrepareQC]) *FullTimeoutVote</code> |
| <code>interface.15.go-consensus-autobahn-types-new-lane-proposal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/lane_proposal.go:18</code> | <code>func NewLaneProposal(block *Block) *LaneProposal</code> |
| <code>interface.15.go-consensus-autobahn-types-new-lane-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/lane_qc.go:19</code> | <code>func NewLaneQC(votes []*Signed[*LaneVote]) *LaneQC</code> |
| <code>interface.15.go-consensus-autobahn-types-new-lane-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/lane_vote.go:18</code> | <code>func NewLaneVote(header *BlockHeader) *LaneVote</code> |
| <code>interface.15.go-consensus-autobahn-types-new-prepare-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/prepare_qc.go:20</code> | <code>func NewPrepareQC(votes []*Signed[*PrepareVote]) *PrepareQC</code> |
| <code>interface.15.go-consensus-autobahn-types-new-prepare-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/prepare_vote.go:18</code> | <code>func NewPrepareVote(proposal *Proposal) *PrepareVote</code> |
| <code>interface.15.go-consensus-autobahn-types-new-proposal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/proposal.go:303</code> | <code>func NewProposal( key SecretKey, committee *Committee, viewSpec ViewSpec, timestamp time.Time, laneQCs map[LaneID]*LaneQC, appQC utils.Option[*AppQC], ) (*FullProposal, error)</code> |
| <code>interface.15.go-consensus-autobahn-types-new-reproposal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/proposal.go:283</code> | <code>func NewReproposal( key SecretKey, viewSpec ViewSpec, ) (*FullProposal, bool)</code> |
| <code>interface.15.go-consensus-autobahn-types-new-round-robin-election-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/committee.go:195</code> | <code>func NewRoundRobinElection(chainID string, replicas []PublicKey, firstBlock GlobalBlockNumber, genesisTimestamp time.Time) (*Committee, error)</code> |
| <code>interface.15.go-consensus-autobahn-types-new-timeout-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/timeout.go:102</code> | <code>func NewTimeoutQC(fullVotes []*FullTimeoutVote) *TimeoutQC</code> |
| <code>interface.15.go-consensus-autobahn-types-next-index-opt-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/opt.go:24</code> | <code>func NextIndexOpt[T interface</code> |
| <code>interface.15.go-consensus-autobahn-types-next-opt-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/opt.go:8</code> | <code>func NextOpt[I ~uint64, T interface</code> |
| <code>interface.15.go-consensus-autobahn-types-next-view-opt-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/opt.go:32</code> | <code>func NextViewOpt[T interface</code> |
| <code>interface.15.go-consensus-autobahn-types-payload-builder-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/block.go:160</code> | <code>type PayloadBuilder struct</code> |
| <code>interface.15.go-consensus-autobahn-types-payload-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/block.go:157</code> | <code>type PayloadHash hashable.Hash[*pb.Payload]</code> |
| <code>interface.15.go-consensus-autobahn-types-payload-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/block.go:168</code> | <code>type Payload struct</code> |
| <code>interface.15.go-consensus-autobahn-types-prepare-qc-conv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/prepare_qc.go:49</code> | <code>var PrepareQCConv = protoutils.Conv[*PrepareQC, *pb.PrepareQC]</code> |
| <code>interface.15.go-consensus-autobahn-types-prepare-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/prepare_qc.go:12</code> | <code>type PrepareQC struct</code> |
| <code>interface.15.go-consensus-autobahn-types-prepare-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/prepare_vote.go:12</code> | <code>type PrepareVote struct</code> |
| <code>interface.15.go-consensus-autobahn-types-proposal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/proposal.go:135</code> | <code>type Proposal struct</code> |
| <code>interface.15.go-consensus-autobahn-types-public-key-from-bytes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/msg.go:111</code> | <code>func PublicKeyFromBytes(b []byte) (PublicKey, error)</code> |
| <code>interface.15.go-consensus-autobahn-types-public-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/msg.go:99</code> | <code>type PublicKey struct</code> |
| <code>interface.15.go-consensus-autobahn-types-road-index-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/proposal.go:79</code> | <code>type RoadIndex uint64</code> |
| <code>interface.15.go-consensus-autobahn-types-secret-key-from-ed25519-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/msg.go:81</code> | <code>func SecretKeyFromED25519(key ed25519.SecretKey) SecretKey</code> |
| <code>interface.15.go-consensus-autobahn-types-secret-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/msg.go:74</code> | <code>type SecretKey struct</code> |
| <code>interface.15.go-consensus-autobahn-types-sign-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/msg.go:165</code> | <code>func Sign[T Msg](key SecretKey, msg T) *Signed[T]</code> |
| <code>interface.15.go-consensus-autobahn-types-signed-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/msg.go:189</code> | <code>type Signed[T Msg] struct</code> |
| <code>interface.15.go-consensus-autobahn-types-signed-msg-conv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/msg.go:326</code> | <code>func SignedMsgConv[T Msg]() *protoutils.Conv[*Signed[T], *pb.SignedMsg]</code> |
| <code>interface.15.go-consensus-autobahn-types-test-keys-with-weight-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/testonly.go:49</code> | <code>func TestKeysWithWeight(c *Committee, keys []SecretKey, minWeight uint64) []SecretKey</code> |
| <code>interface.15.go-consensus-autobahn-types-timeout-qc-conv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/timeout.go:265</code> | <code>var TimeoutQCConv = protoutils.Conv[*TimeoutQC, *pb.TimeoutQC]</code> |
| <code>interface.15.go-consensus-autobahn-types-timeout-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/timeout.go:95</code> | <code>type TimeoutQC struct</code> |
| <code>interface.15.go-consensus-autobahn-types-view-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/proposal.go:88</code> | <code>type View struct</code> |
| <code>interface.15.go-consensus-autobahn-types-view-spec-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/autobahn/types/proposal.go:108</code> | <code>type ViewSpec struct</code> |
| <code>interface.15.go-consensus-cmd-tendermint-commands-add-node-flags-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/run_node.go:15</code> | <code>func AddNodeFlags(cmd *cobra.Command, conf *cfg.Config)</code> |
| <code>interface.15.go-consensus-cmd-tendermint-commands-debug-get-debug-command-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/debug/debug.go:13</code> | <code>func GetDebugCommand() *cobra.Command</code> |
| <code>interface.15.go-consensus-cmd-tendermint-commands-gen-node-key-cmd-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/gen_node_key.go:14</code> | <code>var GenNodeKeyCmd = &amp;cobra.Command</code> |
| <code>interface.15.go-consensus-cmd-tendermint-commands-load-tendermint-state-j57</code> | <code>go_exported</code> | <code>change</code> | <code>consensus/cmd/tendermint/commands/rollback.go:12</code> | <code>func LoadTendermintState(config *config.Config) (state.State, error)</code> |
| <code>interface.15.go-consensus-cmd-tendermint-commands-make-gen-autobahn-config-command-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/gen_autobahn_config.go:23</code> | <code>func MakeGenAutobahnConfigCommand() *cobra.Command</code> |
| <code>interface.15.go-consensus-cmd-tendermint-commands-make-gen-validator-command-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/gen_validator.go:15</code> | <code>func MakeGenValidatorCommand() *cobra.Command</code> |
| <code>interface.15.go-consensus-cmd-tendermint-commands-make-inspect-command-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/inspect.go:17</code> | <code>func MakeInspectCommand(conf *config.Config) *cobra.Command</code> |
| <code>interface.15.go-consensus-cmd-tendermint-commands-make-key-migrate-command-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/key_migrate.go:14</code> | <code>func MakeKeyMigrateCommand(conf *config.Config) *cobra.Command</code> |
| <code>interface.15.go-consensus-cmd-tendermint-commands-make-light-command-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/light.go:28</code> | <code>func MakeLightCommand(conf *config.Config) *cobra.Command</code> |
| <code>interface.15.go-consensus-cmd-tendermint-commands-make-reindex-event-command-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/reindex_event.go:31</code> | <code>func MakeReindexEventCommand(conf *tmcfg.Config) *cobra.Command</code> |
| <code>interface.15.go-consensus-cmd-tendermint-commands-make-reset-command-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/reset.go:19</code> | <code>func MakeResetCommand(conf *config.Config) *cobra.Command</code> |
| <code>interface.15.go-consensus-cmd-tendermint-commands-make-unsafe-reset-all-command-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/reset.go:192</code> | <code>func MakeUnsafeResetAllCommand(conf *config.Config) *cobra.Command</code> |
| <code>interface.15.go-consensus-cmd-tendermint-commands-new-completion-cmd-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/completion.go:12</code> | <code>func NewCompletionCmd(rootCmd *cobra.Command, hidden bool) *cobra.Command</code> |
| <code>interface.15.go-consensus-cmd-tendermint-commands-parse-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/cmd/tendermint/commands/config.go:13</code> | <code>func ParseConfig(conf *config.Config) (*config.Config, error)</code> |
| <code>interface.15.go-consensus-cmd-tendermint-commands-rollback-state-to-target-height-j57</code> | <code>go_exported</code> | <code>change</code> | <code>consensus/cmd/tendermint/commands/rollback.go:32</code> | <code>func RollbackStateToTargetHeight(config *config.Config, removeBlock bool, targetHeight int64) (int64, []byte, error)</code> |
| <code>interface.15.go-consensus-crypto-address-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/crypto.go:27</code> | <code>func AddressHash(bz []byte) Address</code> |
| <code>interface.15.go-consensus-crypto-address-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/crypto.go:21</code> | <code>type Address = bytes.HexBytes</code> |
| <code>interface.15.go-consensus-crypto-address-size-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/crypto.go:15</code> | <code>AddressSize = 20</code> |
| <code>interface.15.go-consensus-crypto-c-rand-bytes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/random.go:9</code> | <code>func CRandBytes(numBytes int) []byte</code> |
| <code>interface.15.go-consensus-crypto-checksum-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/crypto.go:39</code> | <code>func Checksum(bz []byte) Hash</code> |
| <code>interface.15.go-consensus-crypto-ed25519-batch-verifier-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/ed25519/ed25519.go:186</code> | <code>type BatchVerifier struct</code> |
| <code>interface.15.go-consensus-crypto-ed25519-err-bad-sig-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/ed25519/ed25519.go:17</code> | <code>type ErrBadSig struct</code> |
| <code>interface.15.go-consensus-crypto-ed25519-generate-secret-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/ed25519/ed25519.go:95</code> | <code>func GenerateSecretKey() SecretKey</code> |
| <code>interface.15.go-consensus-crypto-ed25519-key-type-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/ed25519/json.go:12</code> | <code>const KeyType = "ed25519"</code> |
| <code>interface.15.go-consensus-crypto-ed25519-new-batch-verifier-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/ed25519/ed25519.go:190</code> | <code>func NewBatchVerifier() *BatchVerifier</code> |
| <code>interface.15.go-consensus-crypto-ed25519-new-tag-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/ed25519/ed25519.go:143</code> | <code>func NewTag(tag string) (Tag, error)</code> |
| <code>interface.15.go-consensus-crypto-ed25519-public-key-from-bytes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/ed25519/conv.go:21</code> | <code>func PublicKeyFromBytes(key []byte) (PublicKey, error)</code> |
| <code>interface.15.go-consensus-crypto-ed25519-public-key-from-string-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/ed25519/conv.go:55</code> | <code>func PublicKeyFromString(s string) (PublicKey, error)</code> |
| <code>interface.15.go-consensus-crypto-ed25519-public-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/ed25519/ed25519.go:121</code> | <code>type PublicKey struct</code> |
| <code>interface.15.go-consensus-crypto-ed25519-public-key-name-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/ed25519/json.go:11</code> | <code>const PublicKeyName = "tendermint/PubKeyEd25519"</code> |
| <code>interface.15.go-consensus-crypto-ed25519-secret-key-from-secret-bytes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/ed25519/ed25519.go:64</code> | <code>func SecretKeyFromSecretBytes(b []byte) (SecretKey, error)</code> |
| <code>interface.15.go-consensus-crypto-ed25519-secret-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/ed25519/ed25519.go:40</code> | <code>type SecretKey struct</code> |
| <code>interface.15.go-consensus-crypto-ed25519-signature-from-bytes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/ed25519/conv.go:29</code> | <code>func SignatureFromBytes(sig []byte) (Signature, error)</code> |
| <code>interface.15.go-consensus-crypto-ed25519-signature-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/ed25519/ed25519.go:127</code> | <code>type Signature struct</code> |
| <code>interface.15.go-consensus-crypto-ed25519-test-secret-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/ed25519/ed25519.go:85</code> | <code>func TestSecretKey(seed []byte) SecretKey</code> |
| <code>interface.15.go-consensus-crypto-err-bad-sig-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/crypto.go:45</code> | <code>type ErrBadSig = ed25519.ErrBadSig</code> |
| <code>interface.15.go-consensus-crypto-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/crypto.go:32</code> | <code>type Hash [sha256.Size]byte</code> |
| <code>interface.15.go-consensus-crypto-hash-size-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/crypto.go:12</code> | <code>HashSize = sha256.Size</code> |
| <code>interface.15.go-consensus-crypto-merkle-default-proof-runtime-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/merkle/proof_op.go:135</code> | <code>func DefaultProofRuntime() (prt *ProofRuntime)</code> |
| <code>interface.15.go-consensus-crypto-merkle-hash-from-byte-slices-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/merkle/tree.go:11</code> | <code>func HashFromByteSlices(items [][]byte) []byte</code> |
| <code>interface.15.go-consensus-crypto-merkle-key-encoding-url-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/merkle/proof_key_path.go:54</code> | <code>KeyEncodingURL keyEncoding = iota</code> |
| <code>interface.15.go-consensus-crypto-merkle-key-path-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/merkle/proof_key_path.go:64</code> | <code>type KeyPath []Key</code> |
| <code>interface.15.go-consensus-crypto-merkle-new-proof-runtime-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/merkle/proof_op.go:80</code> | <code>func NewProofRuntime() *ProofRuntime</code> |
| <code>interface.15.go-consensus-crypto-merkle-op-decoder-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/merkle/proof_op.go:74</code> | <code>type OpDecoder func(tmcrypto.ProofOp) (ProofOperator, error)</code> |
| <code>interface.15.go-consensus-crypto-merkle-proof-from-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/merkle/proof.go:135</code> | <code>func ProofFromProto(pb *tmcrypto.Proof) (*Proof, error)</code> |
| <code>interface.15.go-consensus-crypto-merkle-proof-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/merkle/proof.go:26</code> | <code>type Proof struct</code> |
| <code>interface.15.go-consensus-crypto-merkle-proof-operator-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/merkle/proof_op.go:21</code> | <code>type ProofOperator interface</code> |
| <code>interface.15.go-consensus-crypto-merkle-proof-runtime-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/merkle/proof_op.go:76</code> | <code>type ProofRuntime struct</code> |
| <code>interface.15.go-consensus-crypto-merkle-proofs-from-byte-slices-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/merkle/proof.go:35</code> | <code>func ProofsFromByteSlices(items [][]byte) (rootHash []byte, proofs []*Proof)</code> |
| <code>interface.15.go-consensus-crypto-new-batch-verifier-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/crypto.go:51</code> | <code>func NewBatchVerifier() *BatchVerifier</code> |
| <code>interface.15.go-consensus-crypto-priv-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/crypto.go:42</code> | <code>type PrivKey = ed25519.SecretKey</code> |
| <code>interface.15.go-consensus-crypto-pub-key-from-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/conv.go:28</code> | <code>func PubKeyFromProto(k pb.PublicKey) (PubKey, error)</code> |
| <code>interface.15.go-consensus-crypto-pub-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/crypto.go:41</code> | <code>type PubKey = ed25519.PublicKey</code> |
| <code>interface.15.go-consensus-crypto-pub-key-to-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/conv.go:23</code> | <code>func PubKeyToProto(k PubKey) pb.PublicKey</code> |
| <code>interface.15.go-consensus-crypto-sig-from-bytes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/crypto.go:47</code> | <code>func SigFromBytes(raw []byte) (Sig, error)</code> |
| <code>interface.15.go-consensus-crypto-sig-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/crypto.go:43</code> | <code>type Sig = ed25519.Signature</code> |
| <code>interface.15.go-consensus-crypto-tmhash-size-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/tmhash/hash.go:9</code> | <code>Size = sha256.Size</code> |
| <code>interface.15.go-consensus-crypto-tmhash-sum-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/crypto/tmhash/hash.go:18</code> | <code>func Sum(bz []byte) []byte</code> |
| <code>interface.15.go-consensus-export-new-block-store-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/export/export.go:11</code> | <code>var NewBlockStore = store.NewBlockStore</code> |
| <code>interface.15.go-consensus-export-new-query-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/export/export.go:13</code> | <code>var NewQuery = query.New</code> |
| <code>interface.15.go-consensus-export-new-store-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/export/export.go:12</code> | <code>var NewStore = state.NewStore</code> |
| <code>interface.15.go-consensus-export-query-all-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/export/export.go:14</code> | <code>var QueryAll = query.All</code> |
| <code>interface.15.go-consensus-export-query-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/export/export.go:9</code> | <code>type Query = query.Query</code> |
| <code>interface.15.go-consensus-internal-autobahn-avail-blocks-per-lane-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/avail/state.go:21</code> | <code>const BlocksPerLane = 3 * BlocksPerLanePerCommit</code> |
| <code>interface.15.go-consensus-internal-autobahn-avail-new-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/avail/state.go:153</code> | <code>func NewState(key types.SecretKey, data *data.State, stateDir utils.Option[string]) (*State, error)</code> |
| <code>interface.15.go-consensus-internal-autobahn-avail-run-test-network-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/avail/testonly.go:12</code> | <code>func RunTestNetwork(ctx context.Context, states []*State) error</code> |
| <code>interface.15.go-consensus-internal-autobahn-avail-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/avail/state.go:33</code> | <code>type State struct</code> |
| <code>interface.15.go-consensus-internal-autobahn-consensus-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/consensus/state.go:23</code> | <code>type Config struct</code> |
| <code>interface.15.go-consensus-internal-autobahn-consensus-new-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/consensus/state.go:79</code> | <code>func NewState(cfg *Config, data *data.State) (*State, error)</code> |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-block-persister-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/consensus/persist/blocks.go:172</code> | <code>type BlockPersister struct</code> |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-commit-qc-persister-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/consensus/persist/commitqcs.go:80</code> | <code>type CommitQCPersister struct</code> |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-full-commit-qc-persister-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/consensus/persist/fullcommitqcs.go:65</code> | <code>type FullCommitQCPersister struct</code> |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-global-block-persister-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/consensus/persist/globalblocks.go:101</code> | <code>type GlobalBlockPersister struct</code> |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-loaded-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/consensus/persist/blocks.go:21</code> | <code>type LoadedBlock struct</code> |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-loaded-commit-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/consensus/persist/commitqcs.go:15</code> | <code>type LoadedCommitQC struct</code> |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-loaded-global-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/consensus/persist/globalblocks.go:18</code> | <code>type LoadedGlobalBlock struct</code> |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-new-block-persister-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/consensus/persist/blocks.go:194</code> | <code>func NewBlockPersister(stateDir utils.Option[string]) (*BlockPersister, map[types.LaneID][]LoadedBlock, error)</code> |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-new-commit-qc-persister-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/consensus/persist/commitqcs.go:93</code> | <code>func NewCommitQCPersister(stateDir utils.Option[string]) (*CommitQCPersister, []LoadedCommitQC, error)</code> |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-new-full-commit-qc-persister-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/consensus/persist/fullcommitqcs.go:72</code> | <code>func NewFullCommitQCPersister(stateDir utils.Option[string], committee *types.Committee) (*FullCommitQCPersister, error)</code> |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-new-global-block-persister-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/consensus/persist/globalblocks.go:108</code> | <code>func NewGlobalBlockPersister(stateDir utils.Option[string], committee *types.Committee) (*GlobalBlockPersister, error)</code> |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-new-persister-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/consensus/persist/persist.go:106</code> | <code>func NewPersister[T protoutils.Message](dir utils.Option[string], prefix string) (Persister[T], utils.Option[T], error)</code> |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-persister-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/consensus/persist/persist.go:78</code> | <code>type Persister[T protoutils.Message] interface</code> |
| <code>interface.15.go-consensus-internal-autobahn-consensus-run-test-network-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/consensus/testonly.go:12</code> | <code>func RunTestNetwork(ctx context.Context, states []*State) error</code> |
| <code>interface.15.go-consensus-internal-autobahn-consensus-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/consensus/state.go:39</code> | <code>type State struct</code> |
| <code>interface.15.go-consensus-internal-autobahn-data-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/data/state.go:27</code> | <code>type Config struct</code> |
| <code>interface.15.go-consensus-internal-autobahn-data-err-not-found-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/data/state.go:21</code> | <code>var ErrNotFound = errors.New("not found")</code> |
| <code>interface.15.go-consensus-internal-autobahn-data-err-pruned-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/data/state.go:24</code> | <code>var ErrPruned = errors.New("pruned")</code> |
| <code>interface.15.go-consensus-internal-autobahn-data-new-data-wal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/data/state.go:119</code> | <code>func NewDataWAL(stateDir utils.Option[string], committee *types.Committee) (*DataWAL, error)</code> |
| <code>interface.15.go-consensus-internal-autobahn-data-new-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/data/state.go:301</code> | <code>func NewState(cfg *Config, dataWAL *DataWAL) (*State, error)</code> |
| <code>interface.15.go-consensus-internal-autobahn-data-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/data/state.go:291</code> | <code>type State struct</code> |
| <code>interface.15.go-consensus-internal-autobahn-data-test-commit-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/data/testonly.go:33</code> | <code>func TestCommitQC( rng utils.Rng, committee *types.Committee, keys []types.SecretKey, prev utils.Option[*types.CommitQC], ) (*types.FullCommitQC, []*types.Block)</code> |
| <code>interface.15.go-consensus-internal-autobahn-pb-schema-for-app-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/pb/autobahn.wireguard.go:108</code> | <code>var SchemaForAppQC = &amp;wireguard.Schema</code> |
| <code>interface.15.go-consensus-internal-autobahn-pb-schema-for-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/pb/autobahn.wireguard.go:17</code> | <code>var SchemaForBlock = &amp;wireguard.Schema</code> |
| <code>interface.15.go-consensus-internal-autobahn-pb-schema-for-commit-qc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/pb/autobahn.wireguard.go:56</code> | <code>var SchemaForCommitQC = &amp;wireguard.Schema</code> |
| <code>interface.15.go-consensus-internal-autobahn-pb-schema-for-signed-msg-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/pb/autobahn.wireguard.go:125</code> | <code>var SchemaForSignedMsg = &amp;wireguard.Schema</code> |
| <code>interface.15.go-consensus-internal-autobahn-producer-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/producer/state.go:20</code> | <code>type Config struct</code> |
| <code>interface.15.go-consensus-internal-autobahn-producer-new-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/producer/state.go:50</code> | <code>func NewState(cfg *Config, consensus *consensus.State) *State</code> |
| <code>interface.15.go-consensus-internal-autobahn-producer-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/autobahn/producer/state.go:41</code> | <code>type State struct</code> |
| <code>interface.15.go-consensus-internal-blocksync-block-sync-channel-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/blocksync/reactor.go:28</code> | <code>BlockSyncChannel = p2p.ChannelID(0x40)</code> |
| <code>interface.15.go-consensus-internal-blocksync-consensus-reactor-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/blocksync/reactor.go:72</code> | <code>type ConsensusReactor interface</code> |
| <code>interface.15.go-consensus-internal-blocksync-new-reactor-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/blocksync/reactor.go:166</code> | <code>func NewReactor( stateStore sm.Store, store *store.BlockStore, router *p2p.Router, syncerConfig utils.Option[SyncerConfig], ) (*Reactor, error)</code> |
| <code>interface.15.go-consensus-internal-blocksync-reactor-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/blocksync/reactor.go:116</code> | <code>type Reactor struct</code> |
| <code>interface.15.go-consensus-internal-blocksync-syncer-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/blocksync/reactor.go:104</code> | <code>type SyncerConfig struct</code> |
| <code>interface.15.go-consensus-internal-consensus-data-channel-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/reactor.go:85</code> | <code>DataChannel = p2p.ChannelID(0x21)</code> |
| <code>interface.15.go-consensus-internal-consensus-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:23</code> | <code>type Metrics struct</code> |
| <code>interface.15.go-consensus-internal-consensus-new-handshaker-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/replay.go:123</code> | <code>func NewHandshaker( stateStore sm.Store, state sm.State, store sm.BlockStore, eventBus *eventbus.EventBus, genDoc *types.GenesisDoc, consensusPolicy types.ConsensusPolicy, ) *Handshaker</code> |
| <code>interface.15.go-consensus-internal-consensus-new-reactor-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/reactor.go:112</code> | <code>func NewReactor( cs *State, router *p2p.Router, eventBus *eventbus.EventBus, waitSync bool, metrics *Metrics, cfg *config.Config, ) (*Reactor, error)</code> |
| <code>interface.15.go-consensus-internal-consensus-new-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/state.go:162</code> | <code>func NewState( cfg *config.ConsensusConfig, wal *WAL, store sm.Store, blockExec *sm.BlockExecutor, blockStore sm.BlockStore, txMempool *mempool.TxMempool, evpool evidencePool, eventBus *eventbus.EventBus, traceProviderOps []trace.TracerProviderOption, metrics *Metrics, ) *State</code> |
| <code>interface.15.go-consensus-internal-consensus-nop-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.gen.go:296</code> | <code>func NopMetrics() *Metrics</code> |
| <code>interface.15.go-consensus-internal-consensus-open-wal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/wal.go:198</code> | <code>func OpenWAL(walFile string) (res *WAL, resErr error)</code> |
| <code>interface.15.go-consensus-internal-consensus-prometheus-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.gen.go:11</code> | <code>func PrometheusMetrics(namespace string, labelsAndValues ...string) *Metrics</code> |
| <code>interface.15.go-consensus-internal-consensus-reactor-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/reactor.go:93</code> | <code>type Reactor struct</code> |
| <code>interface.15.go-consensus-internal-consensus-state-channel-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/reactor.go:84</code> | <code>StateChannel = p2p.ChannelID(0x20)</code> |
| <code>interface.15.go-consensus-internal-consensus-types-hrs-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/types/peer_round_state.go:13</code> | <code>type HRS struct</code> |
| <code>interface.15.go-consensus-internal-consensus-types-leader-pin-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/types/round_state.go:422</code> | <code>type LeaderPin struct</code> |
| <code>interface.15.go-consensus-internal-consensus-types-new-height-vote-set-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/types/height_vote_set.go:51</code> | <code>func NewHeightVoteSet(chainID string, height int64, valSet *types.ValidatorSet) *HeightVoteSet</code> |
| <code>interface.15.go-consensus-internal-consensus-types-new-leader-pin-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/types/round_state.go:429</code> | <code>func NewLeaderPin(activationHeight int64, leaders []crypto.PubKey) (LeaderPin, error)</code> |
| <code>interface.15.go-consensus-internal-consensus-types-new-safe-round-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/types/round_state.go:76</code> | <code>func NewSafeRoundState() SafeRoundState</code> |
| <code>interface.15.go-consensus-internal-consensus-types-peer-round-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/types/peer_round_state.go:29</code> | <code>type PeerRoundState struct</code> |
| <code>interface.15.go-consensus-internal-consensus-types-round-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/types/round_state.go:380</code> | <code>type RoundState struct</code> |
| <code>interface.15.go-consensus-internal-consensus-types-round-step-commit-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/types/round_state.go:36</code> | <code>RoundStepCommit = RoundStepType(0x08) // Entered commit state machine</code> |
| <code>interface.15.go-consensus-internal-consensus-types-round-step-new-height-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/types/round_state.go:29</code> | <code>RoundStepNewHeight = RoundStepType(0x01) // Wait til CommitTime + timeoutCommit</code> |
| <code>interface.15.go-consensus-internal-consensus-types-round-step-new-round-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/types/round_state.go:30</code> | <code>RoundStepNewRound = RoundStepType(0x02) // Setup new round and go to RoundStepPropose</code> |
| <code>interface.15.go-consensus-internal-consensus-types-round-step-precommit-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/types/round_state.go:34</code> | <code>RoundStepPrecommit = RoundStepType(0x06) // Did precommit, gossip precommits</code> |
| <code>interface.15.go-consensus-internal-consensus-types-round-step-precommit-wait-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/types/round_state.go:35</code> | <code>RoundStepPrecommitWait = RoundStepType(0x07) // Did receive any +2/3 precommits, start timeout</code> |
| <code>interface.15.go-consensus-internal-consensus-types-round-step-prevote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/types/round_state.go:32</code> | <code>RoundStepPrevote = RoundStepType(0x04) // Did prevote, gossip prevotes</code> |
| <code>interface.15.go-consensus-internal-consensus-types-round-step-prevote-wait-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/types/round_state.go:33</code> | <code>RoundStepPrevoteWait = RoundStepType(0x05) // Did receive any +2/3 prevotes, start timeout</code> |
| <code>interface.15.go-consensus-internal-consensus-types-round-step-propose-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/types/round_state.go:31</code> | <code>RoundStepPropose = RoundStepType(0x03) // Did propose, gossip proposal</code> |
| <code>interface.15.go-consensus-internal-consensus-types-round-step-type-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/types/round_state.go:25</code> | <code>type RoundStepType uint8 // These must be numeric, ordered.</code> |
| <code>interface.15.go-consensus-internal-consensus-types-safe-round-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/types/round_state.go:71</code> | <code>type SafeRoundState struct</code> |
| <code>interface.15.go-consensus-internal-consensus-vote-channel-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/reactor.go:86</code> | <code>VoteChannel = p2p.ChannelID(0x22)</code> |
| <code>interface.15.go-consensus-internal-consensus-vote-set-bits-channel-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/consensus/reactor.go:87</code> | <code>VoteSetBitsChannel = p2p.ChannelID(0x23)</code> |
| <code>interface.15.go-consensus-internal-eventbus-event-bus-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/eventbus/event_bus.go:26</code> | <code>type EventBus struct</code> |
| <code>interface.15.go-consensus-internal-eventbus-new-default-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/eventbus/event_bus.go:32</code> | <code>func NewDefault() *EventBus</code> |
| <code>interface.15.go-consensus-internal-eventbus-subscription-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/eventbus/event_bus.go:18</code> | <code>type Subscription interface</code> |
| <code>interface.15.go-consensus-internal-eventlog-cursor-cursor-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/eventlog/cursor/cursor.go:46</code> | <code>type Cursor struct</code> |
| <code>interface.15.go-consensus-internal-eventlog-cursor-source-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/eventlog/cursor/cursor.go:14</code> | <code>type Source struct</code> |
| <code>interface.15.go-consensus-internal-eventlog-err-log-pruned-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/eventlog/eventlog.go:118</code> | <code>var ErrLogPruned = errors.New("log pruned")</code> |
| <code>interface.15.go-consensus-internal-eventlog-err-stop-scan-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/eventlog/eventlog.go:113</code> | <code>var ErrStopScan = errors.New("stop scanning")</code> |
| <code>interface.15.go-consensus-internal-eventlog-find-type-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/eventlog/item.go:66</code> | <code>func FindType(events []abci.Event) (string, bool)</code> |
| <code>interface.15.go-consensus-internal-eventlog-info-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/eventlog/eventlog.go:139</code> | <code>type Info struct</code> |
| <code>interface.15.go-consensus-internal-eventlog-item-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/eventlog/item.go:37</code> | <code>type Item struct</code> |
| <code>interface.15.go-consensus-internal-eventlog-log-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/eventlog/eventlog.go:25</code> | <code>type Log struct</code> |
| <code>interface.15.go-consensus-internal-eventlog-log-settings-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/eventlog/eventlog.go:121</code> | <code>type LogSettings struct</code> |
| <code>interface.15.go-consensus-internal-eventlog-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/eventlog/metrics.go:10</code> | <code>type Metrics struct</code> |
| <code>interface.15.go-consensus-internal-eventlog-new-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/eventlog/eventlog.go:43</code> | <code>func New(opts LogSettings) (*Log, error)</code> |
| <code>interface.15.go-consensus-internal-eventlog-prometheus-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/eventlog/metrics.gen.go:11</code> | <code>func PrometheusMetrics(namespace string, labelsAndValues ...string) *Metrics</code> |
| <code>interface.15.go-consensus-internal-evidence-evidence-channel-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/evidence/reactor.go:22</code> | <code>EvidenceChannel = p2p.ChannelID(0x38)</code> |
| <code>interface.15.go-consensus-internal-evidence-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/evidence/metrics.go:17</code> | <code>type Metrics struct</code> |
| <code>interface.15.go-consensus-internal-evidence-new-pool-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/evidence/pool.go:76</code> | <code>func NewPool(evidenceDB dbm.DB, stateStore sm.Store, blockStore BlockStore, metrics *Metrics, eventBus *eventbus.EventBus) *Pool</code> |
| <code>interface.15.go-consensus-internal-evidence-new-reactor-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/evidence/reactor.go:62</code> | <code>func NewReactor( router *p2p.Router, evpool *Pool, ) (*Reactor, error)</code> |
| <code>interface.15.go-consensus-internal-evidence-nop-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/evidence/metrics.gen.go:26</code> | <code>func NopMetrics() *Metrics</code> |
| <code>interface.15.go-consensus-internal-evidence-pool-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/evidence/pool.go:45</code> | <code>type Pool struct</code> |
| <code>interface.15.go-consensus-internal-evidence-prometheus-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/evidence/metrics.gen.go:11</code> | <code>func PrometheusMetrics(namespace string, labelsAndValues ...string) *Metrics</code> |
| <code>interface.15.go-consensus-internal-evidence-reactor-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/evidence/reactor.go:47</code> | <code>type Reactor struct</code> |
| <code>interface.15.go-consensus-internal-evidence-verify-duplicate-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/evidence/verify.go:207</code> | <code>func VerifyDuplicateVote(e *types.DuplicateVoteEvidence, chainID string, valSet *types.ValidatorSet) error</code> |
| <code>interface.15.go-consensus-internal-evidence-verify-light-client-attack-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/evidence/verify.go:164</code> | <code>func VerifyLightClientAttack(e *types.LightClientAttackEvidence, commonHeader, trustedHeader *types.SignedHeader, commonVals *types.ValidatorSet, now time.Time, trustPeriod time.Duration) error</code> |
| <code>interface.15.go-consensus-internal-hashable-gen-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/hashable/testonly.go:7</code> | <code>func GenHash[T Hashable](rng utils.Rng) Hash[T]</code> |
| <code>interface.15.go-consensus-internal-hashable-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/hashable/hashable.go:20</code> | <code>type Hash[T Hashable] [sha256.Size]byte</code> |
| <code>interface.15.go-consensus-internal-hashable-parse-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/hashable/hashable.go:23</code> | <code>func ParseHash[T Hashable](raw []byte) (Hash[T], error)</code> |
| <code>interface.15.go-consensus-internal-hashable-to-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/hashable/hashable.go:31</code> | <code>func ToHash[T Hashable](a T) Hash[T]</code> |
| <code>interface.15.go-consensus-internal-inspect-new-from-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/inspect/inspect.go:61</code> | <code>func NewFromConfig(cfg *config.Config) (*Inspector, error)</code> |
| <code>interface.15.go-consensus-internal-inspect-new-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/inspect/inspect.go:46</code> | <code>func New(cfg *config.RPCConfig, bs state.BlockStore, ss state.Store, es []indexer.EventSink) *Inspector</code> |
| <code>interface.15.go-consensus-internal-inspect-rpc-handler-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/inspect/rpc/rpc.go:56</code> | <code>func Handler(rpcConfig *config.RPCConfig, routes core.RoutesMap) http.Handler</code> |
| <code>interface.15.go-consensus-internal-inspect-rpc-routes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/inspect/rpc/rpc.go:32</code> | <code>func Routes(cfg config.RPCConfig, s state.Store, bs state.BlockStore, es []indexer.EventSink) core.RoutesMap</code> |
| <code>interface.15.go-consensus-internal-inspect-rpc-server-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/inspect/rpc/rpc.go:21</code> | <code>type Server struct</code> |
| <code>interface.15.go-consensus-internal-jsontypes-marshal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/jsontypes/jsontypes.go:60</code> | <code>func Marshal(v Tagged) ([]byte, error)</code> |
| <code>interface.15.go-consensus-internal-jsontypes-must-register-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/jsontypes/jsontypes.go:47</code> | <code>func MustRegister(v Tagged)</code> |
| <code>interface.15.go-consensus-internal-jsontypes-tagged-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/jsontypes/jsontypes.go:24</code> | <code>type Tagged interface</code> |
| <code>interface.15.go-consensus-internal-jsontypes-unmarshal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/jsontypes/jsontypes.go:78</code> | <code>func Unmarshal(data []byte, v any) error</code> |
| <code>interface.15.go-consensus-internal-libs-async-parallel-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/async/async.go:127</code> | <code>func Parallel(tasks ...Task) (trs *TaskResultSet, ok bool)</code> |
| <code>interface.15.go-consensus-internal-libs-clist-c-element-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/clist/clist.go:37</code> | <code>type CElement[T any] struct</code> |
| <code>interface.15.go-consensus-internal-libs-clist-c-list-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/clist/clist.go:140</code> | <code>type CList[T any] struct</code> |
| <code>interface.15.go-consensus-internal-libs-clist-err-removed-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/clist/clist.go:47</code> | <code>var ErrRemoved = errors.New("element was removed")</code> |
| <code>interface.15.go-consensus-internal-libs-clist-new-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/clist/clist.go:148</code> | <code>func New[T any]() *CList[T]</code> |
| <code>interface.15.go-consensus-internal-libs-flowrate-monitor-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/flowrate/flowrate.go:16</code> | <code>type Monitor struct</code> |
| <code>interface.15.go-consensus-internal-libs-flowrate-new-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/flowrate/flowrate.go:49</code> | <code>func New(startAt time.Time, sampleRate, windowSize time.Duration) *Monitor</code> |
| <code>interface.15.go-consensus-internal-libs-progressbar-bar-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/progressbar/progressbar.go:8</code> | <code>type Bar struct</code> |
| <code>interface.15.go-consensus-internal-libs-protoio-marshal-delimited-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/protoio/writer.go:110</code> | <code>func MarshalDelimited(msg proto.Message) ([]byte, error)</code> |
| <code>interface.15.go-consensus-internal-libs-protoio-new-delimited-reader-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/protoio/reader.go:47</code> | <code>func NewDelimitedReader(r io.Reader, maxSize int) ReadCloser</code> |
| <code>interface.15.go-consensus-internal-libs-protoio-new-delimited-writer-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/protoio/writer.go:45</code> | <code>func NewDelimitedWriter(w io.Writer) WriteCloser</code> |
| <code>interface.15.go-consensus-internal-libs-protoio-read-closer-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/protoio/io.go:52</code> | <code>type ReadCloser interface</code> |
| <code>interface.15.go-consensus-internal-libs-protoio-unmarshal-delimited-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/protoio/reader.go:103</code> | <code>func UnmarshalDelimited(data []byte, msg proto.Message) error</code> |
| <code>interface.15.go-consensus-internal-libs-protoio-write-closer-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/protoio/io.go:43</code> | <code>type WriteCloser interface</code> |
| <code>interface.15.go-consensus-internal-libs-queue-err-queue-closed-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/queue/queue.go:22</code> | <code>ErrQueueClosed = errors.New("queue is closed")</code> |
| <code>interface.15.go-consensus-internal-libs-queue-new-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/queue/queue.go:64</code> | <code>func New(opts Options) (*Queue, error)</code> |
| <code>interface.15.go-consensus-internal-libs-queue-options-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/queue/queue.go:209</code> | <code>type Options struct</code> |
| <code>interface.15.go-consensus-internal-libs-queue-queue-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/queue/queue.go:45</code> | <code>type Queue struct</code> |
| <code>interface.15.go-consensus-internal-libs-reservoir-new-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/reservoir/reservoir.go:41</code> | <code>func New[T cmp.Ordered](size int, p float64, rng *rand.Rand) *Sampler[T]</code> |
| <code>interface.15.go-consensus-internal-libs-reservoir-sampler-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/reservoir/reservoir.go:26</code> | <code>type Sampler[T cmp.Ordered] struct</code> |
| <code>interface.15.go-consensus-internal-libs-sync-new-waker-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/sync/waker.go:11</code> | <code>func NewWaker() *Waker</code> |
| <code>interface.15.go-consensus-internal-libs-tempfile-write-file-atomic-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/tempfile/tempfile.go:74</code> | <code>func WriteFileAtomic(filename string, data []byte, perm os.FileMode) (err error)</code> |
| <code>interface.15.go-consensus-internal-libs-test-mutate-byte-slice-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/test/mutate.go:9</code> | <code>func MutateByteSlice(bytez []byte) []byte</code> |
| <code>interface.15.go-consensus-internal-libs-timer-new-throttle-timer-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/timer/throttle_timer.go:25</code> | <code>func NewThrottleTimer(name string, dur time.Duration) *ThrottleTimer</code> |
| <code>interface.15.go-consensus-internal-libs-timer-throttle-timer-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/timer/throttle_timer.go:14</code> | <code>type ThrottleTimer struct</code> |
| <code>interface.15.go-consensus-internal-libs-wal-default-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/wal/log.go:36</code> | <code>func DefaultConfig() *Config</code> |
| <code>interface.15.go-consensus-internal-libs-wal-log-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/wal/log.go:139</code> | <code>type Log struct</code> |
| <code>interface.15.go-consensus-internal-libs-wal-open-log-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/libs/wal/log.go:143</code> | <code>func OpenLog(headPath string, cfg *Config) (*Log, error)</code> |
| <code>interface.15.go-consensus-internal-mempool-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/mempool/mempool.go:36</code> | <code>type Config struct</code> |
| <code>interface.15.go-consensus-internal-mempool-default-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/mempool/mempool.go:124</code> | <code>func DefaultConfig() *Config</code> |
| <code>interface.15.go-consensus-internal-mempool-err-tx-in-cache-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/mempool/mempool.go:24</code> | <code>var ErrTxInCache = errors.New("tx already exists in cache")</code> |
| <code>interface.15.go-consensus-internal-mempool-err-tx-too-large-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/mempool/mempool.go:27</code> | <code>var ErrTxTooLarge = errors.New("tx too large")</code> |
| <code>interface.15.go-consensus-internal-mempool-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:50</code> | <code>type Metrics struct</code> |
| <code>interface.15.go-consensus-internal-mempool-new-tx-mempool-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/mempool/mempool.go:208</code> | <code>func NewTxMempool( cfg *Config, app *proxy.Proxy, metrics *Metrics, txConstraintsFetcher TxConstraintsFetcher, ) *TxMempool</code> |
| <code>interface.15.go-consensus-internal-mempool-nop-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.gen.go:154</code> | <code>func NopMetrics() *Metrics</code> |
| <code>interface.15.go-consensus-internal-mempool-nop-tx-constraints-fetcher-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/mempool/types.go:23</code> | <code>func NopTxConstraintsFetcher() (TxConstraints, error)</code> |
| <code>interface.15.go-consensus-internal-mempool-nop-tx-constraints-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/mempool/types.go:16</code> | <code>func NopTxConstraints() TxConstraints</code> |
| <code>interface.15.go-consensus-internal-mempool-prometheus-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.gen.go:11</code> | <code>func PrometheusMetrics(namespace string, labelsAndValues ...string) *Metrics</code> |
| <code>interface.15.go-consensus-internal-mempool-reactor-mempool-channel-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/mempool/reactor/reactor.go:27</code> | <code>const MempoolChannel p2p.ChannelID = 0x30</code> |
| <code>interface.15.go-consensus-internal-mempool-reactor-new-reactor-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/mempool/reactor/reactor.go:47</code> | <code>func NewReactor(cfg *config.MempoolConfig, txmp *mempool.TxMempool, router *p2p.Router) (*Reactor, error)</code> |
| <code>interface.15.go-consensus-internal-mempool-reap-limits-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/mempool/tx.go:622</code> | <code>type ReapLimits struct</code> |
| <code>interface.15.go-consensus-internal-mempool-test-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/mempool/testonly.go:9</code> | <code>func TestConfig() *Config</code> |
| <code>interface.15.go-consensus-internal-mempool-tx-constraints-fetcher-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/mempool/types.go:14</code> | <code>type TxConstraintsFetcher func() (TxConstraints, error)</code> |
| <code>interface.15.go-consensus-internal-mempool-tx-constraints-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/mempool/types.go:7</code> | <code>type TxConstraints struct</code> |
| <code>interface.15.go-consensus-internal-mempool-tx-mempool-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/mempool/mempool.go:171</code> | <code>type TxMempool struct</code> |
| <code>interface.15.go-consensus-internal-p2p-channel-descriptor-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/channel.go:15</code> | <code>type ChannelDescriptor[T proto.Message] = conn.ChannelDescriptorT[T]</code> |
| <code>interface.15.go-consensus-internal-p2p-channel-id-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/channel.go:13</code> | <code>type ChannelID = conn.ChannelID</code> |
| <code>interface.15.go-consensus-internal-p2p-channel-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/channel.go:54</code> | <code>type Channel[T proto.Message] struct</code> |
| <code>interface.15.go-consensus-internal-p2p-conn-challenge-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/conn/secret_connection.go:96</code> | <code>type Challenge [32]byte</code> |
| <code>interface.15.go-consensus-internal-p2p-conn-channel-descriptor-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/conn/oldmux.go:27</code> | <code>type ChannelDescriptor = ChannelDescriptorT[gogoproto.Message]</code> |
| <code>interface.15.go-consensus-internal-p2p-conn-channel-descriptor-t-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/conn/oldmux.go:29</code> | <code>type ChannelDescriptorT[T gogoproto.Message] struct</code> |
| <code>interface.15.go-consensus-internal-p2p-conn-channel-id-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/conn/oldmux.go:25</code> | <code>type ChannelID uint16</code> |
| <code>interface.15.go-consensus-internal-p2p-conn-conn-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/conn/connection.go:13</code> | <code>type Conn interface</code> |
| <code>interface.15.go-consensus-internal-p2p-conn-default-m-conn-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/conn/oldmux.go:131</code> | <code>func DefaultMConnConfig() MConnConfig</code> |
| <code>interface.15.go-consensus-internal-p2p-conn-m-conn-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/conn/oldmux.go:107</code> | <code>type MConnConfig struct</code> |
| <code>interface.15.go-consensus-internal-p2p-conn-m-connection-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/conn/oldmux.go:98</code> | <code>type MConnection struct</code> |
| <code>interface.15.go-consensus-internal-p2p-conn-make-secret-connection-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/conn/secret_connection.go:147</code> | <code>func MakeSecretConnection(ctx context.Context, conn Conn) (*SecretConnection, error)</code> |
| <code>interface.15.go-consensus-internal-p2p-conn-new-m-connection-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/conn/oldmux.go:174</code> | <code>func NewMConnection( conn Conn, chDescs []*ChannelDescriptor, config MConnConfig, ) *MConnection</code> |
| <code>interface.15.go-consensus-internal-p2p-conn-new-test-conn-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/conn/testonly.go:107</code> | <code>func NewTestConn() (*TestConn, *TestConn)</code> |
| <code>interface.15.go-consensus-internal-p2p-conn-read-sized-msg-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/conn/protoio.go:29</code> | <code>func ReadSizedMsg(ctx context.Context, conn Conn, maxSize uint64) ([]byte, error)</code> |
| <code>interface.15.go-consensus-internal-p2p-conn-secret-connection-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/conn/secret_connection.go:107</code> | <code>type SecretConnection struct</code> |
| <code>interface.15.go-consensus-internal-p2p-conn-write-sized-msg-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/conn/protoio.go:15</code> | <code>func WriteSizedMsg(ctx context.Context, conn Conn, msg []byte) error</code> |
| <code>interface.15.go-consensus-internal-p2p-endpoint-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/transport.go:142</code> | <code>type Endpoint struct{ netip.AddrPort }</code> |
| <code>interface.15.go-consensus-internal-p2p-giga-api-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.go:13</code> | <code>type API struct{}</code> |
| <code>interface.15.go-consensus-internal-p2p-giga-new-pool-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/pool.go:35</code> | <code>func NewPool[K comparable, V any]() *Pool[K, V]</code> |
| <code>interface.15.go-consensus-internal-p2p-giga-new-service-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/service.go:16</code> | <code>func NewService(state *consensus.State) *Service</code> |
| <code>interface.15.go-consensus-internal-p2p-giga-node-addr-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/giga_router.go:31</code> | <code>type GigaNodeAddr struct</code> |
| <code>interface.15.go-consensus-internal-p2p-giga-pool-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/pool.go:31</code> | <code>type Pool[K comparable, V any] struct</code> |
| <code>interface.15.go-consensus-internal-p2p-giga-router-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/giga_router.go:41</code> | <code>type GigaRouterConfig struct</code> |
| <code>interface.15.go-consensus-internal-p2p-giga-router-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/giga_router.go:64</code> | <code>type GigaRouter struct</code> |
| <code>interface.15.go-consensus-internal-p2p-giga-service-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/service.go:11</code> | <code>type Service struct</code> |
| <code>interface.15.go-consensus-internal-p2p-make-test-network-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/testonly.go:72</code> | <code>func MakeTestNetwork(t *testing.T, opts TestNetworkOptions) *TestNetwork</code> |
| <code>interface.15.go-consensus-internal-p2p-max-pex-addrs-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/router.go:23</code> | <code>const MaxPexAddrs = 100</code> |
| <code>interface.15.go-consensus-internal-p2p-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/metrics.go:28</code> | <code>type Metrics struct</code> |
| <code>interface.15.go-consensus-internal-p2p-mux-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/mux/mux.go:52</code> | <code>type Config struct</code> |
| <code>interface.15.go-consensus-internal-p2p-mux-mux-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/mux/mux.go:419</code> | <code>type Mux struct</code> |
| <code>interface.15.go-consensus-internal-p2p-mux-new-mux-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/mux/mux.go:427</code> | <code>func NewMux(cfg *Config) *Mux</code> |
| <code>interface.15.go-consensus-internal-p2p-mux-stream-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/mux/stream.go:14</code> | <code>type Stream struct</code> |
| <code>interface.15.go-consensus-internal-p2p-mux-stream-kind-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/mux/mux.go:59</code> | <code>type StreamKindConfig struct</code> |
| <code>interface.15.go-consensus-internal-p2p-mux-stream-kind-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/mux/stream_state.go:10</code> | <code>type StreamKind uint64</code> |
| <code>interface.15.go-consensus-internal-p2p-new-router-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/router.go:63</code> | <code>func NewRouter( metrics *Metrics, privKey NodeSecretKey, nodeInfoProducer func() *types.NodeInfo, db dbm.DB, options *RouterOptions, ) (*Router, error)</code> |
| <code>interface.15.go-consensus-internal-p2p-node-address-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/address.go:32</code> | <code>type NodeAddress struct</code> |
| <code>interface.15.go-consensus-internal-p2p-node-in-slice-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/testonly.go:27</code> | <code>func NodeInSlice(id types.NodeID, ids []types.NodeID) bool</code> |
| <code>interface.15.go-consensus-internal-p2p-node-public-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/conv.go:17</code> | <code>type NodePublicKey ed25519.PublicKey</code> |
| <code>interface.15.go-consensus-internal-p2p-node-secret-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/conv.go:16</code> | <code>type NodeSecretKey ed25519.SecretKey</code> |
| <code>interface.15.go-consensus-internal-p2p-nop-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/metrics.gen.go:80</code> | <code>func NopMetrics() *Metrics</code> |
| <code>interface.15.go-consensus-internal-p2p-open-channel-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/router.go:152</code> | <code>func OpenChannel[T gogoproto.Message](r *Router, chDesc ChannelDescriptor[T]) (*Channel[T], error)</code> |
| <code>interface.15.go-consensus-internal-p2p-parse-node-address-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/address.go:40</code> | <code>func ParseNodeAddress(urlString string) (NodeAddress, error)</code> |
| <code>interface.15.go-consensus-internal-p2p-peer-status-down-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/routeroptions.go:31</code> | <code>PeerStatusDown PeerStatus = "down" // disconnected</code> |
| <code>interface.15.go-consensus-internal-p2p-peer-status-up-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/routeroptions.go:30</code> | <code>PeerStatusUp PeerStatus = "up" // connected and ready</code> |
| <code>interface.15.go-consensus-internal-p2p-peer-update-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/peermanager_updates.go:18</code> | <code>type PeerUpdate struct</code> |
| <code>interface.15.go-consensus-internal-p2p-pex-default-send-interval-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/pex/reactor.go:42</code> | <code>DefaultSendInterval = 10 * time.Second</code> |
| <code>interface.15.go-consensus-internal-p2p-pex-new-reactor-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/pex/reactor.go:76</code> | <code>func NewReactor( router *p2p.Router, sendInterval time.Duration, ) (*Reactor, error)</code> |
| <code>interface.15.go-consensus-internal-p2p-pex-pex-channel-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/pex/reactor.go:29</code> | <code>PexChannel = 0x00</code> |
| <code>interface.15.go-consensus-internal-p2p-prometheus-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/metrics.gen.go:11</code> | <code>func PrometheusMetrics(namespace string, labelsAndValues ...string) *Metrics</code> |
| <code>interface.15.go-consensus-internal-p2p-recv-msg-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/channel.go:24</code> | <code>type RecvMsg[T proto.Message] struct</code> |
| <code>interface.15.go-consensus-internal-p2p-resolve-endpoint-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/transport.go:145</code> | <code>func ResolveEndpoint(addr string) (Endpoint, error)</code> |
| <code>interface.15.go-consensus-internal-p2p-router-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/router.go:32</code> | <code>type Router struct</code> |
| <code>interface.15.go-consensus-internal-p2p-router-options-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/routeroptions.go:35</code> | <code>type RouterOptions struct</code> |
| <code>interface.15.go-consensus-internal-p2p-rpc-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/rpc/rpc.go:87</code> | <code>type Client[API any] struct{ mux *mux.Mux }</code> |
| <code>interface.15.go-consensus-internal-p2p-rpc-in-bytes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/rpc/rpc.go:16</code> | <code>type InBytes uint64</code> |
| <code>interface.15.go-consensus-internal-p2p-rpc-limit-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/rpc/rpc.go:24</code> | <code>type Limit struct</code> |
| <code>interface.15.go-consensus-internal-p2p-rpc-msg-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/rpc/rpc.go:19</code> | <code>type Msg[M protoutils.Message] struct</code> |
| <code>interface.15.go-consensus-internal-p2p-rpc-new-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/rpc/rpc.go:89</code> | <code>func NewClient[API any]() Client[API]</code> |
| <code>interface.15.go-consensus-internal-p2p-rpc-new-server-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/rpc/rpc.go:81</code> | <code>func NewServer[API any]() Server[API]</code> |
| <code>interface.15.go-consensus-internal-p2p-rpc-register-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/rpc/rpc.go:65</code> | <code>func Register[API any, Req, Resp protoutils.Message](kind mux.StreamKind, limit Limit, req Msg[Req], resp Msg[Resp]) *RPC[API, Req, Resp]</code> |
| <code>interface.15.go-consensus-internal-p2p-rpc-server-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/rpc/rpc.go:79</code> | <code>type Server[API any] struct{ mux *mux.Mux }</code> |
| <code>interface.15.go-consensus-internal-p2p-rpc-stream-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/rpc/rpc.go:130</code> | <code>type Stream[SendT, RecvT protoutils.Message] struct{ inner *mux.Stream }</code> |
| <code>interface.15.go-consensus-internal-p2p-test-make-channel-no-cleanup-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/testonly.go:357</code> | <code>func TestMakeChannelNoCleanup[T proto.Message](t *testing.T, n *TestNode, chDesc ChannelDescriptor[T]) *Channel[T]</code> |
| <code>interface.15.go-consensus-internal-p2p-test-network-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/testonly.go:39</code> | <code>type TestNetwork struct</code> |
| <code>interface.15.go-consensus-internal-p2p-test-network-options-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/testonly.go:45</code> | <code>type TestNetworkOptions struct</code> |
| <code>interface.15.go-consensus-internal-p2p-test-node-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/testonly.go:221</code> | <code>type TestNode struct</code> |
| <code>interface.15.go-consensus-internal-p2p-test-node-options-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/p2p/testonly.go:50</code> | <code>type TestNodeOptions struct</code> |
| <code>interface.15.go-consensus-internal-protoutils-conv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/protoutils/conv.go:11</code> | <code>type Conv[T any, P Message] struct</code> |
| <code>interface.15.go-consensus-internal-protoutils-marshal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/protoutils/msg.go:25</code> | <code>func Marshal[T Message](t T) []byte</code> |
| <code>interface.15.go-consensus-internal-protoutils-message-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/protoutils/msg.go:10</code> | <code>type Message interface</code> |
| <code>interface.15.go-consensus-internal-protoutils-size-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/protoutils/msg.go:21</code> | <code>func Size[T Message](t T) int</code> |
| <code>interface.15.go-consensus-internal-protoutils-unmarshal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/protoutils/msg.go:30</code> | <code>func Unmarshal[T Message](bytes []byte) (T, error)</code> |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-must-field-num-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/protoutils/wireguard/wireguard.go:139</code> | <code>func MustFieldNum[T any](protoName string) Number</code> |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-number-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/protoutils/wireguard/wireguard.go:31</code> | <code>type Number = protowire.Number</code> |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-rule-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/protoutils/wireguard/wireguard.go:44</code> | <code>type Rule struct</code> |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-scan-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/protoutils/wireguard/wireguard.go:58</code> | <code>func Scan(bz []byte, schema *Schema) error</code> |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-scanner-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/protoutils/wireguard/wireguard.go:25</code> | <code>type Scanner interface</code> |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-schema-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/protoutils/wireguard/wireguard.go:37</code> | <code>type Schema struct</code> |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-wgtest-commit-with-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/protoutils/wireguard/wgtest/wgtest.go:32</code> | <code>func CommitWith(n int) *tmproto.Commit</code> |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-wgtest-evidence-list-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/protoutils/wireguard/wgtest/wgtest.go:51</code> | <code>func EvidenceList(commits ...*tmproto.Commit) []tmproto.Evidence</code> |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-wgtest-evidence-with-commit-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/protoutils/wireguard/wgtest/wgtest.go:39</code> | <code>func EvidenceWithCommit(c *tmproto.Commit) tmproto.Evidence</code> |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-wgtest-marshal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/protoutils/wireguard/wgtest/wgtest.go:24</code> | <code>func Marshal(t *testing.T, m gogoproto.Message) []byte</code> |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-wgtest-max-commit-signatures-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/protoutils/wireguard/wgtest/wgtest.go:21</code> | <code>const MaxCommitSignatures = types.MaxVotesCount</code> |
| <code>interface.15.go-consensus-internal-proxy-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/proxy/metrics.go:13</code> | <code>type Metrics struct</code> |
| <code>interface.15.go-consensus-internal-proxy-new-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/proxy/proxy.go:23</code> | <code>func New(app types.Application, metrics *Metrics) *Proxy</code> |
| <code>interface.15.go-consensus-internal-proxy-nop-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/proxy/metrics.gen.go:28</code> | <code>func NopMetrics() *Metrics</code> |
| <code>interface.15.go-consensus-internal-proxy-prometheus-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/proxy/metrics.gen.go:11</code> | <code>func PrometheusMetrics(namespace string, labelsAndValues ...string) *Metrics</code> |
| <code>interface.15.go-consensus-internal-proxy-proxy-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/proxy/proxy.go:17</code> | <code>type Proxy struct</code> |
| <code>interface.15.go-consensus-internal-pubsub-buffer-capacity-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/pubsub.go:146</code> | <code>func BufferCapacity(cap int) Option</code> |
| <code>interface.15.go-consensus-internal-pubsub-err-already-subscribed-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/pubsub.go:58</code> | <code>ErrAlreadySubscribed = errors.New("already subscribed")</code> |
| <code>interface.15.go-consensus-internal-pubsub-err-subscription-not-found-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/pubsub.go:54</code> | <code>ErrSubscriptionNotFound = errors.New("subscription not found")</code> |
| <code>interface.15.go-consensus-internal-pubsub-err-terminated-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/subscription.go:20</code> | <code>ErrTerminated = errors.New("subscription terminated by publisher")</code> |
| <code>interface.15.go-consensus-internal-pubsub-err-unsubscribed-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/subscription.go:16</code> | <code>ErrUnsubscribed = errors.New("subscription removed by client")</code> |
| <code>interface.15.go-consensus-internal-pubsub-message-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/subscription.go:76</code> | <code>type Message struct</code> |
| <code>interface.15.go-consensus-internal-pubsub-new-server-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/pubsub.go:125</code> | <code>func NewServer(options ...Option) *Server</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-all-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/query.go:26</code> | <code>var All *Query</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-must-compile-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/query.go:48</code> | <code>func MustCompile(query string) *Query</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-new-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/query.go:35</code> | <code>func New(query string) (*Query, error)</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-query-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/query.go:29</code> | <code>type Query struct</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-condition-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/parser.go:33</code> | <code>type Condition struct</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-date-format-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/scanner.go:67</code> | <code>DateFormat = "2006-01-02"</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-new-scanner-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/scanner.go:83</code> | <code>func NewScanner(r io.Reader) *Scanner</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-parse-date-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/parser.go:208</code> | <code>func ParseDate(s string) (time.Time, error)</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-parse-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/parser.go:14</code> | <code>func Parse(s string) (Query, error)</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-parse-time-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/parser.go:213</code> | <code>func ParseTime(s string) (time.Time, error)</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-query-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/parser.go:20</code> | <code>type Query []Condition</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-and-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/scanner.go:23</code> | <code>TAnd // operator: AND</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-contains-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/scanner.go:24</code> | <code>TContains // operator: CONTAINS</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-date-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/scanner.go:22</code> | <code>TDate // datestamp: DATE yyyy-mm-dd</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-eq-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/scanner.go:26</code> | <code>TEq // operator: =</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-exists-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/scanner.go:25</code> | <code>TExists // operator: EXISTS</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-geq-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/scanner.go:30</code> | <code>TGeq // operator: &gt;=</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-gt-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/scanner.go:29</code> | <code>TGt // operator: &gt;</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-leq-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/scanner.go:28</code> | <code>TLeq // operator: &lt;=</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-lt-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/scanner.go:27</code> | <code>TLt // operator: &lt;</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-matches-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/scanner.go:31</code> | <code>TMatches // operator: MATCHES</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-number-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/scanner.go:20</code> | <code>TNumber // number: 0, 15.5, 100</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-string-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/scanner.go:19</code> | <code>TString // string value: 'foo bar'</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-tag-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/scanner.go:18</code> | <code>TTag // field tag: x.y</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-time-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/scanner.go:21</code> | <code>TTime // timestamp: TIME yyyy-mm-ddThh:mm:ss([-+]hh:mm&#124;Z)</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-time-format-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/scanner.go:64</code> | <code>TimeFormat = time.RFC3339</code> |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-token-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/query/syntax/scanner.go:14</code> | <code>type Token byte</code> |
| <code>interface.15.go-consensus-internal-pubsub-server-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/pubsub.go:93</code> | <code>type Server struct</code> |
| <code>interface.15.go-consensus-internal-pubsub-subscribe-args-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/pubsub.go:66</code> | <code>type SubscribeArgs struct</code> |
| <code>interface.15.go-consensus-internal-pubsub-subscription-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/subscription.go:24</code> | <code>type Subscription struct</code> |
| <code>interface.15.go-consensus-internal-pubsub-unsubscribe-args-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/pubsub/pubsub.go:76</code> | <code>type UnsubscribeArgs struct</code> |
| <code>interface.15.go-consensus-internal-rpc-core-consensus-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/rpc/core/env.go:55</code> | <code>type ConsensusState interface</code> |
| <code>interface.15.go-consensus-internal-rpc-core-environment-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/rpc/core/env.go:66</code> | <code>type Environment struct</code> |
| <code>interface.15.go-consensus-internal-rpc-core-new-routes-map-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:24</code> | <code>func NewRoutesMap(svc RPCService, opts *RouteOptions) RoutesMap</code> |
| <code>interface.15.go-consensus-internal-rpc-core-routes-map-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:12</code> | <code>type RoutesMap map[string]*rpc.RPCFunc</code> |
| <code>interface.15.go-consensus-internal-state-block-executor-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/execution.go:74</code> | <code>type BlockExecutor struct</code> |
| <code>interface.15.go-consensus-internal-state-block-store-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/services.go:20</code> | <code>type BlockStore interface</code> |
| <code>interface.15.go-consensus-internal-state-empty-evidence-pool-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/services.go:58</code> | <code>type EmptyEvidencePool struct{}</code> |
| <code>interface.15.go-consensus-internal-state-err-app-block-height-too-high-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/errors.go:19</code> | <code>ErrAppBlockHeightTooHigh struct {</code> |
| <code>interface.15.go-consensus-internal-state-err-app-block-height-too-low-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/errors.go:24</code> | <code>ErrAppBlockHeightTooLow struct {</code> |
| <code>interface.15.go-consensus-internal-state-err-no-finalize-block-responses-for-height-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/errors.go:49</code> | <code>ErrNoFinalizeBlockResponsesForHeight struct {</code> |
| <code>interface.15.go-consensus-internal-state-err-no-val-set-for-height-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/errors.go:40</code> | <code>ErrNoValSetForHeight struct {</code> |
| <code>interface.15.go-consensus-internal-state-err-open-time-missing-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/execution.go:37</code> | <code>ErrOpenTimeMissing = errors.New("sealed block has no open time")</code> |
| <code>interface.15.go-consensus-internal-state-err-sealed-block-too-large-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/execution.go:40</code> | <code>ErrSealedBlockTooLarge = errors.New("sealed block exceeds the block data bytes")</code> |
| <code>interface.15.go-consensus-internal-state-err-sequence-batch-missing-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/execution.go:35</code> | <code>ErrSequenceBatchMissing = errors.New("sealed block has no sequence batch")</code> |
| <code>interface.15.go-consensus-internal-state-evidence-pool-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/services.go:49</code> | <code>type EvidencePool interface</code> |
| <code>interface.15.go-consensus-internal-state-exec-commit-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/execution.go:840</code> | <code>func ExecCommitBlock( ctx context.Context, be *BlockExecutor, appConn *proxy.Proxy, block *types.Block, store Store, initialHeight int64, s State, ) ([]byte, error)</code> |
| <code>interface.15.go-consensus-internal-state-fire-events-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/execution.go:774</code> | <code>func FireEvents( eventBus types.BlockEventPublisher, block *types.Block, blockID types.BlockID, finalizeBlockResponse *abci.ResponseFinalizeBlock, validatorUpdates []*types.Validator, )</code> |
| <code>interface.15.go-consensus-internal-state-from-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/state.go:209</code> | <code>func FromProto(pb *tmstate.State) (*State, error)</code> |
| <code>interface.15.go-consensus-internal-state-indexer-batch-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/indexer.go:43</code> | <code>type Batch struct</code> |
| <code>interface.15.go-consensus-internal-state-indexer-block-indexer-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/indexer.go:28</code> | <code>type BlockIndexer interface</code> |
| <code>interface.15.go-consensus-internal-state-indexer-block-kv-blocker-indexer-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/block/kv/kv.go:27</code> | <code>type BlockerIndexer struct</code> |
| <code>interface.15.go-consensus-internal-state-indexer-block-kv-new-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/block/kv/kv.go:31</code> | <code>func New(store dbm.DB) *BlockerIndexer</code> |
| <code>interface.15.go-consensus-internal-state-indexer-block-null-blocker-indexer-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/block/null/null.go:15</code> | <code>type BlockerIndexer struct{}</code> |
| <code>interface.15.go-consensus-internal-state-indexer-error-empty-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/indexer.go:66</code> | <code>var ErrorEmptyHash = errors.New("transaction hash cannot be empty")</code> |
| <code>interface.15.go-consensus-internal-state-indexer-event-sink-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/eventsink.go:26</code> | <code>type EventSink interface</code> |
| <code>interface.15.go-consensus-internal-state-indexer-event-sink-type-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/eventsink.go:11</code> | <code>type EventSinkType string</code> |
| <code>interface.15.go-consensus-internal-state-indexer-indexing-enabled-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/indexer_service.go:162</code> | <code>func IndexingEnabled(sinks []EventSink) bool</code> |
| <code>interface.15.go-consensus-internal-state-indexer-kv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/eventsink.go:15</code> | <code>KV EventSinkType = "kv"</code> |
| <code>interface.15.go-consensus-internal-state-indexer-kv-sink-enabled-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/indexer_service.go:151</code> | <code>func KVSinkEnabled(sinks []EventSink) bool</code> |
| <code>interface.15.go-consensus-internal-state-indexer-look-for-ranges-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/query_range.go:78</code> | <code>func LookForRanges(conditions []syntax.Condition) (ranges QueryRanges, indexes []int)</code> |
| <code>interface.15.go-consensus-internal-state-indexer-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/metrics.go:13</code> | <code>type Metrics struct</code> |
| <code>interface.15.go-consensus-internal-state-indexer-new-batch-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/indexer.go:49</code> | <code>func NewBatch(n int64) *Batch</code> |
| <code>interface.15.go-consensus-internal-state-indexer-new-service-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/indexer_service.go:33</code> | <code>func NewService(args ServiceArgs) *Service</code> |
| <code>interface.15.go-consensus-internal-state-indexer-nop-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/metrics.gen.go:44</code> | <code>func NopMetrics() *Metrics</code> |
| <code>interface.15.go-consensus-internal-state-indexer-null-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/eventsink.go:14</code> | <code>NULL EventSinkType = "null"</code> |
| <code>interface.15.go-consensus-internal-state-indexer-prometheus-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/metrics.gen.go:11</code> | <code>func PrometheusMetrics(namespace string, labelsAndValues ...string) *Metrics</code> |
| <code>interface.15.go-consensus-internal-state-indexer-psql-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/eventsink.go:16</code> | <code>PSQL EventSinkType = "psql"</code> |
| <code>interface.15.go-consensus-internal-state-indexer-query-range-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/query_range.go:17</code> | <code>type QueryRange struct</code> |
| <code>interface.15.go-consensus-internal-state-indexer-service-args-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/indexer_service.go:144</code> | <code>type ServiceArgs struct</code> |
| <code>interface.15.go-consensus-internal-state-indexer-service-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/indexer_service.go:18</code> | <code>type Service struct</code> |
| <code>interface.15.go-consensus-internal-state-indexer-sink-event-sinks-from-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/sink/sink.go:16</code> | <code>func EventSinksFromConfig(cfg *config.Config, dbProvider config.DBProvider, chainID string) ([]indexer.EventSink, error)</code> |
| <code>interface.15.go-consensus-internal-state-indexer-sink-kv-new-event-sink-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/sink/kv/kv.go:26</code> | <code>func NewEventSink(store dbm.DB) indexer.EventSink</code> |
| <code>interface.15.go-consensus-internal-state-indexer-sink-null-new-event-sink-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/sink/null/null.go:17</code> | <code>func NewEventSink() indexer.EventSink</code> |
| <code>interface.15.go-consensus-internal-state-indexer-sink-psql-new-event-sink-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/sink/psql/psql.go:39</code> | <code>func NewEventSink(connStr, chainID string) (*EventSink, error)</code> |
| <code>interface.15.go-consensus-internal-state-indexer-tx-indexer-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/indexer.go:13</code> | <code>type TxIndexer interface</code> |
| <code>interface.15.go-consensus-internal-state-indexer-tx-kv-key-from-height-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/tx/kv/kv.go:589</code> | <code>func KeyFromHeight(result *abci.TxResultV2) []byte</code> |
| <code>interface.15.go-consensus-internal-state-indexer-tx-kv-new-tx-index-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/tx/kv/kv.go:33</code> | <code>func NewTxIndex(store dbm.DB) *TxIndex</code> |
| <code>interface.15.go-consensus-internal-state-indexer-tx-kv-tx-index-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/tx/kv/kv.go:28</code> | <code>type TxIndex struct</code> |
| <code>interface.15.go-consensus-internal-state-indexer-tx-null-tx-index-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/indexer/tx/null/null.go:15</code> | <code>type TxIndex struct{}</code> |
| <code>interface.15.go-consensus-internal-state-make-genesis-doc-from-file-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/state.go:317</code> | <code>func MakeGenesisDocFromFile(genDocFile string) (*types.GenesisDoc, error)</code> |
| <code>interface.15.go-consensus-internal-state-make-genesis-state-from-file-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/state.go:308</code> | <code>func MakeGenesisStateFromFile(genDocFile string) (State, error)</code> |
| <code>interface.15.go-consensus-internal-state-make-genesis-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/state.go:330</code> | <code>func MakeGenesisState(genDoc *types.GenesisDoc) (State, error)</code> |
| <code>interface.15.go-consensus-internal-state-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/metrics.go:16</code> | <code>type Metrics struct</code> |
| <code>interface.15.go-consensus-internal-state-new-block-executor-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/execution.go:187</code> | <code>func NewBlockExecutor( stateStore Store, app *proxy.Proxy, pool *mempool.TxMempool, evpool EvidencePool, blockStore BlockStore, eventBus *eventbus.EventBus, metrics *Metrics, consensusPolicy types.ConsensusPolicy, ) *BlockExecutor</code> |
| <code>interface.15.go-consensus-internal-state-new-store-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/store.go:124</code> | <code>func NewStore(db dbm.DB) Store</code> |
| <code>interface.15.go-consensus-internal-state-nop-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/metrics.gen.go:104</code> | <code>func NopMetrics() *Metrics</code> |
| <code>interface.15.go-consensus-internal-state-prometheus-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/metrics.gen.go:11</code> | <code>func PrometheusMetrics(namespace string, labelsAndValues ...string) *Metrics</code> |
| <code>interface.15.go-consensus-internal-state-rollback-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/rollback.go:30</code> | <code>func Rollback(bs BlockStore, ss Store, removeBlock bool, privValidatorConfig *config.PrivValidatorConfig) (int64, []byte, error)</code> |
| <code>interface.15.go-consensus-internal-state-sealed-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/execution.go:47</code> | <code>type SealedBlock = types.SealedBlock</code> |
| <code>interface.15.go-consensus-internal-state-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/state.go:69</code> | <code>type State struct</code> |
| <code>interface.15.go-consensus-internal-state-store-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/store.go:91</code> | <code>type Store interface</code> |
| <code>interface.15.go-consensus-internal-state-test-factory-make-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/test/factory/block.go:45</code> | <code>func MakeBlock(state sm.State, height int64, c *types.Commit) *types.Block</code> |
| <code>interface.15.go-consensus-internal-state-test-factory-make-blocks-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/test/factory/block.go:15</code> | <code>func MakeBlocks(ctx context.Context, t *testing.T, n int, state *sm.State, privVal types.PrivValidator) []*types.Block</code> |
| <code>interface.15.go-consensus-internal-state-tx-constraints-fetcher-from-store-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/tx_filter.go:42</code> | <code>func TxConstraintsFetcherFromStore(store Store) mempool.TxConstraintsFetcher</code> |
| <code>interface.15.go-consensus-internal-state-tx-constraints-for-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/tx_filter.go:55</code> | <code>func TxConstraintsForState(state State) mempool.TxConstraints</code> |
| <code>interface.15.go-consensus-internal-state-version-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/state/state.go:23</code> | <code>type Version struct</code> |
| <code>interface.15.go-consensus-internal-statesync-chunk-channel-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/statesync/reactor.go:71</code> | <code>ChunkChannel = p2p.ChannelID(0x61)</code> |
| <code>interface.15.go-consensus-internal-statesync-light-block-channel-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/statesync/reactor.go:74</code> | <code>LightBlockChannel = p2p.ChannelID(0x62)</code> |
| <code>interface.15.go-consensus-internal-statesync-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/statesync/metrics.go:15</code> | <code>type Metrics struct</code> |
| <code>interface.15.go-consensus-internal-statesync-new-reactor-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/statesync/reactor.go:212</code> | <code>func NewReactor( chainID string, initialHeight int64, cfg config.StateSyncConfig, conn *proxy.Proxy, router *p2p.Router, stateStore sm.Store, blockStore *store.BlockStore, tempDir string, ssMetrics *Metrics, eventBus *eventbus.EventBus, postSyncHook func(context.Context, sm.State) error, needsStateSync bool,</code> |
| <code>interface.15.go-consensus-internal-statesync-nop-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/statesync/metrics.gen.go:62</code> | <code>func NopMetrics() *Metrics</code> |
| <code>interface.15.go-consensus-internal-statesync-params-channel-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/statesync/reactor.go:77</code> | <code>ParamsChannel = p2p.ChannelID(0x63)</code> |
| <code>interface.15.go-consensus-internal-statesync-prometheus-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/statesync/metrics.gen.go:11</code> | <code>func PrometheusMetrics(namespace string, labelsAndValues ...string) *Metrics</code> |
| <code>interface.15.go-consensus-internal-statesync-reactor-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/statesync/reactor.go:149</code> | <code>type Reactor struct</code> |
| <code>interface.15.go-consensus-internal-statesync-snapshot-channel-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/statesync/reactor.go:68</code> | <code>SnapshotChannel = p2p.ChannelID(0x60)</code> |
| <code>interface.15.go-consensus-internal-store-block-store-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/store/store.go:34</code> | <code>type BlockStore struct</code> |
| <code>interface.15.go-consensus-internal-store-new-block-store-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/store/store.go:40</code> | <code>func NewBlockStore(db dbm.DB) *BlockStore</code> |
| <code>interface.15.go-consensus-internal-test-factory-consensus-params-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/test/factory/params.go:11</code> | <code>func ConsensusParams() *types.ConsensusParams</code> |
| <code>interface.15.go-consensus-internal-test-factory-default-test-chain-id-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/test/factory/block.go:12</code> | <code>DefaultTestChainID = "test-chain"</code> |
| <code>interface.15.go-consensus-internal-test-factory-default-test-time-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/test/factory/block.go:16</code> | <code>DefaultTestTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)</code> |
| <code>interface.15.go-consensus-internal-test-factory-genesis-doc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/test/factory/genesis.go:10</code> | <code>func GenesisDoc( config *cfg.Config, time time.Time, validators []*types.Validator, consensusParams *types.ConsensusParams, ) *types.GenesisDoc</code> |
| <code>interface.15.go-consensus-internal-test-factory-make-block-id-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/test/factory/block.go:27</code> | <code>func MakeBlockID() types.BlockID</code> |
| <code>interface.15.go-consensus-internal-test-factory-make-block-id-with-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/test/factory/block.go:31</code> | <code>func MakeBlockIDWithHash(hash []byte) types.BlockID</code> |
| <code>interface.15.go-consensus-internal-test-factory-make-commit-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/test/factory/commit.go:14</code> | <code>func MakeCommit(ctx context.Context, blockID types.BlockID, height int64, round int32, voteSet *types.VoteSet, validators []types.PrivValidator, now time.Time) (*types.Commit, error)</code> |
| <code>interface.15.go-consensus-internal-test-factory-make-header-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/test/factory/block.go:43</code> | <code>func MakeHeader(h *types.Header) *types.Header</code> |
| <code>interface.15.go-consensus-internal-test-factory-make-n-txs-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/test/factory/tx.go:5</code> | <code>func MakeNTxs(height, n int64) []types.Tx</code> |
| <code>interface.15.go-consensus-internal-test-factory-make-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/test/factory/vote.go:14</code> | <code>func MakeVote( ctx context.Context, val types.PrivValidator, chainID string, valIndex int32, height int64, round int32, step int, blockID types.BlockID, time time.Time, ) (*types.Vote, error)</code> |
| <code>interface.15.go-consensus-internal-test-factory-node-id-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/test/factory/p2p.go:15</code> | <code>func NodeID(t *testing.T, str string) types.NodeID</code> |
| <code>interface.15.go-consensus-internal-test-factory-random-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/test/factory/block.go:23</code> | <code>func RandomHash() []byte</code> |
| <code>interface.15.go-consensus-internal-test-factory-validator-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/test/factory/validator.go:11</code> | <code>func Validator(ctx context.Context, votingPower int64) (*types.Validator, types.PrivValidator, error)</code> |
| <code>interface.15.go-consensus-internal-test-factory-validator-set-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/internal/test/factory/validator.go:22</code> | <code>func ValidatorSet(ctx context.Context, numValidators int, votingPower int64) (*types.ValidatorSet, []types.PrivValidator)</code> |
| <code>interface.15.go-consensus-libs-bits-bit-array-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/bits/bit_array.go:17</code> | <code>type BitArray struct</code> |
| <code>interface.15.go-consensus-libs-bits-new-bit-array-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/bits/bit_array.go:25</code> | <code>func NewBitArray(bits int) *BitArray</code> |
| <code>interface.15.go-consensus-libs-bytes-fingerprint-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/bytes/byteslice.go:6</code> | <code>func Fingerprint(slice []byte) []byte</code> |
| <code>interface.15.go-consensus-libs-bytes-hex-bytes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/bytes/bytes.go:12</code> | <code>type HexBytes []byte</code> |
| <code>interface.15.go-consensus-libs-cli-home-flag-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/cli/setup.go:15</code> | <code>HomeFlag = "home"</code> |
| <code>interface.15.go-consensus-libs-cli-new-completion-cmd-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/cli/helper.go:77</code> | <code>func NewCompletionCmd(rootCmd *cobra.Command, hidden bool) *cobra.Command</code> |
| <code>interface.15.go-consensus-libs-cli-output-flag-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/cli/setup.go:17</code> | <code>OutputFlag = "output" // used in the cli</code> |
| <code>interface.15.go-consensus-libs-cli-prepare-base-cmd-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/cli/setup.go:25</code> | <code>func PrepareBaseCmd(cmd *cobra.Command, envPrefix, defaultHome string) *cobra.Command</code> |
| <code>interface.15.go-consensus-libs-json-marshal-indent-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/json/encoder.go:32</code> | <code>func MarshalIndent(v any, prefix, indent string) ([]byte, error)</code> |
| <code>interface.15.go-consensus-libs-json-marshal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/json/encoder.go:22</code> | <code>func Marshal(v any) ([]byte, error)</code> |
| <code>interface.15.go-consensus-libs-json-unmarshal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/json/decoder.go:13</code> | <code>func Unmarshal(bz []byte, v interface</code> |
| <code>interface.15.go-consensus-libs-math-fraction-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/math/fraction.go:13</code> | <code>type Fraction struct</code> |
| <code>interface.15.go-consensus-libs-math-max-int-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/math/math.go:10</code> | <code>func MaxInt(a, b int) int</code> |
| <code>interface.15.go-consensus-libs-math-max-int64-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/math/math.go:3</code> | <code>func MaxInt64(a, b int64) int64</code> |
| <code>interface.15.go-consensus-libs-math-min-int-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/math/math.go:26</code> | <code>func MinInt(a, b int) int</code> |
| <code>interface.15.go-consensus-libs-math-min-int64-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/math/math.go:19</code> | <code>func MinInt64(a, b int64) int64</code> |
| <code>interface.15.go-consensus-libs-math-parse-fraction-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/math/fraction.go:27</code> | <code>func ParseFraction(f string) (Fraction, error)</code> |
| <code>interface.15.go-consensus-libs-math-safe-add-int32-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/math/safemath.go:13</code> | <code>func SafeAddInt32(a, b int32) (int32, error)</code> |
| <code>interface.15.go-consensus-libs-math-safe-convert-int32-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/math/safemath.go:33</code> | <code>func SafeConvertInt32(a int64) (int32, error)</code> |
| <code>interface.15.go-consensus-libs-math-safe-sub-int32-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/math/safemath.go:23</code> | <code>func SafeSubInt32(a, b int32) (int32, error)</code> |
| <code>interface.15.go-consensus-libs-net-connect-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/net/net.go:10</code> | <code>func Connect(protoAddr string) (net.Conn, error)</code> |
| <code>interface.15.go-consensus-libs-net-get-free-port-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/net/net.go:31</code> | <code>func GetFreePort() (int, error)</code> |
| <code>interface.15.go-consensus-libs-net-protocol-and-address-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/net/net.go:19</code> | <code>func ProtocolAndAddress(listenAddr string) (string, string)</code> |
| <code>interface.15.go-consensus-libs-os-copy-file-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/os/os.go:29</code> | <code>func CopyFile(src, dst string) error</code> |
| <code>interface.15.go-consensus-libs-os-ensure-dir-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/os/os.go:15</code> | <code>func EnsureDir(dir string, mode os.FileMode) error</code> |
| <code>interface.15.go-consensus-libs-os-exit-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/os/os.go:84</code> | <code>func Exit(s string)</code> |
| <code>interface.15.go-consensus-libs-os-file-exists-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/os/os.go:23</code> | <code>func FileExists(filePath string) bool</code> |
| <code>interface.15.go-consensus-libs-os-write-file-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/os/os.go:102</code> | <code>func WriteFile(filePath string, contents []byte, mode os.FileMode) error</code> |
| <code>interface.15.go-consensus-libs-rand-bytes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/rand/random.go:48</code> | <code>func Bytes(n int) []byte</code> |
| <code>interface.15.go-consensus-libs-rand-new-rand-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/rand/random.go:75</code> | <code>func NewRand() *Rand</code> |
| <code>interface.15.go-consensus-libs-rand-str-from-source-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/rand/random.go:21</code> | <code>func StrFromSource(r *mrand.Rand, length int) string</code> |
| <code>interface.15.go-consensus-libs-rand-str-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/rand/random.go:17</code> | <code>func Str(length int) string</code> |
| <code>interface.15.go-consensus-libs-rand-uint32-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/rand/random.go:106</code> | <code>func Uint32() uint32</code> |
| <code>interface.15.go-consensus-libs-service-base-service-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/service/service.go:91</code> | <code>type BaseService struct</code> |
| <code>interface.15.go-consensus-libs-service-new-base-service-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/service/service.go:99</code> | <code>func NewBaseService(name string, impl Implementation) *BaseService</code> |
| <code>interface.15.go-consensus-libs-service-service-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/service/service.go:21</code> | <code>type Service interface</code> |
| <code>interface.15.go-consensus-libs-strings-ascii-trim-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/strings/string.go:33</code> | <code>func ASCIITrim(s string) (string, error)</code> |
| <code>interface.15.go-consensus-libs-strings-split-and-trim-empty-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/strings/string.go:13</code> | <code>func SplitAndTrimEmpty(s, sep, cutset string) []string</code> |
| <code>interface.15.go-consensus-libs-strings-string-slice-equal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/strings/string.go:52</code> | <code>func StringSliceEqual(a, b []string) bool</code> |
| <code>interface.15.go-consensus-libs-sync-mutex-2-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/sync/sync.go:9</code> | <code>type Mutex struct</code> |
| <code>interface.15.go-consensus-libs-sync-mutex-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/sync/deadlock.go:11</code> | <code>type Mutex struct</code> |
| <code>interface.15.go-consensus-libs-sync-rw-mutex-2-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/sync/sync.go:14</code> | <code>type RWMutex struct</code> |
| <code>interface.15.go-consensus-libs-sync-rw-mutex-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/sync/deadlock.go:16</code> | <code>type RWMutex struct</code> |
| <code>interface.15.go-consensus-libs-time-canonical-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/time/time.go:15</code> | <code>func Canonical(t time.Time) time.Time</code> |
| <code>interface.15.go-consensus-libs-time-default-source-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/time/time.go:27</code> | <code>type DefaultSource struct{}</code> |
| <code>interface.15.go-consensus-libs-time-now-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/time/time.go:8</code> | <code>func Now() time.Time</code> |
| <code>interface.15.go-consensus-libs-time-source-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/time/time.go:22</code> | <code>type Source interface</code> |
| <code>interface.15.go-consensus-libs-utils-alloc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/proto.go:70</code> | <code>func Alloc[T any](v T) *T</code> |
| <code>interface.15.go-consensus-libs-utils-atomic-recv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/mutex.go:105</code> | <code>type AtomicRecv[T any] struct{ *atomicWatch[T] }</code> |
| <code>interface.15.go-consensus-libs-utils-atomic-send-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/mutex.go:86</code> | <code>type AtomicSend[T any] struct{ atomicWatch[T] }</code> |
| <code>interface.15.go-consensus-libs-utils-clamp-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/math.go:40</code> | <code>func Clamp[To, From constraints.Integer](v From) To</code> |
| <code>interface.15.go-consensus-libs-utils-duration-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/wait.go:120</code> | <code>type Duration time.Duration</code> |
| <code>interface.15.go-consensus-libs-utils-error-as-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/proto.go:11</code> | <code>func ErrorAs[T error](err error) Option[T]</code> |
| <code>interface.15.go-consensus-libs-utils-gen-bool-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/testonly.go:140</code> | <code>func GenBool(rng Rng) bool</code> |
| <code>interface.15.go-consensus-libs-utils-gen-bytes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/testonly.go:164</code> | <code>func GenBytes(rng Rng, n int) []byte</code> |
| <code>interface.15.go-consensus-libs-utils-gen-map-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/testonly.go:190</code> | <code>func GenMap[K comparable, V any](rng Rng, genK GenF[K], genV GenF[V]) map[K]V</code> |
| <code>interface.15.go-consensus-libs-utils-gen-slice-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/testonly.go:176</code> | <code>func GenSlice[T any](rng Rng, gen GenF[T]) []T</code> |
| <code>interface.15.go-consensus-libs-utils-gen-slice-n-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/testonly.go:181</code> | <code>func GenSliceN[T any](rng Rng, n int, gen GenF[T]) []T</code> |
| <code>interface.15.go-consensus-libs-utils-gen-string-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/testonly.go:147</code> | <code>func GenString(rng Rng, n int) string</code> |
| <code>interface.15.go-consensus-libs-utils-gen-timestamp-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/testonly.go:204</code> | <code>func GenTimestamp(rng Rng) time.Time</code> |
| <code>interface.15.go-consensus-libs-utils-ignore-after-cancel-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/testonly.go:229</code> | <code>func IgnoreAfterCancel(ctx context.Context, err error) error</code> |
| <code>interface.15.go-consensus-libs-utils-ignore-cancel-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/wait.go:12</code> | <code>func IgnoreCancel(err error) error</code> |
| <code>interface.15.go-consensus-libs-utils-im-map-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/im/im.go:12</code> | <code>type Map[K comparable, V any] struct{ m *immutable.Map[K, V] }</code> |
| <code>interface.15.go-consensus-libs-utils-im-new-map-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/im/im.go:24</code> | <code>func NewMap[K comparable, V any]() Map[K, V]</code> |
| <code>interface.15.go-consensus-libs-utils-map-opt-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/option.go:51</code> | <code>func MapOpt[T, R any](o Option[T], f func(T) R) Option[R]</code> |
| <code>interface.15.go-consensus-libs-utils-max-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/math.go:15</code> | <code>func Max[I constraints.Integer]() I</code> |
| <code>interface.15.go-consensus-libs-utils-monitor-watch-updates-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/mutex.go:223</code> | <code>func MonitorWatchUpdates[T any](w *Watch[T], f func()) bool</code> |
| <code>interface.15.go-consensus-libs-utils-mutex-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/mutex.go:13</code> | <code>type Mutex[T any] struct</code> |
| <code>interface.15.go-consensus-libs-utils-new-atomic-send-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/mutex.go:93</code> | <code>func NewAtomicSend[T any](value T) (w AtomicSend[T])</code> |
| <code>interface.15.go-consensus-libs-utils-new-mutex-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/mutex.go:19</code> | <code>func NewMutex[T any](value T) (m Mutex[T])</code> |
| <code>interface.15.go-consensus-libs-utils-new-ring-buf-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/ringbuf.go:16</code> | <code>func NewRingBuf[T any](capacity int) RingBuf[T]</code> |
| <code>interface.15.go-consensus-libs-utils-new-rw-mutex-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/mutex.go:43</code> | <code>func NewRWMutex[T any](value T) (m RWMutex[T])</code> |
| <code>interface.15.go-consensus-libs-utils-new-semaphore-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/semaphore.go:13</code> | <code>func NewSemaphore(n int) *Semaphore</code> |
| <code>interface.15.go-consensus-libs-utils-new-watch-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/mutex.go:168</code> | <code>func NewWatch[T any](val T) Watch[T]</code> |
| <code>interface.15.go-consensus-libs-utils-none-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/option.go:20</code> | <code>func None[T any]() (zero Option[T])</code> |
| <code>interface.15.go-consensus-libs-utils-option-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/option.go:8</code> | <code>type Option[T any] struct</code> |
| <code>interface.15.go-consensus-libs-utils-or-panic-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/panic.go:6</code> | <code>func OrPanic(err error)</code> |
| <code>interface.15.go-consensus-libs-utils-or-panic1-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/panic.go:14</code> | <code>func OrPanic1[T any](v T, err error) T</code> |
| <code>interface.15.go-consensus-libs-utils-proto-conv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/proto.go:36</code> | <code>type ProtoConv[T any, P ProtoMessage] struct</code> |
| <code>interface.15.go-consensus-libs-utils-read-only-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/testonly.go:20</code> | <code>type ReadOnly struct{}</code> |
| <code>interface.15.go-consensus-libs-utils-recv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/channels.go:10</code> | <code>func Recv[T any](ctx context.Context, ch &lt;-chan T) (zero T, err error)</code> |
| <code>interface.15.go-consensus-libs-utils-recv-or-closed-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/channels.go:27</code> | <code>func RecvOrClosed[T any](ctx context.Context, ch &lt;-chan T) (T, bool, error)</code> |
| <code>interface.15.go-consensus-libs-utils-require-contains-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:27</code> | <code>var Contains = require.Contains</code> |
| <code>interface.15.go-consensus-libs-utils-require-elements-match-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:29</code> | <code>func ElementsMatch[T any](t TestingT, a []T, b []T, msgAndArgs ...any)</code> |
| <code>interface.15.go-consensus-libs-utils-require-empty-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:51</code> | <code>var Empty = require.Empty</code> |
| <code>interface.15.go-consensus-libs-utils-require-equal-error-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:39</code> | <code>var EqualError = require.EqualError</code> |
| <code>interface.15.go-consensus-libs-utils-require-equal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:106</code> | <code>func Equal[T any](t TestingT, expected, actual T, msgAndArgs ...any)</code> |
| <code>interface.15.go-consensus-libs-utils-require-error-is-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:45</code> | <code>var ErrorIs = require.ErrorIs</code> |
| <code>interface.15.go-consensus-libs-utils-require-error-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:42</code> | <code>var Error = require.Error</code> |
| <code>interface.15.go-consensus-libs-utils-require-eventually-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:34</code> | <code>var Eventually = require.Eventually</code> |
| <code>interface.15.go-consensus-libs-utils-require-fail-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:69</code> | <code>var Fail = require.Fail</code> |
| <code>interface.15.go-consensus-libs-utils-require-fail-now-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:72</code> | <code>var FailNow = require.FailNow</code> |
| <code>interface.15.go-consensus-libs-utils-require-false-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:15</code> | <code>var False = require.False</code> |
| <code>interface.15.go-consensus-libs-utils-require-file-exists-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:78</code> | <code>var FileExists = require.FileExists</code> |
| <code>interface.15.go-consensus-libs-utils-require-greater-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:96</code> | <code>func Greater[T cmp.Ordered](t TestingT, e1, e2 T, msgAndArgs ...any)</code> |
| <code>interface.15.go-consensus-libs-utils-require-greater-or-equal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:101</code> | <code>func GreaterOrEqual[T cmp.Ordered](t TestingT, e1, e2 T, msgAndArgs ...any)</code> |
| <code>interface.15.go-consensus-libs-utils-require-len-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:57</code> | <code>var Len = require.Len</code> |
| <code>interface.15.go-consensus-libs-utils-require-less-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:86</code> | <code>func Less[T cmp.Ordered](t TestingT, e1, e2 T, msgAndArgs ...any)</code> |
| <code>interface.15.go-consensus-libs-utils-require-less-or-equal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:91</code> | <code>func LessOrEqual[T cmp.Ordered](t TestingT, e1, e2 T, msgAndArgs ...any)</code> |
| <code>interface.15.go-consensus-libs-utils-require-nil-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:60</code> | <code>var Nil = require.Nil</code> |
| <code>interface.15.go-consensus-libs-utils-require-no-error-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:48</code> | <code>var NoError = require.NoError</code> |
| <code>interface.15.go-consensus-libs-utils-require-no-file-exists-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:75</code> | <code>var NoFileExists = require.NoFileExists</code> |
| <code>interface.15.go-consensus-libs-utils-require-not-empty-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:54</code> | <code>var NotEmpty = require.NotEmpty</code> |
| <code>interface.15.go-consensus-libs-utils-require-not-equal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:111</code> | <code>func NotEqual[T any](t TestingT, expected, actual T, msgAndArgs ...any)</code> |
| <code>interface.15.go-consensus-libs-utils-require-not-nil-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:63</code> | <code>var NotNil = require.NotNil</code> |
| <code>interface.15.go-consensus-libs-utils-require-panics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:66</code> | <code>var Panics = require.Panics</code> |
| <code>interface.15.go-consensus-libs-utils-require-positive-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:81</code> | <code>func Positive[T cmp.Ordered](t TestingT, e T, msgAndArgs ...any)</code> |
| <code>interface.15.go-consensus-libs-utils-require-true-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:18</code> | <code>var True = require.True</code> |
| <code>interface.15.go-consensus-libs-utils-require-zero-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/require/require.go:21</code> | <code>var Zero = require.Zero</code> |
| <code>interface.15.go-consensus-libs-utils-ring-buf-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/ringbuf.go:9</code> | <code>type RingBuf[T any] struct</code> |
| <code>interface.15.go-consensus-libs-utils-rng-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/testonly.go:70</code> | <code>type Rng struct{ inner *Mutex[*rand.Rand] }</code> |
| <code>interface.15.go-consensus-libs-utils-rw-mutex-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/mutex.go:37</code> | <code>type RWMutex[T any] struct</code> |
| <code>interface.15.go-consensus-libs-utils-safe-cast-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/math.go:29</code> | <code>func SafeCast[To, From constraints.Integer](v From) (x To, ok bool)</code> |
| <code>interface.15.go-consensus-libs-utils-scope-parallel-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/scope/parallel.go:33</code> | <code>func Parallel(main func(ParallelScope) error) error</code> |
| <code>interface.15.go-consensus-libs-utils-scope-parallel-scope-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/scope/parallel.go:15</code> | <code>type ParallelScope struct{ *parallelScope }</code> |
| <code>interface.15.go-consensus-libs-utils-scope-run-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/scope/start.go:147</code> | <code>func Run(ctx context.Context, main func(context.Context, Scope) error) error</code> |
| <code>interface.15.go-consensus-libs-utils-scope-run1-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/scope/start.go:158</code> | <code>func Run1[R any](ctx context.Context, main func(context.Context, Scope) (R, error)) (res R, err error)</code> |
| <code>interface.15.go-consensus-libs-utils-scope-scope-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/scope/start.go:25</code> | <code>type Scope struct{ *scope }</code> |
| <code>interface.15.go-consensus-libs-utils-send-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/channels.go:38</code> | <code>func Send[T any](ctx context.Context, ch chan&lt;- T, v T) error</code> |
| <code>interface.15.go-consensus-libs-utils-shuffle-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/testonly.go:156</code> | <code>func Shuffle[T any](rng Rng, s []T)</code> |
| <code>interface.15.go-consensus-libs-utils-sleep-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/wait.go:62</code> | <code>func Sleep(ctx context.Context, d time.Duration) error</code> |
| <code>interface.15.go-consensus-libs-utils-sleep-until-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/wait.go:72</code> | <code>func SleepUntil(ctx context.Context, t time.Time) error</code> |
| <code>interface.15.go-consensus-libs-utils-slice-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/proto.go:67</code> | <code>func Slice[T any](v ...T) []T</code> |
| <code>interface.15.go-consensus-libs-utils-some-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/option.go:15</code> | <code>func Some[T any](value T) Option[T]</code> |
| <code>interface.15.go-consensus-libs-utils-tcp-conn-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/tcp/tcp.go:24</code> | <code>type Conn struct</code> |
| <code>interface.15.go-consensus-libs-utils-tcp-dial-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/tcp/tcp.go:128</code> | <code>func Dial(ctx context.Context, addr netip.AddrPort) (Conn, error)</code> |
| <code>interface.15.go-consensus-libs-utils-tcp-host-port-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/tcp/tcp.go:141</code> | <code>type HostPort struct</code> |
| <code>interface.15.go-consensus-libs-utils-tcp-i-pv4-loopback-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/tcp/tcp.go:126</code> | <code>func IPv4Loopback() netip.Addr</code> |
| <code>interface.15.go-consensus-libs-utils-tcp-listen-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/tcp/tcp.go:297</code> | <code>func Listen(addr netip.AddrPort) (*Listener, error)</code> |
| <code>interface.15.go-consensus-libs-utils-tcp-parse-host-port-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/tcp/tcp.go:165</code> | <code>func ParseHostPort(hp string) (HostPort, error)</code> |
| <code>interface.15.go-consensus-libs-utils-tcp-test-pipe-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/tcp/tcp.go:362</code> | <code>func TestPipe() (Conn, Conn)</code> |
| <code>interface.15.go-consensus-libs-utils-tcp-test-reserve-addr-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/tcp/tcp.go:337</code> | <code>func TestReserveAddr() netip.AddrPort</code> |
| <code>interface.15.go-consensus-libs-utils-tcp-test-reserve-port-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/tcp/tcp.go:343</code> | <code>func TestReservePort(ip netip.Addr) netip.AddrPort</code> |
| <code>interface.15.go-consensus-libs-utils-test-diff-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/testonly.go:57</code> | <code>func TestDiff[T any](want, got T) error</code> |
| <code>interface.15.go-consensus-libs-utils-test-rng-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/testonly.go:132</code> | <code>func TestRng() Rng</code> |
| <code>interface.15.go-consensus-libs-utils-watch-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/mutex.go:160</code> | <code>type Watch[T any] struct</code> |
| <code>interface.15.go-consensus-libs-utils-with-deadline-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/wait.go:21</code> | <code>func WithDeadline(ctx context.Context, md Option[time.Time], f func(ctx context.Context) error) error</code> |
| <code>interface.15.go-consensus-libs-utils-with-opt-timeout-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/wait.go:39</code> | <code>func WithOptTimeout(ctx context.Context, d Option[time.Duration], f func(ctx context.Context) error) error</code> |
| <code>interface.15.go-consensus-libs-utils-with-opt-timeout1-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/wait.go:54</code> | <code>func WithOptTimeout1[R any](ctx context.Context, d Option[time.Duration], f func(ctx context.Context) (R, error)) (R, error)</code> |
| <code>interface.15.go-consensus-libs-utils-with-timeout-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/wait.go:32</code> | <code>func WithTimeout(ctx context.Context, d time.Duration, f func(ctx context.Context) error) error</code> |
| <code>interface.15.go-consensus-libs-utils-with-timeout1-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/wait.go:47</code> | <code>func WithTimeout1[R any](ctx context.Context, d time.Duration, f func(ctx context.Context) (R, error)) (R, error)</code> |
| <code>interface.15.go-consensus-libs-utils-zero-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/libs/utils/proto.go:73</code> | <code>func Zero[T any]() (zero T)</code> |
| <code>interface.15.go-consensus-light-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/client.go:116</code> | <code>type Client struct</code> |
| <code>interface.15.go-consensus-light-default-trust-level-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/verifier.go:16</code> | <code>DefaultTrustLevel = tmmath.Fraction{Numerator: 1, Denominator: 3}</code> |
| <code>interface.15.go-consensus-light-err-invalid-header-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/errors.go:35</code> | <code>type ErrInvalidHeader struct</code> |
| <code>interface.15.go-consensus-light-err-light-client-attack-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/errors.go:69</code> | <code>var ErrLightClientAttack = errors.New(`attempted attack detected.</code> |
| <code>interface.15.go-consensus-light-err-new-val-set-cant-be-trusted-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/errors.go:25</code> | <code>type ErrNewValSetCantBeTrusted struct</code> |
| <code>interface.15.go-consensus-light-err-no-witnesses-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/errors.go:77</code> | <code>var ErrNoWitnesses = errors.New("no witnesses connected. please reset light client")</code> |
| <code>interface.15.go-consensus-light-err-proposer-priorities-diverge-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/errors.go:95</code> | <code>type ErrProposerPrioritiesDiverge struct</code> |
| <code>interface.15.go-consensus-light-header-expired-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/verifier.go:189</code> | <code>func HeaderExpired(h *types.SignedHeader, trustingPeriod time.Duration, now time.Time) bool</code> |
| <code>interface.15.go-consensus-light-max-block-lag-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/client.go:107</code> | <code>func MaxBlockLag(d time.Duration) Option</code> |
| <code>interface.15.go-consensus-light-max-clock-drift-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/client.go:95</code> | <code>func MaxClockDrift(d time.Duration) Option</code> |
| <code>interface.15.go-consensus-light-new-client-from-trusted-store-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/client.go:225</code> | <code>func NewClientFromTrustedStore( chainID string, trustingPeriod time.Duration, primary provider.Provider, witnesses []provider.Provider, trustedStore store.Store, blacklistTTL time.Duration, options ...Option) (*Client, error)</code> |
| <code>interface.15.go-consensus-light-new-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/client.go:157</code> | <code>func NewClient( ctx context.Context, chainID string, trustOptions TrustOptions, primary provider.Provider, witnesses []provider.Provider, trustedStore store.Store, blacklistTTL time.Duration, options ...Option, ) (*Client, error)</code> |
| <code>interface.15.go-consensus-light-new-http-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/setup.go:18</code> | <code>func NewHTTPClient( ctx context.Context, chainID string, trustOptions TrustOptions, primaryAddress string, witnessesAddresses []string, trustedStore store.Store, blacklistTTL time.Duration, options ...Option) (*Client, error)</code> |
| <code>interface.15.go-consensus-light-option-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/client.go:60</code> | <code>type Option func(*Client)</code> |
| <code>interface.15.go-consensus-light-provider-err-bad-light-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/provider/errors.go:26</code> | <code>type ErrBadLightBlock struct</code> |
| <code>interface.15.go-consensus-light-provider-err-connection-closed-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/provider/errors.go:21</code> | <code>ErrConnectionClosed = errors.New("client closed connection")</code> |
| <code>interface.15.go-consensus-light-provider-err-height-too-high-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/provider/errors.go:11</code> | <code>ErrHeightTooHigh = errors.New("height requested is too high")</code> |
| <code>interface.15.go-consensus-light-provider-err-light-block-not-found-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/provider/errors.go:15</code> | <code>ErrLightBlockNotFound = errors.New("light block not found")</code> |
| <code>interface.15.go-consensus-light-provider-err-no-response-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/provider/errors.go:18</code> | <code>ErrNoResponse = errors.New("client failed to respond")</code> |
| <code>interface.15.go-consensus-light-provider-err-unreliable-provider-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/provider/errors.go:39</code> | <code>type ErrUnreliableProvider struct</code> |
| <code>interface.15.go-consensus-light-provider-http-new-j57</code> | <code>go_exported</code> | <code>change</code> | <code>consensus/light/provider/http/http.go:68</code> | <code>func New(chainID, remote string) (provider.Provider, error)</code> |
| <code>interface.15.go-consensus-light-provider-http-new-with-client-j57</code> | <code>go_exported</code> | <code>change</code> | <code>consensus/light/provider/http/http.go:88</code> | <code>func NewWithClient(chainID string, client rpcclient.RemoteClient) provider.Provider</code> |
| <code>interface.15.go-consensus-light-provider-provider-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/provider/provider.go:13</code> | <code>type Provider interface</code> |
| <code>interface.15.go-consensus-light-proxy-new-proxy-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/proxy/proxy.go:30</code> | <code>func NewProxy( lightClient *light.Client, listenAddr, providerAddr string, config *rpcserver.Config, opts ...lrpc.Option, ) (*Proxy, error)</code> |
| <code>interface.15.go-consensus-light-pruning-size-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/client.go:89</code> | <code>func PruningSize(h uint16) Option</code> |
| <code>interface.15.go-consensus-light-rpc-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/rpc/client.go:43</code> | <code>type Client struct</code> |
| <code>interface.15.go-consensus-light-rpc-default-merkle-key-path-fn-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/rpc/client.go:71</code> | <code>func DefaultMerkleKeyPathFn() KeyPathFunc</code> |
| <code>interface.15.go-consensus-light-rpc-key-path-fn-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/rpc/client.go:62</code> | <code>func KeyPathFn(fn KeyPathFunc) Option</code> |
| <code>interface.15.go-consensus-light-rpc-new-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/rpc/client.go:90</code> | <code>func NewClient(next rpcclient.Client, lc LightClient, opts ...Option) *Client</code> |
| <code>interface.15.go-consensus-light-sequential-verification-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/client.go:65</code> | <code>func SequentialVerification() Option</code> |
| <code>interface.15.go-consensus-light-skipping-verification-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/client.go:78</code> | <code>func SkippingVerification(trustLevel tmmath.Fraction) Option</code> |
| <code>interface.15.go-consensus-light-store-db-new-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/store/db/db.go:40</code> | <code>func New(db dbm.DB) store.Store</code> |
| <code>interface.15.go-consensus-light-store-err-light-block-not-found-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/store/errors.go:8</code> | <code>ErrLightBlockNotFound = errors.New("light block not found")</code> |
| <code>interface.15.go-consensus-light-store-store-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/store/store.go:6</code> | <code>type Store interface</code> |
| <code>interface.15.go-consensus-light-trust-options-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/trust_options.go:21</code> | <code>type TrustOptions struct</code> |
| <code>interface.15.go-consensus-light-validate-trust-level-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/verifier.go:179</code> | <code>func ValidateTrustLevel(lvl tmmath.Fraction) error</code> |
| <code>interface.15.go-consensus-light-verify-adjacent-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/verifier.go:106</code> | <code>func VerifyAdjacent( trustedHeader *types.SignedHeader, // height=X untrustedHeader *types.SignedHeader, // height=X+1 untrustedVals *types.ValidatorSet, // height=X+1 trustingPeriod time.Duration, now time.Time, maxClockDrift time.Duration, ) error</code> |
| <code>interface.15.go-consensus-light-verify-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/verifier.go:158</code> | <code>func Verify( trustedHeader *types.SignedHeader, // height=X trustedVals *types.ValidatorSet, // height=X or height=X+1 untrustedHeader *types.SignedHeader, // height=Y untrustedVals *types.ValidatorSet, // height=Y trustingPeriod time.Duration, now time.Time, maxClockDrift time.Duration, trustLevel tmmath.Fraction) error</code> |
| <code>interface.15.go-consensus-light-verify-non-adjacent-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/light/verifier.go:33</code> | <code>func VerifyNonAdjacent( trustedHeader *types.SignedHeader, // height=X trustedVals *types.ValidatorSet, // height=X or height=X+1 untrustedHeader *types.SignedHeader, // height=Y untrustedVals *types.ValidatorSet, // height=Y trustingPeriod time.Duration, now time.Time, maxClockDrift time.Duration, trustLevel tmmath.Fraction, ) error</code> |
| <code>interface.15.go-consensus-node-default-metrics-provider-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/node/node.go:707</code> | <code>func DefaultMetricsProvider(cfg *config.InstrumentationConfig) metricsProvider</code> |
| <code>interface.15.go-consensus-node-no-op-metrics-provider-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/node/node.go:692</code> | <code>func NoOpMetricsProvider() *NodeMetrics</code> |
| <code>interface.15.go-consensus-node-node-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/node/node.go:677</code> | <code>type NodeMetrics struct</code> |
| <code>interface.15.go-consensus-privval-dial-tcp-fn-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/socket_dialers.go:22</code> | <code>func DialTCPFn(addr string, timeoutReadWrite time.Duration, privKey crypto.PrivKey) SocketDialer</code> |
| <code>interface.15.go-consensus-privval-dial-unix-fn-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/socket_dialers.go:37</code> | <code>func DialUnixFn(addr string) SocketDialer</code> |
| <code>interface.15.go-consensus-privval-file-pv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/file.go:197</code> | <code>type FilePV struct</code> |
| <code>interface.15.go-consensus-privval-file-pv-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/file.go:51</code> | <code>type FilePVKey struct</code> |
| <code>interface.15.go-consensus-privval-gen-file-pv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/file.go:222</code> | <code>func GenFilePV(keyFilePath, stateFilePath, keyType string) (*FilePV, error)</code> |
| <code>interface.15.go-consensus-privval-grpc-dial-remote-signer-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/grpc/util.go:96</code> | <code>func DialRemoteSigner( ctx context.Context, cfg *config.PrivValidatorConfig, chainID string, usePrometheus bool, ) (*SignerClient, error)</code> |
| <code>interface.15.go-consensus-privval-grpc-new-signer-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/grpc/client.go:27</code> | <code>func NewSignerClient(conn *grpc.ClientConn, chainID string) (*SignerClient, error)</code> |
| <code>interface.15.go-consensus-privval-grpc-new-signer-server-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/grpc/server.go:21</code> | <code>func NewSignerServer(chainID string, privVal types.PrivValidator) *SignerServer</code> |
| <code>interface.15.go-consensus-privval-load-file-pv-empty-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/file.go:239</code> | <code>func LoadFilePVEmptyState(keyFilePath, stateFilePath string) (*FilePV, error)</code> |
| <code>interface.15.go-consensus-privval-load-file-pv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/file.go:233</code> | <code>func LoadFilePV(keyFilePath, stateFilePath string) (*FilePV, error)</code> |
| <code>interface.15.go-consensus-privval-load-or-gen-file-pv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/file.go:283</code> | <code>func LoadOrGenFilePV(keyFilePath, stateFilePath string) (*FilePV, error)</code> |
| <code>interface.15.go-consensus-privval-new-file-pv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/file.go:205</code> | <code>func NewFilePV(privKey crypto.PrivKey, keyFilePath, stateFilePath string) *FilePV</code> |
| <code>interface.15.go-consensus-privval-new-retry-signer-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/retry_signer_client.go:23</code> | <code>func NewRetrySignerClient(sc *SignerClient, retries int, timeout time.Duration) *RetrySignerClient</code> |
| <code>interface.15.go-consensus-privval-new-signer-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/signer_client.go:25</code> | <code>func NewSignerClient(ctx context.Context, endpoint *SignerListenerEndpoint, chainID string) (*SignerClient, error)</code> |
| <code>interface.15.go-consensus-privval-new-signer-dialer-endpoint-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/signer_dialer_endpoint.go:50</code> | <code>func NewSignerDialerEndpoint( dialer SocketDialer, options ...SignerServiceEndpointOption, ) *SignerDialerEndpoint</code> |
| <code>interface.15.go-consensus-privval-new-signer-listener-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/utils.go:28</code> | <code>func NewSignerListener(listenAddr string) (*SignerListenerEndpoint, error)</code> |
| <code>interface.15.go-consensus-privval-new-signer-server-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/signer_server.go:31</code> | <code>func NewSignerServer(endpoint *SignerDialerEndpoint, chainID string, privVal types.PrivValidator) *SignerServer</code> |
| <code>interface.15.go-consensus-privval-retry-signer-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/retry_signer_client.go:15</code> | <code>type RetrySignerClient struct</code> |
| <code>interface.15.go-consensus-privval-signer-dialer-endpoint-conn-retries-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/signer_dialer_endpoint.go:26</code> | <code>func SignerDialerEndpointConnRetries(retries int) SignerServiceEndpointOption</code> |
| <code>interface.15.go-consensus-privval-signer-dialer-endpoint-retry-wait-interval-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/signer_dialer_endpoint.go:32</code> | <code>func SignerDialerEndpointRetryWaitInterval(interval time.Duration) SignerServiceEndpointOption</code> |
| <code>interface.15.go-consensus-privval-signer-dialer-endpoint-timeout-read-write-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/signer_dialer_endpoint.go:20</code> | <code>func SignerDialerEndpointTimeoutReadWrite(timeout time.Duration) SignerServiceEndpointOption</code> |
| <code>interface.15.go-consensus-privval-socket-dialer-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/privval/socket_dialers.go:18</code> | <code>type SocketDialer func() (net.Conn, error)</code> |
| <code>interface.15.go-consensus-proto-tendermint-blocksync-schema-for-message-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/proto/tendermint/blocksync/types.wireguard.go:18</code> | <code>var SchemaForMessage = &amp;wireguard.Schema</code> |
| <code>interface.15.go-consensus-proto-tendermint-consensus-schema-for-message-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/proto/tendermint/consensus/types.wireguard.go:18</code> | <code>var SchemaForMessage = &amp;wireguard.Schema</code> |
| <code>interface.15.go-consensus-proto-tendermint-statesync-schema-for-message-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/proto/tendermint/statesync/types.wireguard.go:11</code> | <code>var SchemaForMessage = &amp;wireguard.Schema</code> |
| <code>interface.15.go-consensus-proto-tendermint-types-schema-for-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/block.wireguard.go:10</code> | <code>var SchemaForBlock = &amp;wireguard.Schema</code> |
| <code>interface.15.go-consensus-proto-tendermint-types-schema-for-evidence-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.wireguard.go:39</code> | <code>var SchemaForEvidence = &amp;wireguard.Schema</code> |
| <code>interface.15.go-consensus-proto-tendermint-types-schema-for-light-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.wireguard.go:32</code> | <code>var SchemaForLightBlock = &amp;wireguard.Schema</code> |
| <code>interface.15.go-consensus-proto-tendermint-types-schema-for-proposal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.wireguard.go:17</code> | <code>var SchemaForProposal = &amp;wireguard.Schema</code> |
| <code>interface.15.go-consensus-rpc-client-abci-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/interface.go:55</code> | <code>type ABCIClient interface</code> |
| <code>interface.15.go-consensus-rpc-client-abci-query-options-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/types.go:5</code> | <code>type ABCIQueryOptions struct</code> |
| <code>interface.15.go-consensus-rpc-client-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/interface.go:32</code> | <code>type Client interface</code> |
| <code>interface.15.go-consensus-rpc-client-default-abci-query-options-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/types.go:11</code> | <code>var DefaultABCIQueryOptions = ABCIQueryOptions{Height: 0, Prove: false}</code> |
| <code>interface.15.go-consensus-rpc-client-default-wait-strategy-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/helpers.go:18</code> | <code>func DefaultWaitStrategy(delta int64) (abort error)</code> |
| <code>interface.15.go-consensus-rpc-client-events-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/interface.go:126</code> | <code>type EventsClient interface</code> |
| <code>interface.15.go-consensus-rpc-client-eventstream-err-stop-running-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/eventstream/eventstream.go:22</code> | <code>var ErrStopRunning = errors.New("stop accepting events")</code> |
| <code>interface.15.go-consensus-rpc-client-eventstream-missed-items-error-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/eventstream/eventstream.go:178</code> | <code>type MissedItemsError struct</code> |
| <code>interface.15.go-consensus-rpc-client-eventstream-new-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/eventstream/eventstream.go:36</code> | <code>func New(cli Client, query string, opts *StreamOptions) *Stream</code> |
| <code>interface.15.go-consensus-rpc-client-eventstream-stream-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/eventstream/eventstream.go:25</code> | <code>type Stream struct</code> |
| <code>interface.15.go-consensus-rpc-client-eventstream-stream-options-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/eventstream/eventstream.go:138</code> | <code>type StreamOptions struct</code> |
| <code>interface.15.go-consensus-rpc-client-history-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/interface.go:103</code> | <code>type HistoryClient interface</code> |
| <code>interface.15.go-consensus-rpc-client-http-http-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/http/http.go:55</code> | <code>type HTTP struct</code> |
| <code>interface.15.go-consensus-rpc-client-http-new-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/http/http.go:106</code> | <code>func New(remote string) (*HTTP, error)</code> |
| <code>interface.15.go-consensus-rpc-client-http-new-with-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/http/http.go:127</code> | <code>func NewWithClient(remote string, c *http.Client) (*HTTP, error)</code> |
| <code>interface.15.go-consensus-rpc-client-http-new-with-timeout-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/http/http.go:116</code> | <code>func NewWithTimeout(remote string, t time.Duration) (*HTTP, error)</code> |
| <code>interface.15.go-consensus-rpc-client-local-new-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/local/local.go:58</code> | <code>func New(node NodeService) (*Local, error)</code> |
| <code>interface.15.go-consensus-rpc-client-local-node-service-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/local/local.go:51</code> | <code>type NodeService interface</code> |
| <code>interface.15.go-consensus-rpc-client-mempool-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/interface.go:164</code> | <code>type MempoolClient interface</code> |
| <code>interface.15.go-consensus-rpc-client-mock-call-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/mock/client.go:44</code> | <code>type Call struct</code> |
| <code>interface.15.go-consensus-rpc-client-mock-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/mock/client.go:29</code> | <code>type Client struct</code> |
| <code>interface.15.go-consensus-rpc-client-mock-new-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/mock/client.go:34</code> | <code>func New() Client</code> |
| <code>interface.15.go-consensus-rpc-client-mock-new-status-recorder-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/mock/status.go:43</code> | <code>func NewStatusRecorder(client client.StatusClient) *StatusRecorder</code> |
| <code>interface.15.go-consensus-rpc-client-mock-status-mock-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/mock/status.go:11</code> | <code>type StatusMock struct</code> |
| <code>interface.15.go-consensus-rpc-client-network-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/interface.go:117</code> | <code>type NetworkClient interface</code> |
| <code>interface.15.go-consensus-rpc-client-remote-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/interface.go:177</code> | <code>type RemoteClient interface</code> |
| <code>interface.15.go-consensus-rpc-client-sign-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/interface.go:72</code> | <code>type SignClient interface</code> |
| <code>interface.15.go-consensus-rpc-client-status-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/interface.go:110</code> | <code>type StatusClient interface</code> |
| <code>interface.15.go-consensus-rpc-client-subscription-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/interface.go:134</code> | <code>type SubscriptionClient interface</code> |
| <code>interface.15.go-consensus-rpc-client-wait-for-height-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/helpers.go:36</code> | <code>func WaitForHeight(ctx context.Context, c StatusClient, h int64, waiter Waiter) error</code> |
| <code>interface.15.go-consensus-rpc-client-wait-for-one-event-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/client/helpers.go:59</code> | <code>func WaitForOneEvent(ctx context.Context, c EventsClient, query string) (types.EventData, error)</code> |
| <code>interface.15.go-consensus-rpc-coretypes-application-info-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:149</code> | <code>type ApplicationInfo struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-err-height-exceeds-chain-head-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:24</code> | <code>ErrHeightExceedsChainHead = errors.New("height must be less than or equal to the head of the node's blockchain")</code> |
| <code>interface.15.go-consensus-rpc-coretypes-err-height-not-available-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:25</code> | <code>ErrHeightNotAvailable = errors.New("height is not available")</code> |
| <code>interface.15.go-consensus-rpc-coretypes-err-invalid-request-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:29</code> | <code>ErrInvalidRequest = errors.New("invalid request")</code> |
| <code>interface.15.go-consensus-rpc-coretypes-err-lag-is-too-high-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:26</code> | <code>ErrLagIsTooHigh = errors.New("lag is too high")</code> |
| <code>interface.15.go-consensus-rpc-coretypes-err-page-out-of-range-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:22</code> | <code>ErrPageOutOfRange = errors.New("page should be within range")</code> |
| <code>interface.15.go-consensus-rpc-coretypes-err-zero-or-negative-height-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:23</code> | <code>ErrZeroOrNegativeHeight = errors.New("height must be greater than zero")</code> |
| <code>interface.15.go-consensus-rpc-coretypes-err-zero-or-negative-per-page-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:21</code> | <code>ErrZeroOrNegativePerPage = errors.New("zero or negative per_page")</code> |
| <code>interface.15.go-consensus-rpc-coretypes-event-filter-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:143</code> | <code>type EventFilter struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-event-item-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:494</code> | <code>type EventItem struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-int64-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:149</code> | <code>type Int64 int64</code> |
| <code>interface.15.go-consensus-rpc-coretypes-int64-ptr-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:182</code> | <code>func Int64Ptr(v *int) *Int64</code> |
| <code>interface.15.go-consensus-rpc-coretypes-new-result-commit-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:100</code> | <code>func NewResultCommit(header *types.Header, commit *types.Commit, canonical bool) *ResultCommit</code> |
| <code>interface.15.go-consensus-rpc-coretypes-peer-connection-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:245</code> | <code>type PeerConnection struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-peer-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:239</code> | <code>type Peer struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-peer-state-info-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:274</code> | <code>type PeerStateInfo struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-request-abci-query-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:85</code> | <code>type RequestABCIQuery struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-request-block-by-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:34</code> | <code>type RequestBlockByHash struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-request-block-info-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:30</code> | <code>type RequestBlockInfo struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-request-block-search-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:59</code> | <code>type RequestBlockSearch struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-request-blockchain-info-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:21</code> | <code>type RequestBlockchainInfo struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-request-broadcast-evidence-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:92</code> | <code>type RequestBroadcastEvidence struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-request-broadcast-tx-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:81</code> | <code>type RequestBroadcastTx struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-request-check-tx-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:38</code> | <code>type RequestCheckTx struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-request-consensus-params-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:72</code> | <code>type RequestConsensusParams struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-request-events-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:122</code> | <code>type RequestEvents struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-request-genesis-chunked-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:26</code> | <code>type RequestGenesisChunked struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-request-subscribe-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:13</code> | <code>type RequestSubscribe struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-request-tx-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:46</code> | <code>type RequestTx struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-request-tx-search-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:51</code> | <code>type RequestTxSearch struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-request-unconfirmed-txs-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:76</code> | <code>type RequestUnconfirmedTxs struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-request-unsubscribe-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:17</code> | <code>type RequestUnsubscribe struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-request-validators-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/requests.go:66</code> | <code>type RequestValidators struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-abci-info-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:337</code> | <code>type ResultABCIInfo struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-abci-query-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:342</code> | <code>type ResultABCIQuery struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:72</code> | <code>type ResultBlock struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-block-results-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:89</code> | <code>type ResultBlockResults struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-block-search-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:323</code> | <code>type ResultBlockSearch struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-blockchain-info-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:51</code> | <code>type ResultBlockchainInfo struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-broadcast-evidence-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:347</code> | <code>type ResultBroadcastEvidence struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-broadcast-tx-commit-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:294</code> | <code>type ResultBroadcastTxCommit struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-broadcast-tx-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:285</code> | <code>type ResultBroadcastTx struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-check-tx-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:302</code> | <code>type ResultCheckTx struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-commit-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:83</code> | <code>type ResultCommit struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-consensus-params-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:261</code> | <code>type ResultConsensusParams struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-consensus-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:280</code> | <code>type ResultConsensusState struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-dump-consensus-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:268</code> | <code>type ResultDumpConsensusState struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-event-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:361</code> | <code>type ResultEvent struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-events-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:476</code> | <code>type ResultEvents struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-genesis-chunk-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:65</code> | <code>type ResultGenesisChunk struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-genesis-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:57</code> | <code>type ResultGenesis struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-header-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:78</code> | <code>type ResultHeader struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-health-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:357</code> | <code>ResultHealth struct{}</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-lag-status-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:205</code> | <code>type ResultLagStatus struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-net-info-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:220</code> | <code>type ResultNetInfo struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-status-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:196</code> | <code>type ResultStatus struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-subscribe-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:355</code> | <code>ResultSubscribe struct{}</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-tx-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:307</code> | <code>type ResultTx struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-tx-search-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:317</code> | <code>type ResultTxSearch struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-unconfirmed-txs-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:329</code> | <code>type ResultUnconfirmedTxs struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-unsafe-flush-mempool-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:353</code> | <code>ResultUnsafeFlushMempool struct{}</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-unsubscribe-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:356</code> | <code>ResultUnsubscribe struct{}</code> |
| <code>interface.15.go-consensus-rpc-coretypes-result-validators-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:252</code> | <code>type ResultValidators struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-sync-info-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:113</code> | <code>type SyncInfo struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-validator-info-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:154</code> | <code>type ValidatorInfo struct</code> |
| <code>interface.15.go-consensus-rpc-coretypes-wrap-err-height-not-available-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/coretypes/responses.go:43</code> | <code>func WrapErrHeightNotAvailable(height int64, base utils.Option[int64]) error</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-client-caller-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/client/http_json_client.go:112</code> | <code>type Caller interface</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-client-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/client/http_json_client.go:126</code> | <code>type Client struct</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-client-default-http-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/client/http_json_client.go:388</code> | <code>func DefaultHTTPClient(remoteAddr string) (*http.Client, error)</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-client-new-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/client/http_json_client.go:144</code> | <code>func New(remote string) (*Client, error)</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-client-new-with-http-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/client/http_json_client.go:155</code> | <code>func NewWithHTTPClient(remote string, c *http.Client) (*Client, error)</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-client-new-ws-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/client/ws_client.go:86</code> | <code>func NewWS(remoteAddr, endpoint string) (*WSClient, error)</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-client-request-batch-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/client/http_json_client.go:298</code> | <code>type RequestBatch struct</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-client-ws-client-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/client/ws_client.go:41</code> | <code>type WSClient struct</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/server/http_server.go:24</code> | <code>type Config struct</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-default-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/server/http_server.go:50</code> | <code>func DefaultConfig() *Config</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-listen-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/server/http_server.go:292</code> | <code>func Listen(addr string, maxOpenConnections int) (listener net.Listener, err error)</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-new-rpc-func-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/server/rpc_func.go:154</code> | <code>func NewRPCFunc(f interface</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-new-websocket-manager-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/server/ws_handler.go:37</code> | <code>func NewWebsocketManager(funcMap map[string]*RPCFunc, wsConnOptions ...func(*wsConnection)) *WebsocketManager</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-new-wsrpc-func-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/server/rpc_func.go:164</code> | <code>func NewWSRPCFunc(f interface</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-on-disconnect-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/server/ws_handler.go:149</code> | <code>func OnDisconnect(onDisconnect func(remoteAddr string)) func(*wsConnection)</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-ping-period-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/server/ws_handler.go:165</code> | <code>func PingPeriod(pingPeriod time.Duration) func(*wsConnection)</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-read-limit-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/server/ws_handler.go:173</code> | <code>func ReadLimit(readLimit int64) func(*wsConnection)</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-read-wait-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/server/ws_handler.go:157</code> | <code>func ReadWait(readWait time.Duration) func(*wsConnection)</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-register-rpc-funcs-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/server/rpc_func.go:23</code> | <code>func RegisterRPCFuncs(mux *http.ServeMux, funcMap map[string]*RPCFunc)</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-rpc-func-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/server/rpc_func.go:38</code> | <code>type RPCFunc struct</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-serve-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/server/http_server.go:62</code> | <code>func Serve(ctx context.Context, listener net.Listener, handler http.Handler, config *Config) error</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-serve-tls-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/server/http_server.go:93</code> | <code>func ServeTLS(ctx context.Context, listener net.Listener, handler http.Handler, certFile, keyFile string, config *Config) error</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-call-info-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/types/types.go:277</code> | <code>type CallInfo struct</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-code-internal-error-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/types/types.go:34</code> | <code>CodeInternalError ErrorCode = -32603 // Internal JSON-RPC error</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-code-invalid-params-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/types/types.go:33</code> | <code>CodeInvalidParams ErrorCode = -32602 // Invalid method parameters</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-code-invalid-request-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/types/types.go:31</code> | <code>CodeInvalidRequest ErrorCode = -32600 // The JSON sent is not a valid request object</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-code-lag-is-high-error-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/types/types.go:35</code> | <code>CodeLagIsHighError ErrorCode = -32604 // Lag is too high error</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-code-method-not-found-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/types/types.go:32</code> | <code>CodeMethodNotFound ErrorCode = -32601 // The method does not exist or is unavailable</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-code-parse-error-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/types/types.go:30</code> | <code>CodeParseError ErrorCode = -32700 // Invalid JSON received by the server</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-get-call-info-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/types/types.go:292</code> | <code>func GetCallInfo(ctx context.Context) *CallInfo</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-new-request-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/types/types.go:58</code> | <code>func NewRequest(id int) RPCRequest</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-rpc-error-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/types/types.go:193</code> | <code>type RPCError struct</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-rpc-request-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/types/types.go:50</code> | <code>type RPCRequest struct</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-rpc-response-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/types/types.go:207</code> | <code>type RPCResponse struct</code> |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-with-call-info-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/jsonrpc/types/types.go:286</code> | <code>func WithCallInfo(ctx context.Context, ci *CallInfo) context.Context</code> |
| <code>interface.15.go-consensus-rpc-test-create-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/test/helpers.go:62</code> | <code>func CreateConfig(t *testing.T, testName string) (*config.Config, error)</code> |
| <code>interface.15.go-consensus-rpc-test-start-tendermint-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/test/helpers.go:79</code> | <code>func StartTendermint( ctx context.Context, conf *config.Config, app abci.Application, opts ...func(*Options), ) (service.Service, ServiceCloser, error)</code> |
| <code>interface.15.go-consensus-rpc-test-suppress-stdout-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/rpc/test/helpers.go:134</code> | <code>func SuppressStdout(o *Options)</code> |
| <code>interface.15.go-consensus-scripts-keymigrate-migrate-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/scripts/keymigrate/migrate.go:436</code> | <code>func Migrate(ctx context.Context, db dbm.DB) error</code> |
| <code>interface.15.go-consensus-scripts-metricsgen-testdata-basic-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/scripts/metricsgen/testdata/basic/metrics.go:8</code> | <code>type Metrics struct</code> |
| <code>interface.15.go-consensus-scripts-metricsgen-testdata-basic-nop-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/scripts/metricsgen/testdata/basic/metrics.gen.go:26</code> | <code>func NopMetrics() *Metrics</code> |
| <code>interface.15.go-consensus-scripts-metricsgen-testdata-basic-prometheus-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/scripts/metricsgen/testdata/basic/metrics.gen.go:11</code> | <code>func PrometheusMetrics(namespace string, labelsAndValues ...string) *Metrics</code> |
| <code>interface.15.go-consensus-scripts-metricsgen-testdata-commented-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/scripts/metricsgen/testdata/commented/metrics.go:7</code> | <code>type Metrics struct</code> |
| <code>interface.15.go-consensus-scripts-metricsgen-testdata-commented-nop-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/scripts/metricsgen/testdata/commented/metrics.gen.go:26</code> | <code>func NopMetrics() *Metrics</code> |
| <code>interface.15.go-consensus-scripts-metricsgen-testdata-commented-prometheus-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/scripts/metricsgen/testdata/commented/metrics.gen.go:11</code> | <code>func PrometheusMetrics(namespace string, labelsAndValues ...string) *Metrics</code> |
| <code>interface.15.go-consensus-scripts-metricsgen-testdata-tags-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/scripts/metricsgen/testdata/tags/metrics.go:7</code> | <code>type Metrics struct</code> |
| <code>interface.15.go-consensus-scripts-metricsgen-testdata-tags-nop-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/scripts/metricsgen/testdata/tags/metrics.gen.go:48</code> | <code>func NopMetrics() *Metrics</code> |
| <code>interface.15.go-consensus-scripts-metricsgen-testdata-tags-prometheus-metrics-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/scripts/metricsgen/testdata/tags/metrics.gen.go:11</code> | <code>func PrometheusMetrics(namespace string, labelsAndValues ...string) *Metrics</code> |
| <code>interface.15.go-consensus-scripts-scmigrate-migrate-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/scripts/scmigrate/migrate.go:166</code> | <code>func Migrate(ctx context.Context, db dbm.DB) error</code> |
| <code>interface.15.go-consensus-state-abci-responses-results-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/state/store.go:8</code> | <code>func ABCIResponsesResultsHash(ar *tmstate.ABCIResponses) []byte</code> |
| <code>interface.15.go-consensus-test-e2e-app-config-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/app/app.go:40</code> | <code>type Config struct</code> |
| <code>interface.15.go-consensus-test-e2e-app-new-application-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/app/app.go:95</code> | <code>func NewApplication(cfg *Config) (*Application, error)</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-evidence-age-height-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:50</code> | <code>EvidenceAgeHeight int64 = 7</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-evidence-age-time-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:51</code> | <code>EvidenceAgeTime time.Duration = 500 * time.Millisecond</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-load-testnet-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:110</code> | <code>func LoadTestnet(file string) (*Testnet, error)</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-manifest-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/manifest.go:13</code> | <code>type Manifest struct</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-manifest-node-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/manifest.go:85</code> | <code>type ManifestNode struct</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-mode-full-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:35</code> | <code>ModeFull Mode = "full"</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-mode-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:29</code> | <code>type Mode string</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-mode-light-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:36</code> | <code>ModeLight Mode = "light"</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-mode-seed-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:37</code> | <code>ModeSeed Mode = "seed"</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-mode-validator-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:34</code> | <code>ModeValidator Mode = "validator"</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-node-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:81</code> | <code>type Node struct</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-perturbation-disconnect-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:45</code> | <code>PerturbationDisconnect Perturbation = "disconnect"</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-perturbation-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:31</code> | <code>type Perturbation string</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-perturbation-kill-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:46</code> | <code>PerturbationKill Perturbation = "kill"</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-perturbation-pause-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:47</code> | <code>PerturbationPause Perturbation = "pause"</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-perturbation-restart-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:48</code> | <code>PerturbationRestart Perturbation = "restart"</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-protocol-builtin-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:39</code> | <code>ProtocolBuiltin Protocol = "builtin"</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-protocol-file-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:40</code> | <code>ProtocolFile Protocol = "file"</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-protocol-grpc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:41</code> | <code>ProtocolGRPC Protocol = "grpc"</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-protocol-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:30</code> | <code>type Protocol string</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-protocol-tcp-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:42</code> | <code>ProtocolTCP Protocol = "tcp"</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-protocol-unix-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:43</code> | <code>ProtocolUNIX Protocol = "unix"</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-sort-manifests-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/manifest.go:185</code> | <code>func SortManifests(manifests []Manifest, reverse bool)</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-split-groups-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/manifest.go:243</code> | <code>func SplitGroups(groups int, manifests []Manifest) [][]Manifest</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-state-sync-disabled-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:55</code> | <code>StateSyncDisabled = ""</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-state-sync-p2-p-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:53</code> | <code>StateSyncP2P = "p2p"</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-state-sync-rpc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:54</code> | <code>StateSyncRPC = "rpc"</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-testnet-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/testnet.go:59</code> | <code>type Testnet struct</code> |
| <code>interface.15.go-consensus-test-e2e-pkg-write-manifests-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/test/e2e/pkg/manifest.go:258</code> | <code>func WriteManifests(prefix string, manifests []Manifest) error</code> |
| <code>interface.15.go-consensus-types-abci-pub-key-type-ed25519-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/params.go:26</code> | <code>ABCIPubKeyTypeEd25519 = ed25519.KeyType</code> |
| <code>interface.15.go-consensus-types-address-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/vote.go:47</code> | <code>type Address = crypto.Address</code> |
| <code>interface.15.go-consensus-types-block-event-publisher-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:499</code> | <code>type BlockEventPublisher interface</code> |
| <code>interface.15.go-consensus-types-block-from-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:296</code> | <code>func BlockFromProto(bp *tmproto.Block) (*Block, error)</code> |
| <code>interface.15.go-consensus-types-block-height-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:466</code> | <code>BlockHeightKey = "block.height"</code> |
| <code>interface.15.go-consensus-types-block-id-flag-absent-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:654</code> | <code>BlockIDFlagAbsent BlockIDFlag = iota + 1</code> |
| <code>interface.15.go-consensus-types-block-id-flag-commit-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:656</code> | <code>BlockIDFlagCommit</code> |
| <code>interface.15.go-consensus-types-block-id-flag-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:650</code> | <code>type BlockIDFlag byte</code> |
| <code>interface.15.go-consensus-types-block-id-flag-nil-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:658</code> | <code>BlockIDFlagNil</code> |
| <code>interface.15.go-consensus-types-block-id-from-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:1251</code> | <code>func BlockIDFromProto(bID *tmproto.BlockID) (*BlockID, error)</code> |
| <code>interface.15.go-consensus-types-block-id-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:1180</code> | <code>type BlockID struct</code> |
| <code>interface.15.go-consensus-types-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:51</code> | <code>type Block struct</code> |
| <code>interface.15.go-consensus-types-block-meta-from-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block_meta.go:43</code> | <code>func BlockMetaFromProto(pb *tmproto.BlockMeta) (*BlockMeta, error)</code> |
| <code>interface.15.go-consensus-types-block-meta-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block_meta.go:12</code> | <code>type BlockMeta struct</code> |
| <code>interface.15.go-consensus-types-block-params-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/params.go:55</code> | <code>type BlockParams struct</code> |
| <code>interface.15.go-consensus-types-block-part-size-bytes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/params.go:20</code> | <code>BlockPartSizeBytes uint32 = 1048576 // 1MB</code> |
| <code>interface.15.go-consensus-types-commit-from-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:979</code> | <code>func CommitFromProto(cp *tmproto.Commit) (*Commit, error)</code> |
| <code>interface.15.go-consensus-types-commit-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:805</code> | <code>type Commit struct</code> |
| <code>interface.15.go-consensus-types-commit-sig-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:670</code> | <code>type CommitSig struct</code> |
| <code>interface.15.go-consensus-types-compute-proto-size-for-txs-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/tx.go:154</code> | <code>func ComputeProtoSizeForTxs(txs []Tx) int64</code> |
| <code>interface.15.go-consensus-types-consensus-params-from-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/params.go:529</code> | <code>func ConsensusParamsFromProto(pbParams tmproto.ConsensusParams) ConsensusParams</code> |
| <code>interface.15.go-consensus-types-consensus-params-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/params.go:35</code> | <code>type ConsensusParams struct</code> |
| <code>interface.15.go-consensus-types-consensus-policy-2-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/consensus_policy_mock_block_validation.go:11</code> | <code>type ConsensusPolicy struct{}</code> |
| <code>interface.15.go-consensus-types-consensus-policy-3-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/consensus_policy_mock_chain_validation.go:11</code> | <code>type ConsensusPolicy struct{}</code> |
| <code>interface.15.go-consensus-types-consensus-policy-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/consensus_policy_default.go:5</code> | <code>type ConsensusPolicy struct{}</code> |
| <code>interface.15.go-consensus-types-data-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:1096</code> | <code>type Data struct</code> |
| <code>interface.15.go-consensus-types-default-block-params-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/params.go:135</code> | <code>func DefaultBlockParams() BlockParams</code> |
| <code>interface.15.go-consensus-types-default-consensus-params-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/params.go:122</code> | <code>func DefaultConsensusParams() *ConsensusParams</code> |
| <code>interface.15.go-consensus-types-default-consensus-policy-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/consensus_policy.go:29</code> | <code>func DefaultConsensusPolicy() ConsensusPolicy</code> |
| <code>interface.15.go-consensus-types-duplicate-vote-evidence-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/evidence.go:82</code> | <code>type DuplicateVoteEvidence struct</code> |
| <code>interface.15.go-consensus-types-err-app-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/consensus_policy.go:44</code> | <code>ErrAppHash = &amp;ConsensusPolicyError{"app hash mismatch"}</code> |
| <code>interface.15.go-consensus-types-err-consensus-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/consensus_policy.go:48</code> | <code>ErrConsensusHash = &amp;ConsensusPolicyError{"consensus hash mismatch"}</code> |
| <code>interface.15.go-consensus-types-err-evidence-overflow-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/evidence.go:829</code> | <code>type ErrEvidenceOverflow struct</code> |
| <code>interface.15.go-consensus-types-err-invalid-commit-height-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/errors.go:8</code> | <code>ErrInvalidCommitHeight struct {</code> |
| <code>interface.15.go-consensus-types-err-invalid-commit-signatures-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/errors.go:15</code> | <code>ErrInvalidCommitSignatures struct {</code> |
| <code>interface.15.go-consensus-types-err-invalid-evidence-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/evidence.go:813</code> | <code>type ErrInvalidEvidence struct</code> |
| <code>interface.15.go-consensus-types-err-last-block-id-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/consensus_policy.go:47</code> | <code>ErrLastBlockID = &amp;ConsensusPolicyError{"last block ID mismatch"}</code> |
| <code>interface.15.go-consensus-types-err-last-commit-verify-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/consensus_policy.go:51</code> | <code>ErrLastCommitVerify = &amp;ConsensusPolicyError{"last commit verification failed"}</code> |
| <code>interface.15.go-consensus-types-err-last-results-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/consensus_policy.go:46</code> | <code>ErrLastResultsHash = &amp;ConsensusPolicyError{"last results hash mismatch"}</code> |
| <code>interface.15.go-consensus-types-err-next-validators-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/consensus_policy.go:50</code> | <code>ErrNextValidatorsHash = &amp;ConsensusPolicyError{"next validators hash mismatch"}</code> |
| <code>interface.15.go-consensus-types-err-not-enough-voting-power-signed-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/validator_set.go:759</code> | <code>type ErrNotEnoughVotingPowerSigned struct</code> |
| <code>interface.15.go-consensus-types-err-part-set-invalid-proof-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/part_set.go:23</code> | <code>ErrPartSetInvalidProof = errors.New("error part set invalid proof")</code> |
| <code>interface.15.go-consensus-types-err-part-set-unexpected-index-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/part_set.go:22</code> | <code>ErrPartSetUnexpectedIndex = errors.New("error part set unexpected index")</code> |
| <code>interface.15.go-consensus-types-err-proposer-not-in-validator-set-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/consensus_policy.go:52</code> | <code>ErrProposerNotInValidatorSet = &amp;ConsensusPolicyError{"proposer not in validator set"}</code> |
| <code>interface.15.go-consensus-types-err-too-much-evidence-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/consensus_policy.go:55</code> | <code>ErrTooMuchEvidence = &amp;ConsensusPolicyError{"evidence size exceeds limit"}</code> |
| <code>interface.15.go-consensus-types-err-validators-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/consensus_policy.go:49</code> | <code>ErrValidatorsHash = &amp;ConsensusPolicyError{"validators hash mismatch"}</code> |
| <code>interface.15.go-consensus-types-err-vote-conflicting-votes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/vote.go:30</code> | <code>type ErrVoteConflictingVotes struct</code> |
| <code>interface.15.go-consensus-types-err-vote-invalid-signature-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/vote.go:24</code> | <code>ErrVoteInvalidSignature = errors.New("invalid signature")</code> |
| <code>interface.15.go-consensus-types-err-vote-non-deterministic-signature-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/vote.go:26</code> | <code>ErrVoteNonDeterministicSignature = errors.New("non-deterministic signature")</code> |
| <code>interface.15.go-consensus-types-erroring-mock-pv-err-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/priv_validator.go:136</code> | <code>var ErroringMockPVErr = errors.New("erroringMockPV always returns an error")</code> |
| <code>interface.15.go-consensus-types-event-block-sync-status-value-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:33</code> | <code>EventBlockSyncStatusValue = "BlockSyncStatus"</code> |
| <code>interface.15.go-consensus-types-event-complete-proposal-value-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:30</code> | <code>EventCompleteProposalValue = "CompleteProposal"</code> |
| <code>interface.15.go-consensus-types-event-data-block-sync-status-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:412</code> | <code>type EventDataBlockSyncStatus struct</code> |
| <code>interface.15.go-consensus-types-event-data-complete-proposal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:364</code> | <code>type EventDataCompleteProposal struct</code> |
| <code>interface.15.go-consensus-types-event-data-evidence-validated-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:438</code> | <code>type EventDataEvidenceValidated struct</code> |
| <code>interface.15.go-consensus-types-event-data-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:101</code> | <code>type EventData interface</code> |
| <code>interface.15.go-consensus-types-event-data-new-block-header-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:242</code> | <code>type EventDataNewBlockHeader struct</code> |
| <code>interface.15.go-consensus-types-event-data-new-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:132</code> | <code>type EventDataNewBlock struct</code> |
| <code>interface.15.go-consensus-types-event-data-new-evidence-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:262</code> | <code>type EventDataNewEvidence struct</code> |
| <code>interface.15.go-consensus-types-event-data-new-round-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:349</code> | <code>type EventDataNewRound struct</code> |
| <code>interface.15.go-consensus-types-event-data-round-state-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:331</code> | <code>type EventDataRoundState struct</code> |
| <code>interface.15.go-consensus-types-event-data-state-sync-status-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:426</code> | <code>type EventDataStateSyncStatus struct</code> |
| <code>interface.15.go-consensus-types-event-data-string-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:390</code> | <code>type EventDataString string</code> |
| <code>interface.15.go-consensus-types-event-data-tx-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:276</code> | <code>type EventDataTx struct</code> |
| <code>interface.15.go-consensus-types-event-data-validator-set-updates-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:399</code> | <code>type EventDataValidatorSetUpdates struct</code> |
| <code>interface.15.go-consensus-types-event-data-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:379</code> | <code>type EventDataVote struct</code> |
| <code>interface.15.go-consensus-types-event-evidence-validated-value-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:47</code> | <code>EventEvidenceValidatedValue = "EvidenceValidated"</code> |
| <code>interface.15.go-consensus-types-event-lock-value-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:34</code> | <code>EventLockValue = "Lock"</code> |
| <code>interface.15.go-consensus-types-event-new-block-header-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:62</code> | <code>EventNewBlockHeader = abci.Event{</code> |
| <code>interface.15.go-consensus-types-event-new-block-header-value-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:22</code> | <code>EventNewBlockHeaderValue = "NewBlockHeader"</code> |
| <code>interface.15.go-consensus-types-event-new-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:52</code> | <code>EventNewBlock = abci.Event{</code> |
| <code>interface.15.go-consensus-types-event-new-block-value-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:21</code> | <code>EventNewBlockValue = "NewBlock"</code> |
| <code>interface.15.go-consensus-types-event-new-evidence-value-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:23</code> | <code>EventNewEvidenceValue = "NewEvidence"</code> |
| <code>interface.15.go-consensus-types-event-new-round-step-value-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:36</code> | <code>EventNewRoundStepValue = "NewRoundStep"</code> |
| <code>interface.15.go-consensus-types-event-new-round-value-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:35</code> | <code>EventNewRoundValue = "NewRound"</code> |
| <code>interface.15.go-consensus-types-event-polka-value-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:37</code> | <code>EventPolkaValue = "Polka"</code> |
| <code>interface.15.go-consensus-types-event-query-block-sync-status-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:485</code> | <code>EventQueryBlockSyncStatus = QueryForEvent(EventBlockSyncStatusValue)</code> |
| <code>interface.15.go-consensus-types-event-query-complete-proposal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:470</code> | <code>EventQueryCompleteProposal = QueryForEvent(EventCompleteProposalValue)</code> |
| <code>interface.15.go-consensus-types-event-query-lock-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:471</code> | <code>EventQueryLock = QueryForEvent(EventLockValue)</code> |
| <code>interface.15.go-consensus-types-event-query-new-block-header-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:473</code> | <code>EventQueryNewBlockHeader = QueryForEvent(EventNewBlockHeaderValue)</code> |
| <code>interface.15.go-consensus-types-event-query-new-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:472</code> | <code>EventQueryNewBlock = QueryForEvent(EventNewBlockValue)</code> |
| <code>interface.15.go-consensus-types-event-query-new-round-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:475</code> | <code>EventQueryNewRound = QueryForEvent(EventNewRoundValue)</code> |
| <code>interface.15.go-consensus-types-event-query-new-round-step-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:476</code> | <code>EventQueryNewRoundStep = QueryForEvent(EventNewRoundStepValue)</code> |
| <code>interface.15.go-consensus-types-event-query-polka-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:477</code> | <code>EventQueryPolka = QueryForEvent(EventPolkaValue)</code> |
| <code>interface.15.go-consensus-types-event-query-relock-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:478</code> | <code>EventQueryRelock = QueryForEvent(EventRelockValue)</code> |
| <code>interface.15.go-consensus-types-event-query-state-sync-status-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:486</code> | <code>EventQueryStateSyncStatus = QueryForEvent(EventStateSyncStatusValue)</code> |
| <code>interface.15.go-consensus-types-event-query-timeout-propose-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:479</code> | <code>EventQueryTimeoutPropose = QueryForEvent(EventTimeoutProposeValue)</code> |
| <code>interface.15.go-consensus-types-event-query-timeout-wait-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:480</code> | <code>EventQueryTimeoutWait = QueryForEvent(EventTimeoutWaitValue)</code> |
| <code>interface.15.go-consensus-types-event-query-tx-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:481</code> | <code>EventQueryTx = QueryForEvent(EventTxValue)</code> |
| <code>interface.15.go-consensus-types-event-query-valid-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:483</code> | <code>EventQueryValidBlock = QueryForEvent(EventValidBlockValue)</code> |
| <code>interface.15.go-consensus-types-event-query-validator-set-updates-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:482</code> | <code>EventQueryValidatorSetUpdates = QueryForEvent(EventValidatorSetUpdatesValue)</code> |
| <code>interface.15.go-consensus-types-event-query-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:484</code> | <code>EventQueryVote = QueryForEvent(EventVoteValue)</code> |
| <code>interface.15.go-consensus-types-event-relock-value-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:38</code> | <code>EventRelockValue = "Relock"</code> |
| <code>interface.15.go-consensus-types-event-state-sync-status-value-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:39</code> | <code>EventStateSyncStatusValue = "StateSyncStatus"</code> |
| <code>interface.15.go-consensus-types-event-timeout-propose-value-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:40</code> | <code>EventTimeoutProposeValue = "TimeoutPropose"</code> |
| <code>interface.15.go-consensus-types-event-timeout-wait-value-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:41</code> | <code>EventTimeoutWaitValue = "TimeoutWait"</code> |
| <code>interface.15.go-consensus-types-event-tx-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:82</code> | <code>EventTx = abci.Event{</code> |
| <code>interface.15.go-consensus-types-event-tx-value-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:24</code> | <code>EventTxValue = "Tx"</code> |
| <code>interface.15.go-consensus-types-event-type-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:455</code> | <code>EventTypeKey = "tm.event"</code> |
| <code>interface.15.go-consensus-types-event-valid-block-value-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:42</code> | <code>EventValidBlockValue = "ValidBlock"</code> |
| <code>interface.15.go-consensus-types-event-validator-set-updates-value-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:25</code> | <code>EventValidatorSetUpdatesValue = "ValidatorSetUpdates"</code> |
| <code>interface.15.go-consensus-types-event-vote-value-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:43</code> | <code>EventVoteValue = "Vote"</code> |
| <code>interface.15.go-consensus-types-evidence-from-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/evidence.go:790</code> | <code>func EvidenceFromProto(evidence *tmproto.Evidence) (Evidence, error)</code> |
| <code>interface.15.go-consensus-types-evidence-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/evidence.go:26</code> | <code>type Evidence interface</code> |
| <code>interface.15.go-consensus-types-evidence-list-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/evidence.go:616</code> | <code>type EvidenceList []Evidence</code> |
| <code>interface.15.go-consensus-types-evidence-params-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/params.go:79</code> | <code>type EvidenceParams struct</code> |
| <code>interface.15.go-consensus-types-evidence-to-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/evidence.go:758</code> | <code>func EvidenceToProto(evidence Evidence) (*tmproto.Evidence, error)</code> |
| <code>interface.15.go-consensus-types-gen-node-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/node_key.go:90</code> | <code>func GenNodeKey() NodeKey</code> |
| <code>interface.15.go-consensus-types-genesis-doc-from-file-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/genesis.go:185</code> | <code>func GenesisDocFromFile(genDocFile string) (*GenesisDoc, error)</code> |
| <code>interface.15.go-consensus-types-genesis-doc-from-json-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/genesis.go:170</code> | <code>func GenesisDocFromJSON(jsonBlob []byte) (*GenesisDoc, error)</code> |
| <code>interface.15.go-consensus-types-genesis-doc-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/genesis.go:71</code> | <code>type GenesisDoc struct</code> |
| <code>interface.15.go-consensus-types-genesis-validator-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/genesis.go:32</code> | <code>type GenesisValidator struct</code> |
| <code>interface.15.go-consensus-types-header-from-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:616</code> | <code>func HeaderFromProto(ph *tmproto.Header) (Header, error)</code> |
| <code>interface.15.go-consensus-types-header-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:398</code> | <code>type Header struct</code> |
| <code>interface.15.go-consensus-types-is-err-not-enough-voting-power-signed-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/validator_set.go:753</code> | <code>func IsErrNotEnoughVotingPowerSigned(err error) bool</code> |
| <code>interface.15.go-consensus-types-is-vote-type-valid-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/signed_msg_type.go:6</code> | <code>func IsVoteTypeValid(t tmproto.SignedMsgType) bool</code> |
| <code>interface.15.go-consensus-types-legacy-event-data-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:107</code> | <code>type LegacyEventData interface</code> |
| <code>interface.15.go-consensus-types-legacy-event-data-new-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:148</code> | <code>type LegacyEventDataNewBlock struct</code> |
| <code>interface.15.go-consensus-types-light-block-from-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/light.go:101</code> | <code>func LightBlockFromProto(pb *tmproto.LightBlock) (*LightBlock, error)</code> |
| <code>interface.15.go-consensus-types-light-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/light.go:29</code> | <code>type LightBlock struct</code> |
| <code>interface.15.go-consensus-types-light-client-attack-evidence-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/evidence.go:301</code> | <code>type LightClientAttackEvidence struct</code> |
| <code>interface.15.go-consensus-types-light-client-info-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/light.go:14</code> | <code>type LightClientInfo struct</code> |
| <code>interface.15.go-consensus-types-load-node-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/node_key.go:95</code> | <code>func LoadNodeKey(filePath string) (NodeKey, error)</code> |
| <code>interface.15.go-consensus-types-load-or-gen-node-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/node_key.go:71</code> | <code>func LoadOrGenNodeKey(filePath string) (NodeKey, error)</code> |
| <code>interface.15.go-consensus-types-make-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:375</code> | <code>func MakeBlock(height int64, txs []Tx, lastCommit *Commit, evidence []Evidence) *Block</code> |
| <code>interface.15.go-consensus-types-make-commit-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/test_util.go:13</code> | <code>func MakeCommit(ctx context.Context, blockID BlockID, height int64, round int32, voteSet *VoteSet, validators []PrivValidator, now time.Time) (*Commit, error)</code> |
| <code>interface.15.go-consensus-types-max-block-parts-count-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/params.go:24</code> | <code>MaxBlockPartsCount = (MaxBlockSizeBytes / BlockPartSizeBytes) + 1</code> |
| <code>interface.15.go-consensus-types-max-block-size-bytes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/params.go:17</code> | <code>MaxBlockSizeBytes = 104857600 // 100MB</code> |
| <code>interface.15.go-consensus-types-max-chain-id-len-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/genesis.go:22</code> | <code>MaxChainIDLen = 50</code> |
| <code>interface.15.go-consensus-types-max-commit-bytes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:678</code> | <code>func MaxCommitBytes(valCount int) int64</code> |
| <code>interface.15.go-consensus-types-max-data-bytes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:332</code> | <code>func MaxDataBytes(maxBytes, evidenceBytes int64, valsCount int) int64</code> |
| <code>interface.15.go-consensus-types-max-data-bytes-no-evidence-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:355</code> | <code>func MaxDataBytesNoEvidence(maxBytes int64, valsCount int) int64</code> |
| <code>interface.15.go-consensus-types-max-header-bytes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:37</code> | <code>MaxHeaderBytes int64 = 626</code> |
| <code>interface.15.go-consensus-types-max-node-info-size-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/node_info.go:21</code> | <code>func MaxNodeInfoSize() int</code> |
| <code>interface.15.go-consensus-types-max-overhead-for-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:47</code> | <code>MaxOverheadForBlock int64 = 11</code> |
| <code>interface.15.go-consensus-types-max-total-voting-power-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/validator_set.go:29</code> | <code>MaxTotalVotingPower = int64(math.MaxInt64) / 8</code> |
| <code>interface.15.go-consensus-types-max-votes-count-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/vote_set.go:18</code> | <code>MaxVotesCount = 10000</code> |
| <code>interface.15.go-consensus-types-mock-pv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/priv_validator.go:63</code> | <code>type MockPV struct</code> |
| <code>interface.15.go-consensus-types-new-block-meta-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block_meta.go:20</code> | <code>func NewBlockMeta(block *Block, blockParts *PartSet) *BlockMeta</code> |
| <code>interface.15.go-consensus-types-new-commit-sig-absent-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:686</code> | <code>func NewCommitSigAbsent() CommitSig</code> |
| <code>interface.15.go-consensus-types-new-duplicate-vote-evidence-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/evidence.go:100</code> | <code>func NewDuplicateVoteEvidence(vote1, vote2 *Vote, blockTime time.Time, valSet *ValidatorSet, ) (*DuplicateVoteEvidence, error)</code> |
| <code>interface.15.go-consensus-types-new-encoded-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/evidence.go:64</code> | <code>func NewEncodedVote(v *Vote) *encodedVote</code> |
| <code>interface.15.go-consensus-types-new-err-evidence-overflow-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/evidence.go:835</code> | <code>func NewErrEvidenceOverflow(max, got int64) ErrEvidenceOverflow</code> |
| <code>interface.15.go-consensus-types-new-err-invalid-evidence-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/evidence.go:819</code> | <code>func NewErrInvalidEvidence(ev Evidence, err error) *ErrInvalidEvidence</code> |
| <code>interface.15.go-consensus-types-new-erroring-mock-pv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/priv_validator.go:155</code> | <code>func NewErroringMockPV() *ErroringMockPV</code> |
| <code>interface.15.go-consensus-types-new-mock-duplicate-vote-evidence-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/evidence.go:849</code> | <code>func NewMockDuplicateVoteEvidence(ctx context.Context, height int64, time time.Time, chainID string) (*DuplicateVoteEvidence, error)</code> |
| <code>interface.15.go-consensus-types-new-mock-duplicate-vote-evidence-with-validator-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/evidence.go:855</code> | <code>func NewMockDuplicateVoteEvidenceWithValidator(ctx context.Context, height int64, time time.Time, pv PrivValidator, chainID string) (*DuplicateVoteEvidence, error)</code> |
| <code>interface.15.go-consensus-types-new-mock-pv-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/priv_validator.go:69</code> | <code>func NewMockPV() MockPV</code> |
| <code>interface.15.go-consensus-types-new-mock-pv-with-params-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/priv_validator.go:76</code> | <code>func NewMockPVWithParams(privKey crypto.PrivKey, breakProposalSigning, breakVoteSigning bool) MockPV</code> |
| <code>interface.15.go-consensus-types-new-node-id-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/node_id.go:26</code> | <code>func NewNodeID(nodeID string) (NodeID, error)</code> |
| <code>interface.15.go-consensus-types-new-part-set-from-data-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/part_set.go:176</code> | <code>func NewPartSetFromData(data []byte, partSize uint32) *PartSet</code> |
| <code>interface.15.go-consensus-types-new-part-set-from-header-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/part_set.go:207</code> | <code>func NewPartSetFromHeader(header PartSetHeader) *PartSet</code> |
| <code>interface.15.go-consensus-types-new-proposal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/proposal.go:44</code> | <code>func NewProposal(height int64, round int32, polRound int32, blockID BlockID, ts time.Time, txHashes []TxHash, header Header, lastCommit *Commit, evidenceList EvidenceList, proposerAddress Address) *Proposal</code> |
| <code>interface.15.go-consensus-types-new-results-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/results.go:13</code> | <code>func NewResults(responses []*abci.ResponseDeliverTx) ABCIResults</code> |
| <code>interface.15.go-consensus-types-new-validator-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/validator.go:63</code> | <code>func NewValidator(pubKey crypto.PubKey, votingPower int64) *Validator</code> |
| <code>interface.15.go-consensus-types-new-validator-set-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/validator_set.go:82</code> | <code>func NewValidatorSet(valz []*Validator) *ValidatorSet</code> |
| <code>interface.15.go-consensus-types-new-vote-set-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/vote_set.go:73</code> | <code>func NewVoteSet(chainID string, height int64, round int32, signedMsgType tmproto.SignedMsgType, valSet *ValidatorSet) *VoteSet</code> |
| <code>interface.15.go-consensus-types-node-id-byte-length-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/node_id.go:15</code> | <code>const NodeIDByteLength = crypto.AddressSize</code> |
| <code>interface.15.go-consensus-types-node-id-from-pub-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/node_id.go:38</code> | <code>func NodeIDFromPubKey(pubKey crypto.PubKey) NodeID</code> |
| <code>interface.15.go-consensus-types-node-id-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/node_id.go:22</code> | <code>type NodeID string</code> |
| <code>interface.15.go-consensus-types-node-info-from-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/node_info.go:179</code> | <code>func NodeInfoFromProto(pb *tmp2p.NodeInfo) (NodeInfo, error)</code> |
| <code>interface.15.go-consensus-types-node-info-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/node_info.go:36</code> | <code>type NodeInfo struct</code> |
| <code>interface.15.go-consensus-types-node-info-other-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/node_info.go:57</code> | <code>type NodeInfoOther struct</code> |
| <code>interface.15.go-consensus-types-node-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/node_key.go:21</code> | <code>type NodeKey crypto.PrivKey</code> |
| <code>interface.15.go-consensus-types-part-from-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/part_set.go:75</code> | <code>func PartFromProto(pb *tmproto.Part) (*Part, error)</code> |
| <code>interface.15.go-consensus-types-part-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/part_set.go:26</code> | <code>type Part struct</code> |
| <code>interface.15.go-consensus-types-part-set-header-from-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/part_set.go:141</code> | <code>func PartSetHeaderFromProto(ppsh *tmproto.PartSetHeader) (*PartSetHeader, error)</code> |
| <code>interface.15.go-consensus-types-part-set-header-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/part_set.go:94</code> | <code>type PartSetHeader struct</code> |
| <code>interface.15.go-consensus-types-part-set-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/part_set.go:160</code> | <code>type PartSet struct</code> |
| <code>interface.15.go-consensus-types-pb2-tm-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/protobuf.go:42</code> | <code>var PB2TM = pb2tm{}</code> |
| <code>interface.15.go-consensus-types-priv-validator-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/priv_validator.go:28</code> | <code>type PrivValidator interface</code> |
| <code>interface.15.go-consensus-types-priv-validators-by-address-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/priv_validator.go:35</code> | <code>type PrivValidatorsByAddress []PrivValidator</code> |
| <code>interface.15.go-consensus-types-proposal-from-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/proposal.go:197</code> | <code>func ProposalFromProto(pp *tmproto.Proposal) (*Proposal, error)</code> |
| <code>interface.15.go-consensus-types-proposal-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/proposal.go:27</code> | <code>type Proposal struct</code> |
| <code>interface.15.go-consensus-types-proposal-sign-bytes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/proposal.go:154</code> | <code>func ProposalSignBytes(chainID string, p *tmproto.Proposal) []byte</code> |
| <code>interface.15.go-consensus-types-protocol-version-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/node_info.go:26</code> | <code>type ProtocolVersion struct</code> |
| <code>interface.15.go-consensus-types-query-for-event-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:494</code> | <code>func QueryForEvent(eventValue string) *tmquery.Query</code> |
| <code>interface.15.go-consensus-types-rand-validator-set-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/validator_set.go:994</code> | <code>func RandValidatorSet(numValidators int, votingPower int64) (*ValidatorSet, []PrivValidator)</code> |
| <code>interface.15.go-consensus-types-resolve-address-string-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/node_info.go:207</code> | <code>func ResolveAddressString(addr string) (netip.AddrPort, error)</code> |
| <code>interface.15.go-consensus-types-sealed-block-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/sequencer.go:20</code> | <code>type SealedBlock struct</code> |
| <code>interface.15.go-consensus-types-sequencer-source-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/sequencer.go:10</code> | <code>type SequencerSource interface</code> |
| <code>interface.15.go-consensus-types-signed-header-from-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/light.go:201</code> | <code>func SignedHeaderFromProto(shp *tmproto.SignedHeader) (*SignedHeader, error)</code> |
| <code>interface.15.go-consensus-types-signed-header-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/light.go:127</code> | <code>type SignedHeader struct</code> |
| <code>interface.15.go-consensus-types-skip-last-results-hash-validation-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/block.go:31</code> | <code>var SkipLastResultsHashValidation atomic.Bool</code> |
| <code>interface.15.go-consensus-types-synchrony-params-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/params.go:99</code> | <code>type SynchronyParams struct</code> |
| <code>interface.15.go-consensus-types-time-format-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/canonical.go:13</code> | <code>const TimeFormat = time.RFC3339Nano</code> |
| <code>interface.15.go-consensus-types-timeout-params-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/params.go:105</code> | <code>type TimeoutParams struct</code> |
| <code>interface.15.go-consensus-types-tm2-pb-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/protobuf.go:12</code> | <code>var TM2PB = tm2pb{}</code> |
| <code>interface.15.go-consensus-types-tx-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/mempool.go:14</code> | <code>type TxHash crypto.Hash</code> |
| <code>interface.15.go-consensus-types-tx-hash-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:459</code> | <code>TxHashKey = "tx.hash"</code> |
| <code>interface.15.go-consensus-types-tx-height-key-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:463</code> | <code>TxHeightKey = "tx.height"</code> |
| <code>interface.15.go-consensus-types-tx-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/tx.go:17</code> | <code>type Tx []byte</code> |
| <code>interface.15.go-consensus-types-tx-proof-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/tx.go:93</code> | <code>type TxProof struct</code> |
| <code>interface.15.go-consensus-types-txs-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/tx.go:26</code> | <code>type Txs []Tx</code> |
| <code>interface.15.go-consensus-types-validate-hash-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/validation.go:166</code> | <code>func ValidateHash(h []byte) error</code> |
| <code>interface.15.go-consensus-types-validator-info-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/events.go:344</code> | <code>type ValidatorInfo struct</code> |
| <code>interface.15.go-consensus-types-validator-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/validator.go:20</code> | <code>type Validator struct</code> |
| <code>interface.15.go-consensus-types-validator-list-string-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/validator.go:140</code> | <code>func ValidatorListString(vals []*Validator) string</code> |
| <code>interface.15.go-consensus-types-validator-params-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/params.go:87</code> | <code>type ValidatorParams struct</code> |
| <code>interface.15.go-consensus-types-validator-set-from-existing-validators-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/validator_set.go:904</code> | <code>func ValidatorSetFromExistingValidators(valz []*Validator) (*ValidatorSet, error)</code> |
| <code>interface.15.go-consensus-types-validator-set-from-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/validator_set.go:867</code> | <code>func ValidatorSetFromProto(vp *tmproto.ValidatorSet) (*ValidatorSet, error)</code> |
| <code>interface.15.go-consensus-types-validator-set-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/validator_set.go:58</code> | <code>type ValidatorSet struct</code> |
| <code>interface.15.go-consensus-types-validators-by-voting-power-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/validator_set.go:805</code> | <code>type ValidatorsByVotingPower []*Validator</code> |
| <code>interface.15.go-consensus-types-verify-commit-light-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/validation.go:62</code> | <code>func VerifyCommitLight(chainID string, vals *ValidatorSet, blockID BlockID, height int64, commit *Commit) error</code> |
| <code>interface.15.go-consensus-types-vote-from-proto-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/vote.go:66</code> | <code>func VoteFromProto(pv *tmproto.Vote) (*Vote, error)</code> |
| <code>interface.15.go-consensus-types-vote-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/vote.go:51</code> | <code>type Vote struct</code> |
| <code>interface.15.go-consensus-types-vote-set-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/vote_set.go:55</code> | <code>type VoteSet struct</code> |
| <code>interface.15.go-consensus-types-vote-set-reader-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/vote_set.go:684</code> | <code>type VoteSetReader interface</code> |
| <code>interface.15.go-consensus-types-vote-sign-bytes-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/types/vote.go:123</code> | <code>func VoteSignBytes(chainID string, vote *tmproto.Vote) []byte</code> |
| <code>interface.15.go-consensus-utils-map-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/utils/slice.go:3</code> | <code>func Map[I any, O any](input []I, lambda func(i I) O) []O</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.15.go-consensus-autobahn-types-app-hash-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-app-proposal-j57</code> | notes | callers: cross-package references in importing files; 5 exported methods |
| <code>interface.15.go-consensus-autobahn-types-app-qc-conv-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-app-qc-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-autobahn-types-app-vote-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-autobahn-types-block-conv-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-block-header-hash-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-autobahn-types-block-header-j57</code> | notes | callers: cross-package references in importing files; 7 exported methods |
| <code>interface.15.go-consensus-autobahn-types-block-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-autobahn-types-block-number-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-commit-qc-conv-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-commit-qc-j57</code> | notes | 19 referencing files; first 12 listed; 5 exported methods |
| <code>interface.15.go-consensus-autobahn-types-commit-vote-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-autobahn-types-committee-j57</code> | notes | 17 referencing files; first 12 listed; 16 exported methods |
| <code>interface.15.go-consensus-autobahn-types-consensus-req-commit-vote-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-autobahn-types-consensus-req-conv-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-consensus-req-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-consensus-req-prepare-vote-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-autobahn-types-full-commit-qc-conv-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-full-commit-qc-j57</code> | notes | callers: cross-package references in importing files; 4 exported methods |
| <code>interface.15.go-consensus-autobahn-types-full-proposal-j57</code> | notes | callers: cross-package references in importing files; 5 exported methods |
| <code>interface.15.go-consensus-autobahn-types-full-timeout-vote-conv-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-full-timeout-vote-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-autobahn-types-gen-app-hash-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-gen-app-qc-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-gen-app-vote-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-gen-block-header-hash-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-gen-block-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-gen-block-number-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-gen-commit-qc-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-gen-commit-vote-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-gen-committee-j57</code> | notes | 16 referencing files; first 12 listed |
| <code>interface.15.go-consensus-autobahn-types-gen-full-timeout-vote-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-gen-global-block-number-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-gen-lane-proposal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-gen-lane-vote-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-gen-payload-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-gen-prepare-qc-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-gen-prepare-vote-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-gen-proposal-for-at-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-gen-secret-key-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-gen-signed-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-gen-timeout-qc-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-global-block-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-autobahn-types-global-block-number-j57</code> | notes | 19 referencing files; first 12 listed |
| <code>interface.15.go-consensus-autobahn-types-global-range-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-autobahn-types-global-range-opt-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-hash-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-lane-id-j57</code> | notes | 13 referencing files; first 12 listed |
| <code>interface.15.go-consensus-autobahn-types-lane-proposal-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-autobahn-types-lane-qc-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-autobahn-types-lane-range-j57</code> | notes | callers: cross-package references in importing files; 6 exported methods |
| <code>interface.15.go-consensus-autobahn-types-lane-range-opt-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-lane-vote-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-autobahn-types-max-block-proto-size-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-max-txs-bytes-per-block-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-max-txs-per-block-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-new-app-proposal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-new-app-qc-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-new-app-vote-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-new-block-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-new-commit-qc-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-new-commit-vote-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-new-full-commit-qc-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-new-full-timeout-vote-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-new-lane-proposal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-new-lane-qc-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-new-lane-vote-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-new-prepare-qc-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-new-prepare-vote-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-new-proposal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-new-reproposal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-new-round-robin-election-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-new-timeout-qc-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-next-index-opt-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-next-opt-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-next-view-opt-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-payload-builder-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-autobahn-types-payload-hash-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-payload-j57</code> | notes | callers: cross-package references in importing files; 6 exported methods |
| <code>interface.15.go-consensus-autobahn-types-prepare-qc-conv-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-prepare-qc-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-autobahn-types-prepare-vote-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-autobahn-types-proposal-j57</code> | notes | callers: cross-package references in importing files; 9 exported methods |
| <code>interface.15.go-consensus-autobahn-types-public-key-from-bytes-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-public-key-j57</code> | notes | 14 referencing files; first 12 listed; 6 exported methods |
| <code>interface.15.go-consensus-autobahn-types-road-index-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-secret-key-from-ed25519-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-secret-key-j57</code> | notes | 16 referencing files; first 12 listed; 4 exported methods |
| <code>interface.15.go-consensus-autobahn-types-sign-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-signed-j57</code> | notes | 19 referencing files; first 12 listed; 6 exported methods |
| <code>interface.15.go-consensus-autobahn-types-signed-msg-conv-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-test-keys-with-weight-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-timeout-qc-conv-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-autobahn-types-timeout-qc-j57</code> | notes | callers: cross-package references in importing files; 4 exported methods |
| <code>interface.15.go-consensus-autobahn-types-view-j57</code> | notes | 14 referencing files; first 12 listed; 2 exported methods |
| <code>interface.15.go-consensus-autobahn-types-view-spec-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-cmd-tendermint-commands-add-node-flags-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-cmd-tendermint-commands-debug-get-debug-command-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-cmd-tendermint-commands-gen-node-key-cmd-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-cmd-tendermint-commands-load-tendermint-state-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-cmd-tendermint-commands-make-gen-autobahn-config-command-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-cmd-tendermint-commands-make-gen-validator-command-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-cmd-tendermint-commands-make-inspect-command-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-cmd-tendermint-commands-make-key-migrate-command-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-cmd-tendermint-commands-make-light-command-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-cmd-tendermint-commands-make-reindex-event-command-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-cmd-tendermint-commands-make-reset-command-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-cmd-tendermint-commands-make-unsafe-reset-all-command-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-cmd-tendermint-commands-new-completion-cmd-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-cmd-tendermint-commands-parse-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-cmd-tendermint-commands-rollback-state-to-target-height-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-address-hash-j57</code> | notes | 13 referencing files; first 12 listed |
| <code>interface.15.go-consensus-crypto-address-j57</code> | notes | 14 referencing files; first 12 listed |
| <code>interface.15.go-consensus-crypto-address-size-j57</code> | notes | 18 referencing files; first 12 listed; const in grouped declaration |
| <code>interface.15.go-consensus-crypto-c-rand-bytes-j57</code> | notes | 23 referencing files; first 12 listed |
| <code>interface.15.go-consensus-crypto-checksum-j57</code> | notes | 15 referencing files; first 12 listed |
| <code>interface.15.go-consensus-crypto-ed25519-batch-verifier-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-crypto-ed25519-err-bad-sig-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-crypto-ed25519-generate-secret-key-j57</code> | notes | 30 referencing files; first 12 listed |
| <code>interface.15.go-consensus-crypto-ed25519-key-type-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-ed25519-new-batch-verifier-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-ed25519-new-tag-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-ed25519-public-key-from-bytes-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-ed25519-public-key-from-string-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-ed25519-public-key-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-crypto-ed25519-public-key-name-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-ed25519-secret-key-from-secret-bytes-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-ed25519-secret-key-j57</code> | notes | callers: cross-package references in importing files; 4 exported methods |
| <code>interface.15.go-consensus-crypto-ed25519-signature-from-bytes-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-ed25519-signature-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-ed25519-test-secret-key-j57</code> | notes | 27 referencing files; first 12 listed |
| <code>interface.15.go-consensus-crypto-err-bad-sig-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-hash-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-crypto-hash-size-j57</code> | notes | 26 referencing files; first 12 listed; const in grouped declaration |
| <code>interface.15.go-consensus-crypto-merkle-default-proof-runtime-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-merkle-hash-from-byte-slices-j57</code> | notes | 19 referencing files; first 12 listed |
| <code>interface.15.go-consensus-crypto-merkle-key-encoding-url-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-crypto-merkle-key-path-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-crypto-merkle-new-proof-runtime-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-merkle-op-decoder-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-merkle-proof-from-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-merkle-proof-j57</code> | notes | callers: cross-package references in importing files; 6 exported methods |
| <code>interface.15.go-consensus-crypto-merkle-proof-operator-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-merkle-proof-runtime-j57</code> | notes | callers: cross-package references in importing files; 6 exported methods |
| <code>interface.15.go-consensus-crypto-merkle-proofs-from-byte-slices-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-new-batch-verifier-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-priv-key-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-pub-key-from-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-pub-key-j57</code> | notes | 36 referencing files; first 12 listed |
| <code>interface.15.go-consensus-crypto-pub-key-to-proto-j57</code> | notes | 18 referencing files; first 12 listed |
| <code>interface.15.go-consensus-crypto-sig-from-bytes-j57</code> | notes | 27 referencing files; first 12 listed |
| <code>interface.15.go-consensus-crypto-sig-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-crypto-tmhash-size-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-crypto-tmhash-sum-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-export-new-block-store-j57</code> | notes | no in-repo importer of package export at HEAD; exported for consumers outside this repository |
| <code>interface.15.go-consensus-export-new-query-j57</code> | notes | no in-repo importer of package export at HEAD; exported for consumers outside this repository |
| <code>interface.15.go-consensus-export-new-store-j57</code> | notes | no in-repo importer of package export at HEAD; exported for consumers outside this repository |
| <code>interface.15.go-consensus-export-query-all-j57</code> | notes | no in-repo importer of package export at HEAD; exported for consumers outside this repository |
| <code>interface.15.go-consensus-export-query-j57</code> | notes | no in-repo importer of package export at HEAD; exported for consumers outside this repository |
| <code>interface.15.go-consensus-internal-autobahn-avail-blocks-per-lane-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-avail-new-state-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-avail-run-test-network-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-avail-state-j57</code> | notes | callers: cross-package references in importing files; 18 exported methods |
| <code>interface.15.go-consensus-internal-autobahn-consensus-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-consensus-new-state-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-block-persister-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-commit-qc-persister-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-full-commit-qc-persister-j57</code> | notes | callers: cross-package references in importing files; 6 exported methods |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-global-block-persister-j57</code> | notes | callers: cross-package references in importing files; 7 exported methods |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-loaded-block-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-loaded-commit-qc-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-loaded-global-block-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-new-block-persister-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-new-commit-qc-persister-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-new-full-commit-qc-persister-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-new-global-block-persister-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-new-persister-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-consensus-persist-persister-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-autobahn-consensus-run-test-network-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-consensus-state-j57</code> | notes | callers: cross-package references in importing files; 13 exported methods |
| <code>interface.15.go-consensus-internal-autobahn-data-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-data-err-not-found-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-data-err-pruned-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-data-new-data-wal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-data-new-state-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-data-state-j57</code> | notes | callers: cross-package references in importing files; 14 exported methods |
| <code>interface.15.go-consensus-internal-autobahn-data-test-commit-qc-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-pb-schema-for-app-qc-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-pb-schema-for-block-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-pb-schema-for-commit-qc-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-pb-schema-for-signed-msg-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-producer-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-producer-new-state-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-autobahn-producer-state-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-blocksync-block-sync-channel-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-blocksync-consensus-reactor-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-blocksync-new-reactor-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-blocksync-reactor-j57</code> | notes | callers: cross-package references in importing files; 6 exported methods |
| <code>interface.15.go-consensus-internal-blocksync-syncer-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-consensus-data-channel-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-consensus-metrics-j57</code> | notes | callers: cross-package references in importing files; 16 exported methods |
| <code>interface.15.go-consensus-internal-consensus-new-handshaker-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-consensus-new-reactor-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-consensus-new-state-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-consensus-nop-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-consensus-open-wal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-consensus-prometheus-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-consensus-reactor-j57</code> | notes | callers: cross-package references in importing files; 8 exported methods |
| <code>interface.15.go-consensus-internal-consensus-state-channel-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-consensus-types-hrs-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-consensus-types-leader-pin-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-consensus-types-new-height-vote-set-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-consensus-types-new-leader-pin-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-consensus-types-new-safe-round-state-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-consensus-types-peer-round-state-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-consensus-types-round-state-j57</code> | notes | callers: cross-package references in importing files; 8 exported methods |
| <code>interface.15.go-consensus-internal-consensus-types-round-step-commit-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-consensus-types-round-step-new-height-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-consensus-types-round-step-new-round-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-consensus-types-round-step-precommit-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-consensus-types-round-step-precommit-wait-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-consensus-types-round-step-prevote-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-consensus-types-round-step-prevote-wait-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-consensus-types-round-step-propose-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-consensus-types-round-step-type-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-internal-consensus-types-safe-round-state-j57</code> | notes | callers: cross-package references in importing files; 49 exported methods |
| <code>interface.15.go-consensus-internal-consensus-vote-channel-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-consensus-vote-set-bits-channel-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-eventbus-event-bus-j57</code> | notes | 19 referencing files; first 12 listed; 27 exported methods |
| <code>interface.15.go-consensus-internal-eventbus-new-default-j57</code> | notes | 16 referencing files; first 12 listed |
| <code>interface.15.go-consensus-internal-eventbus-subscription-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-eventlog-cursor-cursor-j57</code> | notes | callers: cross-package references in importing files; 6 exported methods |
| <code>interface.15.go-consensus-internal-eventlog-cursor-source-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-eventlog-err-log-pruned-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-eventlog-err-stop-scan-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-eventlog-find-type-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-eventlog-info-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-eventlog-item-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-eventlog-log-j57</code> | notes | callers: cross-package references in importing files; 4 exported methods |
| <code>interface.15.go-consensus-internal-eventlog-log-settings-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-eventlog-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-eventlog-new-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-eventlog-prometheus-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-evidence-evidence-channel-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-evidence-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-evidence-new-pool-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-evidence-new-reactor-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-evidence-nop-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-evidence-pool-j57</code> | notes | callers: cross-package references in importing files; 11 exported methods |
| <code>interface.15.go-consensus-internal-evidence-prometheus-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-evidence-reactor-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-internal-evidence-verify-duplicate-vote-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-evidence-verify-light-client-attack-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-hashable-gen-hash-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-hashable-hash-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-hashable-parse-hash-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-hashable-to-hash-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-inspect-new-from-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-inspect-new-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-inspect-rpc-handler-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-inspect-rpc-routes-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-inspect-rpc-server-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-internal-jsontypes-marshal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-jsontypes-must-register-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-jsontypes-tagged-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-jsontypes-unmarshal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-libs-async-parallel-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-libs-clist-c-element-j57</code> | notes | callers: cross-package references in importing files; 5 exported methods |
| <code>interface.15.go-consensus-internal-libs-clist-c-list-j57</code> | notes | callers: cross-package references in importing files; 7 exported methods |
| <code>interface.15.go-consensus-internal-libs-clist-err-removed-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-libs-clist-new-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-libs-flowrate-monitor-j57</code> | notes | callers: cross-package references in importing files; 8 exported methods |
| <code>interface.15.go-consensus-internal-libs-flowrate-new-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-libs-progressbar-bar-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-internal-libs-protoio-marshal-delimited-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-libs-protoio-new-delimited-reader-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-libs-protoio-new-delimited-writer-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-libs-protoio-read-closer-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-libs-protoio-unmarshal-delimited-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-libs-protoio-write-closer-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-libs-queue-err-queue-closed-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-internal-libs-queue-new-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-libs-queue-options-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-libs-queue-queue-j57</code> | notes | callers: cross-package references in importing files; 4 exported methods |
| <code>interface.15.go-consensus-internal-libs-reservoir-new-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-libs-reservoir-sampler-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-internal-libs-sync-new-waker-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-libs-tempfile-write-file-atomic-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-libs-test-mutate-byte-slice-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-libs-timer-new-throttle-timer-j57</code> | notes | no in-repo importer of package timer at HEAD; exported for consumers outside this repository |
| <code>interface.15.go-consensus-internal-libs-timer-throttle-timer-j57</code> | notes | no in-repo importer of package timer at HEAD; exported for consumers outside this repository; 2 exported methods |
| <code>interface.15.go-consensus-internal-libs-wal-default-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-libs-wal-log-j57</code> | notes | callers: cross-package references in importing files; 6 exported methods |
| <code>interface.15.go-consensus-internal-libs-wal-open-log-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-mempool-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-mempool-default-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-mempool-err-tx-in-cache-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-mempool-err-tx-too-large-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-mempool-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-mempool-new-tx-mempool-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-mempool-nop-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-mempool-nop-tx-constraints-fetcher-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-mempool-nop-tx-constraints-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-mempool-prometheus-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-mempool-reactor-mempool-channel-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-mempool-reactor-new-reactor-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-mempool-reap-limits-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-mempool-test-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-mempool-tx-constraints-fetcher-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-mempool-tx-constraints-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-mempool-tx-mempool-j57</code> | notes | callers: cross-package references in importing files; 23 exported methods |
| <code>interface.15.go-consensus-internal-p2p-channel-descriptor-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-channel-id-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-channel-j57</code> | notes | 14 referencing files; first 12 listed; 5 exported methods |
| <code>interface.15.go-consensus-internal-p2p-conn-challenge-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-conn-channel-descriptor-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-conn-channel-descriptor-t-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-p2p-conn-channel-id-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-conn-conn-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-conn-default-m-conn-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-conn-m-conn-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-conn-m-connection-j57</code> | notes | callers: cross-package references in importing files; 7 exported methods |
| <code>interface.15.go-consensus-internal-p2p-conn-make-secret-connection-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-conn-new-m-connection-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-conn-new-test-conn-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-conn-read-sized-msg-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-conn-secret-connection-j57</code> | notes | callers: cross-package references in importing files; 7 exported methods |
| <code>interface.15.go-consensus-internal-p2p-conn-write-sized-msg-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-endpoint-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-internal-p2p-giga-api-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-giga-new-pool-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-giga-new-service-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-giga-node-addr-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-p2p-giga-pool-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-internal-p2p-giga-router-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-giga-router-j57</code> | notes | callers: cross-package references in importing files; 9 exported methods |
| <code>interface.15.go-consensus-internal-p2p-giga-service-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-internal-p2p-make-test-network-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-max-pex-addrs-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-mux-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-mux-mux-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-internal-p2p-mux-new-mux-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-mux-stream-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-internal-p2p-mux-stream-kind-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-mux-stream-kind-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-new-router-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-node-address-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-internal-p2p-node-in-slice-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-node-public-key-j57</code> | notes | callers: cross-package references in importing files; 6 exported methods |
| <code>interface.15.go-consensus-internal-p2p-node-secret-key-j57</code> | notes | callers: cross-package references in importing files; 4 exported methods |
| <code>interface.15.go-consensus-internal-p2p-nop-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-open-channel-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-parse-node-address-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-peer-status-down-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-p2p-peer-status-up-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-p2p-peer-update-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-pex-default-send-interval-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-p2p-pex-new-reactor-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-pex-pex-channel-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-p2p-prometheus-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-recv-msg-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-resolve-endpoint-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-router-j57</code> | notes | callers: cross-package references in importing files; 14 exported methods |
| <code>interface.15.go-consensus-internal-p2p-router-options-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-p2p-rpc-client-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-p2p-rpc-in-bytes-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-rpc-limit-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-rpc-msg-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-rpc-new-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-rpc-new-server-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-rpc-register-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-rpc-server-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-p2p-rpc-stream-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-internal-p2p-test-make-channel-no-cleanup-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-test-network-j57</code> | notes | callers: cross-package references in importing files; 9 exported methods |
| <code>interface.15.go-consensus-internal-p2p-test-network-options-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-p2p-test-node-j57</code> | notes | callers: cross-package references in importing files; 7 exported methods |
| <code>interface.15.go-consensus-internal-p2p-test-node-options-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-protoutils-conv-j57</code> | notes | 22 referencing files; first 12 listed; 7 exported methods |
| <code>interface.15.go-consensus-internal-protoutils-marshal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-protoutils-message-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-protoutils-size-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-protoutils-unmarshal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-must-field-num-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-number-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-rule-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-scan-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-scanner-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-schema-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-wgtest-commit-with-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-wgtest-evidence-list-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-wgtest-evidence-with-commit-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-wgtest-marshal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-protoutils-wireguard-wgtest-max-commit-signatures-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-proxy-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-proxy-new-j57</code> | notes | 23 referencing files; first 12 listed |
| <code>interface.15.go-consensus-internal-proxy-nop-metrics-j57</code> | notes | 23 referencing files; first 12 listed |
| <code>interface.15.go-consensus-internal-proxy-prometheus-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-proxy-proxy-j57</code> | notes | 20 referencing files; first 12 listed; 15 exported methods |
| <code>interface.15.go-consensus-internal-pubsub-buffer-capacity-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-pubsub-err-already-subscribed-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-err-subscription-not-found-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-err-terminated-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-err-unsubscribed-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-message-j57</code> | notes | callers: cross-package references in importing files; 4 exported methods |
| <code>interface.15.go-consensus-internal-pubsub-new-server-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-pubsub-query-all-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-pubsub-query-must-compile-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-pubsub-query-new-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-pubsub-query-query-j57</code> | notes | 20 referencing files; first 12 listed; 3 exported methods |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-condition-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-date-format-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-new-scanner-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-parse-date-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-parse-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-parse-time-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-query-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-and-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-contains-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-date-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-eq-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-exists-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-geq-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-gt-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-leq-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-lt-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-matches-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-number-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-string-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-tag-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-t-time-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-time-format-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-pubsub-query-syntax-token-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-pubsub-server-j57</code> | notes | callers: cross-package references in importing files; 12 exported methods |
| <code>interface.15.go-consensus-internal-pubsub-subscribe-args-j57</code> | notes | 14 referencing files; first 12 listed |
| <code>interface.15.go-consensus-internal-pubsub-subscription-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-internal-pubsub-unsubscribe-args-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-rpc-core-consensus-state-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-rpc-core-environment-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-internal-rpc-core-new-routes-map-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-rpc-core-routes-map-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-block-executor-j57</code> | notes | callers: cross-package references in importing files; 10 exported methods |
| <code>interface.15.go-consensus-internal-state-block-store-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-empty-evidence-pool-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-err-app-block-height-too-high-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-state-err-app-block-height-too-low-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-state-err-no-finalize-block-responses-for-height-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-state-err-no-val-set-for-height-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-internal-state-err-open-time-missing-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-internal-state-err-sealed-block-too-large-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-internal-state-err-sequence-batch-missing-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-internal-state-evidence-pool-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-exec-commit-block-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-fire-events-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-from-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-batch-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-internal-state-indexer-block-indexer-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-block-kv-blocker-indexer-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-internal-state-indexer-block-kv-new-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-block-null-blocker-indexer-j57</code> | notes | no in-repo importer of package null at HEAD; exported for consumers outside this repository; 3 exported methods |
| <code>interface.15.go-consensus-internal-state-indexer-error-empty-hash-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-event-sink-j57</code> | notes | 16 referencing files; first 12 listed |
| <code>interface.15.go-consensus-internal-state-indexer-event-sink-type-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-indexing-enabled-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-kv-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-state-indexer-kv-sink-enabled-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-look-for-ranges-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-new-batch-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-new-service-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-nop-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-null-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-state-indexer-prometheus-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-psql-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-state-indexer-query-range-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-internal-state-indexer-service-args-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-service-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-internal-state-indexer-sink-event-sinks-from-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-sink-kv-new-event-sink-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-sink-null-new-event-sink-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-sink-psql-new-event-sink-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-tx-indexer-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-tx-kv-key-from-height-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-tx-kv-new-tx-index-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-indexer-tx-kv-tx-index-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-internal-state-indexer-tx-null-tx-index-j57</code> | notes | no in-repo importer of package null at HEAD; exported for consumers outside this repository; 4 exported methods |
| <code>interface.15.go-consensus-internal-state-make-genesis-doc-from-file-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-make-genesis-state-from-file-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-make-genesis-state-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-new-block-executor-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-new-store-j57</code> | notes | 21 referencing files; first 12 listed |
| <code>interface.15.go-consensus-internal-state-nop-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-prometheus-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-rollback-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-sealed-block-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-state-j57</code> | notes | 32 referencing files; first 12 listed; 7 exported methods |
| <code>interface.15.go-consensus-internal-state-store-j57</code> | notes | 18 referencing files; first 12 listed |
| <code>interface.15.go-consensus-internal-state-test-factory-make-block-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-test-factory-make-blocks-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-tx-constraints-fetcher-from-store-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-tx-constraints-for-state-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-state-version-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-internal-statesync-chunk-channel-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-statesync-light-block-channel-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-statesync-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-statesync-new-reactor-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-statesync-nop-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-statesync-params-channel-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-statesync-prometheus-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-statesync-reactor-j57</code> | notes | callers: cross-package references in importing files; 11 exported methods |
| <code>interface.15.go-consensus-internal-statesync-snapshot-channel-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-store-block-store-j57</code> | notes | callers: cross-package references in importing files; 17 exported methods |
| <code>interface.15.go-consensus-internal-store-new-block-store-j57</code> | notes | 17 referencing files; first 12 listed |
| <code>interface.15.go-consensus-internal-test-factory-consensus-params-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-test-factory-default-test-chain-id-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-internal-test-factory-default-test-time-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-internal-test-factory-genesis-doc-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-test-factory-make-block-id-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-test-factory-make-block-id-with-hash-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-test-factory-make-commit-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-test-factory-make-header-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-test-factory-make-n-txs-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-test-factory-make-vote-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-test-factory-node-id-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-test-factory-random-hash-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-test-factory-validator-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-internal-test-factory-validator-set-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-bits-bit-array-j57</code> | notes | callers: cross-package references in importing files; 16 exported methods |
| <code>interface.15.go-consensus-libs-bits-new-bit-array-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-bytes-fingerprint-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-bytes-hex-bytes-j57</code> | notes | 61 referencing files; first 12 listed; 9 exported methods |
| <code>interface.15.go-consensus-libs-cli-home-flag-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-libs-cli-new-completion-cmd-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-cli-output-flag-j57</code> | notes | 23 referencing files; first 12 listed; const in grouped declaration |
| <code>interface.15.go-consensus-libs-cli-prepare-base-cmd-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-json-marshal-indent-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-json-marshal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-json-unmarshal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-math-fraction-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-libs-math-max-int-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-math-max-int64-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-math-min-int-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-math-min-int64-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-math-parse-fraction-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-math-safe-add-int32-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-math-safe-convert-int32-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-math-safe-sub-int32-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-net-connect-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-net-get-free-port-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-net-protocol-and-address-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-os-copy-file-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-os-ensure-dir-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-os-exit-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-os-file-exists-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-os-write-file-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-rand-bytes-j57</code> | notes | 36 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-rand-new-rand-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-rand-str-from-source-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-rand-str-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-rand-uint32-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-service-base-service-j57</code> | notes | 15 referencing files; first 12 listed; 7 exported methods |
| <code>interface.15.go-consensus-libs-service-new-base-service-j57</code> | notes | 16 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-service-service-j57</code> | notes | 14 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-strings-ascii-trim-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-strings-split-and-trim-empty-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-strings-string-slice-equal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-sync-mutex-2-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-sync-mutex-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-sync-rw-mutex-2-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-sync-rw-mutex-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-time-canonical-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-time-default-source-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-time-now-j57</code> | notes | 23 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-time-source-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-libs-utils-alloc-j57</code> | notes | 17 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-atomic-recv-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-atomic-send-j57</code> | notes | 13 referencing files; first 12 listed; 2 exported methods |
| <code>interface.15.go-consensus-libs-utils-clamp-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-duration-j57</code> | notes | callers: cross-package references in importing files; 4 exported methods |
| <code>interface.15.go-consensus-libs-utils-error-as-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-gen-bool-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-gen-bytes-j57</code> | notes | 13 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-gen-map-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-gen-slice-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-gen-slice-n-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-gen-string-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-gen-timestamp-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-ignore-after-cancel-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-ignore-cancel-j57</code> | notes | 23 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-im-map-j57</code> | notes | callers: cross-package references in importing files; 7 exported methods |
| <code>interface.15.go-consensus-libs-utils-im-new-map-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-map-opt-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-max-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-monitor-watch-updates-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-mutex-j57</code> | notes | 14 referencing files; first 12 listed; 1 exported methods |
| <code>interface.15.go-consensus-libs-utils-new-atomic-send-j57</code> | notes | 16 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-new-mutex-j57</code> | notes | 17 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-new-ring-buf-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-new-rw-mutex-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-new-semaphore-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-new-watch-j57</code> | notes | 15 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-none-j57</code> | notes | 78 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-option-j57</code> | notes | 73 referencing files; first 12 listed; 6 exported methods |
| <code>interface.15.go-consensus-libs-utils-or-panic-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-or-panic1-j57</code> | notes | 54 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-proto-conv-j57</code> | notes | callers: cross-package references in importing files; 5 exported methods |
| <code>interface.15.go-consensus-libs-utils-read-only-j57</code> | notes | 16 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-recv-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-recv-or-closed-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-contains-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-elements-match-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-empty-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-equal-error-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-equal-j57</code> | notes | 49 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-require-error-is-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-error-j57</code> | notes | 32 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-require-eventually-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-fail-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-fail-now-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-false-j57</code> | notes | 18 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-require-file-exists-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-greater-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-greater-or-equal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-len-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-less-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-less-or-equal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-nil-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-no-error-j57</code> | notes | 56 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-require-no-file-exists-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-not-empty-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-not-equal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-not-nil-j57</code> | notes | 13 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-require-panics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-positive-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-require-true-j57</code> | notes | 33 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-require-zero-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-ring-buf-j57</code> | notes | callers: cross-package references in importing files; 8 exported methods |
| <code>interface.15.go-consensus-libs-utils-rng-j57</code> | notes | 19 referencing files; first 12 listed; 8 exported methods |
| <code>interface.15.go-consensus-libs-utils-rw-mutex-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-libs-utils-safe-cast-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-scope-parallel-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-scope-parallel-scope-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-scope-run-j57</code> | notes | 51 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-scope-run1-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-scope-scope-j57</code> | notes | 54 referencing files; first 12 listed; 5 exported methods |
| <code>interface.15.go-consensus-libs-utils-send-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-shuffle-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-sleep-j57</code> | notes | 19 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-sleep-until-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-slice-j57</code> | notes | 21 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-some-j57</code> | notes | 129 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-tcp-conn-j57</code> | notes | callers: cross-package references in importing files; 7 exported methods |
| <code>interface.15.go-consensus-libs-utils-tcp-dial-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-tcp-host-port-j57</code> | notes | callers: cross-package references in importing files; 4 exported methods |
| <code>interface.15.go-consensus-libs-utils-tcp-i-pv4-loopback-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-tcp-listen-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-tcp-parse-host-port-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-tcp-test-pipe-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-tcp-test-reserve-addr-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-tcp-test-reserve-port-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-test-diff-j57</code> | notes | 33 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-test-rng-j57</code> | notes | 36 referencing files; first 12 listed |
| <code>interface.15.go-consensus-libs-utils-watch-j57</code> | notes | 16 referencing files; first 12 listed; 1 exported methods |
| <code>interface.15.go-consensus-libs-utils-with-deadline-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-with-opt-timeout-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-with-opt-timeout1-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-with-timeout-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-with-timeout1-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-libs-utils-zero-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-client-j57</code> | notes | callers: cross-package references in importing files; 14 exported methods |
| <code>interface.15.go-consensus-light-default-trust-level-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-light-err-invalid-header-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-light-err-light-client-attack-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-err-new-val-set-cant-be-trusted-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-light-err-no-witnesses-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-err-proposer-priorities-diverge-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-light-header-expired-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-max-block-lag-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-max-clock-drift-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-new-client-from-trusted-store-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-new-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-new-http-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-option-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-provider-err-bad-light-block-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-light-provider-err-connection-closed-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-light-provider-err-height-too-high-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-light-provider-err-light-block-not-found-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-light-provider-err-no-response-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-light-provider-err-unreliable-provider-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-light-provider-http-new-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-provider-http-new-with-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-provider-provider-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-proxy-new-proxy-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-pruning-size-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-rpc-client-j57</code> | notes | callers: cross-package references in importing files; 41 exported methods |
| <code>interface.15.go-consensus-light-rpc-default-merkle-key-path-fn-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-rpc-key-path-fn-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-rpc-new-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-sequential-verification-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-skipping-verification-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-store-db-new-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-store-err-light-block-not-found-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-light-store-store-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-trust-options-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-light-validate-trust-level-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-verify-adjacent-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-verify-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-light-verify-non-adjacent-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-node-default-metrics-provider-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-node-no-op-metrics-provider-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-node-node-metrics-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-dial-tcp-fn-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-dial-unix-fn-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-file-pv-j57</code> | notes | callers: cross-package references in importing files; 7 exported methods |
| <code>interface.15.go-consensus-privval-file-pv-key-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-privval-gen-file-pv-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-grpc-dial-remote-signer-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-grpc-new-signer-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-grpc-new-signer-server-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-load-file-pv-empty-state-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-load-file-pv-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-load-or-gen-file-pv-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-new-file-pv-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-new-retry-signer-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-new-signer-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-new-signer-dialer-endpoint-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-new-signer-listener-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-new-signer-server-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-retry-signer-client-j57</code> | notes | callers: cross-package references in importing files; 7 exported methods |
| <code>interface.15.go-consensus-privval-signer-dialer-endpoint-conn-retries-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-signer-dialer-endpoint-retry-wait-interval-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-signer-dialer-endpoint-timeout-read-write-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-privval-socket-dialer-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-proto-tendermint-blocksync-schema-for-message-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-proto-tendermint-consensus-schema-for-message-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-proto-tendermint-statesync-schema-for-message-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-proto-tendermint-types-schema-for-block-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-proto-tendermint-types-schema-for-evidence-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-proto-tendermint-types-schema-for-light-block-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-proto-tendermint-types-schema-for-proposal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-abci-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-abci-query-options-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-default-abci-query-options-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-default-wait-strategy-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-events-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-eventstream-err-stop-running-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-eventstream-missed-items-error-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-rpc-client-eventstream-new-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-eventstream-stream-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-rpc-client-eventstream-stream-options-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-history-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-http-http-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-rpc-client-http-new-j57</code> | notes | 14 referencing files; first 12 listed |
| <code>interface.15.go-consensus-rpc-client-http-new-with-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-http-new-with-timeout-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-local-new-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-local-node-service-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-mempool-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-mock-call-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-rpc-client-mock-client-j57</code> | notes | callers: cross-package references in importing files; 21 exported methods |
| <code>interface.15.go-consensus-rpc-client-mock-new-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-mock-new-status-recorder-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-mock-status-mock-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-rpc-client-network-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-remote-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-sign-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-status-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-subscription-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-wait-for-height-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-client-wait-for-one-event-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-application-info-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-err-height-exceeds-chain-head-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-rpc-coretypes-err-height-not-available-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-rpc-coretypes-err-invalid-request-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-rpc-coretypes-err-lag-is-too-high-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-rpc-coretypes-err-page-out-of-range-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-rpc-coretypes-err-zero-or-negative-height-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-rpc-coretypes-err-zero-or-negative-per-page-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-rpc-coretypes-event-filter-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-event-item-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-int64-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-rpc-coretypes-int64-ptr-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-new-result-commit-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-peer-connection-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-peer-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-peer-state-info-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-request-abci-query-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-request-block-by-hash-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-request-block-info-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-request-block-search-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-request-blockchain-info-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-request-broadcast-evidence-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-rpc-coretypes-request-broadcast-tx-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-request-check-tx-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-request-consensus-params-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-request-events-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-request-genesis-chunked-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-request-subscribe-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-request-tx-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-request-tx-search-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-request-unconfirmed-txs-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-request-unsubscribe-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-request-validators-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-abci-info-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-abci-query-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-block-j57</code> | notes | 35 referencing files; first 12 listed |
| <code>interface.15.go-consensus-rpc-coretypes-result-block-results-j57</code> | notes | 15 referencing files; first 12 listed |
| <code>interface.15.go-consensus-rpc-coretypes-result-block-search-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-blockchain-info-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-broadcast-evidence-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-broadcast-tx-commit-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-broadcast-tx-j57</code> | notes | 15 referencing files; first 12 listed |
| <code>interface.15.go-consensus-rpc-coretypes-result-check-tx-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-commit-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-consensus-params-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-consensus-state-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-dump-consensus-state-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-event-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-rpc-coretypes-result-events-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-genesis-chunk-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-genesis-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-header-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-health-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-lag-status-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-net-info-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-status-j57</code> | notes | 25 referencing files; first 12 listed; 1 exported methods |
| <code>interface.15.go-consensus-rpc-coretypes-result-subscribe-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-tx-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-tx-search-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-unconfirmed-txs-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-unsafe-flush-mempool-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-unsubscribe-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-coretypes-result-validators-j57</code> | notes | 17 referencing files; first 12 listed |
| <code>interface.15.go-consensus-rpc-coretypes-sync-info-j57</code> | notes | 14 referencing files; first 12 listed |
| <code>interface.15.go-consensus-rpc-coretypes-validator-info-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-rpc-coretypes-wrap-err-height-not-available-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-client-caller-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-client-client-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-rpc-jsonrpc-client-default-http-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-client-new-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-client-new-with-http-client-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-client-new-ws-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-client-request-batch-j57</code> | notes | callers: cross-package references in importing files; 4 exported methods |
| <code>interface.15.go-consensus-rpc-jsonrpc-client-ws-client-j57</code> | notes | callers: cross-package references in importing files; 10 exported methods |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-default-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-listen-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-new-rpc-func-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-new-websocket-manager-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-new-wsrpc-func-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-on-disconnect-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-ping-period-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-read-limit-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-read-wait-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-register-rpc-funcs-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-rpc-func-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-serve-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-server-serve-tls-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-call-info-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-code-internal-error-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-code-invalid-params-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-code-invalid-request-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-code-lag-is-high-error-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-code-method-not-found-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-code-parse-error-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-get-call-info-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-new-request-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-rpc-error-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-rpc-request-j57</code> | notes | callers: cross-package references in importing files; 9 exported methods |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-rpc-response-j57</code> | notes | callers: cross-package references in importing files; 4 exported methods |
| <code>interface.15.go-consensus-rpc-jsonrpc-types-with-call-info-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-test-create-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-test-start-tendermint-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-rpc-test-suppress-stdout-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-scripts-keymigrate-migrate-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-scripts-metricsgen-testdata-basic-metrics-j57</code> | notes | no in-repo importer of package basic at HEAD; exported for consumers outside this repository |
| <code>interface.15.go-consensus-scripts-metricsgen-testdata-basic-nop-metrics-j57</code> | notes | no in-repo importer of package basic at HEAD; exported for consumers outside this repository |
| <code>interface.15.go-consensus-scripts-metricsgen-testdata-basic-prometheus-metrics-j57</code> | notes | no in-repo importer of package basic at HEAD; exported for consumers outside this repository |
| <code>interface.15.go-consensus-scripts-metricsgen-testdata-commented-metrics-j57</code> | notes | no in-repo importer of package commented at HEAD; exported for consumers outside this repository |
| <code>interface.15.go-consensus-scripts-metricsgen-testdata-commented-nop-metrics-j57</code> | notes | no in-repo importer of package commented at HEAD; exported for consumers outside this repository |
| <code>interface.15.go-consensus-scripts-metricsgen-testdata-commented-prometheus-metrics-j57</code> | notes | no in-repo importer of package commented at HEAD; exported for consumers outside this repository |
| <code>interface.15.go-consensus-scripts-metricsgen-testdata-tags-metrics-j57</code> | notes | no in-repo importer of package tags at HEAD; exported for consumers outside this repository |
| <code>interface.15.go-consensus-scripts-metricsgen-testdata-tags-nop-metrics-j57</code> | notes | no in-repo importer of package tags at HEAD; exported for consumers outside this repository |
| <code>interface.15.go-consensus-scripts-metricsgen-testdata-tags-prometheus-metrics-j57</code> | notes | no in-repo importer of package tags at HEAD; exported for consumers outside this repository |
| <code>interface.15.go-consensus-scripts-scmigrate-migrate-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-state-abci-responses-results-hash-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-test-e2e-app-config-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-test-e2e-app-new-application-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-test-e2e-pkg-evidence-age-height-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-test-e2e-pkg-evidence-age-time-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-test-e2e-pkg-load-testnet-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-test-e2e-pkg-manifest-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-test-e2e-pkg-manifest-node-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-test-e2e-pkg-mode-full-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-test-e2e-pkg-mode-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-test-e2e-pkg-mode-light-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-test-e2e-pkg-mode-seed-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-test-e2e-pkg-mode-validator-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-test-e2e-pkg-node-j57</code> | notes | callers: cross-package references in importing files; 5 exported methods |
| <code>interface.15.go-consensus-test-e2e-pkg-perturbation-disconnect-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-test-e2e-pkg-perturbation-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-test-e2e-pkg-perturbation-kill-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-test-e2e-pkg-perturbation-pause-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-test-e2e-pkg-perturbation-restart-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-test-e2e-pkg-protocol-builtin-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-test-e2e-pkg-protocol-file-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-test-e2e-pkg-protocol-grpc-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-test-e2e-pkg-protocol-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-test-e2e-pkg-protocol-tcp-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-test-e2e-pkg-protocol-unix-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-test-e2e-pkg-sort-manifests-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-test-e2e-pkg-split-groups-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-test-e2e-pkg-state-sync-disabled-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-test-e2e-pkg-state-sync-p2-p-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-test-e2e-pkg-state-sync-rpc-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-test-e2e-pkg-testnet-j57</code> | notes | 13 referencing files; first 12 listed; 5 exported methods |
| <code>interface.15.go-consensus-test-e2e-pkg-write-manifests-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-abci-pub-key-type-ed25519-j57</code> | notes | 13 referencing files; first 12 listed; const in grouped declaration |
| <code>interface.15.go-consensus-types-address-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-block-event-publisher-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-block-from-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-block-height-key-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-block-id-flag-absent-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-block-id-flag-commit-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-block-id-flag-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-block-id-flag-nil-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-block-id-from-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-block-id-j57</code> | notes | 62 referencing files; first 12 listed; 7 exported methods |
| <code>interface.15.go-consensus-types-block-j57</code> | notes | 40 referencing files; first 12 listed; 11 exported methods |
| <code>interface.15.go-consensus-types-block-meta-from-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-block-meta-j57</code> | notes | 16 referencing files; first 12 listed; 2 exported methods |
| <code>interface.15.go-consensus-types-block-params-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-types-block-part-size-bytes-j57</code> | notes | 16 referencing files; first 12 listed; const in grouped declaration |
| <code>interface.15.go-consensus-types-commit-from-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-commit-j57</code> | notes | 49 referencing files; first 12 listed; 14 exported methods |
| <code>interface.15.go-consensus-types-commit-sig-j57</code> | notes | 14 referencing files; first 12 listed; 5 exported methods |
| <code>interface.15.go-consensus-types-compute-proto-size-for-txs-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-consensus-params-from-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-consensus-params-j57</code> | notes | 17 referencing files; first 12 listed; 7 exported methods |
| <code>interface.15.go-consensus-types-consensus-policy-2-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-consensus-policy-3-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-consensus-policy-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-data-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-types-default-block-params-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-default-consensus-params-j57</code> | notes | 16 referencing files; first 12 listed |
| <code>interface.15.go-consensus-types-default-consensus-policy-j57</code> | notes | 14 referencing files; first 12 listed |
| <code>interface.15.go-consensus-types-duplicate-vote-evidence-j57</code> | notes | callers: cross-package references in importing files; 11 exported methods |
| <code>interface.15.go-consensus-types-err-app-hash-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-err-consensus-hash-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-err-evidence-overflow-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-types-err-invalid-commit-height-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-types-err-invalid-commit-signatures-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-types-err-invalid-evidence-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-types-err-last-block-id-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-err-last-commit-verify-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-err-last-results-hash-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-err-next-validators-hash-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-err-not-enough-voting-power-signed-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-types-err-part-set-invalid-proof-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-err-part-set-unexpected-index-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-err-proposer-not-in-validator-set-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-err-too-much-evidence-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-err-validators-hash-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-err-vote-conflicting-votes-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-types-err-vote-invalid-signature-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-err-vote-non-deterministic-signature-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-erroring-mock-pv-err-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-event-block-sync-status-value-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-event-complete-proposal-value-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-event-data-block-sync-status-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-types-event-data-complete-proposal-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-types-event-data-evidence-validated-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-types-event-data-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-event-data-new-block-header-j57</code> | notes | 24 referencing files; first 12 listed; 2 exported methods |
| <code>interface.15.go-consensus-types-event-data-new-block-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-types-event-data-new-evidence-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-types-event-data-new-round-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-types-event-data-round-state-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-types-event-data-state-sync-status-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-types-event-data-string-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-types-event-data-tx-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-types-event-data-validator-set-updates-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-types-event-data-vote-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-types-event-evidence-validated-value-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-event-lock-value-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-event-new-block-header-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-new-block-header-value-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-event-new-block-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-new-block-value-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-event-new-evidence-value-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-event-new-round-step-value-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-event-new-round-value-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-event-polka-value-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-event-query-block-sync-status-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-query-complete-proposal-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-query-lock-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-query-new-block-header-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-query-new-block-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-query-new-round-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-query-new-round-step-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-query-polka-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-query-relock-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-query-state-sync-status-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-query-timeout-propose-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-query-timeout-wait-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-query-tx-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-query-valid-block-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-query-validator-set-updates-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-query-vote-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-relock-value-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-event-state-sync-status-value-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-event-timeout-propose-value-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-event-timeout-wait-value-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-event-tx-j57</code> | notes | callers: cross-package references in importing files; var in grouped declaration |
| <code>interface.15.go-consensus-types-event-tx-value-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-event-type-key-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-event-valid-block-value-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-event-validator-set-updates-value-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-event-vote-value-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-evidence-from-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-evidence-j57</code> | notes | 27 referencing files; first 12 listed |
| <code>interface.15.go-consensus-types-evidence-list-j57</code> | notes | callers: cross-package references in importing files; 10 exported methods |
| <code>interface.15.go-consensus-types-evidence-params-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-evidence-to-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-gen-node-key-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-genesis-doc-from-file-j57</code> | notes | 17 referencing files; first 12 listed |
| <code>interface.15.go-consensus-types-genesis-doc-from-json-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-genesis-doc-j57</code> | notes | 44 referencing files; first 12 listed; 6 exported methods |
| <code>interface.15.go-consensus-types-genesis-validator-j57</code> | notes | callers: cross-package references in importing files; 2 exported methods |
| <code>interface.15.go-consensus-types-header-from-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-header-j57</code> | notes | 40 referencing files; first 12 listed; 5 exported methods |
| <code>interface.15.go-consensus-types-is-err-not-enough-voting-power-signed-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-is-vote-type-valid-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-legacy-event-data-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-legacy-event-data-new-block-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-light-block-from-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-light-block-j57</code> | notes | 25 referencing files; first 12 listed; 4 exported methods |
| <code>interface.15.go-consensus-types-light-client-attack-evidence-j57</code> | notes | callers: cross-package references in importing files; 13 exported methods |
| <code>interface.15.go-consensus-types-light-client-info-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-load-node-key-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-load-or-gen-node-key-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-make-block-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-make-commit-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-max-block-parts-count-j57</code> | notes | 16 referencing files; first 12 listed; const in grouped declaration |
| <code>interface.15.go-consensus-types-max-block-size-bytes-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-max-chain-id-len-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-max-commit-bytes-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-max-data-bytes-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-max-data-bytes-no-evidence-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-max-header-bytes-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-max-node-info-size-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-max-overhead-for-block-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-max-total-voting-power-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-max-votes-count-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-mock-pv-j57</code> | notes | callers: cross-package references in importing files; 6 exported methods |
| <code>interface.15.go-consensus-types-new-block-meta-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-new-commit-sig-absent-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-new-duplicate-vote-evidence-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-new-encoded-vote-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-new-err-evidence-overflow-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-new-err-invalid-evidence-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-new-erroring-mock-pv-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-new-mock-duplicate-vote-evidence-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-new-mock-duplicate-vote-evidence-with-validator-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-new-mock-pv-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-new-mock-pv-with-params-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-new-node-id-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-new-part-set-from-data-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-new-part-set-from-header-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-new-proposal-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-new-results-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-new-validator-j57</code> | notes | 33 referencing files; first 12 listed |
| <code>interface.15.go-consensus-types-new-validator-set-j57</code> | notes | 36 referencing files; first 12 listed |
| <code>interface.15.go-consensus-types-new-vote-set-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-node-id-byte-length-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-node-id-from-pub-key-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-node-id-j57</code> | notes | 59 referencing files; first 12 listed; 3 exported methods |
| <code>interface.15.go-consensus-types-node-info-from-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-node-info-j57</code> | notes | 16 referencing files; first 12 listed; 5 exported methods |
| <code>interface.15.go-consensus-types-node-info-other-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-node-key-j57</code> | notes | callers: cross-package references in importing files; 5 exported methods |
| <code>interface.15.go-consensus-types-part-from-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-part-j57</code> | notes | callers: cross-package references in importing files; 4 exported methods |
| <code>interface.15.go-consensus-types-part-set-header-from-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-part-set-header-j57</code> | notes | 32 referencing files; first 12 listed; 5 exported methods |
| <code>interface.15.go-consensus-types-part-set-j57</code> | notes | callers: cross-package references in importing files; 14 exported methods |
| <code>interface.15.go-consensus-types-pb2-tm-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-priv-validator-j57</code> | notes | 44 referencing files; first 12 listed |
| <code>interface.15.go-consensus-types-priv-validators-by-address-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-types-proposal-from-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-proposal-j57</code> | notes | 14 referencing files; first 12 listed; 4 exported methods |
| <code>interface.15.go-consensus-types-proposal-sign-bytes-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-protocol-version-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-query-for-event-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-rand-validator-set-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-resolve-address-string-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-sealed-block-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-sequencer-source-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-signed-header-from-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-signed-header-j57</code> | notes | 21 referencing files; first 12 listed; 4 exported methods |
| <code>interface.15.go-consensus-types-skip-last-results-hash-validation-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-synchrony-params-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-types-time-format-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-timeout-params-j57</code> | notes | callers: cross-package references in importing files; 4 exported methods |
| <code>interface.15.go-consensus-types-tm2-pb-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-tx-hash-j57</code> | notes | callers: cross-package references in importing files; 4 exported methods |
| <code>interface.15.go-consensus-types-tx-hash-key-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-tx-height-key-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-types-tx-j57</code> | notes | 68 referencing files; first 12 listed; 2 exported methods |
| <code>interface.15.go-consensus-types-tx-proof-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-types-txs-j57</code> | notes | 16 referencing files; first 12 listed; 8 exported methods |
| <code>interface.15.go-consensus-types-validate-hash-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-validator-info-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-validator-j57</code> | notes | 49 referencing files; first 12 listed; 8 exported methods |
| <code>interface.15.go-consensus-types-validator-list-string-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-validator-params-j57</code> | notes | callers: cross-package references in importing files; 1 exported methods |
| <code>interface.15.go-consensus-types-validator-set-from-existing-validators-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-validator-set-from-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-validator-set-j57</code> | notes | 43 referencing files; first 12 listed; 25 exported methods |
| <code>interface.15.go-consensus-types-validators-by-voting-power-j57</code> | notes | callers: cross-package references in importing files; 3 exported methods |
| <code>interface.15.go-consensus-types-verify-commit-light-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-vote-from-proto-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-vote-j57</code> | notes | 32 referencing files; first 12 listed; 6 exported methods |
| <code>interface.15.go-consensus-types-vote-set-j57</code> | notes | callers: cross-package references in importing files; 25 exported methods |
| <code>interface.15.go-consensus-types-vote-set-reader-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-types-vote-sign-bytes-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-utils-map-j57</code> | notes | no in-repo importer of package utils at HEAD; exported for consumers outside this repository |

Contract context <code>contract_context.15.offline-cli-j57</code>:

| Field | Frozen rule |
|---|---|
| interface | Offline consensus CLI commands of consensus/cmd/tendermint/commands: reset blockchain, reindex-event, inspect, and the LoadTendermintState and RollbackStateToTargetHeight functions of consensus/cmd/tendermint/commands/rollback.go:12 and :32 |
| storage | data/autobahn is the default Autobahn persistent state directory (consensus/cmd/tendermint/commands/gen_autobahn_config.go:113); CometBFT stores under data/ and data/tendermint/ |
| errors | ErrAutobahnOfflineStore (planned) when autobahn-config-file is set |
| events | None |
| gas | Not applicable |
| activation | Refusal is decided by Config.AutobahnConfigFile at consensus/config/config.go:80 |
| invariants | No offline tool opens CometBFT stores of an Autobahn node; reset blockchain removes data/autobahn |
| pause | No pause |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.15.277.1</code> | req | 15277.1 |
| <code>acceptance.15.277.1</code> | scenario | Given an http provider from NewWithClient and an RPCError whose Data is the ErrCommitNotAvailable message, when the test calls parseRPCError, then the result is a provider.ErrUnreliableProvider whose Reason is that RPCError |
| <code>acceptance.15.277.1</code> | test | consensus/light/provider/http/http_internal_test.go::TestParseRPCErrorCommitNotAvailableIsUnreliable (planned) |
| <code>acceptance.15.277.1</code> | qualification | UNRUN |
| <code>req.15277</code> | ac_1 | The light provider drops a node that answers ErrCommitNotAvailable as unreliable; at HEAD parseRPCError at consensus/light/provider/http/http.go:303-321 maps it to ErrBadLightBlock. |
| <code>capability.15.28.1</code> | title | Treat a commit-not-available answer as an unreliable provider |
| <code>capability.15.28.1</code> | reqs | 15277.1 |
| <code>resolution.17.968</code> | title | decision.1524 binds: IBC light-client verification of this chain under Autobahn is out of scope |
| <code>resolution.17.968</code> | applies_to | decision.1524<br>task.5.68.1 |
| <code>resolution.17.968</code> | source | decision.1524 in fragments/J57/decision.kvx; tasks from affects |

## make-targets

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.15.mk-consensus-all-j57</code> | <code>makefile_target</code> | <code>change</code> | <code>consensus/Makefile:57</code> | <code>all: check build test install</code> |
| <code>interface.15.mk-consensus-build-c-amazonlinux-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/Makefile:230</code> | <code>build_c-amazonlinux:</code> |
| <code>interface.15.mk-consensus-build-contract-tests-hooks-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/Makefile:247</code> | <code>build-contract-tests-hooks:</code> |
| <code>interface.15.mk-consensus-build-docker-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/Makefile:186</code> | <code>build-docker:</code> |
| <code>interface.15.mk-consensus-build-docker-localnode-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/Makefile:223</code> | <code>build-docker-localnode:</code> |
| <code>interface.15.mk-consensus-build-docs-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/Makefile:171</code> | <code>build-docs:</code> |
| <code>interface.15.mk-consensus-build-j57</code> | <code>makefile_target</code> | <code>change</code> | <code>consensus/Makefile:66</code> | <code>build: $(BUILDDIR)/</code> |
| <code>interface.15.mk-consensus-build-linux-j57</code> | <code>makefile_target</code> | <code>stable</code> | <code>consensus/Makefile:219</code> | <code>build-linux:</code> |
| <code>interface.15.mk-consensus-build-privval-server-j57</code> | <code>makefile_target</code> | <code>change</code> | <code>consensus/Makefile:81</code> | <code>build_privval_server:</code> |
| <code>interface.15.mk-consensus-build-reproducible-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/Makefile:267</code> | <code>build-reproducible:</code> |
| <code>interface.15.mk-consensus-clean-certs-j57</code> | <code>makefile_target</code> | <code>stable</code> | <code>consensus/Makefile:146</code> | <code>clean_certs:</code> |
| <code>interface.15.mk-consensus-clean-j57</code> | <code>makefile_target</code> | <code>stable</code> | <code>consensus/Makefile:264</code> | <code>clean:</code> |
| <code>interface.15.mk-consensus-contract-tests-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/Makefile:260</code> | <code>contract-tests:</code> |
| <code>interface.15.mk-consensus-dist-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/Makefile:104</code> | <code>dist:</code> |
| <code>interface.15.mk-consensus-draw-deps-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/Makefile:118</code> | <code>draw_deps:</code> |
| <code>interface.15.mk-consensus-format-j57</code> | <code>makefile_target</code> | <code>change</code> | <code>consensus/Makefile:155</code> | <code>format:</code> |
| <code>interface.15.mk-consensus-gen-certs-j57</code> | <code>makefile_target</code> | <code>stable</code> | <code>consensus/Makefile:136</code> | <code>gen_certs: clean_certs</code> |
| <code>interface.15.mk-consensus-generate-test-cert-j57</code> | <code>makefile_target</code> | <code>stable</code> | <code>consensus/Makefile:85</code> | <code>generate_test_cert:</code> |
| <code>interface.15.mk-consensus-get-deps-bin-size-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/Makefile:124</code> | <code>get_deps_bin_size:</code> |
| <code>interface.15.mk-consensus-go-mod-cache-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/Makefile:108</code> | <code>go-mod-cache: go.sum</code> |
| <code>interface.15.mk-consensus-go-sum-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/Makefile:113</code> | <code>go.sum: go.mod</code> |
| <code>interface.15.mk-consensus-install-j57</code> | <code>makefile_target</code> | <code>change</code> | <code>consensus/Makefile:70</code> | <code>install:</code> |
| <code>interface.15.mk-consensus-lint-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/Makefile:160</code> | <code>lint:</code> |
| <code>interface.15.mk-consensus-localnet-start-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/Makefile:236</code> | <code>localnet-start: localnet-stop build-docker-localnode</code> |
| <code>interface.15.mk-consensus-localnet-stop-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/Makefile:242</code> | <code>localnet-stop:</code> |
| <code>interface.15.mk-consensus-metrics-j57</code> | <code>makefile_target</code> | <code>stable</code> | <code>consensus/Makefile:203</code> | <code>metrics: testdata-metrics</code> |
| <code>interface.15.mk-consensus-mockery-j57</code> | <code>makefile_target</code> | <code>stable</code> | <code>consensus/Makefile:195</code> | <code>mockery:</code> |
| <code>interface.15.mk-consensus-networks-local-all-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/networks/local/Makefile:3</code> | <code>all:</code> |
| <code>interface.15.mk-consensus-split-test-packages-j57</code> | <code>makefile_target</code> | <code>stable</code> | <code>consensus/Makefile:295</code> | <code>split-test-packages:$(BUILDDIR)/packages.txt</code> |
| <code>interface.15.mk-consensus-test-build-docker-test-image-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/test/Makefile:11</code> | <code>build_docker_test_image:</code> |
| <code>interface.15.mk-consensus-test-group-pct-j57</code> | <code>makefile_target</code> | <code>stable</code> | <code>consensus/Makefile:297</code> | <code>test-group-%:split-test-packages</code> |
| <code>interface.15.mk-consensus-test-test-abci-apps-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/test/Makefile:21</code> | <code>test_abci_apps:</code> |
| <code>interface.15.mk-consensus-test-test-cover-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/test/Makefile:16</code> | <code>test_cover:</code> |
| <code>interface.15.mk-consensus-test-test-integrations-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/test/Makefile:25</code> | <code>test_integrations:</code> |
| <code>interface.15.mk-consensus-test-test-j57</code> | <code>makefile_target</code> | <code>stable</code> | <code>consensus/test/Makefile:43</code> | <code>test:</code> |
| <code>interface.15.mk-consensus-test-test-race-j57</code> | <code>makefile_target</code> | <code>stable</code> | <code>consensus/test/Makefile:48</code> | <code>test_race:</code> |
| <code>interface.15.mk-consensus-test-test-release-j57</code> | <code>makefile_target</code> | <code>stable</code> | <code>consensus/test/Makefile:34</code> | <code>test_release:</code> |
| <code>interface.15.mk-consensus-test-test100-j57</code> | <code>makefile_target</code> | <code>retire</code> | <code>consensus/test/Makefile:38</code> | <code>test100:</code> |
| <code>interface.15.mk-consensus-testdata-metrics-j57</code> | <code>makefile_target</code> | <code>stable</code> | <code>consensus/Makefile:210</code> | <code>testdata-metrics:</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.15.mk-consensus-all-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-build-c-amazonlinux-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-build-contract-tests-hooks-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-build-docker-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-build-docker-localnode-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-build-docs-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-build-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-build-linux-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-build-privval-server-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-build-reproducible-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-clean-certs-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-clean-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-contract-tests-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-dist-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-draw-deps-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-format-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-gen-certs-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-generate-test-cert-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-get-deps-bin-size-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-go-mod-cache-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-go-sum-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-install-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-lint-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-localnet-start-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-localnet-stop-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-metrics-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-mockery-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-networks-local-all-j57</code> | notes | target of consensus/networks/local/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-split-test-packages-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-test-build-docker-test-image-j57</code> | notes | target of consensus/test/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-test-group-pct-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-test-test-abci-apps-j57</code> | notes | target of consensus/test/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-test-test-cover-j57</code> | notes | target of consensus/test/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-test-test-integrations-j57</code> | notes | target of consensus/test/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-test-test-j57</code> | notes | target of consensus/test/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-test-test-race-j57</code> | notes | target of consensus/test/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-test-test-release-j57</code> | notes | target of consensus/test/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-test-test100-j57</code> | notes | target of consensus/test/Makefile; invoked from the consensus directory |
| <code>interface.15.mk-consensus-testdata-metrics-j57</code> | notes | target of consensus/Makefile; invoked from the consensus directory |

Contract context <code>contract_context.15.make-targets-j57</code>:

| Field | Frozen rule |
|---|---|
| interface | Make targets of consensus/Makefile and consensus/test/Makefile kept or retired by the fork |
| storage | No storage |
| errors | make exits non-zero with No rule to make target for a retired target |
| events | None |
| gas | Not applicable |
| activation | Kept targets: all, build, install, build_privval_server, format, test and the targets not listed as retired |
| invariants | make -n of every kept target resolves inside the monorepo; LD_FLAGS and goimports -local name the fork module path |
| pause | No pause |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.15.242.1</code> | req | 15242.1 |
| <code>acceptance.15.242.1</code> | scenario | Given consensus/Makefile with LD_FLAGS rewritten (planned change of consensus/Makefile:14), when the test runs make -n -C consensus build VERSION=9.9.9, then make exits 0 and the go build line holds -X github.com/Sidiora-Labs/Paxeer-X-Network/consensus/version.TMVersion=9.9.9 |
| <code>acceptance.15.242.1</code> | test | consensus/test/maketargets/consensus_makefile_test.go::TestMakeBuildLinksConsensusVersion (planned) |
| <code>acceptance.15.242.1</code> | qualification | UNRUN |
| <code>req.15242</code> | ac_1 | make build links the version into github.com/Sidiora-Labs/Paxeer-X-Network/consensus/version.TMVersion; at HEAD consensus/Makefile:14 names the pre-fork module path, so the flag is silently ignored. |
| <code>acceptance.15.242.2</code> | req | 15242.2 |
| <code>acceptance.15.242.2</code> | scenario | Given the install recipe of consensus/Makefile:70-71 with quoted tags (planned), when the test runs make -n -C consensus install BUILD_TAGS="alpha beta", then the go install line holds -tags 'alpha beta' as one argument |
| <code>acceptance.15.242.2</code> | test | consensus/test/maketargets/consensus_makefile_test.go::TestMakeInstallQuotesBuildTags (planned) |
| <code>acceptance.15.242.2</code> | qualification | UNRUN |
| <code>req.15242</code> | ac_2 | make install passes every build tag as one -tags argument; at HEAD the unquoted -tags $(BUILD_TAGS) at consensus/Makefile:71 splits a multi-tag value into a stray package argument. |
| <code>acceptance.15.243.1</code> | req | 15243.1 |
| <code>acceptance.15.243.1</code> | scenario | Given all: build test install at consensus/Makefile:57 (planned), when the test runs make -n -C consensus all, then make exits 0 and prints the go build recipe of consensus/Makefile:67 and the go install recipe of consensus/Makefile:71 |
| <code>acceptance.15.243.1</code> | test | consensus/test/maketargets/consensus_makefile_test.go::TestMakeAllDryRunResolves (planned) |
| <code>acceptance.15.243.1</code> | qualification | UNRUN |
| <code>req.15243</code> | ac_1 | make all resolves every prerequisite; at HEAD consensus/Makefile:57 names a check target that does not exist, so make all stops with No rule to make target. |
| <code>acceptance.15.244.1</code> | req | 15244.1 |
| <code>acceptance.15.244.1</code> | scenario | Given build_privval_server at consensus/Makefile:81-82 without -i (planned), when the test runs make -n -C consensus build_privval_server, then make exits 0 and the printed go build line carries no -i field |
| <code>acceptance.15.244.1</code> | test | consensus/test/maketargets/consensus_makefile_test.go::TestMakeBuildPrivvalServerDryRun (planned) |
| <code>acceptance.15.244.1</code> | qualification | UNRUN |
| <code>req.15244</code> | ac_1 | make build_privval_server prints a go build command the current go toolchain accepts; at HEAD consensus/Makefile:82 passes the removed -i flag. |
| <code>acceptance.15.245.1</code> | req | 15245.1 |
| <code>acceptance.15.245.1</code> | scenario | Given the format recipe of consensus/Makefile:155-157 with the fork prefix (planned), when the test runs make -n -C consensus format, then the goimports line holds -local github.com/Sidiora-Labs/Paxeer-X-Network and no other -local value |
| <code>acceptance.15.245.1</code> | test | consensus/test/maketargets/consensus_makefile_test.go::TestMakeFormatUsesModuleLocalPrefix (planned) |
| <code>acceptance.15.245.1</code> | qualification | UNRUN |
| <code>req.15245</code> | ac_1 | make format groups local imports under github.com/Sidiora-Labs/Paxeer-X-Network; at HEAD consensus/Makefile:157 groups them under the pre-fork module path, so every fork import lands in the third-party group. |
| <code>acceptance.15.246.1</code> | req | 15246.1 |
| <code>acceptance.15.246.1</code> | scenario | Given consensus/Makefile without the targets lint, build-contract-tests-hooks, contract-tests (planned), when the test runs make -n -C consensus &lt;target&gt; for each of them, then each run exits non-zero with No rule to make target and make -n -C consensus build still exits 0 |
| <code>acceptance.15.246.1</code> | test | consensus/test/maketargets/consensus_makefile_test.go::TestMakeRetiredContractTestTargets (planned) |
| <code>acceptance.15.246.1</code> | qualification | UNRUN |
| <code>req.15246</code> | ac_1 | The consensus Makefile no longer defines lint, build-contract-tests-hooks, contract-tests; at HEAD they sit at consensus/Makefile:160, :247 and :260 and depend on configuration files absent from consensus/. |
| <code>acceptance.15.247.1</code> | req | 15247.1 |
| <code>acceptance.15.247.1</code> | scenario | Given consensus/Makefile without the targets build-docs, build-docker, build-docker-localnode, build_c-amazonlinux, localnet-start, localnet-stop (planned), when the test runs make -n -C consensus &lt;target&gt; for each of them, then each run exits non-zero with No rule to make target and make -n -C consensus build still exits 0 |
| <code>acceptance.15.247.1</code> | test | consensus/test/maketargets/consensus_makefile_test.go::TestMakeRetiredDockerTargets (planned) |
| <code>acceptance.15.247.1</code> | qualification | UNRUN |
| <code>req.15247</code> | ac_1 | The consensus Makefile no longer defines build-docs, build-docker, build-docker-localnode, build_c-amazonlinux, localnet-start, localnet-stop; at HEAD they sit at consensus/Makefile:171, :186, :223, :230, :236 and :242 and reference directories and compose files absent from consensus/. |
| <code>acceptance.15.248.1</code> | req | 15248.1 |
| <code>acceptance.15.248.1</code> | scenario | Given consensus/Makefile without the targets dist, go-mod-cache, go.sum, draw_deps, get_deps_bin_size, build-reproducible (planned), when the test runs make -n -C consensus &lt;target&gt; for each of them, then each run exits non-zero with No rule to make target and make -n -C consensus build still exits 0 |
| <code>acceptance.15.248.1</code> | test | consensus/test/maketargets/consensus_makefile_test.go::TestMakeRetiredDistributionTargets (planned) |
| <code>acceptance.15.248.1</code> | qualification | UNRUN |
| <code>req.15248</code> | ac_1 | The consensus Makefile no longer defines dist, go-mod-cache, go.sum, draw_deps, get_deps_bin_size, build-reproducible; at HEAD they sit at consensus/Makefile:104, :108, :113, :118, :124 and :267 and assume a standalone consensus module. |
| <code>acceptance.15.249.1</code> | req | 15249.1 |
| <code>acceptance.15.249.1</code> | scenario | Given consensus/test/Makefile without test_abci_apps, test_integrations, test100 (planned), when the test runs make -n -C consensus &lt;target&gt; for each of them, then each run exits non-zero with No rule to make target and make -n -C consensus test still exits 0 |
| <code>acceptance.15.249.1</code> | test | consensus/test/maketargets/test_makefile_test.go::TestMakeRetiredIntegrationTargets (planned) |
| <code>acceptance.15.249.1</code> | qualification | UNRUN |
| <code>req.15249</code> | ac_1 | The consensus test Makefile no longer defines test_abci_apps, test_integrations, test100; at HEAD they sit at consensus/test/Makefile:21, :25 and :38 and call scripts and targets absent from consensus/. |
| <code>acceptance.15.250.1</code> | req | 15250.1 |
| <code>acceptance.15.250.1</code> | scenario | Given consensus/test/Makefile without build_docker_test_image (planned), when the test runs make -n -C consensus &lt;target&gt; for each of them, then each run exits non-zero with No rule to make target and make -n -C consensus test still exits 0 |
| <code>acceptance.15.250.1</code> | test | consensus/test/maketargets/test_makefile_test.go::TestMakeRetiredDockerTestImageTarget (planned) |
| <code>acceptance.15.250.1</code> | qualification | UNRUN |
| <code>req.15250</code> | ac_1 | The consensus test Makefile no longer defines build_docker_test_image; at HEAD it sits at consensus/test/Makefile:11 and builds consensus/test/docker/Dockerfile, which is deleted with it. |
| <code>acceptance.15.251.1</code> | req | 15251.1 |
| <code>acceptance.15.251.1</code> | scenario | Given consensus/test/Makefile without test_cover (planned), when the test runs make -n -C consensus &lt;target&gt; for each of them, then each run exits non-zero with No rule to make target and make -n -C consensus test still exits 0 |
| <code>acceptance.15.251.1</code> | test | consensus/test/maketargets/test_makefile_test.go::TestMakeRetiredCoverTarget (planned) |
| <code>acceptance.15.251.1</code> | qualification | UNRUN |
| <code>req.15251</code> | ac_1 | The consensus test Makefile no longer defines test_cover; at HEAD it sits at consensus/test/Makefile:16 and runs consensus/test/test_cover.sh, which is deleted with it. |
| <code>capability.15.21.2</code> | title | Point consensus LD_FLAGS at the fork module path and quote install tags |
| <code>capability.15.21.2</code> | reqs | 15242.1<br>15242.2 |
| <code>capability.15.22.1</code> | title | Drop the missing check prerequisite from the consensus all target |
| <code>capability.15.22.1</code> | reqs | 15243.1 |
| <code>capability.15.22.2</code> | title | Drop the removed -i flag from build_privval_server |
| <code>capability.15.22.2</code> | reqs | 15244.1 |
| <code>capability.15.22.3</code> | title | Format consensus imports with the fork module local prefix |
| <code>capability.15.22.3</code> | reqs | 15245.1 |
| <code>capability.15.22.4</code> | title | Retire lint and the contract-test targets of the consensus Makefile |
| <code>capability.15.22.4</code> | reqs | 15246.1 |
| <code>capability.15.22.5</code> | title | Retire the docs, docker and localnet targets with consensus/networks/local |
| <code>capability.15.22.5</code> | reqs | 15247.1 |
| <code>capability.15.22.6</code> | title | Retire the distribution and dependency-graph targets |
| <code>capability.15.22.6</code> | reqs | 15248.1 |
| <code>capability.15.23.1</code> | title | Retire test_abci_apps, test_integrations and test100 |
| <code>capability.15.23.1</code> | reqs | 15249.1 |
| <code>capability.15.23.2</code> | title | Retire build_docker_test_image with the consensus/test/docker files |
| <code>capability.15.23.2</code> | reqs | 15250.1 |
| <code>capability.15.23.3</code> | title | Retire test_cover with consensus/test/test_cover.sh |
| <code>capability.15.23.3</code> | reqs | 15251.1 |
| <code>resolution.17.966</code> | title | decision.1522 binds: consensus/networks/remote stays out of the gated surface |
| <code>resolution.17.966</code> | applies_to | decision.1522<br>task.5.62.5 |
| <code>resolution.17.966</code> | source | decision.1522 in fragments/J57/decision.kvx; tasks from affects |

## metrics

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.15.met-consensus-internal-consensus-apply-block-latency-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:169</code> | <code>tendermint_consensus_apply_block_latency (ApplyBlockLatency metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-consensus-block-gossip-parts-received-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:83</code> | <code>tendermint_consensus_block_gossip_parts_received (BlockGossipPartsReceived metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-consensus-block-gossip-receive-latency-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:78</code> | <code>tendermint_consensus_block_gossip_receive_latency (BlockGossipReceiveLatency metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-consensus-block-interval-seconds-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:54</code> | <code>tendermint_consensus_block_interval_seconds (BlockIntervalSeconds metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-consensus-block-parts-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:70</code> | <code>tendermint_consensus_block_parts (BlockParts metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-consensus-block-size-bytes-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:59</code> | <code>tendermint_consensus_block_size_bytes (BlockSizeBytes metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-consensus-block-syncing-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:65</code> | <code>tendermint_consensus_block_syncing (BlockSyncing metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-byzantine-validators-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:49</code> | <code>tendermint_consensus_byzantine_validators (ByzantineValidators metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-byzantine-validators-power-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:51</code> | <code>tendermint_consensus_byzantine_validators_power (ByzantineValidatorsPower metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-complete-proposal-time-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:166</code> | <code>tendermint_consensus_complete_proposal_time (CompleteProposalTime metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-consensus-consensus-time-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:161</code> | <code>tendermint_consensus_consensus_time (ConsensusTime metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-consensus-final-round-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:147</code> | <code>tendermint_consensus_final_round (FinalRound metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-consensus-full-prevote-delay-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:113</code> | <code>tendermint_consensus_full_prevote_delay (FullPrevoteDelay metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-height-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:25</code> | <code>tendermint_consensus_height (Height metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-late-votes-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:143</code> | <code>tendermint_consensus_late_votes (LateVotes metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-consensus-latest-block-height-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:63</code> | <code>tendermint_consensus_latest_block_height (CommittedHeight metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-missing-txs-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:95</code> | <code>tendermint_consensus_missing_txs (MissingTxs metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-missing-validators-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:45</code> | <code>tendermint_consensus_missing_validators (MissingValidators metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-missing-validators-power-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:47</code> | <code>tendermint_consensus_missing_validators_power (MissingValidatorsPower metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-num-txs-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:57</code> | <code>tendermint_consensus_num_txs (NumTxs metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-prevote-latency-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:157</code> | <code>tendermint_consensus_prevote_latency (PrevoteLatency metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-consensus-proposal-block-created-on-propose-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:86</code> | <code>tendermint_consensus_proposal_block_created_on_propose (ProposalBlockCreatedOnPropose metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-consensus-proposal-create-count-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:131</code> | <code>tendermint_consensus_proposal_create_count (ProposalCreateCount metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-consensus-proposal-missing-txs-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:92</code> | <code>tendermint_consensus_proposal_missing_txs (ProposalMissingTxs metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-proposal-receive-count-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:126</code> | <code>tendermint_consensus_proposal_receive_count (ProposalReceiveCount metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-consensus-proposal-timestamp-difference-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:119</code> | <code>tendermint_consensus_proposal_timestamp_difference (ProposalTimestampDifference metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-consensus-proposal-txs-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:89</code> | <code>tendermint_consensus_proposal_txs (ProposalTxs metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-propose-latency-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:152</code> | <code>tendermint_consensus_propose_latency (ProposeLatency metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-consensus-quorum-prevote-delay-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:107</code> | <code>tendermint_consensus_quorum_prevote_delay (QuorumPrevoteDelay metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-round-duration-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:34</code> | <code>tendermint_consensus_round_duration (RoundDuration metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-consensus-round-voting-power-percent-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:137</code> | <code>tendermint_consensus_round_voting_power_percent (RoundVotingPowerPercent metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-rounds-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:31</code> | <code>tendermint_consensus_rounds (Rounds metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-state-syncing-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:67</code> | <code>tendermint_consensus_state_syncing (StateSyncing metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-step-count-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:173</code> | <code>tendermint_consensus_step_count (StepCount metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-step-duration-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:73</code> | <code>tendermint_consensus_step_duration (StepDuration metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-consensus-step-latency-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:171</code> | <code>tendermint_consensus_step_latency (StepLatency metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-total-txs-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:61</code> | <code>tendermint_consensus_total_txs (TotalTxs metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-validator-last-signed-height-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:28</code> | <code>tendermint_consensus_validator_last_signed_height (ValidatorLastSignedHeight metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-validator-missed-blocks-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:43</code> | <code>tendermint_consensus_validator_missed_blocks (ValidatorMissedBlocks metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-validator-power-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:41</code> | <code>tendermint_consensus_validator_power (ValidatorPower metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-validators-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:37</code> | <code>tendermint_consensus_validators (Validators metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-consensus-validators-power-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/consensus/metrics.go:39</code> | <code>tendermint_consensus_validators_power (ValidatorsPower metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-eventlog-num-items-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/eventlog/metrics.go:13</code> | <code>tendermint_eventlog_num_items (numItems metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-evidence-num-evidence-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/evidence/metrics.go:19</code> | <code>tendermint_evidence_pool_num_evidence (NumEvidence metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-mempool-cache-size-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:58</code> | <code>tendermint_mempool_cache_size (CacheSize metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-mempool-check-tx-dropped-by-priority-hint-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:130</code> | <code>tendermint_mempool_check_tx_dropped_by_priority_hint (CheckTxDroppedByPriorityHint metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-mempool-check-tx-met-drop-utilisation-threshold-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:134</code> | <code>tendermint_mempool_check_tx_met_drop_utilisation_threshold (CheckTxMetDropUtilisationThreshold metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-mempool-check-tx-priority-distribution-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:126</code> | <code>tendermint_mempool_check_tx_priority_distribution (CheckTxPriorityDistribution metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-mempool-duplicate-tx-max-occurrences-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:67</code> | <code>tendermint_mempool_duplicate_tx_max_occurrences (DuplicateTxMaxOccurrences metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-mempool-duplicate-tx-total-occurrences-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:70</code> | <code>tendermint_mempool_duplicate_tx_total_occurrences (DuplicateTxTotalOccurrences metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-mempool-evicted-txs-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:102</code> | <code>tendermint_mempool_evicted_txs (EvictedTxs metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-mempool-expired-txs-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:108</code> | <code>tendermint_mempool_expired_txs (ExpiredTxs metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-mempool-failed-txs-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:88</code> | <code>tendermint_mempool_failed_txs (FailedTxs metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-mempool-inserted-txs-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:117</code> | <code>tendermint_mempool_inserted_txs (InsertedTxs metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-mempool-number-of-duplicate-txs-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:73</code> | <code>tendermint_mempool_number_of_duplicate_txs (NumberOfDuplicateTxs metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-mempool-number-of-failed-check-txs-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:82</code> | <code>tendermint_mempool_number_of_failed_check_txs (NumberOfFailedCheckTxs metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-mempool-number-of-local-check-tx-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:85</code> | <code>tendermint_mempool_number_of_local_check_tx (NumberOfLocalCheckTx metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-mempool-number-of-non-duplicate-txs-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:76</code> | <code>tendermint_mempool_number_of_non_duplicate_txs (NumberOfNonDuplicateTxs metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-mempool-number-of-successful-check-txs-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:79</code> | <code>tendermint_mempool_number_of_successful_check_txs (NumberOfSuccessfulCheckTxs metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-mempool-pending-size-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:55</code> | <code>tendermint_mempool_pending_size (PendingSize metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-mempool-recheck-times-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:111</code> | <code>tendermint_mempool_recheck_times (RecheckTimes metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-mempool-rejected-txs-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:95</code> | <code>tendermint_mempool_rejected_txs (RejectedTxs metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-mempool-removed-txs-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:114</code> | <code>tendermint_mempool_removed_txs (RemovedTxs metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-mempool-size-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:52</code> | <code>tendermint_mempool_size (Size metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-mempool-total-txs-size-bytes-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:64</code> | <code>tendermint_mempool_total_txs_size_bytes (TotalTxsSizeBytes metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-mempool-tx-size-bytes-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:61</code> | <code>tendermint_mempool_tx_size_bytes (TxSizeBytes metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-p2p-channel-msgs-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/p2p/metrics.go:55</code> | <code>tendermint_p2p_channel_msgs (ChannelMsgs metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-p2p-new-connections-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/p2p/metrics.go:38</code> | <code>tendermint_p2p_new_connections (NewConnections metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-p2p-peer-pending-send-bytes-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/p2p/metrics.go:36</code> | <code>tendermint_p2p_peer_pending_send_bytes (PeerPendingSendBytes metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-p2p-peer-receive-bytes-total-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/p2p/metrics.go:32</code> | <code>tendermint_p2p_peer_receive_bytes_total (PeerReceiveBytesTotal metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-p2p-peer-send-bytes-total-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/p2p/metrics.go:34</code> | <code>tendermint_p2p_peer_send_bytes_total (PeerSendBytesTotal metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-p2p-peers-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/p2p/metrics.go:30</code> | <code>tendermint_p2p_peers (Peers metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-p2p-queue-dropped-msgs-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/p2p/metrics.go:59</code> | <code>tendermint_p2p_queue_dropped_msgs (QueueDroppedMsgs metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-p2p-router-channel-queue-send-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/p2p/metrics.go:53</code> | <code>tendermint_p2p_router_channel_queue_send (RouterChannelQueueSend metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-p2p-router-peer-queue-recv-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/p2p/metrics.go:43</code> | <code>tendermint_p2p_router_peer_queue_recv (RouterPeerQueueRecv metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-p2p-router-peer-queue-send-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/p2p/metrics.go:48</code> | <code>tendermint_p2p_router_peer_queue_send (RouterPeerQueueSend metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-proxy-method-timing-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/proxy/metrics.go:15</code> | <code>tendermint_abci_connection_method_timing (MethodTiming metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-state-application-commit-time-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/state/metrics.go:31</code> | <code>tendermint_state_application_commit_time (ApplicationCommitTime metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-state-block-processing-time-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/state/metrics.go:18</code> | <code>tendermint_state_block_processing_time (BlockProcessingTime metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-state-consensus-param-updates-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/state/metrics.go:23</code> | <code>tendermint_state_consensus_param_updates (ConsensusParamUpdates metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-state-finalize-block-latency-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/state/metrics.go:38</code> | <code>tendermint_state_finalize_block_latency (FinalizeBlockLatency metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-state-fire-events-latency-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/state/metrics.go:50</code> | <code>tendermint_state_fire_events_latency (FireEventsLatency metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-state-indexer-block-events-seconds-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/state/indexer/metrics.go:15</code> | <code>tendermint_indexer_block_events_seconds (BlockEventsSeconds metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-state-indexer-blocks-indexed-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/state/indexer/metrics.go:21</code> | <code>tendermint_indexer_blocks_indexed (BlocksIndexed metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-state-indexer-transactions-indexed-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/state/indexer/metrics.go:24</code> | <code>tendermint_indexer_transactions_indexed (TransactionsIndexed metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-state-indexer-tx-events-seconds-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/state/indexer/metrics.go:18</code> | <code>tendermint_indexer_tx_events_seconds (TxEventsSeconds metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-state-proposer-priority-hash-height-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/state/metrics.go:63</code> | <code>tendermint_state_proposer_priority_hash_height (ProposerPriorityHashHeight metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-state-proposer-priority-hash-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/state/metrics.go:58</code> | <code>tendermint_state_proposer_priority_hash (ProposerPriorityHash metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-state-prune-block-latency-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/state/metrics.go:47</code> | <code>tendermint_state_prune_block_latency (PruneBlockLatency metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-state-save-block-latency-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/state/metrics.go:44</code> | <code>tendermint_state_save_block_latency (SaveBlockLatency metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-state-save-block-response-latency-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/state/metrics.go:41</code> | <code>tendermint_state_save_block_response_latency (SaveBlockResponseLatency metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-state-update-mempool-time-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/state/metrics.go:35</code> | <code>tendermint_state_update_mempool_time (UpdateMempoolTime metrics.Histogram)</code> |
| <code>interface.15.met-consensus-internal-state-validator-set-updates-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/state/metrics.go:28</code> | <code>tendermint_state_validator_set_updates (ValidatorSetUpdates metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-statesync-back-fill-blocks-total-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/statesync/metrics.go:29</code> | <code>tendermint_statesync_back_fill_blocks_total (BackFillBlocksTotal metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-statesync-back-filled-blocks-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/statesync/metrics.go:27</code> | <code>tendermint_statesync_back_filled_blocks (BackFilledBlocks metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-statesync-chunk-process-avg-time-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/statesync/metrics.go:19</code> | <code>tendermint_statesync_chunk_process_avg_time (ChunkProcessAvgTime metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-statesync-snapshot-chunk-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/statesync/metrics.go:23</code> | <code>tendermint_statesync_snapshot_chunk (SnapshotChunk metrics.Counter)</code> |
| <code>interface.15.met-consensus-internal-statesync-snapshot-chunk-total-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/statesync/metrics.go:25</code> | <code>tendermint_statesync_snapshot_chunk_total (SnapshotChunkTotal metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-statesync-snapshot-height-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/statesync/metrics.go:21</code> | <code>tendermint_statesync_snapshot_height (SnapshotHeight metrics.Gauge)</code> |
| <code>interface.15.met-consensus-internal-statesync-total-snapshots-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/statesync/metrics.go:17</code> | <code>tendermint_statesync_total_snapshots (TotalSnapshots metrics.Counter)</code> |
| <code>interface.15.met-otel-pax-unsafe-validation-skipped-total-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/types/policy_metrics.go:20</code> | <code>Int64Counter("pax_unsafe_validation_skipped_total")</code> |
| <code>interface.15.met-otel-tendermint-mempool-compact-duration-seconds-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:34</code> | <code>Float64Histogram("tendermint_mempool_compact_duration_seconds")</code> |
| <code>interface.15.met-otel-tendermint-mempool-compact-total-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/mempool/metrics.go:30</code> | <code>Int64Counter("tendermint_mempool_compact_total")</code> |
| <code>interface.15.met-prom-pax-data-latency-j57</code> | <code>metric</code> | <code>stable</code> | <code>consensus/internal/autobahn/data/metrics.go:23</code> | <code>Name: "pax_data__latency"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.15.met-consensus-internal-consensus-apply-block-latency-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:276 |
| <code>interface.15.met-consensus-internal-consensus-block-gossip-parts-received-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:162 |
| <code>interface.15.met-consensus-internal-consensus-block-gossip-receive-latency-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:154 |
| <code>interface.15.met-consensus-internal-consensus-block-interval-seconds-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:94 |
| <code>interface.15.met-consensus-internal-consensus-block-parts-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:140 |
| <code>interface.15.met-consensus-internal-consensus-block-size-bytes-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:108 |
| <code>interface.15.met-consensus-internal-consensus-block-syncing-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:128 |
| <code>interface.15.met-consensus-internal-consensus-byzantine-validators-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:82 |
| <code>interface.15.met-consensus-internal-consensus-byzantine-validators-power-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:88 |
| <code>interface.15.met-consensus-internal-consensus-complete-proposal-time-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:268 |
| <code>interface.15.met-consensus-internal-consensus-consensus-time-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:260 |
| <code>interface.15.met-consensus-internal-consensus-final-round-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:236 |
| <code>interface.15.met-consensus-internal-consensus-full-prevote-delay-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:198 |
| <code>interface.15.met-consensus-internal-consensus-height-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:20 |
| <code>interface.15.met-consensus-internal-consensus-late-votes-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:230 |
| <code>interface.15.met-consensus-internal-consensus-latest-block-height-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:122 |
| <code>interface.15.met-consensus-internal-consensus-missing-txs-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:186 |
| <code>interface.15.met-consensus-internal-consensus-missing-validators-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:70 |
| <code>interface.15.met-consensus-internal-consensus-missing-validators-power-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:76 |
| <code>interface.15.met-consensus-internal-consensus-num-txs-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:102 |
| <code>interface.15.met-consensus-internal-consensus-prevote-latency-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:252 |
| <code>interface.15.met-consensus-internal-consensus-proposal-block-created-on-propose-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:168 |
| <code>interface.15.met-consensus-internal-consensus-proposal-create-count-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:218 |
| <code>interface.15.met-consensus-internal-consensus-proposal-missing-txs-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:180 |
| <code>interface.15.met-consensus-internal-consensus-proposal-receive-count-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:212 |
| <code>interface.15.met-consensus-internal-consensus-proposal-timestamp-difference-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:204 |
| <code>interface.15.met-consensus-internal-consensus-proposal-txs-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:174 |
| <code>interface.15.met-consensus-internal-consensus-propose-latency-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:244 |
| <code>interface.15.met-consensus-internal-consensus-quorum-prevote-delay-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:192 |
| <code>interface.15.met-consensus-internal-consensus-round-duration-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:38 |
| <code>interface.15.met-consensus-internal-consensus-round-voting-power-percent-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:224 |
| <code>interface.15.met-consensus-internal-consensus-rounds-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:32 |
| <code>interface.15.met-consensus-internal-consensus-state-syncing-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:134 |
| <code>interface.15.met-consensus-internal-consensus-step-count-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:290 |
| <code>interface.15.met-consensus-internal-consensus-step-duration-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:146 |
| <code>interface.15.met-consensus-internal-consensus-step-latency-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:284 |
| <code>interface.15.met-consensus-internal-consensus-total-txs-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:116 |
| <code>interface.15.met-consensus-internal-consensus-validator-last-signed-height-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:26 |
| <code>interface.15.met-consensus-internal-consensus-validator-missed-blocks-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:64 |
| <code>interface.15.met-consensus-internal-consensus-validator-power-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:58 |
| <code>interface.15.met-consensus-internal-consensus-validators-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:46 |
| <code>interface.15.met-consensus-internal-consensus-validators-power-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/consensus/metrics.gen.go:52 |
| <code>interface.15.met-consensus-internal-eventlog-num-items-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/eventlog/metrics.gen.go:20 |
| <code>interface.15.met-consensus-internal-evidence-num-evidence-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/evidence/metrics.gen.go:20 |
| <code>interface.15.met-consensus-internal-mempool-cache-size-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:32 |
| <code>interface.15.met-consensus-internal-mempool-check-tx-dropped-by-priority-hint-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:142 |
| <code>interface.15.met-consensus-internal-mempool-check-tx-met-drop-utilisation-threshold-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:148 |
| <code>interface.15.met-consensus-internal-mempool-check-tx-priority-distribution-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:134 |
| <code>interface.15.met-consensus-internal-mempool-duplicate-tx-max-occurrences-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:50 |
| <code>interface.15.met-consensus-internal-mempool-duplicate-tx-total-occurrences-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:56 |
| <code>interface.15.met-consensus-internal-mempool-evicted-txs-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:104 |
| <code>interface.15.met-consensus-internal-mempool-expired-txs-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:110 |
| <code>interface.15.met-consensus-internal-mempool-failed-txs-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:92 |
| <code>interface.15.met-consensus-internal-mempool-inserted-txs-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:128 |
| <code>interface.15.met-consensus-internal-mempool-number-of-duplicate-txs-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:62 |
| <code>interface.15.met-consensus-internal-mempool-number-of-failed-check-txs-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:80 |
| <code>interface.15.met-consensus-internal-mempool-number-of-local-check-tx-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:86 |
| <code>interface.15.met-consensus-internal-mempool-number-of-non-duplicate-txs-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:68 |
| <code>interface.15.met-consensus-internal-mempool-number-of-successful-check-txs-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:74 |
| <code>interface.15.met-consensus-internal-mempool-pending-size-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:26 |
| <code>interface.15.met-consensus-internal-mempool-recheck-times-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:116 |
| <code>interface.15.met-consensus-internal-mempool-rejected-txs-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:98 |
| <code>interface.15.met-consensus-internal-mempool-removed-txs-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:122 |
| <code>interface.15.met-consensus-internal-mempool-size-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:20 |
| <code>interface.15.met-consensus-internal-mempool-total-txs-size-bytes-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:44 |
| <code>interface.15.met-consensus-internal-mempool-tx-size-bytes-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/mempool/metrics.gen.go:38 |
| <code>interface.15.met-consensus-internal-p2p-channel-msgs-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/p2p/metrics.gen.go:68 |
| <code>interface.15.met-consensus-internal-p2p-new-connections-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/p2p/metrics.gen.go:44 |
| <code>interface.15.met-consensus-internal-p2p-peer-pending-send-bytes-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/p2p/metrics.gen.go:38 |
| <code>interface.15.met-consensus-internal-p2p-peer-receive-bytes-total-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/p2p/metrics.gen.go:26 |
| <code>interface.15.met-consensus-internal-p2p-peer-send-bytes-total-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/p2p/metrics.gen.go:32 |
| <code>interface.15.met-consensus-internal-p2p-peers-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/p2p/metrics.gen.go:20 |
| <code>interface.15.met-consensus-internal-p2p-queue-dropped-msgs-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/p2p/metrics.gen.go:74 |
| <code>interface.15.met-consensus-internal-p2p-router-channel-queue-send-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/p2p/metrics.gen.go:62 |
| <code>interface.15.met-consensus-internal-p2p-router-peer-queue-recv-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/p2p/metrics.gen.go:50 |
| <code>interface.15.met-consensus-internal-p2p-router-peer-queue-send-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/p2p/metrics.gen.go:56 |
| <code>interface.15.met-consensus-internal-proxy-method-timing-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/proxy/metrics.gen.go:20 |
| <code>interface.15.met-consensus-internal-state-application-commit-time-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/state/metrics.gen.go:40 |
| <code>interface.15.met-consensus-internal-state-block-processing-time-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/state/metrics.gen.go:20 |
| <code>interface.15.met-consensus-internal-state-consensus-param-updates-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/state/metrics.gen.go:28 |
| <code>interface.15.met-consensus-internal-state-finalize-block-latency-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/state/metrics.gen.go:52 |
| <code>interface.15.met-consensus-internal-state-fire-events-latency-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/state/metrics.gen.go:84 |
| <code>interface.15.met-consensus-internal-state-indexer-block-events-seconds-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/state/indexer/metrics.gen.go:20 |
| <code>interface.15.met-consensus-internal-state-indexer-blocks-indexed-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/state/indexer/metrics.gen.go:32 |
| <code>interface.15.met-consensus-internal-state-indexer-transactions-indexed-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/state/indexer/metrics.gen.go:38 |
| <code>interface.15.met-consensus-internal-state-indexer-tx-events-seconds-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/state/indexer/metrics.gen.go:26 |
| <code>interface.15.met-consensus-internal-state-proposer-priority-hash-height-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/state/metrics.gen.go:98 |
| <code>interface.15.met-consensus-internal-state-proposer-priority-hash-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/state/metrics.gen.go:92 |
| <code>interface.15.met-consensus-internal-state-prune-block-latency-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/state/metrics.gen.go:76 |
| <code>interface.15.met-consensus-internal-state-save-block-latency-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/state/metrics.gen.go:68 |
| <code>interface.15.met-consensus-internal-state-save-block-response-latency-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/state/metrics.gen.go:60 |
| <code>interface.15.met-consensus-internal-state-update-mempool-time-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/state/metrics.gen.go:46 |
| <code>interface.15.met-consensus-internal-state-validator-set-updates-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/state/metrics.gen.go:34 |
| <code>interface.15.met-consensus-internal-statesync-back-fill-blocks-total-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/statesync/metrics.gen.go:56 |
| <code>interface.15.met-consensus-internal-statesync-back-filled-blocks-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/statesync/metrics.gen.go:50 |
| <code>interface.15.met-consensus-internal-statesync-chunk-process-avg-time-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/statesync/metrics.gen.go:26 |
| <code>interface.15.met-consensus-internal-statesync-snapshot-chunk-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/statesync/metrics.gen.go:38 |
| <code>interface.15.met-consensus-internal-statesync-snapshot-chunk-total-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/statesync/metrics.gen.go:44 |
| <code>interface.15.met-consensus-internal-statesync-snapshot-height-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/statesync/metrics.gen.go:32 |
| <code>interface.15.met-consensus-internal-statesync-total-snapshots-j57</code> | notes | namespace prefix from instrumentation.namespace; generated constructor in consensus/internal/statesync/metrics.gen.go:20 |
| <code>interface.15.met-otel-pax-unsafe-validation-skipped-total-j57</code> | notes | OpenTelemetry instrument exported through the node meter provider |
| <code>interface.15.met-otel-tendermint-mempool-compact-duration-seconds-j57</code> | notes | OpenTelemetry instrument exported through the node meter provider |
| <code>interface.15.met-otel-tendermint-mempool-compact-total-j57</code> | notes | OpenTelemetry instrument exported through the node meter provider |
| <code>interface.15.met-prom-pax-data-latency-j57</code> | notes | Prometheus collector registered by the owning package |

## openapi

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.15.fmt-consensus-rpc-openapi-j57</code> | <code>file_format</code> | <code>change</code> | <code>consensus/rpc/openapi/openapi.yaml:1</code> | <code>openapi: 3.0.0 document describing the consensus JSON-RPC routes</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.15.fmt-consensus-rpc-openapi-j57</code> | notes | documents dial_peers and dial_seeds paths that NewRoutesMap does not register |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.15.266.1</code> | req | 15266.1 |
| <code>acceptance.15.266.1</code> | scenario | Given consensus/rpc/openapi/openapi.yaml without /dial_seeds and /dial_peers (planned), when the test compares its paths with the RoutesMap of NewRoutesMap with Unsafe true, then every documented path is a registered route |
| <code>acceptance.15.266.1</code> | test | consensus/internal/rpc/core/openapi_routes_test.go::TestOpenAPIPathsMatchRoutesMap (planned) |
| <code>acceptance.15.266.1</code> | qualification | UNRUN |
| <code>req.15266</code> | ac_1 | Every path of the consensus OpenAPI document is a registered JSON-RPC route; at HEAD consensus/rpc/openapi/openapi.yaml:688-769 documents dial_seeds and dial_peers, which consensus/internal/rpc/core/routes.go:24-80 does not register. |
| <code>capability.15.24.15</code> | title | Drop the unregistered dial_seeds and dial_peers paths from the OpenAPI document |
| <code>capability.15.24.15</code> | reqs | 15266.1 |

## proto

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.15.grpcs-consensus-proto-tendermint-privval-priv-validator-api-get-pub-key-j57</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>consensus/proto/tendermint/privval/service.proto:13</code> | <code>rpc GetPubKey(PubKeyRequest) returns (PubKeyResponse)</code> |
| <code>interface.15.grpcs-consensus-proto-tendermint-privval-priv-validator-api-sign-proposal-j57</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>consensus/proto/tendermint/privval/service.proto:15</code> | <code>rpc SignProposal(SignProposalRequest) returns (SignedProposalResponse)</code> |
| <code>interface.15.grpcs-consensus-proto-tendermint-privval-priv-validator-api-sign-vote-j57</code> | <code>grpc_service_method</code> | <code>stable</code> | <code>consensus/proto/tendermint/privval/service.proto:14</code> | <code>rpc SignVote(SignVoteRequest) returns (SignedVoteResponse)</code> |
| <code>interface.15.proto-consensus-internal-autobahn-app-proposal-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:186</code> | <code>message AppProposal</code> |
| <code>interface.15.proto-consensus-internal-autobahn-app-qc-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:181</code> | <code>message AppQC</code> |
| <code>interface.15.proto-consensus-internal-autobahn-block-header-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:69</code> | <code>message BlockHeader</code> |
| <code>interface.15.proto-consensus-internal-autobahn-block-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:87</code> | <code>message Block</code> |
| <code>interface.15.proto-consensus-internal-autobahn-commit-qc-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:134</code> | <code>message CommitQC</code> |
| <code>interface.15.proto-consensus-internal-autobahn-consensus-req-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:222</code> | <code>message ConsensusReq</code> |
| <code>interface.15.proto-consensus-internal-autobahn-duration-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:28</code> | <code>message Duration</code> |
| <code>interface.15.proto-consensus-internal-autobahn-full-commit-qc-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:139</code> | <code>message FullCommitQC</code> |
| <code>interface.15.proto-consensus-internal-autobahn-full-proposal-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:122</code> | <code>message FullProposal</code> |
| <code>interface.15.proto-consensus-internal-autobahn-full-timeout-vote-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:155</code> | <code>message FullTimeoutVote</code> |
| <code>interface.15.proto-consensus-internal-autobahn-lane-qc-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:93</code> | <code>message LaneQC</code> |
| <code>interface.15.proto-consensus-internal-autobahn-lane-range-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:98</code> | <code>message LaneRange</code> |
| <code>interface.15.proto-consensus-internal-autobahn-msg-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:198</code> | <code>message Msg</code> |
| <code>interface.15.proto-consensus-internal-autobahn-payload-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:77</code> | <code>message Payload</code> |
| <code>interface.15.proto-consensus-internal-autobahn-persisted-avail-prune-anchor-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:176</code> | <code>message PersistedAvailPruneAnchor</code> |
| <code>interface.15.proto-consensus-internal-autobahn-persisted-inner-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:164</code> | <code>message PersistedInner</code> |
| <code>interface.15.proto-consensus-internal-autobahn-prepare-qc-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:129</code> | <code>message PrepareQC</code> |
| <code>interface.15.proto-consensus-internal-autobahn-proposal-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:112</code> | <code>message Proposal</code> |
| <code>interface.15.proto-consensus-internal-autobahn-public-key-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:58</code> | <code>message PublicKey</code> |
| <code>interface.15.proto-consensus-internal-autobahn-signature-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:63</code> | <code>message Signature</code> |
| <code>interface.15.proto-consensus-internal-autobahn-signed-msg-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:216</code> | <code>message SignedMsg</code> |
| <code>interface.15.proto-consensus-internal-autobahn-timeout-qc-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:150</code> | <code>message TimeoutQC</code> |
| <code>interface.15.proto-consensus-internal-autobahn-timeout-vote-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:144</code> | <code>message TimeoutVote</code> |
| <code>interface.15.proto-consensus-internal-autobahn-timestamp-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:13</code> | <code>message Timestamp</code> |
| <code>interface.15.proto-consensus-internal-autobahn-transaction-header-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:39</code> | <code>message TransactionHeader</code> |
| <code>interface.15.proto-consensus-internal-autobahn-transaction-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:45</code> | <code>message Transaction</code> |
| <code>interface.15.proto-consensus-internal-autobahn-transaction-resp-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:52</code> | <code>message TransactionResp</code> |
| <code>interface.15.proto-consensus-internal-autobahn-transaction-timestamps-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:34</code> | <code>message TransactionTimestamps</code> |
| <code>interface.15.proto-consensus-internal-autobahn-view-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/autobahn/autobahn.proto:106</code> | <code>message View</code> |
| <code>interface.15.proto-consensus-internal-hashable-internal-testonly-enum-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/hashable/internal/testonly.proto:9</code> | <code>enum TestonlyEnum</code> |
| <code>interface.15.proto-consensus-internal-hashable-internal-testonly-msg-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/hashable/internal/testonly.proto:27</code> | <code>message TestonlyMsg</code> |
| <code>interface.15.proto-consensus-internal-hashable-internal-testonly-nested-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/hashable/internal/testonly.proto:15</code> | <code>message TestonlyNested</code> |
| <code>interface.15.proto-consensus-internal-p2p-giga-app-vote-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.proto:32</code> | <code>message AppVote</code> |
| <code>interface.15.proto-consensus-internal-p2p-giga-consensus-resp-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.proto:19</code> | <code>message ConsensusResp {}</code> |
| <code>interface.15.proto-consensus-internal-p2p-giga-get-block-req-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.proto:50</code> | <code>message GetBlockReq</code> |
| <code>interface.15.proto-consensus-internal-p2p-giga-get-block-resp-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.proto:54</code> | <code>message GetBlockResp</code> |
| <code>interface.15.proto-consensus-internal-p2p-giga-lane-proposal-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.proto:28</code> | <code>message LaneProposal</code> |
| <code>interface.15.proto-consensus-internal-p2p-giga-lane-q-cs-resp-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.proto:17</code> | <code>message LaneQCsResp {}</code> |
| <code>interface.15.proto-consensus-internal-p2p-giga-lane-req-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.proto:9</code> | <code>message LaneReq</code> |
| <code>interface.15.proto-consensus-internal-p2p-giga-lane-resp-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.proto:13</code> | <code>message LaneResp</code> |
| <code>interface.15.proto-consensus-internal-p2p-giga-lane-vote-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.proto:24</code> | <code>message LaneVote</code> |
| <code>interface.15.proto-consensus-internal-p2p-giga-ping-req-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.proto:21</code> | <code>message PingReq {}</code> |
| <code>interface.15.proto-consensus-internal-p2p-giga-ping-resp-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.proto:22</code> | <code>message PingResp {}</code> |
| <code>interface.15.proto-consensus-internal-p2p-giga-stream-app-q-cs-req-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.proto:40</code> | <code>message StreamAppQCsReq {}</code> |
| <code>interface.15.proto-consensus-internal-p2p-giga-stream-app-q-cs-resp-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.proto:41</code> | <code>message StreamAppQCsResp</code> |
| <code>interface.15.proto-consensus-internal-p2p-giga-stream-app-votes-req-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.proto:48</code> | <code>message StreamAppVotesReq {}</code> |
| <code>interface.15.proto-consensus-internal-p2p-giga-stream-commit-q-cs-req-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.proto:46</code> | <code>message StreamCommitQCsReq {}</code> |
| <code>interface.15.proto-consensus-internal-p2p-giga-stream-full-commit-q-cs-req-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.proto:58</code> | <code>message StreamFullCommitQCsReq</code> |
| <code>interface.15.proto-consensus-internal-p2p-giga-stream-lane-proposals-req-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.proto:36</code> | <code>message StreamLaneProposalsReq</code> |
| <code>interface.15.proto-consensus-internal-p2p-giga-stream-lane-votes-req-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/giga/api.proto:47</code> | <code>message StreamLaneVotesReq {}</code> |
| <code>interface.15.proto-consensus-internal-p2p-handshake-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/p2p.proto:36</code> | <code>message Handshake</code> |
| <code>interface.15.proto-consensus-internal-p2p-mux-handshake-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/mux/mux.proto:13</code> | <code>message Handshake</code> |
| <code>interface.15.proto-consensus-internal-p2p-mux-header-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/mux/mux.proto:25</code> | <code>message Header</code> |
| <code>interface.15.proto-consensus-internal-p2p-mux-stream-kind-config-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/mux/mux.proto:7</code> | <code>message StreamKindConfig</code> |
| <code>interface.15.proto-consensus-internal-p2p-node-public-key-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/p2p.proto:25</code> | <code>message NodePublicKey</code> |
| <code>interface.15.proto-consensus-internal-p2p-packet-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/p2p.proto:17</code> | <code>message Packet</code> |
| <code>interface.15.proto-consensus-internal-p2p-packet-msg-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/p2p.proto:11</code> | <code>message PacketMsg</code> |
| <code>interface.15.proto-consensus-internal-p2p-packet-ping-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/p2p.proto:7</code> | <code>message PacketPing {}</code> |
| <code>interface.15.proto-consensus-internal-p2p-packet-pong-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/p2p.proto:9</code> | <code>message PacketPong {}</code> |
| <code>interface.15.proto-consensus-internal-p2p-preface-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/p2p/p2p.proto:31</code> | <code>message Preface</code> |
| <code>interface.15.proto-consensus-internal-protoutils-testonly-child-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/protoutils/testonly.proto:15</code> | <code>message TestonlyChild</code> |
| <code>interface.15.proto-consensus-internal-protoutils-testonly-msg-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/internal/protoutils/testonly.proto:7</code> | <code>message TestonlyMsg</code> |
| <code>interface.15.proto-consensus-proto-tendermint-abci-block-params-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/abci/types.proto:149</code> | <code>message BlockParams</code> |
| <code>interface.15.proto-consensus-proto-tendermint-abci-consensus-params-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/abci/types.proto:160</code> | <code>message ConsensusParams</code> |
| <code>interface.15.proto-consensus-proto-tendermint-abci-event-attribute-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/abci/types.proto:115</code> | <code>message EventAttribute</code> |
| <code>interface.15.proto-consensus-proto-tendermint-abci-event-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/abci/types.proto:106</code> | <code>message Event</code> |
| <code>interface.15.proto-consensus-proto-tendermint-abci-evm-tx-info-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/abci/types.proto:182</code> | <code>message EvmTxInfo</code> |
| <code>interface.15.proto-consensus-proto-tendermint-abci-exec-tx-result-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/abci/types.proto:124</code> | <code>message ExecTxResult</code> |
| <code>interface.15.proto-consensus-proto-tendermint-abci-misbehavior-type-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/abci/types.proto:176</code> | <code>enum MisbehaviorType</code> |
| <code>interface.15.proto-consensus-proto-tendermint-abci-response-begin-block-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/abci/types.proto:40</code> | <code>message ResponseBeginBlock</code> |
| <code>interface.15.proto-consensus-proto-tendermint-abci-response-check-tx-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/abci/types.proto:47</code> | <code>message ResponseCheckTx</code> |
| <code>interface.15.proto-consensus-proto-tendermint-abci-response-commit-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/abci/types.proto:84</code> | <code>message ResponseCommit</code> |
| <code>interface.15.proto-consensus-proto-tendermint-abci-response-deliver-tx-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/abci/types.proto:60</code> | <code>message ResponseDeliverTx</code> |
| <code>interface.15.proto-consensus-proto-tendermint-abci-response-end-block-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/abci/types.proto:75</code> | <code>message ResponseEndBlock</code> |
| <code>interface.15.proto-consensus-proto-tendermint-abci-response-finalize-block-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/abci/types.proto:89</code> | <code>message ResponseFinalizeBlock</code> |
| <code>interface.15.proto-consensus-proto-tendermint-abci-response-info-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/abci/types.proto:18</code> | <code>message ResponseInfo</code> |
| <code>interface.15.proto-consensus-proto-tendermint-abci-response-query-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/abci/types.proto:27</code> | <code>message ResponseQuery</code> |
| <code>interface.15.proto-consensus-proto-tendermint-abci-tx-result-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/abci/types.proto:142</code> | <code>message TxResult</code> |
| <code>interface.15.proto-consensus-proto-tendermint-abci-validator-update-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/abci/types.proto:171</code> | <code>message ValidatorUpdate</code> |
| <code>interface.15.proto-consensus-proto-tendermint-blocksync-block-request-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/blocksync/types.proto:10</code> | <code>message BlockRequest</code> |
| <code>interface.15.proto-consensus-proto-tendermint-blocksync-block-response-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/blocksync/types.proto:21</code> | <code>message BlockResponse</code> |
| <code>interface.15.proto-consensus-proto-tendermint-blocksync-message-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/blocksync/types.proto:36</code> | <code>message Message</code> |
| <code>interface.15.proto-consensus-proto-tendermint-blocksync-no-block-response-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/blocksync/types.proto:16</code> | <code>message NoBlockResponse</code> |
| <code>interface.15.proto-consensus-proto-tendermint-blocksync-status-request-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/blocksync/types.proto:28</code> | <code>message StatusRequest {}</code> |
| <code>interface.15.proto-consensus-proto-tendermint-blocksync-status-response-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/blocksync/types.proto:31</code> | <code>message StatusResponse</code> |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-block-part-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/consensus/types.proto:47</code> | <code>message BlockPart</code> |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-end-height-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/consensus/wal.proto:32</code> | <code>message EndHeight</code> |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-has-vote-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/consensus/types.proto:59</code> | <code>message HasVote</code> |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-message-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/consensus/types.proto:90</code> | <code>message Message</code> |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-msg-info-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/consensus/wal.proto:14</code> | <code>message MsgInfo</code> |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-new-round-step-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/consensus/types.proto:13</code> | <code>message NewRoundStep</code> |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-new-valid-block-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/consensus/types.proto:26</code> | <code>message NewValidBlock</code> |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-proposal-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/consensus/types.proto:35</code> | <code>message Proposal</code> |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-proposal-pol-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/consensus/types.proto:40</code> | <code>message ProposalPOL</code> |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-timed-wal-message-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/consensus/wal.proto:46</code> | <code>message TimedWALMessage</code> |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-timeout-info-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/consensus/wal.proto:20</code> | <code>message TimeoutInfo</code> |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-vote-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/consensus/types.proto:54</code> | <code>message Vote</code> |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-vote-set-bits-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/consensus/types.proto:79</code> | <code>message VoteSetBits</code> |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-vote-set-maj23-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/consensus/types.proto:66</code> | <code>message VoteSetMaj23</code> |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-wal-message-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/consensus/wal.proto:36</code> | <code>message WALMessage</code> |
| <code>interface.15.proto-consensus-proto-tendermint-crypto-domino-op-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/crypto/proof.proto:24</code> | <code>message DominoOp</code> |
| <code>interface.15.proto-consensus-proto-tendermint-crypto-proof-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/crypto/proof.proto:9</code> | <code>message Proof</code> |
| <code>interface.15.proto-consensus-proto-tendermint-crypto-proof-op-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/crypto/proof.proto:33</code> | <code>message ProofOp</code> |
| <code>interface.15.proto-consensus-proto-tendermint-crypto-proof-ops-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/crypto/proof.proto:40</code> | <code>message ProofOps</code> |
| <code>interface.15.proto-consensus-proto-tendermint-crypto-public-key-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/crypto/keys.proto:10</code> | <code>message PublicKey</code> |
| <code>interface.15.proto-consensus-proto-tendermint-crypto-value-op-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/crypto/proof.proto:16</code> | <code>message ValueOp</code> |
| <code>interface.15.proto-consensus-proto-tendermint-libs-bits-bit-array-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/libs/bits/types.proto:7</code> | <code>message BitArray</code> |
| <code>interface.15.proto-consensus-proto-tendermint-mempool-message-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/mempool/types.proto:11</code> | <code>message Message</code> |
| <code>interface.15.proto-consensus-proto-tendermint-mempool-txs-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/mempool/types.proto:7</code> | <code>message Txs</code> |
| <code>interface.15.proto-consensus-proto-tendermint-p2p-node-info-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/p2p/types.proto:16</code> | <code>message NodeInfo</code> |
| <code>interface.15.proto-consensus-proto-tendermint-p2p-node-info-other-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/p2p/types.proto:27</code> | <code>message NodeInfoOther</code> |
| <code>interface.15.proto-consensus-proto-tendermint-p2p-peer-address-info-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/p2p/types.proto:38</code> | <code>message PeerAddressInfo</code> |
| <code>interface.15.proto-consensus-proto-tendermint-p2p-peer-info-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/p2p/types.proto:32</code> | <code>message PeerInfo</code> |
| <code>interface.15.proto-consensus-proto-tendermint-p2p-pex-address-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/p2p/pex.proto:7</code> | <code>message PexAddress</code> |
| <code>interface.15.proto-consensus-proto-tendermint-p2p-pex-message-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/p2p/pex.proto:18</code> | <code>message PexMessage</code> |
| <code>interface.15.proto-consensus-proto-tendermint-p2p-pex-request-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/p2p/pex.proto:12</code> | <code>message PexRequest {}</code> |
| <code>interface.15.proto-consensus-proto-tendermint-p2p-pex-response-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/p2p/pex.proto:14</code> | <code>message PexResponse</code> |
| <code>interface.15.proto-consensus-proto-tendermint-p2p-protocol-version-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/p2p/types.proto:10</code> | <code>message ProtocolVersion</code> |
| <code>interface.15.proto-consensus-proto-tendermint-privval-auth-sig-message-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/privval/types.proto:82</code> | <code>message AuthSigMessage</code> |
| <code>interface.15.proto-consensus-proto-tendermint-privval-errors-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/privval/types.proto:11</code> | <code>enum Errors</code> |
| <code>interface.15.proto-consensus-proto-tendermint-privval-message-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/privval/types.proto:66</code> | <code>message Message</code> |
| <code>interface.15.proto-consensus-proto-tendermint-privval-ping-request-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/privval/types.proto:61</code> | <code>message PingRequest {}</code> |
| <code>interface.15.proto-consensus-proto-tendermint-privval-ping-response-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/privval/types.proto:64</code> | <code>message PingResponse {}</code> |
| <code>interface.15.proto-consensus-proto-tendermint-privval-pub-key-request-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/privval/types.proto:26</code> | <code>message PubKeyRequest</code> |
| <code>interface.15.proto-consensus-proto-tendermint-privval-pub-key-response-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/privval/types.proto:31</code> | <code>message PubKeyResponse</code> |
| <code>interface.15.proto-consensus-proto-tendermint-privval-remote-signer-error-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/privval/types.proto:20</code> | <code>message RemoteSignerError</code> |
| <code>interface.15.proto-consensus-proto-tendermint-privval-sign-proposal-request-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/privval/types.proto:49</code> | <code>message SignProposalRequest</code> |
| <code>interface.15.proto-consensus-proto-tendermint-privval-sign-vote-request-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/privval/types.proto:37</code> | <code>message SignVoteRequest</code> |
| <code>interface.15.proto-consensus-proto-tendermint-privval-signed-proposal-response-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/privval/types.proto:55</code> | <code>message SignedProposalResponse</code> |
| <code>interface.15.proto-consensus-proto-tendermint-privval-signed-vote-response-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/privval/types.proto:43</code> | <code>message SignedVoteResponse</code> |
| <code>interface.15.proto-consensus-proto-tendermint-state-abci-responses-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/state/types.proto:15</code> | <code>message ABCIResponses</code> |
| <code>interface.15.proto-consensus-proto-tendermint-state-consensus-params-info-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/state/types.proto:28</code> | <code>message ConsensusParamsInfo</code> |
| <code>interface.15.proto-consensus-proto-tendermint-state-state-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/state/types.proto:38</code> | <code>message State</code> |
| <code>interface.15.proto-consensus-proto-tendermint-state-validators-info-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/state/types.proto:22</code> | <code>message ValidatorsInfo</code> |
| <code>interface.15.proto-consensus-proto-tendermint-state-version-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/state/types.proto:33</code> | <code>message Version</code> |
| <code>interface.15.proto-consensus-proto-tendermint-statesync-chunk-request-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/statesync/types.proto:34</code> | <code>message ChunkRequest</code> |
| <code>interface.15.proto-consensus-proto-tendermint-statesync-chunk-response-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/statesync/types.proto:40</code> | <code>message ChunkResponse</code> |
| <code>interface.15.proto-consensus-proto-tendermint-statesync-light-block-request-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/statesync/types.proto:48</code> | <code>message LightBlockRequest</code> |
| <code>interface.15.proto-consensus-proto-tendermint-statesync-light-block-response-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/statesync/types.proto:52</code> | <code>message LightBlockResponse</code> |
| <code>interface.15.proto-consensus-proto-tendermint-statesync-message-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/statesync/types.proto:11</code> | <code>message Message</code> |
| <code>interface.15.proto-consensus-proto-tendermint-statesync-params-request-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/statesync/types.proto:56</code> | <code>message ParamsRequest</code> |
| <code>interface.15.proto-consensus-proto-tendermint-statesync-params-response-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/statesync/types.proto:60</code> | <code>message ParamsResponse</code> |
| <code>interface.15.proto-consensus-proto-tendermint-statesync-snapshots-request-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/statesync/types.proto:24</code> | <code>message SnapshotsRequest {}</code> |
| <code>interface.15.proto-consensus-proto-tendermint-statesync-snapshots-response-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/statesync/types.proto:26</code> | <code>message SnapshotsResponse</code> |
| <code>interface.15.proto-consensus-proto-tendermint-store-block-store-state-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/store/types.proto:7</code> | <code>message BlockStoreState</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-abci-params-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/params.proto:140</code> | <code>message ABCIParams</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-block-id-flag-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:15</code> | <code>enum BlockIDFlag</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-block-id-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:52</code> | <code>message BlockID</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-block-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/block.proto:10</code> | <code>message Block</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-block-meta-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:182</code> | <code>message BlockMeta</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-block-params-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/params.proto:24</code> | <code>message BlockParams</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-canonical-block-id-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/canonical.proto:11</code> | <code>message CanonicalBlockID</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-canonical-part-set-header-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/canonical.proto:16</code> | <code>message CanonicalPartSetHeader</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-canonical-proposal-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/canonical.proto:21</code> | <code>message CanonicalProposal</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-canonical-vote-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/canonical.proto:34</code> | <code>message CanonicalVote</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-commit-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:127</code> | <code>message Commit</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-commit-sig-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:141</code> | <code>message CommitSig</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-consensus-params-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/params.proto:13</code> | <code>message ConsensusParams</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-data-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:90</code> | <code>message Data</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-duplicate-vote-evidence-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:209</code> | <code>message DuplicateVoteEvidence</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-event-data-round-state-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/events.proto:7</code> | <code>message EventDataRoundState</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-evidence-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:200</code> | <code>message Evidence</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-evidence-list-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:233</code> | <code>message EvidenceList</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-evidence-params-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/params.proto:39</code> | <code>message EvidenceParams</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-hashed-params-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/params.proto:76</code> | <code>message HashedParams</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-header-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:60</code> | <code>message Header</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-light-block-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:177</code> | <code>message LightBlock</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-light-client-attack-evidence-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:222</code> | <code>message LightClientAttackEvidence</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-part-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:45</code> | <code>message Part</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-part-set-header-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:40</code> | <code>message PartSetHeader</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-proposal-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:151</code> | <code>message Proposal</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-signed-header-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:172</code> | <code>message SignedHeader</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-signed-msg-type-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:26</code> | <code>enum SignedMsgType</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-simple-validator-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/validator.proto:23</code> | <code>message SimpleValidator</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-synchrony-params-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/params.proto:85</code> | <code>message SynchronyParams</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-timeout-params-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/params.proto:95</code> | <code>message TimeoutParams</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-tx-key-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:97</code> | <code>message TxKey</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-tx-proof-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:194</code> | <code>message TxProof</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-validator-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/validator.proto:16</code> | <code>message Validator</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-validator-params-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/params.proto:64</code> | <code>message ValidatorParams</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-validator-set-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/validator.proto:10</code> | <code>message ValidatorSet</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-version-params-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/params.proto:69</code> | <code>message VersionParams</code> |
| <code>interface.15.proto-consensus-proto-tendermint-types-vote-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/types/types.proto:103</code> | <code>message Vote</code> |
| <code>interface.15.proto-consensus-proto-tendermint-version-consensus-j57</code> | <code>proto_message</code> | <code>stable</code> | <code>consensus/proto/tendermint/version/types.proto:12</code> | <code>message Consensus</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.15.grpcs-consensus-proto-tendermint-privval-priv-validator-api-get-pub-key-j57</code> | notes | service tendermint.privval.PrivValidatorAPI |
| <code>interface.15.grpcs-consensus-proto-tendermint-privval-priv-validator-api-sign-proposal-j57</code> | notes | service tendermint.privval.PrivValidatorAPI |
| <code>interface.15.grpcs-consensus-proto-tendermint-privval-priv-validator-api-sign-vote-j57</code> | notes | service tendermint.privval.PrivValidatorAPI |
| <code>interface.15.proto-consensus-internal-autobahn-app-proposal-j57</code> | notes | fq autobahn.AppProposal; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-app-qc-j57</code> | notes | fq autobahn.AppQC; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-block-header-j57</code> | notes | fq autobahn.BlockHeader; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-block-j57</code> | notes | fq autobahn.Block; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-commit-qc-j57</code> | notes | fq autobahn.CommitQC; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-consensus-req-j57</code> | notes | fq autobahn.ConsensusReq; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-duration-j57</code> | notes | fq autobahn.Duration; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-full-commit-qc-j57</code> | notes | fq autobahn.FullCommitQC; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-full-proposal-j57</code> | notes | fq autobahn.FullProposal; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-full-timeout-vote-j57</code> | notes | fq autobahn.FullTimeoutVote; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-lane-qc-j57</code> | notes | fq autobahn.LaneQC; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-lane-range-j57</code> | notes | fq autobahn.LaneRange; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-msg-j57</code> | notes | fq autobahn.Msg; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-payload-j57</code> | notes | fq autobahn.Payload; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-persisted-avail-prune-anchor-j57</code> | notes | fq autobahn.PersistedAvailPruneAnchor; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-persisted-inner-j57</code> | notes | fq autobahn.PersistedInner; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-prepare-qc-j57</code> | notes | fq autobahn.PrepareQC; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-proposal-j57</code> | notes | fq autobahn.Proposal; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-public-key-j57</code> | notes | fq autobahn.PublicKey; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-signature-j57</code> | notes | fq autobahn.Signature; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-signed-msg-j57</code> | notes | fq autobahn.SignedMsg; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-timeout-qc-j57</code> | notes | fq autobahn.TimeoutQC; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-timeout-vote-j57</code> | notes | fq autobahn.TimeoutVote; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-timestamp-j57</code> | notes | fq autobahn.Timestamp; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-transaction-header-j57</code> | notes | fq autobahn.TransactionHeader; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-transaction-j57</code> | notes | fq autobahn.Transaction; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-transaction-resp-j57</code> | notes | fq autobahn.TransactionResp; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-transaction-timestamps-j57</code> | notes | fq autobahn.TransactionTimestamps; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-autobahn-view-j57</code> | notes | fq autobahn.View; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-hashable-internal-testonly-enum-j57</code> | notes | fq hashable.internal.TestonlyEnum; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-hashable-internal-testonly-msg-j57</code> | notes | fq hashable.internal.TestonlyMsg; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-hashable-internal-testonly-nested-j57</code> | notes | fq hashable.internal.TestonlyNested; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-giga-app-vote-j57</code> | notes | fq p2p.giga.AppVote; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-giga-consensus-resp-j57</code> | notes | fq p2p.giga.ConsensusResp; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-giga-get-block-req-j57</code> | notes | fq p2p.giga.GetBlockReq; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-giga-get-block-resp-j57</code> | notes | fq p2p.giga.GetBlockResp; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-giga-lane-proposal-j57</code> | notes | fq p2p.giga.LaneProposal; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-giga-lane-q-cs-resp-j57</code> | notes | fq p2p.giga.LaneQCsResp; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-giga-lane-req-j57</code> | notes | fq p2p.giga.LaneReq; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-giga-lane-resp-j57</code> | notes | fq p2p.giga.LaneResp; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-giga-lane-vote-j57</code> | notes | fq p2p.giga.LaneVote; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-giga-ping-req-j57</code> | notes | fq p2p.giga.PingReq; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-giga-ping-resp-j57</code> | notes | fq p2p.giga.PingResp; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-giga-stream-app-q-cs-req-j57</code> | notes | fq p2p.giga.StreamAppQCsReq; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-giga-stream-app-q-cs-resp-j57</code> | notes | fq p2p.giga.StreamAppQCsResp; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-giga-stream-app-votes-req-j57</code> | notes | fq p2p.giga.StreamAppVotesReq; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-giga-stream-commit-q-cs-req-j57</code> | notes | fq p2p.giga.StreamCommitQCsReq; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-giga-stream-full-commit-q-cs-req-j57</code> | notes | fq p2p.giga.StreamFullCommitQCsReq; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-giga-stream-lane-proposals-req-j57</code> | notes | fq p2p.giga.StreamLaneProposalsReq; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-giga-stream-lane-votes-req-j57</code> | notes | fq p2p.giga.StreamLaneVotesReq; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-handshake-j57</code> | notes | fq p2p.Handshake; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-mux-handshake-j57</code> | notes | fq mux.Handshake; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-mux-header-j57</code> | notes | fq mux.Header; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-mux-stream-kind-config-j57</code> | notes | fq mux.StreamKindConfig; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-node-public-key-j57</code> | notes | fq p2p.NodePublicKey; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-packet-j57</code> | notes | fq p2p.Packet; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-packet-msg-j57</code> | notes | fq p2p.PacketMsg; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-packet-ping-j57</code> | notes | fq p2p.PacketPing; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-packet-pong-j57</code> | notes | fq p2p.PacketPong; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-p2p-preface-j57</code> | notes | fq p2p.Preface; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-protoutils-testonly-child-j57</code> | notes | fq protoutils.TestonlyChild; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-internal-protoutils-testonly-msg-j57</code> | notes | fq protoutils.TestonlyMsg; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-abci-block-params-j57</code> | notes | fq tendermint.abci.BlockParams; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-abci-consensus-params-j57</code> | notes | fq tendermint.abci.ConsensusParams; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-abci-event-attribute-j57</code> | notes | fq tendermint.abci.EventAttribute; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-abci-event-j57</code> | notes | fq tendermint.abci.Event; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-abci-evm-tx-info-j57</code> | notes | fq tendermint.abci.EvmTxInfo; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-abci-exec-tx-result-j57</code> | notes | fq tendermint.abci.ExecTxResult; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-abci-misbehavior-type-j57</code> | notes | fq tendermint.abci.MisbehaviorType; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-abci-response-begin-block-j57</code> | notes | fq tendermint.abci.ResponseBeginBlock; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-abci-response-check-tx-j57</code> | notes | fq tendermint.abci.ResponseCheckTx; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-abci-response-commit-j57</code> | notes | fq tendermint.abci.ResponseCommit; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-abci-response-deliver-tx-j57</code> | notes | fq tendermint.abci.ResponseDeliverTx; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-abci-response-end-block-j57</code> | notes | fq tendermint.abci.ResponseEndBlock; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-abci-response-finalize-block-j57</code> | notes | fq tendermint.abci.ResponseFinalizeBlock; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-abci-response-info-j57</code> | notes | fq tendermint.abci.ResponseInfo; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-abci-response-query-j57</code> | notes | fq tendermint.abci.ResponseQuery; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-abci-tx-result-j57</code> | notes | fq tendermint.abci.TxResult; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-abci-validator-update-j57</code> | notes | fq tendermint.abci.ValidatorUpdate; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-blocksync-block-request-j57</code> | notes | fq tendermint.blocksync.BlockRequest; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-blocksync-block-response-j57</code> | notes | fq tendermint.blocksync.BlockResponse; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-blocksync-message-j57</code> | notes | fq tendermint.blocksync.Message; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-blocksync-no-block-response-j57</code> | notes | fq tendermint.blocksync.NoBlockResponse; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-blocksync-status-request-j57</code> | notes | fq tendermint.blocksync.StatusRequest; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-blocksync-status-response-j57</code> | notes | fq tendermint.blocksync.StatusResponse; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-block-part-j57</code> | notes | fq tendermint.consensus.BlockPart; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-end-height-j57</code> | notes | fq tendermint.consensus.EndHeight; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-has-vote-j57</code> | notes | fq tendermint.consensus.HasVote; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-message-j57</code> | notes | fq tendermint.consensus.Message; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-msg-info-j57</code> | notes | fq tendermint.consensus.MsgInfo; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-new-round-step-j57</code> | notes | fq tendermint.consensus.NewRoundStep; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-new-valid-block-j57</code> | notes | fq tendermint.consensus.NewValidBlock; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-proposal-j57</code> | notes | fq tendermint.consensus.Proposal; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-proposal-pol-j57</code> | notes | fq tendermint.consensus.ProposalPOL; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-timed-wal-message-j57</code> | notes | fq tendermint.consensus.TimedWALMessage; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-timeout-info-j57</code> | notes | fq tendermint.consensus.TimeoutInfo; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-vote-j57</code> | notes | fq tendermint.consensus.Vote; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-vote-set-bits-j57</code> | notes | fq tendermint.consensus.VoteSetBits; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-vote-set-maj23-j57</code> | notes | fq tendermint.consensus.VoteSetMaj23; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-consensus-wal-message-j57</code> | notes | fq tendermint.consensus.WALMessage; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-crypto-domino-op-j57</code> | notes | fq tendermint.crypto.DominoOp; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-crypto-proof-j57</code> | notes | fq tendermint.crypto.Proof; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-crypto-proof-op-j57</code> | notes | fq tendermint.crypto.ProofOp; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-crypto-proof-ops-j57</code> | notes | fq tendermint.crypto.ProofOps; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-crypto-public-key-j57</code> | notes | fq tendermint.crypto.PublicKey; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-crypto-value-op-j57</code> | notes | fq tendermint.crypto.ValueOp; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-libs-bits-bit-array-j57</code> | notes | fq tendermint.libs.bits.BitArray; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-mempool-message-j57</code> | notes | fq tendermint.mempool.Message; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-mempool-txs-j57</code> | notes | fq tendermint.mempool.Txs; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-p2p-node-info-j57</code> | notes | fq tendermint.p2p.NodeInfo; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-p2p-node-info-other-j57</code> | notes | fq tendermint.p2p.NodeInfoOther; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-p2p-peer-address-info-j57</code> | notes | fq tendermint.p2p.PeerAddressInfo; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-p2p-peer-info-j57</code> | notes | fq tendermint.p2p.PeerInfo; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-p2p-pex-address-j57</code> | notes | fq tendermint.p2p.PexAddress; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-p2p-pex-message-j57</code> | notes | fq tendermint.p2p.PexMessage; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-p2p-pex-request-j57</code> | notes | fq tendermint.p2p.PexRequest; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-p2p-pex-response-j57</code> | notes | fq tendermint.p2p.PexResponse; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-p2p-protocol-version-j57</code> | notes | fq tendermint.p2p.ProtocolVersion; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-privval-auth-sig-message-j57</code> | notes | fq tendermint.privval.AuthSigMessage; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-privval-errors-j57</code> | notes | fq tendermint.privval.Errors; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-privval-message-j57</code> | notes | fq tendermint.privval.Message; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-privval-ping-request-j57</code> | notes | fq tendermint.privval.PingRequest; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-privval-ping-response-j57</code> | notes | fq tendermint.privval.PingResponse; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-privval-pub-key-request-j57</code> | notes | fq tendermint.privval.PubKeyRequest; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-privval-pub-key-response-j57</code> | notes | fq tendermint.privval.PubKeyResponse; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-privval-remote-signer-error-j57</code> | notes | fq tendermint.privval.RemoteSignerError; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-privval-sign-proposal-request-j57</code> | notes | fq tendermint.privval.SignProposalRequest; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-privval-sign-vote-request-j57</code> | notes | fq tendermint.privval.SignVoteRequest; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-privval-signed-proposal-response-j57</code> | notes | fq tendermint.privval.SignedProposalResponse; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-privval-signed-vote-response-j57</code> | notes | fq tendermint.privval.SignedVoteResponse; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-state-abci-responses-j57</code> | notes | fq tendermint.state.ABCIResponses; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-state-consensus-params-info-j57</code> | notes | fq tendermint.state.ConsensusParamsInfo; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-state-state-j57</code> | notes | fq tendermint.state.State; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-state-validators-info-j57</code> | notes | fq tendermint.state.ValidatorsInfo; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-state-version-j57</code> | notes | fq tendermint.state.Version; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-statesync-chunk-request-j57</code> | notes | fq tendermint.statesync.ChunkRequest; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-statesync-chunk-response-j57</code> | notes | fq tendermint.statesync.ChunkResponse; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-statesync-light-block-request-j57</code> | notes | fq tendermint.statesync.LightBlockRequest; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-statesync-light-block-response-j57</code> | notes | fq tendermint.statesync.LightBlockResponse; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-statesync-message-j57</code> | notes | fq tendermint.statesync.Message; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-statesync-params-request-j57</code> | notes | fq tendermint.statesync.ParamsRequest; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-statesync-params-response-j57</code> | notes | fq tendermint.statesync.ParamsResponse; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-statesync-snapshots-request-j57</code> | notes | fq tendermint.statesync.SnapshotsRequest; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-statesync-snapshots-response-j57</code> | notes | fq tendermint.statesync.SnapshotsResponse; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-store-block-store-state-j57</code> | notes | fq tendermint.store.BlockStoreState; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-abci-params-j57</code> | notes | fq tendermint.types.ABCIParams; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-block-id-flag-j57</code> | notes | fq tendermint.types.BlockIDFlag; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-block-id-j57</code> | notes | fq tendermint.types.BlockID; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-block-j57</code> | notes | fq tendermint.types.Block; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-block-meta-j57</code> | notes | fq tendermint.types.BlockMeta; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-block-params-j57</code> | notes | fq tendermint.types.BlockParams; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-canonical-block-id-j57</code> | notes | fq tendermint.types.CanonicalBlockID; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-canonical-part-set-header-j57</code> | notes | fq tendermint.types.CanonicalPartSetHeader; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-canonical-proposal-j57</code> | notes | fq tendermint.types.CanonicalProposal; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-canonical-vote-j57</code> | notes | fq tendermint.types.CanonicalVote; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-commit-j57</code> | notes | fq tendermint.types.Commit; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-commit-sig-j57</code> | notes | fq tendermint.types.CommitSig; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-consensus-params-j57</code> | notes | fq tendermint.types.ConsensusParams; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-data-j57</code> | notes | fq tendermint.types.Data; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-duplicate-vote-evidence-j57</code> | notes | fq tendermint.types.DuplicateVoteEvidence; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-event-data-round-state-j57</code> | notes | fq tendermint.types.EventDataRoundState; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-evidence-j57</code> | notes | fq tendermint.types.Evidence; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-evidence-list-j57</code> | notes | fq tendermint.types.EvidenceList; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-evidence-params-j57</code> | notes | fq tendermint.types.EvidenceParams; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-hashed-params-j57</code> | notes | fq tendermint.types.HashedParams; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-header-j57</code> | notes | fq tendermint.types.Header; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-light-block-j57</code> | notes | fq tendermint.types.LightBlock; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-light-client-attack-evidence-j57</code> | notes | fq tendermint.types.LightClientAttackEvidence; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-part-j57</code> | notes | fq tendermint.types.Part; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-part-set-header-j57</code> | notes | fq tendermint.types.PartSetHeader; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-proposal-j57</code> | notes | fq tendermint.types.Proposal; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-signed-header-j57</code> | notes | fq tendermint.types.SignedHeader; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-signed-msg-type-j57</code> | notes | fq tendermint.types.SignedMsgType; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-simple-validator-j57</code> | notes | fq tendermint.types.SimpleValidator; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-synchrony-params-j57</code> | notes | fq tendermint.types.SynchronyParams; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-timeout-params-j57</code> | notes | fq tendermint.types.TimeoutParams; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-tx-key-j57</code> | notes | fq tendermint.types.TxKey; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-tx-proof-j57</code> | notes | fq tendermint.types.TxProof; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-validator-j57</code> | notes | fq tendermint.types.Validator; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-validator-params-j57</code> | notes | fq tendermint.types.ValidatorParams; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-validator-set-j57</code> | notes | fq tendermint.types.ValidatorSet; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-version-params-j57</code> | notes | fq tendermint.types.VersionParams; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-types-vote-j57</code> | notes | fq tendermint.types.Vote; Go type generated beside the proto by the root buf.gen.yaml |
| <code>interface.15.proto-consensus-proto-tendermint-version-consensus-j57</code> | notes | fq tendermint.version.Consensus; Go type generated beside the proto by the root buf.gen.yaml |

## rpc-autobahn

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.15.go-consensus-internal-autobahn-data-state-first-block-j57</code> | <code>go_exported</code> | <code>new</code> | <code>consensus/internal/autobahn/data/state.go</code> (planned) | <code>func (s *State) FirstBlock() types.GlobalBlockNumber</code> |
| <code>interface.15.go-consensus-internal-p2p-giga-router-earliest-block-number-j57</code> | <code>go_exported</code> | <code>new</code> | <code>consensus/internal/p2p/giga_router.go</code> (planned) | <code>func (r *GigaRouter) EarliestBlockNumber() int64</code> |
| <code>interface.15.go-consensus-rpc-coretypes-err-consensus-state-not-available-j57</code> | <code>go_exported</code> | <code>new</code> | <code>consensus/rpc/coretypes/responses.go</code> (planned) | <code>ErrConsensusStateNotAvailable = errors.New("consensus round state is not available under autobahn consensus")</code> |
| <code>interface.15.go-consensus-rpc-coretypes-err-evidence-not-accepted-j57</code> | <code>go_exported</code> | <code>new</code> | <code>consensus/rpc/coretypes/responses.go</code> (planned) | <code>ErrEvidenceNotAccepted = errors.New("evidence is not accepted under autobahn consensus")</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.15.go-consensus-internal-autobahn-data-state-first-block-j57</code> | notes | planned accessor for inner.first (consensus/internal/autobahn/data/state.go:171) under the inner lock |
| <code>interface.15.go-consensus-internal-p2p-giga-router-earliest-block-number-j57</code> | notes | planned method returning the lowest retained global block number of the data state |
| <code>interface.15.go-consensus-rpc-coretypes-err-consensus-state-not-available-j57</code> | notes | planned addition to the error var block at consensus/rpc/coretypes/responses.go:20-30; returned by DumpConsensusState and GetConsensusState when a GigaRouter is present |
| <code>interface.15.go-consensus-rpc-coretypes-err-evidence-not-accepted-j57</code> | notes | planned addition to the error var block at consensus/rpc/coretypes/responses.go:20-30; returned by BroadcastEvidence when a GigaRouter is present |

Contract context <code>contract_context.15.autobahn-events-j57</code>:

| Field | Frozen rule |
|---|---|
| interface | GigaRouter.SetEventBus(BlockEventPublisher) planned in consensus/internal/p2p/giga_router.go and its publication from executeBlock at consensus/internal/p2p/giga_router.go:296 |
| storage | No storage; events feed the indexer service and its kv EventSink |
| errors | Publish errors are logged and never returned from executeBlock |
| events | tm.event='NewBlockHeader' with EventDataNewBlockHeader and tm.event='Tx' with EventDataTx, the types of consensus/types/events.go:242 |
| gas | Not metered |
| activation | Active once the node calls SetEventBus (wiring.J57.1) |
| invariants | Events of height h are published after SaveFinalizeBlockResponses(h) and before PushAppHash(h); one Tx event per tx in block order |
| pause | No pause |

Contract context <code>contract_context.15.rpc-autobahn-j57</code>:

| Field | Frozen rule |
|---|---|
| interface | Consensus JSON-RPC routes of consensus/internal/rpc/core/routes.go:24-80 served by Environment under Autobahn: status, lag_status, blockchain, header, header_by_hash, commit, tx, tx_search, block_search, consensus_params, dump_consensus_state, consensus_state, broadcast_evidence |
| storage | Reads GigaRouter.BlockByNumber, BlockByHash, LastCommittedBlockNumber and the planned EarliestBlockNumber; the CometBFT BlockStore and StateStore stay unfed except FinalizeBlock responses |
| errors | coretypes.ErrCommitNotAvailable, ErrConsensusStateNotAvailable, ErrEvidenceNotAccepted (planned) and ErrHeightNotAvailable from consensus/rpc/coretypes/responses.go:25 |
| events | None emitted by the routes |
| gas | Not metered; JSON-RPC handlers |
| activation | Active whenever env.gigaRouter() at consensus/internal/rpc/core/env.go:132 holds a router |
| invariants | No handler panics under Autobahn; every chain-data handler answers a result or a named sentinel; heights below EarliestBlockNumber answer ErrHeightNotAvailable |
| pause | No pause; Autobahn is selected by autobahn-config-file at consensus/config/config.go:80 |

## rpc-routes

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.15.go-consensus-rpc-coretypes-err-commit-not-available-j57</code> | <code>go_exported</code> | <code>new</code> | <code>consensus/rpc/coretypes/responses.go</code> (planned) | <code>ErrCommitNotAvailable = errors.New("commit is not available under autobahn consensus")</code> |
| <code>interface.15.rpc-consensus-abci-info-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:71</code> | <code>"abci_info": rpc.NewRPCFunc(svc.ABCIInfo)</code> |
| <code>interface.15.rpc-consensus-abci-query-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:70</code> | <code>"abci_query": rpc.NewRPCFunc(svc.ABCIQuery)</code> |
| <code>interface.15.rpc-consensus-block-by-hash-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:47</code> | <code>"block_by_hash": rpc.NewRPCFunc(svc.BlockByHash)</code> |
| <code>interface.15.rpc-consensus-block-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:46</code> | <code>"block": rpc.NewRPCFunc(svc.Block)</code> |
| <code>interface.15.rpc-consensus-block-results-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:48</code> | <code>"block_results": rpc.NewRPCFunc(svc.BlockResults)</code> |
| <code>interface.15.rpc-consensus-block-search-j57</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>consensus/internal/rpc/core/routes.go:53</code> | <code>"block_search": rpc.NewRPCFunc(svc.BlockSearch)</code> |
| <code>interface.15.rpc-consensus-blockchain-j57</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>consensus/internal/rpc/core/routes.go:41</code> | <code>"blockchain": rpc.NewRPCFunc(svc.BlockchainInfo)</code> |
| <code>interface.15.rpc-consensus-broadcast-evidence-j57</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>consensus/internal/rpc/core/routes.go:74</code> | <code>"broadcast_evidence": rpc.NewRPCFunc(svc.BroadcastEvidence)</code> |
| <code>interface.15.rpc-consensus-broadcast-tx-async-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:67</code> | <code>"broadcast_tx_async": rpc.NewRPCFunc(svc.BroadcastTxAsync)</code> |
| <code>interface.15.rpc-consensus-broadcast-tx-commit-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:65</code> | <code>"broadcast_tx_commit": rpc.NewRPCFunc(svc.BroadcastTxCommit)</code> |
| <code>interface.15.rpc-consensus-broadcast-tx-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:62</code> | <code>"broadcast_tx": rpc.NewRPCFunc(svc.BroadcastTx)</code> |
| <code>interface.15.rpc-consensus-broadcast-tx-sync-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:66</code> | <code>"broadcast_tx_sync": rpc.NewRPCFunc(svc.BroadcastTx)</code> |
| <code>interface.15.rpc-consensus-check-tx-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:50</code> | <code>"check_tx": rpc.NewRPCFunc(svc.CheckTx)</code> |
| <code>interface.15.rpc-consensus-commit-j57</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>consensus/internal/rpc/core/routes.go:49</code> | <code>"commit": rpc.NewRPCFunc(svc.Commit)</code> |
| <code>interface.15.rpc-consensus-consensus-params-j57</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>consensus/internal/rpc/core/routes.go:57</code> | <code>"consensus_params": rpc.NewRPCFunc(svc.ConsensusParams)</code> |
| <code>interface.15.rpc-consensus-consensus-state-j57</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>consensus/internal/rpc/core/routes.go:56</code> | <code>"consensus_state": rpc.NewRPCFunc(svc.GetConsensusState)</code> |
| <code>interface.15.rpc-consensus-dump-consensus-state-j57</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>consensus/internal/rpc/core/routes.go:55</code> | <code>"dump_consensus_state": rpc.NewRPCFunc(svc.DumpConsensusState)</code> |
| <code>interface.15.rpc-consensus-events-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:31</code> | <code>"events": rpc.NewRPCFunc(svc.Events).Timeout(0)</code> |
| <code>interface.15.rpc-consensus-genesis-chunked-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:43</code> | <code>"genesis_chunked": rpc.NewRPCFunc(svc.GenesisChunked)</code> |
| <code>interface.15.rpc-consensus-genesis-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:42</code> | <code>"genesis": rpc.NewRPCFunc(svc.Genesis)</code> |
| <code>interface.15.rpc-consensus-header-by-hash-j57</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>consensus/internal/rpc/core/routes.go:45</code> | <code>"header_by_hash": rpc.NewRPCFunc(svc.HeaderByHash)</code> |
| <code>interface.15.rpc-consensus-header-j57</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>consensus/internal/rpc/core/routes.go:44</code> | <code>"header": rpc.NewRPCFunc(svc.Header)</code> |
| <code>interface.15.rpc-consensus-health-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:37</code> | <code>"health": rpc.NewRPCFunc(svc.Health)</code> |
| <code>interface.15.rpc-consensus-lag-status-j57</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>consensus/internal/rpc/core/routes.go:39</code> | <code>"lag_status": rpc.NewRPCFunc(svc.LagStatus)</code> |
| <code>interface.15.rpc-consensus-net-info-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:40</code> | <code>"net_info": rpc.NewRPCFunc(svc.NetInfo)</code> |
| <code>interface.15.rpc-consensus-num-unconfirmed-txs-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:59</code> | <code>"num_unconfirmed_txs": rpc.NewRPCFunc(svc.NumUnconfirmedTxs)</code> |
| <code>interface.15.rpc-consensus-status-j57</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>consensus/internal/rpc/core/routes.go:38</code> | <code>"status": rpc.NewRPCFunc(svc.Status)</code> |
| <code>interface.15.rpc-consensus-subscribe-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:32</code> | <code>"subscribe": rpc.NewWSRPCFunc(svc.Subscribe)</code> |
| <code>interface.15.rpc-consensus-tx-j57</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>consensus/internal/rpc/core/routes.go:51</code> | <code>"tx": rpc.NewRPCFunc(svc.Tx)</code> |
| <code>interface.15.rpc-consensus-tx-search-j57</code> | <code>jsonrpc_method</code> | <code>change</code> | <code>consensus/internal/rpc/core/routes.go:52</code> | <code>"tx_search": rpc.NewRPCFunc(svc.TxSearch)</code> |
| <code>interface.15.rpc-consensus-unconfirmed-txs-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:58</code> | <code>"unconfirmed_txs": rpc.NewRPCFunc(svc.UnconfirmedTxs)</code> |
| <code>interface.15.rpc-consensus-unsafe-flush-mempool-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:77</code> | <code>out["unsafe_flush_mempool"] = rpc.NewRPCFunc(u.UnsafeFlushMempool)</code> |
| <code>interface.15.rpc-consensus-unsubscribe-all-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:34</code> | <code>"unsubscribe_all": rpc.NewWSRPCFunc(svc.UnsubscribeAll)</code> |
| <code>interface.15.rpc-consensus-unsubscribe-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:33</code> | <code>"unsubscribe": rpc.NewWSRPCFunc(svc.Unsubscribe)</code> |
| <code>interface.15.rpc-consensus-validators-j57</code> | <code>jsonrpc_method</code> | <code>stable</code> | <code>consensus/internal/rpc/core/routes.go:54</code> | <code>"validators": rpc.NewRPCFunc(svc.Validators)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.15.go-consensus-rpc-coretypes-err-commit-not-available-j57</code> | notes | sentinel returned by Environment.Commit when a GigaRouter is present; planned addition to the error var block at consensus/rpc/coretypes/responses.go:20-30 |
| <code>interface.15.rpc-consensus-abci-info-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-abci-query-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-block-by-hash-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-block-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-block-results-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-block-search-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-blockchain-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-broadcast-evidence-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-broadcast-tx-async-j57</code> | notes | no in-repo Go client call by method name; reached over HTTP and websocket by external clients |
| <code>interface.15.rpc-consensus-broadcast-tx-commit-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-broadcast-tx-j57</code> | notes | no in-repo Go client call by method name; reached over HTTP and websocket by external clients |
| <code>interface.15.rpc-consensus-broadcast-tx-sync-j57</code> | notes | no in-repo Go client call by method name; reached over HTTP and websocket by external clients |
| <code>interface.15.rpc-consensus-check-tx-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-commit-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-consensus-params-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-consensus-state-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-dump-consensus-state-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-events-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-genesis-chunked-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-genesis-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-header-by-hash-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-header-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-health-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-lag-status-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-net-info-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-num-unconfirmed-txs-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-status-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-subscribe-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-tx-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-tx-search-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-unconfirmed-txs-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-unsafe-flush-mempool-j57</code> | notes | no in-repo Go client call by method name; reached over HTTP and websocket by external clients; registered only when RouteOptions.Unsafe is set |
| <code>interface.15.rpc-consensus-unsubscribe-all-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-unsubscribe-j57</code> | notes | client calls through the consensus RPC client |
| <code>interface.15.rpc-consensus-validators-j57</code> | notes | client calls through the consensus RPC client |

Contract context <code>contract_context.15.rpc-autobahn-j57</code>:

| Field | Frozen rule |
|---|---|
| interface | Consensus JSON-RPC routes of consensus/internal/rpc/core/routes.go:24-80 served by Environment under Autobahn: status, lag_status, blockchain, header, header_by_hash, commit, tx, tx_search, block_search, consensus_params, dump_consensus_state, consensus_state, broadcast_evidence |
| storage | Reads GigaRouter.BlockByNumber, BlockByHash, LastCommittedBlockNumber and the planned EarliestBlockNumber; the CometBFT BlockStore and StateStore stay unfed except FinalizeBlock responses |
| errors | coretypes.ErrCommitNotAvailable, ErrConsensusStateNotAvailable, ErrEvidenceNotAccepted (planned) and ErrHeightNotAvailable from consensus/rpc/coretypes/responses.go:25 |
| events | None emitted by the routes |
| gas | Not metered; JSON-RPC handlers |
| activation | Active whenever env.gigaRouter() at consensus/internal/rpc/core/env.go:132 holds a router |
| invariants | No handler panics under Autobahn; every chain-data handler answers a result or a named sentinel; heights below EarliestBlockNumber answer ErrHeightNotAvailable |
| pause | No pause; Autobahn is selected by autobahn-config-file at consensus/config/config.go:80 |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.15.252.1</code> | req | 15252.1 |
| <code>acceptance.15.252.1</code> | scenario | Given the three planned sentinels, when the test compares them with errors.Is against each other and against ErrHeightNotAvailable and ErrHeightExceedsChainHead of consensus/rpc/coretypes/responses.go:25-26, then no pair matches and no sentinel message contains either of those two messages |
| <code>acceptance.15.252.1</code> | test | consensus/rpc/coretypes/responses_autobahn_test.go::TestAutobahnRPCSentinelsDistinct (planned) |
| <code>acceptance.15.252.1</code> | qualification | UNRUN |
| <code>req.15252</code> | ac_1 | ErrCommitNotAvailable, ErrConsensusStateNotAvailable and ErrEvidenceNotAccepted are distinct sentinels whose messages contain neither "height is not available" nor the chain-head message, so the light provider string match at consensus/light/provider/http/http.go:306-311 cannot mistake them for a missing block. |
| <code>acceptance.15.253.1</code> | req | 15253.1 |
| <code>acceptance.15.253.1</code> | scenario | Given an Environment whose GigaRouter finalized block 1 through newAutobahnTestRouter, when the test calls Commit with height 1 and again with a nil height, then both calls return a nil result and an error matching coretypes.ErrCommitNotAvailable under errors.Is |
| <code>acceptance.15.253.1</code> | test | consensus/internal/rpc/core/blocks_autobahn_commit_test.go::TestCommitUnderAutobahnReturnsErrCommitNotAvailable (planned) |
| <code>acceptance.15.253.1</code> | qualification | UNRUN |
| <code>req.15253</code> | ac_1 | Under Autobahn the commit route answers ErrCommitNotAvailable for every height; at HEAD Commit at consensus/internal/rpc/core/blocks.go:216-218 validates against the unfed BlockStore and answers ErrHeightExceedsChainHead, which the light provider maps to ErrHeightTooHigh and retries. |
| <code>acceptance.15.254.1</code> | req | 15254.1 |
| <code>acceptance.15.254.1</code> | scenario | Given a GigaRouter from newAutobahnTestRouter that finalized a block holding two txs and a kv EventSink holding the TxResult of the second tx, when the test calls Tx with that hash and Prove true, then the result Proof.Validate succeeds against the Merkle root of the block's Data.Txs |
| <code>acceptance.15.254.1</code> | test | consensus/internal/rpc/core/tx_autobahn_prove_test.go::TestTxProveUnderAutobahnReturnsProof (planned) |
| <code>acceptance.15.254.1</code> | qualification | UNRUN |
| <code>req.15254</code> | ac_1 | tx with prove under Autobahn returns a TxProof built from the finalized Autobahn block; at HEAD consensus/internal/rpc/core/tx.go:34-35 dereferences the nil block of the unfed BlockStore and panics. |
| <code>acceptance.15.254.2</code> | req | 15254.2 |
| <code>acceptance.15.254.2</code> | scenario | Given a classic Environment with an empty BlockStore and a kv EventSink holding a TxResult at height 5, when the test calls Tx with Prove true, then it returns an error matching coretypes.ErrHeightNotAvailable and does not panic |
| <code>acceptance.15.254.2</code> | test | consensus/internal/rpc/core/tx_autobahn_prove_test.go::TestTxProveMissingBlockReturnsErrHeightNotAvailable (planned) |
| <code>acceptance.15.254.2</code> | qualification | UNRUN |
| <code>req.15254</code> | ac_2 | tx with prove on the classic path answers ErrHeightNotAvailable when the block of an indexed tx is absent from the BlockStore. |
| <code>acceptance.15.255.1</code> | req | 15255.1 |
| <code>acceptance.15.255.1</code> | scenario | Given a GigaRouter that finalized a block with one tx and a kv EventSink holding its TxResult, when the test calls TxSearch with query tx.height=&lt;h&gt; and Prove true, then one result is returned and its Proof.Validate succeeds against the Merkle root of the block's Data.Txs |
| <code>acceptance.15.255.1</code> | test | consensus/internal/rpc/core/tx_autobahn_prove_test.go::TestTxSearchProveUnderAutobahnReturnsProof (planned) |
| <code>acceptance.15.255.1</code> | qualification | UNRUN |
| <code>req.15255</code> | ac_1 | tx_search with prove under Autobahn returns a TxProof for every result; at HEAD consensus/internal/rpc/core/tx.go:112-113 dereferences the nil block of the unfed BlockStore. |
| <code>acceptance.15.256.1</code> | req | 15256.1 |
| <code>acceptance.15.256.1</code> | scenario | Given a GigaRouter from newAutobahnTestRouter that executed blocks 1..3, when the test calls Status, then SyncInfo.LatestBlockHash equals BlockID.Hash and SyncInfo.LatestBlockTime equals Block.Header.Time of BlockByNumber(LatestBlockHeight) |
| <code>acceptance.15.256.1</code> | test | consensus/internal/rpc/core/status_autobahn_test.go::TestStatusUnderAutobahnReportsLatestBlockHashAndTime (planned) |
| <code>acceptance.15.256.1</code> | qualification | UNRUN |
| <code>req.15256</code> | ac_1 | Under Autobahn status reports the latest block hash and time of the executed block; at HEAD consensus/internal/rpc/core/status.go:84-97 sets only the height and app hash. |
| <code>acceptance.15.257.1</code> | req | 15257.1 |
| <code>acceptance.15.257.1</code> | scenario | Given a GigaRouter that executed and pruned blocks under a retention window, when the test calls Status, then SyncInfo.EarliestBlockHeight equals giga.EarliestBlockNumber(), is greater than 1, and EarliestBlockHash equals BlockID.Hash of BlockByNumber at that height |
| <code>acceptance.15.257.1</code> | test | consensus/internal/rpc/core/status_autobahn_test.go::TestStatusUnderAutobahnReportsEarliestRetainedBlock (planned) |
| <code>acceptance.15.257.1</code> | qualification | UNRUN |
| <code>req.15257</code> | ac_1 | Under Autobahn status reports the earliest retained Autobahn block; at HEAD the earliest fields come from BlockStore.LoadBaseMeta at consensus/internal/rpc/core/status.go:49-54, which is empty under Autobahn. |
| <code>acceptance.15.258.1</code> | req | 15258.1 |
| <code>acceptance.15.258.1</code> | scenario | Given a GigaRouter from newAutobahnTestRouter that executed blocks 1..3, when the test calls LagStatus, then CurrentHeight equals ABCIInfo LastBlockHeight, MaxPeerHeight equals giga.LastCommittedBlockNumber() and Lag equals their non-negative difference |
| <code>acceptance.15.258.1</code> | test | consensus/internal/rpc/core/lag_status_autobahn_test.go::TestLagStatusUnderAutobahnUsesExecutedAndCommittedHeights (planned) |
| <code>acceptance.15.258.1</code> | qualification | UNRUN |
| <code>req.15258</code> | ac_1 | Under Autobahn lag_status reports the executed height, the committed height and their gap; at HEAD consensus/internal/rpc/core/lag_status.go:11 reads the unfed BlockStore height and always answers 0. |
| <code>acceptance.15.259.1</code> | req | 15259.1 |
| <code>acceptance.15.259.1</code> | scenario | Given an Environment with a GigaRouter, a GenDoc whose ConsensusParams.Block.MaxGas is 12345 and an empty StateStore, when the test calls ConsensusParams with a nil height, then the answer holds MaxGas 12345 and BlockHeight equals ABCIInfo LastBlockHeight |
| <code>acceptance.15.259.1</code> | test | consensus/internal/rpc/core/consensus_autobahn_test.go::TestConsensusParamsUnderAutobahnReturnsGenesisParams (planned) |
| <code>acceptance.15.259.1</code> | qualification | UNRUN |
| <code>req.15259</code> | ac_1 | Under Autobahn consensus_params answers the genesis consensus parameters; at HEAD consensus/internal/rpc/core/consensus.go:167 reads StateStore.LoadConsensusParams, which Autobahn never writes. |
| <code>acceptance.15.260.1</code> | req | 15260.1 |
| <code>acceptance.15.260.1</code> | scenario | Given an Environment with a GigaRouter and no consensus reactor, when the test calls DumpConsensusState, then it returns a nil result and an error matching coretypes.ErrConsensusStateNotAvailable |
| <code>acceptance.15.260.1</code> | test | consensus/internal/rpc/core/consensus_autobahn_test.go::TestDumpConsensusStateUnderAutobahnRefused (planned) |
| <code>acceptance.15.260.1</code> | qualification | UNRUN |
| <code>req.15260</code> | ac_1 | Under Autobahn dump_consensus_state answers ErrConsensusStateNotAvailable; at HEAD DumpConsensusState at consensus/internal/rpc/core/consensus.go:92 reads the CometBFT round state that Autobahn does not run. |
| <code>acceptance.15.260.2</code> | req | 15260.2 |
| <code>acceptance.15.260.2</code> | scenario | Given the same Environment, when the test calls GetConsensusState, then it returns a nil result and an error matching coretypes.ErrConsensusStateNotAvailable |
| <code>acceptance.15.260.2</code> | test | consensus/internal/rpc/core/consensus_autobahn_test.go::TestGetConsensusStateUnderAutobahnRefused (planned) |
| <code>acceptance.15.260.2</code> | qualification | UNRUN |
| <code>req.15260</code> | ac_2 | Under Autobahn consensus_state answers ErrConsensusStateNotAvailable; at HEAD GetConsensusState at consensus/internal/rpc/core/consensus.go:146 reads the CometBFT round state. |
| <code>acceptance.15.261.1</code> | req | 15261.1 |
| <code>acceptance.15.261.1</code> | scenario | Given an Environment with a GigaRouter and a well-formed DuplicateVoteEvidence, when the test calls BroadcastEvidence, then it returns a nil result and an error matching coretypes.ErrEvidenceNotAccepted |
| <code>acceptance.15.261.1</code> | test | consensus/internal/rpc/core/evidence_autobahn_test.go::TestBroadcastEvidenceUnderAutobahnRefused (planned) |
| <code>acceptance.15.261.1</code> | qualification | UNRUN |
| <code>req.15261</code> | ac_1 | Under Autobahn broadcast_evidence answers ErrEvidenceNotAccepted; at HEAD consensus/internal/rpc/core/evidence.go:12-27 hands the evidence to the CometBFT evidence pool, which Autobahn consensus never reads. |
| <code>acceptance.15.262.1</code> | req | 15262.1 |
| <code>acceptance.15.262.1</code> | scenario | Given a GigaRouter from newAutobahnTestRouter that finalized block 2, when the test calls Header with height 2, then the returned Header equals Block.Header of Block at height 2 |
| <code>acceptance.15.262.1</code> | test | consensus/internal/rpc/core/blocks_autobahn_header_test.go::TestHeaderUnderAutobahnMatchesBlock (planned) |
| <code>acceptance.15.262.1</code> | qualification | UNRUN |
| <code>req.15262</code> | ac_1 | Under Autobahn header answers the header of the finalized Autobahn block; at HEAD Header at consensus/internal/rpc/core/blocks.go:188-201 reads only the unfed BlockStore. |
| <code>acceptance.15.263.1</code> | req | 15263.1 |
| <code>acceptance.15.263.1</code> | scenario | Given a GigaRouter that finalized block 2, when the test calls HeaderByHash with BlockID.Hash of BlockByNumber(2), then the returned Header equals that block's header, and a random 32-byte hash yields a ResultHeader with a nil Header |
| <code>acceptance.15.263.1</code> | test | consensus/internal/rpc/core/blocks_autobahn_header_test.go::TestHeaderByHashUnderAutobahnMatchesBlock (planned) |
| <code>acceptance.15.263.1</code> | qualification | UNRUN |
| <code>req.15263</code> | ac_1 | Under Autobahn header_by_hash answers the header of the finalized Autobahn block with that hash; at HEAD HeaderByHash at consensus/internal/rpc/core/blocks.go:204-213 reads only the unfed BlockStore. |
| <code>acceptance.15.264.1</code> | req | 15264.1 |
| <code>acceptance.15.264.1</code> | scenario | Given a GigaRouter that executed blocks 1..3, when the test calls BlockchainInfo with min 1 and max 3, then LastHeight is 3 and BlockMetas holds heights 3, 2, 1 with each BlockID.Hash equal to BlockByNumber at that height |
| <code>acceptance.15.264.1</code> | test | consensus/internal/rpc/core/blocks_autobahn_blockchain_test.go::TestBlockchainInfoUnderAutobahnListsFinalizedBlocks (planned) |
| <code>acceptance.15.264.1</code> | qualification | UNRUN |
| <code>req.15264</code> | ac_1 | Under Autobahn blockchain lists the metas of the finalized Autobahn blocks; at HEAD BlockchainInfo at consensus/internal/rpc/core/blocks.go:28-53 reads only the unfed BlockStore and answers an empty list. |
| <code>acceptance.15.265.1</code> | req | 15265.1 |
| <code>acceptance.15.265.1</code> | scenario | Given a GigaRouter that executed and pruned blocks so that EarliestBlockNumber is E greater than 1, when the test calls Block with height E-1, then the error matches coretypes.ErrHeightNotAvailable and its message names lowest height E |
| <code>acceptance.15.265.1</code> | test | consensus/internal/rpc/core/blocks_autobahn_lower_bound_test.go::TestAutobahnHeightBelowRetainedRangeRejected (planned) |
| <code>acceptance.15.265.1</code> | qualification | UNRUN |
| <code>req.15265</code> | ac_1 | Under Autobahn height validation rejects heights below the retained range with the lowest retained height in the message; at HEAD autobahnCheckAndGetHeight at consensus/internal/rpc/core/blocks.go:147-152 validates against BlockStore.Base(), always 0 under Autobahn. |
| <code>acceptance.15.267.1</code> | req | 15267.1 |
| <code>acceptance.15.267.1</code> | scenario | Given a GigaRouter that executed blocks 1..3 and a kv EventSink holding block events for heights 2 and 3, when the test calls BlockSearch with query block.height&gt;=2, then TotalCount is 2 and Blocks holds two entries whose BlockID.Hash equals BlockByNumber at heights 3 and 2 |
| <code>acceptance.15.267.1</code> | test | consensus/internal/rpc/core/blocks_autobahn_search_test.go::TestBlockSearchUnderAutobahnReturnsBlocks (planned) |
| <code>acceptance.15.267.1</code> | qualification | UNRUN |
| <code>req.15267</code> | ac_1 | Under Autobahn block_search returns the matching finalized Autobahn blocks; at HEAD consensus/internal/rpc/core/blocks.go:354 loads from the unfed BlockStore and returns an empty Blocks list with a non-zero TotalCount. |
| <code>acceptance.15.274.1</code> | req | 15274.1 |
| <code>acceptance.15.274.1</code> | scenario | Given a State whose committee FirstBlock is F and blocks F..F+5 pushed with QCs and AppProposals, when the test calls PruneBefore(F+4), then FirstBlock returns F+3 and GlobalBlock(F+3) succeeds while GlobalBlock(F+2) returns ErrPruned |
| <code>acceptance.15.274.1</code> | test | consensus/internal/autobahn/data/state_first_block_test.go::TestStateFirstBlockAdvancesOnPrune (planned) |
| <code>acceptance.15.274.1</code> | qualification | UNRUN |
| <code>req.15274</code> | ac_1 | data.State reports its first retained block number, which advances with PruneBefore; at HEAD inner.first at consensus/internal/autobahn/data/state.go:171 is unexported. |
| <code>acceptance.15.275.1</code> | req | 15275.1 |
| <code>acceptance.15.275.1</code> | scenario | Given a GigaRouter that executed blocks and pruned below retain height R, when the test calls EarliestBlockNumber, then it returns E with E at most R, BlockByNumber(E) succeeds and BlockByNumber(E-1) returns an error matching coretypes.ErrHeightNotAvailable |
| <code>acceptance.15.275.1</code> | test | consensus/internal/p2p/giga_router_earliest_test.go::TestGigaRouterEarliestBlockNumberFollowsPruning (planned) |
| <code>acceptance.15.275.1</code> | qualification | UNRUN |
| <code>req.15275</code> | ac_1 | GigaRouter.EarliestBlockNumber reports the lowest block BlockByNumber still serves. |
| <code>acceptance.15.278.1</code> | req | 15278.1 |
| <code>acceptance.15.278.1</code> | scenario | Given an Environment with a GigaRouter that executed blocks 1..3 and an indexer fed by its events, when the test calls each of the seventeen listed handlers with height 2 or the hash of block 2, then no call panics and each answers a result or one of the four named sentinels |
| <code>acceptance.15.278.1</code> | test | consensus/internal/rpc/core/autobahn_routes_conformance_test.go::TestAutobahnRoutesAnswerWithoutPanic (planned) |
| <code>acceptance.15.278.1</code> | qualification | UNRUN |
| <code>req.15278</code> | ac_1 | Every consensus RPC handler that reads chain data answers under Autobahn with a result or a named sentinel, never a panic or a silent empty answer. |
| <code>capability.15.24.1</code> | title | Declare the Autobahn RPC sentinel errors in coretypes |
| <code>capability.15.24.1</code> | reqs | 15252.1 |
| <code>capability.15.24.10</code> | title | Refuse broadcast_evidence under Autobahn |
| <code>capability.15.24.10</code> | reqs | 15261.1 |
| <code>capability.15.24.11</code> | title | Serve header from the Autobahn block |
| <code>capability.15.24.11</code> | reqs | 15262.1 |
| <code>capability.15.24.12</code> | title | Serve header_by_hash from the Autobahn block |
| <code>capability.15.24.12</code> | reqs | 15263.1 |
| <code>capability.15.24.13</code> | title | Serve blockchain from Autobahn blocks |
| <code>capability.15.24.13</code> | reqs | 15264.1 |
| <code>capability.15.24.14</code> | title | Pass the Autobahn lower bound to height validation |
| <code>capability.15.24.14</code> | reqs | 15265.1 |
| <code>capability.15.24.16</code> | title | Answer block_search results from Autobahn blocks |
| <code>capability.15.24.16</code> | reqs | 15267.1 |
| <code>capability.15.24.2</code> | title | Return ErrCommitNotAvailable from the commit route under Autobahn |
| <code>capability.15.24.2</code> | reqs | 15253.1 |
| <code>capability.15.24.3</code> | title | Build tx proofs from the Autobahn block when tx is called with prove |
| <code>capability.15.24.3</code> | reqs | 15254.1<br>15254.2 |
| <code>capability.15.24.4</code> | title | Build tx_search proofs from the Autobahn block |
| <code>capability.15.24.4</code> | reqs | 15255.1 |
| <code>capability.15.24.5</code> | title | Report latest block hash and time in status under Autobahn |
| <code>capability.15.24.5</code> | reqs | 15256.1 |
| <code>capability.15.24.6</code> | title | Report the earliest retained Autobahn block in status |
| <code>capability.15.24.6</code> | reqs | 15257.1 |
| <code>capability.15.24.7</code> | title | Compute lag_status from Autobahn executed and committed heights |
| <code>capability.15.24.7</code> | reqs | 15258.1 |
| <code>capability.15.24.8</code> | title | Serve consensus_params from the genesis document under Autobahn |
| <code>capability.15.24.8</code> | reqs | 15259.1 |
| <code>capability.15.24.9</code> | title | Refuse dump_consensus_state and consensus_state under Autobahn |
| <code>capability.15.24.9</code> | reqs | 15260.1<br>15260.2 |
| <code>capability.15.27.2</code> | title | Expose the first retained block of the Autobahn data state |
| <code>capability.15.27.2</code> | reqs | 15274.1 |
| <code>capability.15.27.3</code> | title | Report the earliest retained block number from the GigaRouter |
| <code>capability.15.27.3</code> | reqs | 15275.1 |
| <code>capability.15.29.1</code> | title | Sweep the consensus RPC routes under Autobahn for a defined answer |
| <code>capability.15.29.1</code> | reqs | 15278.1 |
| <code>resolution.17.965</code> | title | decision.1521 binds: Commit under Autobahn answers a named sentinel |
| <code>resolution.17.965</code> | applies_to | decision.1521<br>task.5.64.1<br>task.5.64.2<br>task.5.68.1 |
| <code>resolution.17.965</code> | source | decision.1521 in fragments/J57/decision.kvx; tasks from affects |
| <code>resolution.17.969</code> | title | decision.1525 binds: lag_status under Autobahn measures execution behind consensus |
| <code>resolution.17.969</code> | applies_to | decision.1525<br>task.5.64.7 |
| <code>resolution.17.969</code> | source | decision.1525 in fragments/J57/decision.kvx; tasks from affects |
| <code>resolution.17.970</code> | title | decision.1526 binds: consensus_params under Autobahn answers the genesis parameters |
| <code>resolution.17.970</code> | applies_to | decision.1526<br>task.5.64.8 |
| <code>resolution.17.970</code> | source | decision.1526 in fragments/J57/decision.kvx; tasks from affects |

## version-pins

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.15.go-consensus-version-abci-sem-ver-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/version/version.go:13</code> | <code>ABCISemVer = "0.17.0"</code> |
| <code>interface.15.go-consensus-version-abci-version-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/version/version.go:15</code> | <code>ABCIVersion = ABCISemVer</code> |
| <code>interface.15.go-consensus-version-block-protocol-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/version/version.go:24</code> | <code>BlockProtocol uint64 = 11</code> |
| <code>interface.15.go-consensus-version-consensus-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/version/version.go:27</code> | <code>type Consensus struct</code> |
| <code>interface.15.go-consensus-version-p2-p-protocol-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/version/version.go:20</code> | <code>P2PProtocol uint64 = 8</code> |
| <code>interface.15.go-consensus-version-request-info-j57</code> | <code>go_exported</code> | <code>stable</code> | <code>consensus/version/version.go:42</code> | <code>var RequestInfo = abci.RequestInfo</code> |
| <code>interface.15.go-consensus-version-tm-version-j57</code> | <code>go_exported</code> | <code>change</code> | <code>consensus/version/version.go:16</code> | <code>TMVersion = TMVersionDefault</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.15.go-consensus-version-abci-sem-ver-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-version-abci-version-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-version-block-protocol-j57</code> | notes | 26 referencing files; first 12 listed; const in grouped declaration |
| <code>interface.15.go-consensus-version-consensus-j57</code> | notes | 23 referencing files; first 12 listed; 1 exported methods |
| <code>interface.15.go-consensus-version-p2-p-protocol-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |
| <code>interface.15.go-consensus-version-request-info-j57</code> | notes | callers: cross-package references in importing files |
| <code>interface.15.go-consensus-version-tm-version-j57</code> | notes | callers: cross-package references in importing files; const in grouped declaration |

Contract context <code>contract_context.15.make-targets-j57</code>:

| Field | Frozen rule |
|---|---|
| interface | Make targets of consensus/Makefile and consensus/test/Makefile kept or retired by the fork |
| storage | No storage |
| errors | make exits non-zero with No rule to make target for a retired target |
| events | None |
| gas | Not applicable |
| activation | Kept targets: all, build, install, build_privval_server, format, test and the targets not listed as retired |
| invariants | make -n of every kept target resolves inside the monorepo; LD_FLAGS and goimports -local name the fork module path |
| pause | No pause |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.15.241.1</code> | req | 15241.1 |
| <code>acceptance.15.241.1</code> | scenario | Given TMVersion declared as a var (planned change of consensus/version/version.go:16), when the test runs go run -ldflags "-X github.com/Sidiora-Labs/Paxeer-X-Network/consensus/version.TMVersion=9.9.9-link" ./testdata/printversion, then the process exits 0 and prints exactly 9.9.9-link |
| <code>acceptance.15.241.1</code> | test | consensus/version/version_test.go::TestTMVersionSetByLinkFlag (planned) |
| <code>acceptance.15.241.1</code> | qualification | UNRUN |
| <code>req.15241</code> | ac_1 | TMVersion is a package-level var, so -X github.com/Sidiora-Labs/Paxeer-X-Network/consensus/version.TMVersion=&lt;v&gt; sets the reported version; at HEAD it sits in a const block at consensus/version/version.go:16 and the linker cannot set it. |
| <code>acceptance.15.241.2</code> | req | 15241.2 |
| <code>acceptance.15.241.2</code> | scenario | Given the same printversion program, when the test runs it with go run and no -ldflags, then it prints exactly 0.35.0-unreleased, the TMVersionDefault of consensus/version/version.go:11 |
| <code>acceptance.15.241.2</code> | test | consensus/version/version_test.go::TestTMVersionDefaultsWithoutLinkFlag (planned) |
| <code>acceptance.15.241.2</code> | qualification | UNRUN |
| <code>req.15241</code> | ac_2 | Without a link flag TMVersion equals TMVersionDefault "0.35.0-unreleased" declared at consensus/version/version.go:11. |
| <code>capability.15.21.1</code> | title | Make TMVersion a linker-settable variable |
| <code>capability.15.21.1</code> | reqs | 15241.1<br>15241.2 |
| <code>resolution.17.967</code> | title | decision.1523 binds: Version stamping targets the fork module path |
| <code>resolution.17.967</code> | applies_to | decision.1523<br>task.5.61.1<br>task.5.61.2 |
| <code>resolution.17.967</code> | source | decision.1523 in fragments/J57/decision.kvx; tasks from affects |
| <code>resolution.17.992</code> | title | decision.1791 binds: J57 consensus and interchain tasks belong to domains 5 and 6 |
| <code>resolution.17.992</code> | applies_to | decision.1791<br>task.5.61.1<br>task.6.13.1 |
| <code>resolution.17.992</code> | source | decision.1791 in fragments/J79/decision.kvx; tasks from affects |

## wiring-integration

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.15.961.1</code> | req | 15961.1 |
| <code>acceptance.15.961.1</code> | scenario | Given a multistore with one committed key, when the test proves the present and an absent key and encodes both chains with the app hash, then both verify through DefaultProofRuntime and the bytes equal testdata/chain_proof_v1.hex. |
| <code>acceptance.15.961.1</code> | test | sdk/store/rootmulti/chain_proof_vector_test.go::TestChainProofVectorV1 (planned) |
| <code>acceptance.15.961.1</code> | qualification | UNRUN |
| <code>req.15961</code> | ac_1 | The committed chain_proof_v1.hex holds one existence and one absence proof chain that verify against its app hash. |
