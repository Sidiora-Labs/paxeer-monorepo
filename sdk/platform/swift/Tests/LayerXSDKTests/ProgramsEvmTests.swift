import Foundation
import XCTest

@testable import LayerXSDK

#if canImport(FoundationNetworking)
  import FoundationNetworking
#endif

final class ProgramsEvmTests: XCTestCase {
  private struct Fixture: Decodable {
    let payload_hex: String
    let signed_activity_hex: String
    let idempotency_key_hex: String
    let evm: EVM
    struct EVM: Decodable {
      let to: String
      let chainId: UInt64
      let calldata: String
    }
  }

  private func fixture(_ name: String) throws -> Fixture {
    let root = URL(fileURLWithPath: #filePath)
      .deletingLastPathComponent().deletingLastPathComponent()
      .deletingLastPathComponent().deletingLastPathComponent()
    return try JSONDecoder().decode(
      Fixture.self,
      from: Data(
        contentsOf:
          root.appendingPathComponent("conformance/fixtures/\(name).json")))
  }

  private func client(_ server: RPCFixtureServer) throws -> ProgramsClient {
    try ProgramsClient(
      rpcURL: server.url, sender: Data(repeating: 0x55, count: 20),
      actor: Data(repeating: 0x66, count: 32), receiptKeys: [],
      maxFeePerGas: 100, maxPriorityFeePerGas: 2)
  }

  private func nativeCall(_ fixture: Fixture) throws -> ProgramCall {
    try ProgramCall(
      nativeCall: NativeProgramCall.decode(nativeTransportHex(fixture.payload_hex)),
      feeLimit: ProtocolAmount("1000"))
  }

  private func assertTransaction(
    _ tx: UnsignedProgramTransaction, fixture: Fixture,
    requests: [[String: JSONValue]], file: StaticString = #filePath,
    line: UInt = #line
  ) throws {
    XCTAssertEqual(tx.type, 2, file: file, line: line)
    XCTAssertEqual(tx.chainID, fixture.evm.chainId, file: file, line: line)
    XCTAssertEqual(
      try ProgramABI.hex(tx.to), try ProgramABI.hex(normalize(fixture.evm.to)), file: file,
      line: line)
    XCTAssertEqual(tx.calldata, try ProgramABI.hex(fixture.evm.calldata), file: file, line: line)
    XCTAssertEqual(tx.nonce, 7, file: file, line: line)
    XCTAssertEqual(tx.gas, 100_000, file: file, line: line)
    XCTAssertEqual(tx.maxFeePerGas, 100, file: file, line: line)
    XCTAssertEqual(tx.maxPriorityFeePerGas, 2, file: file, line: line)
    XCTAssertEqual(tx.value, 0, file: file, line: line)
    XCTAssertEqual(tx.signingPayload().first, 2, file: file, line: line)
    let signingFields = try decodeRLPList(Data(tx.signingPayload().dropFirst()))
    XCTAssertEqual(signingFields.count, 9, file: file, line: line)
    XCTAssertEqual(signingFields[0], Data([125]), file: file, line: line)
    XCTAssertEqual(signingFields[1], Data([7]), file: file, line: line)
    XCTAssertEqual(signingFields[2], Data([2]), file: file, line: line)
    XCTAssertEqual(signingFields[3], Data([100]), file: file, line: line)
    XCTAssertEqual(signingFields[4], Data([1, 0x86, 0xa0]), file: file, line: line)
    XCTAssertEqual(signingFields[5], try ProgramABI.hex(tx.to), file: file, line: line)
    XCTAssertEqual(signingFields[6], Data(), file: file, line: line)
    XCTAssertEqual(signingFields[7], tx.calldata, file: file, line: line)
    XCTAssertEqual(signingFields[8], Data(), file: file, line: line)
    XCTAssertEqual(requests.count, 2, file: file, line: line)
    XCTAssertEqual(
      requests[0]["method"], .string("eth_getTransactionCount"), file: file, line: line)
    XCTAssertEqual(
      requests[0]["params"],
      .array([
        .string("0x" + String(repeating: "55", count: 20)), .string("pending"),
      ]), file: file, line: line)
    XCTAssertEqual(requests[1]["method"], .string("eth_estimateGas"), file: file, line: line)
    XCTAssertEqual(
      requests[1]["params"], .array([tx.rpcValue(includeGas: false), .string("pending")]),
      file: file, line: line)
    XCTAssertNil(tx.rpcValue().objectValue?["r"], file: file, line: line)
    XCTAssertNil(tx.rpcValue().objectValue?["s"], file: file, line: line)
  }

  private func normalize(_ address: String) -> String {
    let digits = String(address.dropFirst(2)).lowercased()
    return "0x" + String(repeating: "0", count: 40 - digits.count) + digits
  }

  func testProgramCallBuildsLayerxprogramsCallTransaction() async throws {
    let vector = try fixture("native-program-call-v4")
    let server = try RPCFixtureServer()
    defer { server.stop() }
    let programs = try client(server)
    let call = try nativeCall(vector)
    let transaction = try await programs.submit(
      call, idempotencyKey: IdempotencyKey(vector.idempotency_key_hex))
    try assertTransaction(transaction, fixture: vector, requests: server.requests())
    XCTAssertNil(call.transactionHash)
    let maximum = try ProgramCall(
      nativeCall: XCTUnwrap(call.nativeCall),
      feeLimit: ProtocolAmount("340282366920938463463374607431768211455"))
    XCTAssertEqual(
      try maximum.evmCalldata().subdata(in: 452..<484),
      Data(repeating: 0, count: 16) + Data(repeating: 255, count: 16))
    let signed = try ProgramCall(
      nativeCall: XCTUnwrap(call.nativeCall), feeLimit: ProtocolAmount("1000"),
      signedActivity: nativeTransportHex(vector.signed_activity_hex))
    XCTAssertEqual(try signed.evmCalldata(), transaction.calldata)
  }

  func testProgramSimulateUsesPendingEthCall() async throws {
    let vector = try fixture("native-program-call-v4")
    let server = try RPCFixtureServer()
    defer { server.stop() }
    let simulation = try await client(server).simulate(nativeCall(vector))
    XCTAssertEqual(simulation.terminalResult, 0)
    XCTAssertEqual(simulation.terminalKind, 1)
    XCTAssertEqual(simulation.response, Data([0, 0x61, 0xff, 0x10]))
    let requests = try server.requests()
    XCTAssertEqual(requests.count, 1)
    XCTAssertEqual(requests[0]["method"], .string("eth_call"))
    let calldata = "0x3044bd56" + vector.evm.calldata.dropFirst(10)
    XCTAssertEqual(
      requests[0]["params"],
      .array([
        .object([
          "from": .string("0x" + String(repeating: "55", count: 20)),
          "to": .string(Programs.address), "data": .string(String(calldata)),
        ]), .string("pending"),
      ]))
    XCTAssertEqual(requests[0]["http_path"], .string("/"))
    XCTAssertFalse(
      requests.contains { $0["http_path"]?.stringValue?.hasPrefix("/v1/programs") == true })
  }

  func testProgramDeployBuildsLayerxprogramsDeployTransaction() async throws {
    let vector = try fixture("native-program-deploy-v4")
    let operation = try NativeProgramDeploy.decode(nativeTransportHex(vector.payload_hex))
    let request = try NativeProgramLifecycleRequest(
      operation: operation,
      signedActivity: nativeTransportHex(vector.signed_activity_hex))
    XCTAssertEqual(try request.calldata(), try ProgramABI.hex(vector.evm.calldata))
    let server = try RPCFixtureServer()
    defer { server.stop() }
    let transaction = try await client(server).deploy(
      operation,
      signedActivity: nativeTransportHex(vector.signed_activity_hex),
      idempotencyKey: IdempotencyKey(vector.idempotency_key_hex))
    try assertTransaction(transaction, fixture: vector, requests: server.requests())
    XCTAssertEqual(
      try NativeProgramLifecycleRequest.calldata(operation: operation), transaction.calldata)
    var invalid = try nativeTransportHex(vector.payload_hex)
    invalid[invalid.count - 1] ^= 1
    XCTAssertThrowsError(try NativeProgramDeploy.decode(invalid))
  }

  private func decodeRLPList(_ bytes: Data) throws -> [Data] {
    let bytes = [UInt8](bytes)
    var index = 0
    func next() throws -> (Data, Bool) {
      let prefix = try XCTUnwrap(bytes.indices.contains(index) ? bytes[index] : nil)
      index += 1
      if prefix < 128 { return (Data([prefix]), false) }
      let list = prefix >= 192
      let short = list ? 192 : 128
      let long = list ? 247 : 183
      let length: Int
      if Int(prefix) <= long {
        length = Int(prefix) - short
      } else {
        let width = Int(prefix) - long
        guard width <= 8, index + width <= bytes.count else { throw NativeProgramCallError.invalid }
        length = bytes[index..<(index + width)].reduce(0) { ($0 << 8) | Int($1) }
        index += width
      }
      guard length <= bytes.count - index else { throw NativeProgramCallError.invalid }
      let payload = Data(bytes[index..<(index + length)])
      index += length
      return (payload, list)
    }
    let outer = try next()
    XCTAssertTrue(outer.1)
    XCTAssertEqual(index, bytes.count)
    let inner = [UInt8](outer.0)
    var cursor = 0
    var fields: [Data] = []
    while cursor < inner.count {
      let prefix = Int(inner[cursor])
      cursor += 1
      if prefix < 128 {
        fields.append(Data([UInt8(prefix)]))
        continue
      }
      if prefix == 192 {
        fields.append(Data())
        continue
      }
      guard prefix < 192 else { throw NativeProgramCallError.invalid }
      var length = prefix - 128
      if prefix > 183 {
        let width = prefix - 183
        guard cursor + width <= inner.count else { throw NativeProgramCallError.invalid }
        length = inner[cursor..<(cursor + width)].reduce(0) { ($0 << 8) | Int($1) }
        cursor += width
      }
      guard length <= inner.count - cursor else { throw NativeProgramCallError.invalid }
      fields.append(Data(inner[cursor..<(cursor + length)]))
      cursor += length
    }
    return fields
  }
}

private final class RPCFixtureServer {
  let url: URL
  private let process: Process
  private let directory: URL
  private let transcript: URL

  init() throws {
    directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    transcript = directory.appendingPathComponent("rpc.jsonl")
    process = Process()
    process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
    process.arguments = [
      "python3", "-u", "-c",
      #"""
      import http.server, json, signal, sys
      signal.pthread_sigmask(signal.SIG_UNBLOCK, {signal.SIGTERM})
      class Handler(http.server.BaseHTTPRequestHandler):
          def log_message(self, *args): pass
          def do_POST(self):
              request = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
              with open(sys.argv[1], 'a') as output:
                  output.write(json.dumps(dict(request, http_path=self.path)) + '\n')
              method = request['method']
              if method == 'eth_getTransactionCount': result = '0x7'
              elif method == 'eth_estimateGas': result = '0x186a0'
              elif method == 'eth_call':
                  result = '0x' + ''.join(format(v, '064x') for v in [0, 1, 96, 4]) + '0061ff10' + '00' * 28
              else:
                  self.send_error(400)
                  return
              body = json.dumps({'jsonrpc': '2.0', 'id': request['id'], 'result': result}).encode()
              self.send_response(200)
              self.send_header('Content-Type', 'application/json')
              self.send_header('Content-Length', str(len(body)))
              self.end_headers()
              self.wfile.write(body)
      server = http.server.HTTPServer(('127.0.0.1', 0), Handler)
      print(server.server_port, flush=True)
      server.serve_forever()
      """#, transcript.path,
    ]
    let stdout = Pipe()
    process.standardOutput = stdout
    process.standardError = FileHandle.standardError
    try process.run()
    var port = Data()
    while true {
      let byte = stdout.fileHandleForReading.readData(ofLength: 1)
      guard !byte.isEmpty else { throw NativeProgramCallError.invalid }
      if byte == Data([10]) { break }
      port.append(byte)
    }
    guard let text = String(data: port, encoding: .utf8), let number = UInt16(text),
      let address = URL(string: "http://127.0.0.1:\(number)")
    else { throw NativeProgramCallError.invalid }
    url = address
  }
  func requests() throws -> [[String: JSONValue]] {
    let bytes = try String(contentsOf: transcript, encoding: .utf8)
    return try bytes.split(separator: "\n").map {
      try XCTUnwrap(JSONDecoder().decode(JSONValue.self, from: Data($0.utf8)).objectValue)
    }
  }
  func stop() {
    if process.isRunning {
      process.terminate()
      process.waitUntilExit()
    }
    try? FileManager.default.removeItem(at: directory)
  }
  deinit { stop() }
}
