const assert = require("node:assert/strict");
const path = require("node:path");
const test = require("node:test");
const ts = require("typescript");

function compileProbe(source) {
	const fileName = path.join(__dirname, "tendermint-declarations-probe.ts");
	const options = {
		strict: true,
		noEmit: true,
		target: ts.ScriptTarget.ES2022,
		module: ts.ModuleKind.CommonJS,
		moduleResolution: ts.ModuleResolutionKind.Node10,
		types: [],
	};
	const host = ts.createCompilerHost(options);
	const getSourceFile = host.getSourceFile.bind(host);
	host.getSourceFile = (name, languageVersion, onError, shouldCreateNewSourceFile) =>
		name === fileName
			? ts.createSourceFile(name, source, languageVersion, true)
			: getSourceFile(name, languageVersion, onError, shouldCreateNewSourceFile);
	const program = ts.createProgram([fileName], options, host);
	return ts.getPreEmitDiagnostics(program);
}

function diagnosticSummary(diagnostics) {
	return diagnostics.map((diagnostic) => ({
		code: diagnostic.code,
		text: diagnostic.file?.text.slice(
			diagnostic.start,
			diagnostic.start + diagnostic.length,
		),
		message: ts.flattenDiagnosticMessageText(diagnostic.messageText, "\n"),
	}));
}

function tendermint_declarations_accept_head_fields() {
	const diagnostics = compileProbe(`
import type { Proposal, Header, Commit, TxKey } from "../dist/types/types/tendermint/types/types";
import { SignedMsgType } from "../dist/types/types/tendermint/types/types";
import type { EvidenceList } from "../dist/types/types/tendermint/types/evidence";
import type { BlockParams } from "../dist/types/types/tendermint/types/params";
import type { PublicKey } from "../dist/types/types/tendermint/crypto/keys";
import type { Consensus } from "../dist/types/types/tendermint/version/types";
import type { ResponseQuery, ValidatorUpdate } from "../dist/types/types/tendermint/abci/types";

const bytes = new Uint8Array([1, 2, 3]);
const txKey: TxKey = { tx_key: bytes };
const evidence: EvidenceList = { evidence: [] };
const lastCommit: Commit = { height: 8, round: 1, signatures: [] };
const header: Header = {
  chain_id: "pax", height: 9, last_commit_hash: bytes, data_hash: bytes,
  validators_hash: bytes, next_validators_hash: bytes, consensus_hash: bytes,
  app_hash: bytes, last_results_hash: bytes, evidence_hash: bytes,
  proposer_address: bytes,
};
const proposal: Proposal = {
  type: SignedMsgType.SIGNED_MSG_TYPE_PROPOSAL, height: 9, round: 2,
  pol_round: 1, signature: bytes, txKeys: [txKey], evidence, lastCommit,
  header, proposerAddress: bytes,
};
const params: BlockParams = {
  max_bytes: 1024, max_gas: -1, minTxsInBlock: 1, maxGasWanted: 5000,
};
const publicKey: PublicKey = { ed25519: bytes };
const consensus: Consensus = { block: 11, app: 1 };
const validator: ValidatorUpdate = { pub_key: publicKey, power: 10 };
const query: ResponseQuery = {
  code: 0, log: "", info: "", index: 0, key: bytes, value: bytes,
  height: 9, codespace: "",
};
const txKeys: TxKey[] = proposal.txKeys;
const address: Uint8Array = proposal.proposerAddress;
const proposedEvidence: EvidenceList | undefined = proposal.evidence;
const proposedCommit: Commit | undefined = proposal.lastCommit;
const proposedHeader: Header | undefined = proposal.header;
const minTxs: number = params.minTxsInBlock;
const maxGas: number = params.maxGasWanted;
`);
	assert.deepEqual(diagnosticSummary(diagnostics), []);
}

function tendermint_declarations_refuse_retired_messages() {
	const retiredAbci = [
		"Request", "RequestEcho", "RequestFlush", "RequestInfo", "RequestInitChain",
		"RequestQuery", "RequestCheckTx", "RequestCommit", "RequestListSnapshots",
		"RequestOfferSnapshot", "RequestLoadSnapshotChunk", "RequestApplySnapshotChunk",
		"RequestPrepareProposal", "RequestProcessProposal", "RequestExtendVote",
		"RequestVerifyVoteExtension", "RequestFinalizeBlock", "Response",
		"ResponseException", "ResponseEcho", "ResponseFlush", "ResponseInitChain",
		"ResponseListSnapshots", "ResponseOfferSnapshot", "ResponseLoadSnapshotChunk",
		"ResponseApplySnapshotChunk", "ResponsePrepareProposal", "ResponseProcessProposal",
		"ResponseExtendVote", "ResponseVerifyVoteExtension", "CommitInfo",
		"ExtendedCommitInfo", "TxRecord", "Validator", "VoteInfo", "ExtendedVoteInfo",
		"Misbehavior", "Snapshot",
	];
	const retiredTypes = ["ExtendedCommit", "ExtendedCommitSig"];
	const imports = [
		...retiredAbci.map((name) =>
			`import type { ${name} } from "../dist/types/types/tendermint/abci/types";`),
		...retiredTypes.map((name) =>
			`import type { ${name} } from "../dist/types/types/tendermint/types/types";`),
		'import type { App } from "../dist/types/types/tendermint/version/types";',
	];
	const diagnostics = compileProbe(`
${imports.join("\n")}
import type { PublicKey } from "../dist/types/types/tendermint/crypto/keys";
const secpKey: PublicKey = { secp256k1: new Uint8Array([1]) };
const srKey: PublicKey = { sr25519: new Uint8Array([2]) };
`);
	const summary = diagnosticSummary(diagnostics);
	assert.deepEqual(
		summary.map(({ code, text }) => ({ code, text })),
		[
			...[...retiredAbci, ...retiredTypes, "App"].map((name) => ({
				code: 2305,
				text: name,
			})),
			{ code: 2353, text: "secp256k1" },
			{ code: 2353, text: "sr25519" },
		],
		JSON.stringify(summary, null, 2),
	);
}

test(tendermint_declarations_accept_head_fields);
test(tendermint_declarations_refuse_retired_messages);
