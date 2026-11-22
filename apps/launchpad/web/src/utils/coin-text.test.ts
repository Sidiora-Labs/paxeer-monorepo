import { expect, test } from 'vitest';
import { displayCoinText } from './coin-text';
import { hasCoinTextControls, validCreationCoinText } from './coin-text-policy.mjs';
import { shortAddress } from './format';

const tokenAddress = '0x1234567890abcdef1234567890abcdef12345678';
const directionControls = [0x061c, 0x200e, 0x200f, 0x202a, 0x202b, 0x202c, 0x202d, 0x202e, 0x2066, 0x2067, 0x2068, 0x2069, 0x206a, 0x206b, 0x206c, 0x206d, 0x206e, 0x206f];

function coinTextBlankLabelsUseTokenAddress() {
  for (const value of ['', '   ', '\t\n', '\u00a0\u2003', '\u0000', '\u202e', undefined, null]) {
    expect(displayCoinText(value, tokenAddress)).toBe(shortAddress(tokenAddress));
  }
}
test(coinTextBlankLabelsUseTokenAddress.name, coinTextBlankLabelsUseTokenAddress);

function coinTextRemovesControlAndDirectionCharacters() {
  const controls = [...Array.from({ length: 32 }, (_, code) => code), ...Array.from({ length: 33 }, (_, code) => code + 127), ...directionControls];
  for (const code of controls) {
    const character = String.fromCodePoint(code);
    expect(hasCoinTextControls(character)).toBe(true);
    expect(displayCoinText(`Coin${character}Name`, tokenAddress)).toBe('CoinName');
    expect(displayCoinText(character, tokenAddress)).toBe(shortAddress(tokenAddress));
  }
}
test(coinTextRemovesControlAndDirectionCharacters.name, coinTextRemovesControlAndDirectionCharacters);

function coinTextPreservesOrdinaryAndLongUnicodeText() {
  for (const value of ['Paxe', 'MiXeD', '  Coin Name  ', 'عملة', 'מטבע', 'e\u0301', '👩‍🚀', '<Coin & Name>', 'X'.repeat(100), '🪙'.repeat(100)]) {
    expect(displayCoinText(value, tokenAddress)).toBe(value);
  }
  expect([...displayCoinText('🪙'.repeat(100), tokenAddress)]).toHaveLength(100);
}
test(coinTextPreservesOrdinaryAndLongUnicodeText.name, coinTextPreservesOrdinaryAndLongUnicodeText);

function creationAuthorityRejectsControlAndDirectionCharactersInBothFields() {
  for (const character of ['\u0000', '\n', '\u0085', ...directionControls.map(code => String.fromCodePoint(code))]) {
    expect(validCreationCoinText(`A${character}`, 'COIN')).toBe(false);
    expect(validCreationCoinText('Coin', `C${character}`)).toBe(false);
  }
  expect(validCreationCoinText('Coin', 'MiXeD')).toBe(true);
  expect(validCreationCoinText('N'.repeat(64), 'S'.repeat(16))).toBe(true);
  for (const [name, symbol] of [['', 'S'], ['N', ''], ['N'.repeat(65), 'S'], ['N', 'S'.repeat(17)]]) {
    expect(validCreationCoinText(name, symbol)).toBe(false);
  }
  expect(validCreationCoinText(null, 'S')).toBe(false);
  expect(validCreationCoinText('N', 1)).toBe(false);
}
test(creationAuthorityRejectsControlAndDirectionCharactersInBothFields.name, creationAuthorityRejectsControlAndDirectionCharactersInBothFields);
