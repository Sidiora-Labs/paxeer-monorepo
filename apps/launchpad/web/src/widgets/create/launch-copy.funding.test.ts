import { expect, test } from 'vitest';
import { KnownCapabilityAdmissionError } from './cap-admission';
import { KnownPurchaseNotSubmittedError } from './creation-adapters';
import { fundingCopy } from './fee-summary';
import { CREATION_PAUSED, plainLaunchError } from './launch-copy';

const usdlMessage = 'insufficient_usdl: USDL must cover the one USDL fee plus the selected first purchase.';
const gasMessage = 'insufficient_gas: Your USDL balance is sufficient, but PAX is required separately for network gas.';

function fundingCopyUsdlError() {
  expect(plainLaunchError(new Error(usdlMessage))).toBe('Not enough USDL. You need the 1 USDL fee plus your first buy');
}

function fundingCopyGasError() {
  expect(plainLaunchError(new Error(gasMessage))).toBe('Not enough PAX to pay the network fee');
}

function fundingCopyUsdlAdmissionError() {
  const cause = new Error(usdlMessage);
  expect(plainLaunchError(new KnownCapabilityAdmissionError(cause.message, cause))).toBe(fundingCopy.insufficient_usdl);
}

function fundingCopyGasAdmissionError() {
  const cause = new Error(gasMessage);
  expect(plainLaunchError(new KnownCapabilityAdmissionError(cause.message, cause))).toBe(fundingCopy.insufficient_gas);
}

function fundingCopyPrefixPriority() {
  for (const [prefix, expected] of [
    ['insufficient_usdl', fundingCopy.insufficient_usdl],
    ['insufficient_gas', fundingCopy.insufficient_gas],
  ]) {
    const message = `${prefix}: admission insufficient funds another tab web locks image metadata purchase recover wrong network wallet changed draft expired`;
    expect(plainLaunchError(message)).toBe(expected);
    expect(plainLaunchError(new TypeError(message))).toBe(expected);
    expect(plainLaunchError(new KnownCapabilityAdmissionError(message, new Error(message)))).toBe(expected);
  }
}

function fundingCopyFirstBuyError() {
  const cause = new Error('Purchase simulation failed.');
  expect(plainLaunchError(new KnownPurchaseNotSubmittedError(cause.message, cause))).toBe("Your first buy didn't go through. Try again");
}

function fundingCopyAdmissionRefusal() {
  const cause = new Error('Creation admission failed before wallet dispatch.');
  expect(plainLaunchError(new KnownCapabilityAdmissionError(cause.message, cause))).toBe(CREATION_PAUSED);
  expect(plainLaunchError(new KnownCapabilityAdmissionError(`Creation admission: ${usdlMessage}`, new Error(usdlMessage)))).toBe(CREATION_PAUSED);
}

function fundingCopyWalletRejection() {
  expect(plainLaunchError(new Error('User rejected the request.'))).toBe('You cancelled in your wallet');
  for (const message of [usdlMessage, gasMessage]) {
    const cause = Object.assign(new Error(message), { code: 4001 });
    expect(plainLaunchError(cause)).toBe('You cancelled in your wallet');
    expect(plainLaunchError(new KnownCapabilityAdmissionError(cause.message, cause))).toBe('You cancelled in your wallet');
    const rejected = `${message} User rejected the request.`;
    expect(plainLaunchError(new Error(rejected))).toBe('You cancelled in your wallet');
    expect(plainLaunchError(new KnownCapabilityAdmissionError(rejected, new Error(rejected)))).toBe('You cancelled in your wallet');
  }
}

function fundingCopyMatchesSummary() {
  expect(plainLaunchError(usdlMessage)).toBe(fundingCopy.insufficient_usdl);
  expect(plainLaunchError(gasMessage)).toBe(fundingCopy.insufficient_gas);
}

test('funding copy names the missing USDL balance', fundingCopyUsdlError);
test('funding copy names the missing PAX balance', fundingCopyGasError);
test('funding copy names USDL through an admission error', fundingCopyUsdlAdmissionError);
test('funding copy names PAX through an admission error', fundingCopyGasAdmissionError);
test('funding copy prefixes take priority over other error text', fundingCopyPrefixPriority);
test('funding copy preserves the first buy message', fundingCopyFirstBuyError);
test('funding copy preserves a creation admission refusal', fundingCopyAdmissionRefusal);
test('funding copy preserves wallet rejection priority', fundingCopyWalletRejection);
test('funding copy matches the funding summary', fundingCopyMatchesSummary);
