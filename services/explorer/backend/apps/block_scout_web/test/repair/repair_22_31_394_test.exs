defmodule BlockScoutWeb.Repair.Repair2231394Test do
  @moduledoc """
  Repair record for audit 22.30.143, finding raw-trace-producer-shape:
  the callTracer, creation/error, Parity and opcode/JavaScript tests below prove
  the node adapter -> HTTP -> controller -> OpenAPI response-validator contract.
  Native callTracer recordings retain input/result from the corresponding files in
  core/deps/geth/eth/tracers/internal/tracetest/testdata/call_tracer; they are embedded
  because the explorer builder mounts only the backend. Other recordings are the
  committed explorer JSON-RPC and VCR fixtures. No out-of-path wiring is required.
  No baseline verification was run: the bounded gate order requires the complete
  implementation before build and hf-done, and forbids a separate verification run.
  The previous candidate's verification exited 2 with six tests and two failures;
  the retained excerpt does not identify the failing cases. The union must disable
  generated structs so validation can return both recorded arrays and objects.
  The runner's final evidence record supplies the verified candidate revision.
  """
  use BlockScoutWeb.ConnCase

  @behaviour :cowboy_handler

  alias EthereumJSONRPC.{Geth, HTTP, PaxeerX, RSK, Nethermind, Erigon, Besu}
  alias EthereumJSONRPC.Geth.Tracer

  @rpc_fixtures Path.expand("../../../ethereum_jsonrpc/test/support/fixture", __DIR__)
  @vcr_fixtures Path.expand("../../../explorer/test/support/fixture/vcr_cassettes", __DIR__)

  @native_recordings Jason.decode!(~S"""
                     {
                       "create": {
                         "input": "0xf907ef098504e3b29200830897be8080b9079c606060405260405160208061077c83398101604052808051906020019091905050600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161415151561007d57600080fd5b336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555080600160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055506001600460006101000a81548160ff02191690831515021790555050610653806101296000396000f300606060405260043610610083576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806305e4382a146100855780631c02708d146100ae5780632e1a7d4d146100c35780635114cb52146100e6578063a37dda2c146100fe578063ae200e7914610153578063b5769f70146101a8575b005b341561009057600080fd5b6100986101d1565b6040518082815260200191505060405180910390f35b34156100b957600080fd5b6100c16101d7565b005b34156100ce57600080fd5b6100e460048080359060200190919050506102eb565b005b6100fc6004808035906020019091905050610513565b005b341561010957600080fd5b6101116105d6565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b341561015e57600080fd5b6101666105fc565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34156101b357600080fd5b6101bb610621565b6040518082815260200191505060405180910390f35b60025481565b60011515600460009054906101000a900460ff1615151415156101f957600080fd5b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614806102a15750600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16145b15156102ac57600080fd5b6000600460006101000a81548160ff0219169083151502179055506003543073ffffffffffffffffffffffffffffffffffffffff163103600281905550565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614806103935750600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16145b151561039e57600080fd5b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141561048357600060025411801561040757506002548111155b151561041257600080fd5b80600254036002819055506000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166108fc829081150290604051600060405180830381858888f19350505050151561047e57600080fd5b610510565b600060035411801561049757506003548111155b15156104a257600080fd5b8060035403600381905550600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166108fc829081150290604051600060405180830381858888f19350505050151561050f57600080fd5b5b50565b60011515600460009054906101000a900460ff16151514151561053557600080fd5b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614801561059657506003548160035401115b80156105bd575080600354013073ffffffffffffffffffffffffffffffffffffffff163110155b15156105c857600080fd5b806003540160038190555050565b600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b600354815600a165627a7a72305820c3b849e8440987ce43eae3097b77672a69234d516351368b03fe5b7de03807910029000000000000000000000000c65e620a3a55451316168d57e268f5702ef56a1129a01060f46676a5dff6f407f0f51eb6f37f5c8c54e238c70221e18e65fc29d3ea65a0557b01c50ff4ffaac8ed6e5d31237a4ecbac843ab1bfe8bb0165a0060df7c54f",
                         "result": {
                           "from": "0x13e4acefe6a6700604929946e70e6443e4e73447",
                           "gas": "0x897be",
                           "gasUsed": "0x897be",
                           "input": "0x606060405260405160208061077c83398101604052808051906020019091905050600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff161415151561007d57600080fd5b336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555080600160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055506001600460006101000a81548160ff02191690831515021790555050610653806101296000396000f300606060405260043610610083576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806305e4382a146100855780631c02708d146100ae5780632e1a7d4d146100c35780635114cb52146100e6578063a37dda2c146100fe578063ae200e7914610153578063b5769f70146101a8575b005b341561009057600080fd5b6100986101d1565b6040518082815260200191505060405180910390f35b34156100b957600080fd5b6100c16101d7565b005b34156100ce57600080fd5b6100e460048080359060200190919050506102eb565b005b6100fc6004808035906020019091905050610513565b005b341561010957600080fd5b6101116105d6565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b341561015e57600080fd5b6101666105fc565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34156101b357600080fd5b6101bb610621565b6040518082815260200191505060405180910390f35b60025481565b60011515600460009054906101000a900460ff1615151415156101f957600080fd5b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614806102a15750600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16145b15156102ac57600080fd5b6000600460006101000a81548160ff0219169083151502179055506003543073ffffffffffffffffffffffffffffffffffffffff163103600281905550565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614806103935750600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16145b151561039e57600080fd5b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141561048357600060025411801561040757506002548111155b151561041257600080fd5b80600254036002819055506000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166108fc829081150290604051600060405180830381858888f19350505050151561047e57600080fd5b610510565b600060035411801561049757506003548111155b15156104a257600080fd5b8060035403600381905550600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166108fc829081150290604051600060405180830381858888f19350505050151561050f57600080fd5b5b50565b60011515600460009054906101000a900460ff16151514151561053557600080fd5b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614801561059657506003548160035401115b80156105bd575080600354013073ffffffffffffffffffffffffffffffffffffffff163110155b15156105c857600080fd5b806003540160038190555050565b600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b600354815600a165627a7a72305820c3b849e8440987ce43eae3097b77672a69234d516351368b03fe5b7de03807910029000000000000000000000000c65e620a3a55451316168d57e268f5702ef56a11",
                           "output": "0x606060405260043610610083576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806305e4382a146100855780631c02708d146100ae5780632e1a7d4d146100c35780635114cb52146100e6578063a37dda2c146100fe578063ae200e7914610153578063b5769f70146101a8575b005b341561009057600080fd5b6100986101d1565b6040518082815260200191505060405180910390f35b34156100b957600080fd5b6100c16101d7565b005b34156100ce57600080fd5b6100e460048080359060200190919050506102eb565b005b6100fc6004808035906020019091905050610513565b005b341561010957600080fd5b6101116105d6565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b341561015e57600080fd5b6101666105fc565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b34156101b357600080fd5b6101bb610621565b6040518082815260200191505060405180910390f35b60025481565b60011515600460009054906101000a900460ff1615151415156101f957600080fd5b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614806102a15750600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16145b15156102ac57600080fd5b6000600460006101000a81548160ff0219169083151502179055506003543073ffffffffffffffffffffffffffffffffffffffff163103600281905550565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614806103935750600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16145b151561039e57600080fd5b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff16141561048357600060025411801561040757506002548111155b151561041257600080fd5b80600254036002819055506000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166108fc829081150290604051600060405180830381858888f19350505050151561047e57600080fd5b610510565b600060035411801561049757506003548111155b15156104a257600080fd5b8060035403600381905550600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166108fc829081150290604051600060405180830381858888f19350505050151561050f57600080fd5b5b50565b60011515600460009054906101000a900460ff16151514151561053557600080fd5b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff163373ffffffffffffffffffffffffffffffffffffffff1614801561059657506003548160035401115b80156105bd575080600354013073ffffffffffffffffffffffffffffffffffffffff163110155b15156105c857600080fd5b806003540160038190555050565b600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b600354815600a165627a7a72305820c3b849e8440987ce43eae3097b77672a69234d516351368b03fe5b7de03807910029",
                           "to": "0x7dc9c9730689ff0b0fd506c67db815f12d90a448",
                           "type": "CREATE",
                           "value": "0x0"
                         }
                       },
                       "revert": {
                         "input": "0xf9018b0a8505d21dba00832dc6c094abbcd5b340c80b5f1c0545c04c987b87310296ae80b9012473b40a5c000000000000000000000000400de2e016bda6577407dfc379faba9899bc73ef0000000000000000000000002cc31912b2b0f3075a87b3640923d45a26cef3ee000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000064d79d8e6c7265636f76657279416464726573730000000000000000000000000000000000000000000000000000000000383e3ec32dc0f66d8fe60dbdc2f6815bdf73a988383e3ec32dc0f66d8fe60dbdc2f6815bdf73a988000000000000000000000000000000000000000000000000000000000000000000000000000000001ba0fd659d76a4edbd2a823e324c93f78ad6803b30ff4a9c8bce71ba82798975c70ca06571eecc0b765688ec6c78942c5ee8b585e00988c0141b518287e9be919bc48a",
                         "result": {
                           "error": "execution reverted",
                           "from": "0x0f6cef2b7fbb504782e35aa82a2207e816a2b7a9",
                           "gas": "0x2dc6c0",
                           "gasUsed": "0x719b",
                           "input": "0x73b40a5c000000000000000000000000400de2e016bda6577407dfc379faba9899bc73ef0000000000000000000000002cc31912b2b0f3075a87b3640923d45a26cef3ee000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000064d79d8e6c7265636f76657279416464726573730000000000000000000000000000000000000000000000000000000000383e3ec32dc0f66d8fe60dbdc2f6815bdf73a988383e3ec32dc0f66d8fe60dbdc2f6815bdf73a98800000000000000000000000000000000000000000000000000000000000000000000000000000000",
                           "to": "0xabbcd5b340c80b5f1c0545c04c987b87310296ae",
                           "type": "CALL",
                           "value": "0x0"
                         }
                       },
                       "revert_reason": {
                         "input": "0xf888068449504f80832dc6c094f58833cf0c791881b494eb79d461e08a1f043f5280a45c19a95c000000000000000000000000f7579c3d8a669c89d5ed246a22eb6db8f6fedbf12da0264664db3e71fae1dbdaf2f53954be149ad3b7ba8a5054b4d89c70febfacc8b1a0212e8398757963f419681839ae8c5a54b411e252473c82d93dda68405ca63294",
                         "result": {
                           "error": "execution reverted",
                           "from": "0xf7579c3d8a669c89d5ed246a22eb6db8f6fedbf1",
                           "gas": "0x2dc6c0",
                           "gasUsed": "0x5940",
                           "input": "0x5c19a95c000000000000000000000000f7579c3d8a669c89d5ed246a22eb6db8f6fedbf1",
                           "to": "0xf58833cf0c791881b494eb79d461e08a1f043f52",
                           "type": "CALL",
                           "value": "0x0",
                           "output": "0x08c379a00000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000001e53656c662d64656c65676174696f6e20697320646973616c6c6f7765642e0000",
                           "revertReason": "Self-delegation is disallowed."
                         }
                       },
                       "inner_revert_reason": {
                         "input": "0x02f9029d82053980849502f90085010c388d00832dc6c08080b90241608060405234801561001057600080fd5b50600060405161001f906100a2565b604051809103906000f08015801561003b573d6000803e3d6000fd5b5090508073ffffffffffffffffffffffffffffffffffffffff1663c04062266040518163ffffffff1660e01b815260040160006040518083038186803b15801561008457600080fd5b505afa158015610098573d6000803e3d6000fd5b50505050506100af565b610145806100fc83390190565b603f806100bd6000396000f3fe6080604052600080fdfea264697066735822122077f7dbd3450d6e817079cf3fe27107de5768bb3163a402b94e2206b468eb025664736f6c63430008070033608060405234801561001057600080fd5b50610125806100206000396000f3fe6080604052348015600f57600080fd5b506004361060285760003560e01c8063c040622614602d575b600080fd5b60336035565b005b60036002116076576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401606d906097565b60405180910390fd5b565b6000608360128360b5565b9150608c8260c6565b602082019050919050565b6000602082019050818103600083015260ae816078565b9050919050565b600082825260208201905092915050565b7f546869732063616c6c6564206661696c6564000000000000000000000000000060008201525056fea264697066735822122033f8d92e29d467e5ea08d0024eab0b36b86b8cdb3542c6e89dbaabeb8ffaa42064736f6c63430008070033c001a07566181071cabaf58b70fc41557eb813bfc7a24f5c58554e7fed0bf7c031f169a0420af50b5fe791a4d839e181a676db5250b415dfb35cb85d544db7a1475ae2cc",
                         "result": {
                           "from": "0x3623191d4ccfbbdf09e8ebf6382a1f8257417bc1",
                           "gas": "0x2dc6c0",
                           "gasUsed": "0x25590",
                           "input": "0x608060405234801561001057600080fd5b50600060405161001f906100a2565b604051809103906000f08015801561003b573d6000803e3d6000fd5b5090508073ffffffffffffffffffffffffffffffffffffffff1663c04062266040518163ffffffff1660e01b815260040160006040518083038186803b15801561008457600080fd5b505afa158015610098573d6000803e3d6000fd5b50505050506100af565b610145806100fc83390190565b603f806100bd6000396000f3fe6080604052600080fdfea264697066735822122077f7dbd3450d6e817079cf3fe27107de5768bb3163a402b94e2206b468eb025664736f6c63430008070033608060405234801561001057600080fd5b50610125806100206000396000f3fe6080604052348015600f57600080fd5b506004361060285760003560e01c8063c040622614602d575b600080fd5b60336035565b005b60036002116076576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401606d906097565b60405180910390fd5b565b6000608360128360b5565b9150608c8260c6565b602082019050919050565b6000602082019050818103600083015260ae816078565b9050919050565b600082825260208201905092915050565b7f546869732063616c6c6564206661696c6564000000000000000000000000000060008201525056fea264697066735822122033f8d92e29d467e5ea08d0024eab0b36b86b8cdb3542c6e89dbaabeb8ffaa42064736f6c63430008070033",
                           "output": "0x08c379a000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000012546869732063616c6c6564206661696c65640000000000000000000000000000",
                           "error": "execution reverted",
                           "revertReason": "This called failed",
                           "calls": [
                             {
                               "from": "0xdebfb4b387033eac57af7b3de5116dd60056803b",
                               "gas": "0x2ba851",
                               "gasUsed": "0xe557",
                               "to": "0xd15abca351f79181dedfb6d019e382db90f3628a",
                               "input": "0x608060405234801561001057600080fd5b50610125806100206000396000f3fe6080604052348015600f57600080fd5b506004361060285760003560e01c8063c040622614602d575b600080fd5b60336035565b005b60036002116076576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401606d906097565b60405180910390fd5b565b6000608360128360b5565b9150608c8260c6565b602082019050919050565b6000602082019050818103600083015260ae816078565b9050919050565b600082825260208201905092915050565b7f546869732063616c6c6564206661696c6564000000000000000000000000000060008201525056fea264697066735822122033f8d92e29d467e5ea08d0024eab0b36b86b8cdb3542c6e89dbaabeb8ffaa42064736f6c63430008070033",
                               "output": "0x6080604052348015600f57600080fd5b506004361060285760003560e01c8063c040622614602d575b600080fd5b60336035565b005b60036002116076576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401606d906097565b60405180910390fd5b565b6000608360128360b5565b9150608c8260c6565b602082019050919050565b6000602082019050818103600083015260ae816078565b9050919050565b600082825260208201905092915050565b7f546869732063616c6c6564206661696c6564000000000000000000000000000060008201525056fea264697066735822122033f8d92e29d467e5ea08d0024eab0b36b86b8cdb3542c6e89dbaabeb8ffaa42064736f6c63430008070033",
                               "value": "0x0",
                               "type": "CREATE"
                             },
                             {
                               "from": "0xdebfb4b387033eac57af7b3de5116dd60056803b",
                               "gas": "0x2ac548",
                               "gasUsed": "0x1b2",
                               "to": "0xd15abca351f79181dedfb6d019e382db90f3628a",
                               "input": "0xc0406226",
                               "output": "0x08c379a000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000012546869732063616c6c6564206661696c65640000000000000000000000000000",
                               "error": "execution reverted",
                               "revertReason": "This called failed",
                               "type": "STATICCALL"
                             }
                           ],
                           "value": "0x0",
                           "type": "CREATE"
                         }
                       },
                       "inner_throw_outer_revert": {
                         "input": "0xf86d808504e3b292008307dfa69433056b5dcac09a9b4becad0e1dcf92c19bd0af76880e92596fd62900008029a0e5f27bb66431f7081bb7f1f242003056d7f3f35414c352cd3d1848b52716dac2a07d0be78980edb0bd2a0678fc53aa90ea9558ce346b0d947967216918ac74ccea",
                         "result": {
                           "calls": [
                             {
                               "error": "invalid opcode: INVALID",
                               "from": "0x33056b5dcac09a9b4becad0e1dcf92c19bd0af76",
                               "gas": "0x75fe3",
                               "gasUsed": "0x75fe3",
                               "input": "0xa9059cbb000000000000000000000000d4fcab9f0a6dc0493af47c864f6f17a8a5e2e82600000000000000000000000000000000000000000000000000000000000002f4",
                               "to": "0xe819f024b41358d2c08e3a868a5c5dd0566078d4",
                               "type": "CALL",
                               "value": "0x0"
                             }
                           ],
                           "error": "execution reverted",
                           "from": "0xd4fcab9f0a6dc0493af47c864f6f17a8a5e2e826",
                           "gas": "0x7dfa6",
                           "gasUsed": "0x7c1c8",
                           "input": "0x",
                           "to": "0x33056b5dcac09a9b4becad0e1dcf92c19bd0af76",
                           "type": "CALL",
                           "value": "0xe92596fd6290000"
                         }
                       },
                       "oog": {
                         "input": "0xf8ab820109855d21dba00082ca1d9443064693d3d38ad6a7cb579e0d6d9718c8aa6b6280b844a9059cbb000000000000000000000000e77b1ac803616503510bed0086e3a7be2627a69900000000000000000000000000000000000000000000000000000009502f90001ba0ce3ad83f5530136467b7c2bb225f406bd170f4ad59c254e5103c34eeabb5bd69a0455154527224a42ab405cacf0fe92918a75641ce4152f8db292019a5527aa956",
                         "result": {
                           "error": "out of gas",
                           "from": "0x94194bc2aaf494501d7880b61274a169f6502a54",
                           "gas": "0xca1d",
                           "gasUsed": "0xca1d",
                           "input": "0xa9059cbb000000000000000000000000e77b1ac803616503510bed0086e3a7be2627a69900000000000000000000000000000000000000000000000000000009502f9000",
                           "to": "0x43064693d3d38ad6a7cb579e0d6d9718c8aa6b62",
                           "type": "CALL",
                           "value": "0x0"
                         }
                       },
                       "selfdestruct": {
                         "input": "0xf88b8271908506fc23ac0083015f90943b873a919aa0512d5a0f09e6dcceaa4a6727fafe80a463e4bff40000000000000000000000000024f658a46fbb89d8ac105e98d7ac7cbbaf27c52aa0bdce0b59e8761854e857fe64015f06dd08a4fbb7624f6094893a79a72e6ad6bea01d9dde033cff7bb235a3163f348a6d7ab8d6b52bc0963a95b91612e40ca766a4",
                         "result": {
                           "calls": [
                             {
                               "from": "0x3b873a919aa0512d5a0f09e6dcceaa4a6727fafe",
                               "gas": "0x0",
                               "gasUsed": "0x0",
                               "input": "0x",
                               "to": "0x000000000000000000000000000000000000dead",
                               "type": "SELFDESTRUCT",
                               "value": "0x4d87094125a369d9bd5"
                             }
                           ],
                           "from": "0xb436ba50d378d4bbc8660d312a13df6af6e89dfb",
                           "gas": "0x15f90",
                           "gasUsed": "0x6fcb",
                           "input": "0x63e4bff40000000000000000000000000024f658a46fbb89d8ac105e98d7ac7cbbaf27c5",
                           "to": "0x3b873a919aa0512d5a0f09e6dcceaa4a6727fafe",
                           "type": "CALL",
                           "value": "0x0"
                         }
                       }
                     }
                     """)

  setup do
    initial_rpc = Application.get_env(:explorer, :json_rpc_named_arguments)
    initial_geth = Application.get_env(:ethereum_jsonrpc, Geth)

    on_exit(fn ->
      Application.put_env(:explorer, :json_rpc_named_arguments, initial_rpc)
      Application.put_env(:ethereum_jsonrpc, Geth, initial_geth)
    end)

    :ok
  end

  test "raw-trace-producer-shape: callTracer nested calls survive each Geth adapter and response validation", %{
    conn: conn
  } do
    recording = rpc_fixture("geth/trace/struct_logger.json")
    trace = rpc_fixture("geth/trace/calltracer.json")["result"]
    hash = recording["tx"]["hash"]
    transaction = persist_transaction(hash, trace)
    assert Enum.any?(trace["calls"], &Map.has_key?(&1, "calls"))
    assert Enum.any?(trace["calls"], &(&1["type"] == "STATICCALL" and not Map.has_key?(&1, "value")))

    for adapter <- [PaxeerX, Geth, RSK] do
      response = request_trace(conn, transaction, adapter, "call_tracer", trace)
      assert response["calls"] == trace["calls"]
      assert response["output"] == trace["output"]
    end
  end

  test "raw-trace-producer-shape: recorded creations and execution errors survive the controller", %{conn: conn} do
    for name <- ~w(create revert revert_reason inner_revert_reason inner_throw_outer_revert oog selfdestruct) do
      recording = Map.fetch!(@native_recordings, name)
      trace = recording["result"]
      hash = signed_transaction_hash(recording["input"])
      transaction = persist_transaction(hash, trace)
      response = request_trace(conn, transaction, PaxeerX, "call_tracer", trace)

      case name do
        "create" ->
          assert response["type"] == "CREATE"
          assert response["to"] == trace["to"]
          assert response["output"] == trace["output"]

        "revert" ->
          assert response["error"] == "execution reverted"
          refute Map.has_key?(response, "output")

        "revert_reason" ->
          assert response["revertReason"] == "Self-delegation is disallowed."

        "inner_revert_reason" ->
          assert response["type"] == "CREATE"
          assert response["error"] == "execution reverted"
          assert response["revertReason"] == "This called failed"
          assert Enum.any?(response["calls"], &(&1["error"] == "execution reverted"))

        "inner_throw_outer_revert" ->
          assert response["error"] == "execution reverted"
          assert [inner] = response["calls"]
          assert inner["error"] == "invalid opcode: INVALID"

        "oog" ->
          assert response["error"] == "out of gas"

        "selfdestruct" ->
          assert [inner] = response["calls"]
          assert inner["type"] == "SELFDESTRUCT"
      end
    end
  end

  test "raw-trace-producer-shape: Parity call and creation recordings pass the supported adapters", %{conn: conn} do
    for name <- [
          "internal_transaction_importer_import_1.json",
          "internal_transaction_importer_import_1_with_contract_creation.json"
        ] do
      [recording] = @vcr_fixtures |> Path.join(name) |> File.read!() |> Jason.decode!()
      request = Jason.decode!(recording["request"]["body"])
      result = Jason.decode!(recording["response"]["body"])["result"]
      [trace | _] = result["trace"]
      hash = hd(request["params"])
      transaction = persist_transaction(hash, trace["action"])

      for adapter <- [Nethermind, Erigon, Besu] do
        node_result = if adapter == Besu, do: result["trace"], else: result
        response = request_trace(conn, transaction, adapter, "call_tracer", node_result)
        assert response == result["trace"]

        if trace["type"] == "create" do
          assert hd(response)["action"]["init"] == trace["action"]["init"]
          assert hd(response)["result"]["address"] == trace["result"]["address"]
        else
          assert hd(response)["action"]["callType"] == "call"
        end
      end
    end
  end

  test "raw-trace-producer-shape: opcode and JavaScript tracer outputs pass the controller", %{conn: conn} do
    recording = rpc_fixture("geth/trace/struct_logger.json")
    transaction = persist_transaction(recording["tx"]["hash"], recording["tx"])

    for tracer <- ~w(opcode polygon_edge) do
      response = request_trace(conn, transaction, PaxeerX, tracer, recording["result"])
      assert response["structLogs"] == recording["result"]["structLogs"]
      assert length(response["structLogs"]) > 0
      assert response["failed"] == false
    end

    frames = Tracer.replay(recording["result"], recording["receipt"], recording["tx"])
    assert length(frames) > 1
    assert Enum.any?(frames, &(length(&1["traceAddress"]) > 1))
    assert request_trace(conn, transaction, Geth, "js", frames) == frames
  end

  test "raw-trace-producer-shape: recursive call frames reject malformed nested fields" do
    trace = rpc_fixture("geth/trace/calltracer.json")["result"]
    [inner | rest] = trace["calls"]
    spec = BlockScoutWeb.Specs.Public.spec()
    schema = response_schema(spec)

    for malformed <- [
          Map.delete(inner, "from"),
          Map.put(inner, "type", "unknown"),
          Map.put(inner, "gas", 1),
          Map.put(inner, "to", "0x123"),
          Map.put(inner, "error", 42),
          Map.put(inner, "unexpected", true)
        ] do
      assert {:error, _} = OpenApiSpex.cast_value(%{trace | "calls" => [malformed | rest]}, schema, spec)
    end

    for invalid <- [nil, %{}, %{"calls" => []}, Map.put(trace, "calls", %{}), Map.delete(trace, "type")] do
      assert {:error, _} = OpenApiSpex.cast_value(invalid, schema, spec)
    end
  end

  test "raw-trace-producer-shape: legacy and opcode variants keep their required fields and types" do
    [recording] =
      @vcr_fixtures
      |> Path.join("internal_transaction_importer_import_1_with_contract_creation.json")
      |> File.read!()
      |> Jason.decode!()

    [trace] = Jason.decode!(recording["response"]["body"])["result"]["trace"]
    opcode = rpc_fixture("geth/trace/struct_logger.json")["result"]
    [first_log | _] = opcode["structLogs"]
    spec = BlockScoutWeb.Specs.Public.spec()
    schema = response_schema(spec)

    for invalid <- [
          [%{trace | "action" => Map.delete(trace["action"], "init")}],
          [%{trace | "result" => Map.put(trace["result"], "address", "invalid")}],
          [%{trace | "traceAddress" => [-1]}],
          [%{trace | "subtraces" => -1}],
          [%{trace | "result" => Map.delete(trace["result"], "gasUsed")}],
          Map.delete(opcode, "structLogs"),
          Map.put(opcode, "failed", []),
          Map.put(opcode, "gas", -1),
          Map.put(opcode, "structLogs", [Map.put(first_log, "stack", ["0x" <> String.duplicate("f", 65)])]),
          Map.put(opcode, "structLogs", [Map.put(first_log, "memory", ["0x00"])]),
          Map.put(opcode, "structLogs", [%{"op" => "CALL"}])
        ] do
      assert {:error, _} = OpenApiSpex.cast_value(invalid, schema, spec)
    end
  end

  @impl :cowboy_handler
  def init(req, {test_pid, result} = state) do
    {:ok, body, req} = :cowboy_req.read_body(req)
    request = Jason.decode!(body)
    send(test_pid, {:trace_request, request})
    response = Jason.encode!(%{"jsonrpc" => "2.0", "id" => request["id"], "result" => result})
    req = :cowboy_req.reply(200, %{"content-type" => "application/json"}, response, req)
    {:ok, req, state}
  end

  defp request_trace(conn, transaction, adapter, tracer, result) do
    listener = make_ref()
    dispatch = :cowboy_router.compile([{:_, [{"/", __MODULE__, {self(), result}}]}])

    {:ok, _} =
      :cowboy.start_clear(listener, %{socket_opts: [ip: {127, 0, 0, 1}, port: 0]}, %{env: %{dispatch: dispatch}})

    on_exit(fn -> :cowboy.stop_listener(listener) end)
    url = "http://127.0.0.1:#{:ranch.get_port(listener)}"

    Application.put_env(:ethereum_jsonrpc, Geth, tracer: tracer, debug_trace_timeout: "5s")

    Application.put_env(:explorer, :json_rpc_named_arguments,
      variant: adapter,
      transport: HTTP,
      transport_options: [
        http: HTTP.HTTPoison,
        http_options: [recv_timeout: 5_000, timeout: 5_000],
        urls: [url],
        trace_urls: [url]
      ]
    )

    response = conn |> get("/api/v2/transactions/#{transaction.hash}/raw-trace") |> json_response(200)
    expected = if adapter in [Nethermind, Erigon], do: result["trace"], else: result
    assert response == expected
    assert_receive {:trace_request, request}
    assert request["jsonrpc"] == "2.0"
    assert request["id"] == 0
    hash = to_string(transaction.hash)

    cond do
      adapter in [Nethermind, Erigon] ->
        assert request["method"] == "trace_replayTransaction"
        assert request["params"] == [hash, ["trace"]]

      adapter == Besu ->
        assert request["method"] == "trace_transaction"
        assert request["params"] == [hash]

      true ->
        assert request["method"] == "debug_traceTransaction"
        assert [^hash, options] = request["params"]
        assert options["timeout"] == "5s"

        case tracer do
          "call_tracer" ->
            assert options["tracer"] == "callTracer"
            refute Map.has_key?(options, "tracerConfig")

          "js" ->
            assert options["tracer"] ==
                     File.read!(
                       Path.join(
                         :code.priv_dir(:ethereum_jsonrpc),
                         "js/ethereum_jsonrpc/geth/debug_traceTransaction/tracer.js"
                       )
                     )

          opcode when opcode in ~w(opcode polygon_edge) ->
            refute Map.has_key?(options, "tracer")
            assert options["enableMemory"] == true
            assert options["disableStack"] == false
        end
    end

    spec = BlockScoutWeb.Specs.Public.spec()
    schema = response_schema(spec)
    OpenApiSpex.TestAssertions.assert_raw_schema(response, schema, spec)
    assert {:ok, cast_response} = OpenApiSpex.cast_value(response, schema, spec)

    if is_list(response) do
      assert is_list(cast_response)
      assert length(cast_response) == length(response)
    else
      assert is_map(cast_response)
    end

    response
  end

  defp response_schema(spec) do
    spec.paths
    |> Map.fetch!("/v2/transactions/{transaction_hash_param}/raw-trace")
    |> Map.fetch!(:get)
    |> Map.fetch!(:responses)
    |> Map.fetch!(200)
    |> Map.fetch!(:content)
    |> Map.fetch!("application/json")
    |> Map.fetch!(:schema)
  end

  defp persist_transaction(hash, trace) do
    sender = insert(:address, hash: trace["from"])
    recipient = if trace["to"], do: insert(:address, hash: trace["to"])

    transaction =
      insert(:transaction,
        hash: hash,
        from_address: sender,
        to_address: recipient,
        input: trace["input"] || trace["init"]
      )
      |> with_block(status: :ok)

    insert(:internal_transaction,
      transaction: transaction,
      transaction_index: transaction.index,
      index: 0,
      block_hash: transaction.block_hash,
      block_number: transaction.block_number,
      block_index: 0
    )

    transaction
  end

  defp rpc_fixture(name), do: @rpc_fixtures |> Path.join(name) |> File.read!() |> Jason.decode!()

  defp signed_transaction_hash("0x" <> input) do
    "0x" <> (input |> Base.decode16!(case: :mixed) |> ExKeccak.hash_256() |> Base.encode16(case: :lower))
  end
end
