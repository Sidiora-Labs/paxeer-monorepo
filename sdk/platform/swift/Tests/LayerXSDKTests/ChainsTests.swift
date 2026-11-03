import Foundation
import XCTest

@testable import LayerXSDK

final class ChainsTests: XCTestCase {
  private struct Vectors: Decodable {
    struct Method: Decodable {
      let name: String
      let signature: String
      let selector: String
      let args: [JSONValue]
      let calldata: String
    }
    struct Event: Decodable {
      let name: String
      let signature: String
      let topic0: String
    }
    struct Example<T: Decodable>: Decodable {
      let returndata: String
      let decoded: T
    }
    let address: String
    let methods: [Method]
    let events: [Event]
    let getChain: Example<Chains.Chain>
    let params: Example<Chains.Params>
  }

  private func vectors() throws -> Vectors {
    let url = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent().deletingLastPathComponent()
      .appendingPathComponent("tests/vectors/chains/abi-vectors.json")
    return try JSONDecoder().decode(Vectors.self, from: Data(contentsOf: url))
  }

  private func hex(_ text: String) throws -> Data {
    XCTAssertTrue(text.hasPrefix("0x"))
    let chars = Array(text.dropFirst(2))
    XCTAssertEqual(chars.count % 2, 0)
    return try Data(
      stride(from: 0, to: chars.count, by: 2).map {
        try XCTUnwrap(UInt8(String(chars[$0...$0 + 1]), radix: 16))
      })
  }

  private func call(_ vector: Vectors.Method) throws -> Chains.Call {
    func number(_ index: Int) throws -> UInt64 {
      try XCTUnwrap(UInt64(exactly: XCTUnwrap(vector.args[index].integerValue)))
    }
    func id() throws -> UInt32 { try XCTUnwrap(UInt32(exactly: number(0))) }
    func text(_ index: Int) throws -> String { try XCTUnwrap(vector.args[index].stringValue) }
    switch vector.name {
    case "createChain":
      return .createChain(
        name: try text(0), gasAllowance: try number(1),
        byteAllowance: try number(2), feeShareBps: try XCTUnwrap(UInt16(exactly: number(3))))
    case "fundChain": return .fundChain(try id())
    case "pauseChain": return .pauseChain(try id())
    case "resumeChain": return .resumeChain(try id())
    case "retireChain": return .retireChain(try id())
    case "governancePause":
      guard case let .boolean(paused) = vector.args[1] else {
        throw PlatformSDKError(code: .decodeFailure, retry: .never)
      }
      return .governancePause(try id(), paused: paused)
    case "setOperator": return .setOperator(try id(), operator: try text(1))
    case "getChain": return .getChain(try id())
    case "chainByName": return .chainByName(try text(0))
    case "chainCount": return .chainCount
    case "chainIdOf": return .chainIdOf(try id())
    case "params": return .params
    default: throw PlatformSDKError(code: .decodeFailure, retry: .never)
    }
  }

  func testChainsVectorsMatchAndRoundTrip() throws {
    let fixture = try vectors()
    XCTAssertEqual(Chains.address, fixture.address)
    XCTAssertEqual(fixture.methods.count, 12)
    XCTAssertEqual(Set(fixture.methods.map(\.name)), Set(Chains.Method.allCases.map(\.rawValue)))
    for vector in fixture.methods {
      let method = try XCTUnwrap(Chains.Method(rawValue: vector.name))
      XCTAssertEqual(method.signature, vector.signature, vector.name)
      XCTAssertEqual(method.selector, try hex(vector.selector), vector.name)
      let value = try call(vector)
      let encoded = try Chains.encodeCall(value)
      XCTAssertEqual(encoded, try hex(vector.calldata), vector.name)
      XCTAssertEqual(Data(encoded.prefix(4)), try hex(vector.selector), vector.name)
      XCTAssertEqual(try Chains.decodeCall(encoded), value, vector.name)
      XCTAssertEqual(
        try Chains.encodeCall(Chains.decodeCall(hex(vector.calldata))), encoded, vector.name)
      XCTAssertThrowsError(try Chains.decodeCall(encoded + Data([0])), vector.name)
      XCTAssertThrowsError(try Chains.decodeCall(Data(encoded.dropLast())), vector.name)
    }

    XCTAssertEqual(fixture.events.count, 4)
    XCTAssertEqual(Set(fixture.events.map(\.name)), Set(Chains.EventKind.allCases.map(\.rawValue)))
    for vector in fixture.events {
      let event = try XCTUnwrap(Chains.EventKind(rawValue: vector.name))
      XCTAssertEqual(event.signature, vector.signature, vector.name)
      XCTAssertEqual(event.topic0, try hex(vector.topic0), vector.name)
    }

    let chainBytes = try hex(fixture.getChain.returndata)
    let chain = try Chains.decodeGetChain(chainBytes)
    XCTAssertEqual(chain, fixture.getChain.decoded)
    XCTAssertEqual(try Chains.encodeGetChain(chain), chainBytes)
    XCTAssertEqual(try Chains.decodeGetChain(Chains.encodeGetChain(chain)), chain)
    let paramsBytes = try hex(fixture.params.returndata)
    let params = try Chains.decodeParams(paramsBytes)
    XCTAssertEqual(params, fixture.params.decoded)
    XCTAssertEqual(try Chains.encodeParams(params), paramsBytes)
    XCTAssertEqual(try Chains.decodeParams(Chains.encodeParams(params)), params)

    XCTAssertEqual(Chains.virtualChainId(chain.id), chain.chainId)
    XCTAssertEqual(Chains.virtualChainId(0), 125_000_000)
    XCTAssertEqual(Chains.virtualChainId(UInt32.max), 4_419_967_295)
    XCTAssertEqual(
      Chains.chainRpcUrl("https://rpc.example", chain.id), "https://rpc.example/chain/1")
    XCTAssertEqual(
      Chains.chainRpcUrl("https://rpc.example///", chain.id), "https://rpc.example/chain/1")
    XCTAssertEqual(
      Chains.chainRpcUrl("https://rpc.example/evm/", UInt32.max),
      "https://rpc.example/evm/chain/4294967295")

    func field(_ offset: Int) -> Data { chainBytes.subdata(in: offset..<(offset + 32)) }
    let id = field(0)
    let operatorTopic = field(64)
    var nameOffset = Data(repeating: 0, count: 32)
    nameOffset[31] = 64
    let createdData = nameOffset + field(128) + Data(chainBytes.dropFirst(352))
    let logs: [(Chains.EventKind, [Data], Data, Chains.Event)] = [
      (
        .chainCreated, [id, operatorTopic], createdData,
        .chainCreated(
          id: chain.id, name: chain.name, operator: chain.operator, chainId: chain.chainId)
      ),
      (
        .chainStatusChanged, [id], field(96),
        .chainStatusChanged(id: chain.id, status: chain.status)
      ),
      (
        .chainFunded, [id, operatorTopic], field(256),
        .chainFunded(id: chain.id, from: chain.operator, amount: chain.bond)
      ),
      (
        .chainOperatorChanged, [id, operatorTopic], Data(),
        .chainOperatorChanged(id: chain.id, operator: chain.operator)
      ),
    ]
    for (kind, indexed, data, expected) in logs {
      let topics = [kind.topic0] + indexed
      XCTAssertEqual(try Chains.decodeEvent(topics: topics, data: data), expected, kind.rawValue)
      XCTAssertThrowsError(
        try Chains.decodeEvent(topics: topics, data: data + Data([0])), kind.rawValue)
      XCTAssertThrowsError(
        try Chains.decodeEvent(topics: Array(topics.dropLast()), data: data), kind.rawValue)
      var badID = id
      badID[0] = 1
      var badTopics = topics
      badTopics[1] = badID
      XCTAssertThrowsError(try Chains.decodeEvent(topics: badTopics, data: data), kind.rawValue)
    }
    XCTAssertThrowsError(try Chains.decodeEvent(topics: [], data: Data()))
    XCTAssertThrowsError(
      try Chains.decodeEvent(topics: [Data(repeating: 0, count: 32), id], data: field(96)))

    let boundaryCalls: [Chains.Call] = [
      .createChain(name: "", gasAllowance: 0, byteAllowance: 0, feeShareBps: 0),
      .createChain(
        name: String(repeating: "鏈", count: 33), gasAllowance: UInt64.max,
        byteAllowance: UInt64.max, feeShareBps: UInt16.max),
      .fundChain(UInt32.max), .governancePause(UInt32.max, paused: false), .chainByName(""),
      .chainByName(String(repeating: "a", count: 32)),
      .chainByName(String(repeating: "b", count: 33)),
    ]
    for value in boundaryCalls {
      XCTAssertEqual(try Chains.decodeCall(Chains.encodeCall(value)), value)
    }
    XCTAssertThrowsError(try Chains.encodeCall(.setOperator(1, operator: "0x1234")))
    var badBool = try Chains.encodeCall(.governancePause(1, paused: true))
    badBool[67] = 2
    XCTAssertThrowsError(try Chains.decodeCall(badBool))
    var badIDCall = try Chains.encodeCall(.getChain(1))
    badIDCall[4] = 1
    XCTAssertThrowsError(try Chains.decodeCall(badIDCall))
    var badAddressCall = try Chains.encodeCall(.setOperator(1, operator: chain.operator))
    badAddressCall[36] = 1
    XCTAssertThrowsError(try Chains.decodeCall(badAddressCall))
    var badPadding = try Chains.encodeCall(.chainByName("a"))
    badPadding[badPadding.count - 1] = 1
    XCTAssertThrowsError(try Chains.decodeCall(badPadding))
    var badUTF8 = try Chains.encodeCall(.chainByName("a"))
    badUTF8[68] = 255
    XCTAssertThrowsError(try Chains.decodeCall(badUTF8))

    for bytes in [Data(chainBytes.dropLast()), chainBytes + Data([0])] {
      XCTAssertThrowsError(try Chains.decodeGetChain(bytes))
    }
    var badChain = chainBytes
    badChain[31] = 0
    badChain[0] = 1
    XCTAssertThrowsError(try Chains.decodeGetChain(badChain))
    var badOffset = chainBytes
    badOffset[63] = 0
    XCTAssertThrowsError(try Chains.decodeGetChain(badOffset))
    for bytes in [Data(paramsBytes.dropLast()), paramsBytes + Data([0])] {
      XCTAssertThrowsError(try Chains.decodeParams(bytes))
    }
    var badParams = paramsBytes
    badParams[0] = 1
    XCTAssertThrowsError(try Chains.decodeParams(badParams))
    badParams = paramsBytes
    badParams[192] = 1
    XCTAssertThrowsError(try Chains.decodeParams(badParams))

    var maxParamsBytes = paramsBytes
    maxParamsBytes.replaceSubrange(32..<64, with: Data(repeating: 255, count: 32))
    let maxParams = try Chains.decodeParams(maxParamsBytes)
    XCTAssertEqual(
      maxParams.minBond,
      "115792089237316195423570985008687907853269984665640564039457584007913129639935")
    XCTAssertEqual(try Chains.encodeParams(maxParams), maxParamsBytes)
    let overflow = Chains.Params(
      authority: params.authority,
      minBond: "115792089237316195423570985008687907853269984665640564039457584007913129639936",
      creationFee: params.creationFee, minGasAllowance: params.minGasAllowance,
      maxGasAllowance: params.maxGasAllowance, totalGasAllowanceLeft: params.totalGasAllowanceLeft,
      maxChains: params.maxChains)
    XCTAssertThrowsError(try Chains.encodeParams(overflow))
  }
}
