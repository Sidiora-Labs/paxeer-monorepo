import Foundation

public enum Chains {
  public static let address = "0x000000000000000000000000000000000000101e"

  public enum Call: Equatable, Sendable {
    case createChain(name: String, gasAllowance: UInt64, byteAllowance: UInt64, feeShareBps: UInt16)
    case fundChain(UInt32)
    case pauseChain(UInt32)
    case resumeChain(UInt32)
    case retireChain(UInt32)
    case governancePause(UInt32, paused: Bool)
    case setOperator(UInt32, operator: String)
    case getChain(UInt32)
    case chainByName(String)
    case chainCount
    case chainIdOf(UInt32)
    case params

    public var method: Method {
      switch self {
      case .createChain: return .createChain
      case .fundChain: return .fundChain
      case .pauseChain: return .pauseChain
      case .resumeChain: return .resumeChain
      case .retireChain: return .retireChain
      case .governancePause: return .governancePause
      case .setOperator: return .setOperator
      case .getChain: return .getChain
      case .chainByName: return .chainByName
      case .chainCount: return .chainCount
      case .chainIdOf: return .chainIdOf
      case .params: return .params
      }
    }
  }

  public enum Method: String, CaseIterable, Sendable {
    case createChain, fundChain, pauseChain, resumeChain, retireChain, governancePause
    case setOperator, getChain, chainByName, chainCount, chainIdOf, params

    public var signature: String {
      switch self {
      case .createChain: return "createChain(string,uint64,uint64,uint16)"
      case .governancePause: return "governancePause(uint32,bool)"
      case .setOperator: return "setOperator(uint32,address)"
      case .chainByName: return "chainByName(string)"
      case .chainCount, .params: return rawValue + "()"
      default: return rawValue + "(uint32)"
      }
    }

    public var selector: Data { Data(CommittedWire.keccak256(Data(signature.utf8)).prefix(4)) }
  }

  public struct Chain: Codable, Equatable, Sendable {
    public let id: UInt32
    public let name: String
    public let `operator`: String
    public let status: UInt8
    public let chainId: UInt64
    public let gasAllowance: UInt64
    public let byteAllowance: UInt64
    public let feeShareBps: UInt16
    public let bond: String
    public let baseFeePerGas: String
    public let createdHeight: UInt64
  }

  public struct Params: Codable, Equatable, Sendable {
    public let authority: String
    public let minBond: String
    public let creationFee: String
    public let minGasAllowance: UInt64
    public let maxGasAllowance: UInt64
    public let totalGasAllowanceLeft: UInt64
    public let maxChains: UInt32
  }

  public enum EventKind: String, CaseIterable, Sendable {
    case chainCreated = "ChainCreated"
    case chainStatusChanged = "ChainStatusChanged"
    case chainFunded = "ChainFunded"
    case chainOperatorChanged = "ChainOperatorChanged"

    public var signature: String {
      switch self {
      case .chainCreated: return "ChainCreated(uint32,string,address,uint64)"
      case .chainStatusChanged: return "ChainStatusChanged(uint32,uint8)"
      case .chainFunded: return "ChainFunded(uint32,address,uint256)"
      case .chainOperatorChanged: return "ChainOperatorChanged(uint32,address)"
      }
    }

    public var topic0: Data { CommittedWire.keccak256(Data(signature.utf8)) }
  }

  public enum Event: Equatable, Sendable {
    case chainCreated(id: UInt32, name: String, operator: String, chainId: UInt64)
    case chainStatusChanged(id: UInt32, status: UInt8)
    case chainFunded(id: UInt32, from: String, amount: String)
    case chainOperatorChanged(id: UInt32, operator: String)
  }

  public static func virtualChainId(_ id: UInt32) -> UInt64 { 125_000_000 + UInt64(id) }

  public static func chainRpcUrl(_ base: String, _ id: UInt32) -> String {
    var base = base
    while base.hasSuffix("/") { base.removeLast() }
    return base + "/chain/" + String(id)
  }

  public static func encodeCall(_ call: Call) throws -> Data {
    let fields: [ProgramABI.Field]
    switch call {
    case let .createChain(name, gas, bytes, fee):
      fields = [
        .bytes(Data(name.utf8)), .word(word(gas)), .word(word(bytes)), .word(word(UInt64(fee))),
      ]
    case let .fundChain(id), let .pauseChain(id), let .resumeChain(id), let .retireChain(id),
      let .getChain(id), let .chainIdOf(id):
      fields = [.word(word(UInt64(id)))]
    case let .governancePause(id, paused):
      fields = [.word(word(UInt64(id))), .word(word(paused ? 1 : 0))]
    case let .setOperator(id, value):
      fields = [.word(word(UInt64(id))), .word(try addressWord(value))]
    case let .chainByName(name): fields = [.bytes(Data(name.utf8))]
    case .chainCount, .params: fields = []
    }
    return try tuple(fields, selector: call.method.selector)
  }

  public static func decodeCall(_ encoded: Data) throws -> Call {
    let encoded = Data(encoded)
    guard encoded.count >= 4,
      let method = Method.allCases.first(where: { $0.selector == encoded.prefix(4) })
    else { throw invalid() }
    let bytes = Data(encoded.dropFirst(4))
    func id() throws -> UInt32 { UInt32(try integer(bytes, 0, width: 4)) }
    let call: Call
    switch method {
    case .createChain:
      call = .createChain(
        name: try string(bytes, 0, headSize: 128),
        gasAllowance: try integer(bytes, 32), byteAllowance: try integer(bytes, 64),
        feeShareBps: UInt16(try integer(bytes, 96, width: 2)))
    case .fundChain: call = .fundChain(try id())
    case .pauseChain: call = .pauseChain(try id())
    case .resumeChain: call = .resumeChain(try id())
    case .retireChain: call = .retireChain(try id())
    case .getChain: call = .getChain(try id())
    case .chainIdOf: call = .chainIdOf(try id())
    case .governancePause:
      let paused = try integer(bytes, 32, width: 1)
      guard paused <= 1 else { throw invalid() }
      call = .governancePause(try id(), paused: paused == 1)
    case .setOperator: call = .setOperator(try id(), operator: try addressValue(bytes, 32))
    case .chainByName: call = .chainByName(try string(bytes, 0, headSize: 32))
    case .chainCount: call = .chainCount
    case .params: call = .params
    }
    guard try encodeCall(call) == encoded else { throw invalid() }
    return call
  }

  public static func decodeGetChain(_ encoded: Data) throws -> Chain {
    let bytes = Data(encoded)
    let chain = Chain(
      id: UInt32(try integer(bytes, 0, width: 4)),
      name: try string(bytes, 32, headSize: 352), operator: try addressValue(bytes, 64),
      status: UInt8(try integer(bytes, 96, width: 1)), chainId: try integer(bytes, 128),
      gasAllowance: try integer(bytes, 160), byteAllowance: try integer(bytes, 192),
      feeShareBps: UInt16(try integer(bytes, 224, width: 2)), bond: try decimal(bytes, 256),
      baseFeePerGas: try decimal(bytes, 288), createdHeight: try integer(bytes, 320))
    guard try encodeGetChain(chain) == bytes else { throw invalid() }
    return chain
  }

  public static func encodeGetChain(_ chain: Chain) throws -> Data {
    try tuple([
      .word(word(UInt64(chain.id))), .bytes(Data(chain.name.utf8)),
      .word(addressWord(chain.operator)), .word(word(UInt64(chain.status))),
      .word(word(chain.chainId)), .word(word(chain.gasAllowance)), .word(word(chain.byteAllowance)),
      .word(word(UInt64(chain.feeShareBps))), .word(decimalWord(chain.bond)),
      .word(decimalWord(chain.baseFeePerGas)), .word(word(chain.createdHeight)),
    ])
  }

  public static func decodeParams(_ encoded: Data) throws -> Params {
    let bytes = Data(encoded)
    guard bytes.count == 224 else { throw invalid() }
    return Params(
      authority: try addressValue(bytes, 0), minBond: try decimal(bytes, 32),
      creationFee: try decimal(bytes, 64), minGasAllowance: try integer(bytes, 96),
      maxGasAllowance: try integer(bytes, 128), totalGasAllowanceLeft: try integer(bytes, 160),
      maxChains: UInt32(try integer(bytes, 192, width: 4)))
  }

  public static func encodeParams(_ params: Params) throws -> Data {
    try tuple([
      .word(addressWord(params.authority)), .word(decimalWord(params.minBond)),
      .word(decimalWord(params.creationFee)), .word(word(params.minGasAllowance)),
      .word(word(params.maxGasAllowance)), .word(word(params.totalGasAllowanceLeft)),
      .word(word(UInt64(params.maxChains))),
    ])
  }

  public static func decodeEvent(topics: [Data], data: Data) throws -> Event {
    guard let topic = topics.first,
      let kind = EventKind.allCases.first(where: { $0.topic0 == topic }),
      topics.count == (kind == .chainStatusChanged ? 2 : 3),
      topics.allSatisfy({ $0.count == 32 })
    else { throw invalid() }
    let id = UInt32(try integer(Data(topics[1]), 0, width: 4))
    let bytes = Data(data)
    switch kind {
    case .chainCreated:
      return .chainCreated(
        id: id, name: try string(bytes, 0, headSize: 64),
        operator: try addressValue(Data(topics[2]), 0), chainId: try integer(bytes, 32))
    case .chainStatusChanged:
      guard bytes.count == 32 else { throw invalid() }
      return .chainStatusChanged(id: id, status: UInt8(try integer(bytes, 0, width: 1)))
    case .chainFunded:
      guard bytes.count == 32 else { throw invalid() }
      return .chainFunded(
        id: id, from: try addressValue(Data(topics[2]), 0), amount: try decimal(bytes, 0))
    case .chainOperatorChanged:
      guard bytes.isEmpty else { throw invalid() }
      return .chainOperatorChanged(id: id, operator: try addressValue(Data(topics[2]), 0))
    }
  }

  private static func invalid() -> PlatformSDKError {
    PlatformSDKError(code: .decodeFailure, retry: .never)
  }

  private static func word(_ value: UInt64) -> Data { ProgramABI.word(value) }

  private static func tuple(_ fields: [ProgramABI.Field], selector: Data = Data()) throws -> Data {
    let encoded = try ProgramABI.encode(selector: "00000000", fields: fields)
    return selector + Data(encoded.dropFirst(4))
  }

  private static func integer(_ bytes: Data, _ offset: Int, width: Int = 8) throws -> UInt64 {
    try ProgramABI.integer(bytes, at: offset, width: width)
  }

  private static func string(_ bytes: Data, _ offset: Int, headSize: Int) throws -> String {
    let value = try ProgramABI.dynamic(bytes, at: offset, headSize: headSize)
    guard let text = String(data: value, encoding: .utf8) else { throw invalid() }
    return text
  }

  private static func addressWord(_ text: String) throws -> Data {
    let bytes = try ProgramABI.hex(text)
    guard bytes.count == 20 else { throw invalid() }
    return Data(repeating: 0, count: 12) + bytes
  }

  private static func addressValue(_ bytes: Data, _ offset: Int) throws -> String {
    guard offset >= 0, offset <= bytes.count - 32,
      bytes[offset..<(offset + 12)].allSatisfy({ $0 == 0 })
    else { throw invalid() }
    return "0x" + bytes[(offset + 12)..<(offset + 32)].map { String(format: "%02x", $0) }.joined()
  }

  private static func decimalWord(_ text: String) throws -> Data {
    guard !text.isEmpty, text == "0" || text.first != "0",
      text.utf8.allSatisfy({ (48...57).contains($0) })
    else { throw invalid() }
    var bytes = [UInt8](repeating: 0, count: 32)
    for digit in text.utf8 {
      var carry = UInt16(digit - 48)
      for i in bytes.indices.reversed() {
        carry += UInt16(bytes[i]) * 10
        bytes[i] = UInt8(truncatingIfNeeded: carry)
        carry >>= 8
      }
      guard carry == 0 else { throw invalid() }
    }
    return Data(bytes)
  }

  private static func decimal(_ bytes: Data, _ offset: Int) throws -> String {
    guard offset >= 0, offset <= bytes.count - 32 else { throw invalid() }
    var digits = [UInt16](repeating: 0, count: 1)
    for byte in bytes[offset..<(offset + 32)] {
      var carry = UInt16(byte)
      for i in digits.indices {
        carry += digits[i] * 256
        digits[i] = carry % 10
        carry /= 10
      }
      while carry > 0 {
        digits.append(carry % 10)
        carry /= 10
      }
    }
    return digits.reversed().map(String.init).joined()
  }
}
