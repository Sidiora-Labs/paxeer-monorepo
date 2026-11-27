import React from 'react';
import { renderToString } from 'react-dom/server';
import { expect, test } from 'vitest';
import type { V2ContractTypes } from '@kindle/sdk';
import { USDL_ADDRESS } from '@/core/network/contracts';
import { shortAddress } from '@/utils/format';
import { rewardAmountWords, rewardProgramTitle, rewardPrograms } from './reward-disclosure';
import { RewardCard } from './rewards-view';

type Reward = V2ContractTypes['Reward'];
const anotherAsset = '0x1111111111111111111111111111111111111111';
const reward: Reward = {
  rewardId: 'creator:1',
  programId: 'creator_fees',
  wallet: anotherAsset,
  assetAddress: USDL_ADDRESS,
  claimableRaw: '135000',
  claimedRaw: '0',
  earnedRaw: '135000',
  sourcePosition: null,
};

function bodyText(markup: string): string {
  return markup.replace(/<[^>]*>/g, ' ').replace(/&#x27;/g, "'").replace(/&amp;/g, '&');
}

function reward_program_title_is_plain() {
  const other = rewardPrograms.find(program => program.type === 'other')!;
  for (const [id, title] of [
    ['creator_fees', 'Creator fees'],
    ['creator-fees', 'Creator fees'],
    ['referral', 'Referrals'],
    ['cashback', 'Cashback'],
    ['weekly_trading_cup', 'Weekly trading cup'],
    ['weekly-trading-cup', 'Weekly trading cup'],
    ['', other.title],
    ['---___ ', other.title],
  ]) {
    expect(rewardProgramTitle(id)).toBe(title);
    expect(rewardProgramTitle(id)).not.toContain('_');
  }
  for (const program of rewardPrograms) {
    expect(rewardProgramTitle(program.type)).toBe(program.title);
  }
}
test(reward_program_title_is_plain.name, reward_program_title_is_plain);

function reward_amounts_of_the_quote_token_read_in_usdl() {
  for (const address of [USDL_ADDRESS.toLowerCase(), `0x${USDL_ADDRESS.slice(2).toUpperCase()}`]) {
    expect(rewardAmountWords(address, '135000', '0')).toEqual({
      usdl: true,
      claimable: '0.135 USDL',
      claimed: '0 USDL',
    });
    expect(rewardAmountWords(address, '1234567890000', '135000')).toEqual({
      usdl: true,
      claimable: '1,234,567.89 USDL',
      claimed: '0.135 USDL',
    });
    expect(rewardAmountWords(address, '1', '1000000')).toEqual({
      usdl: true,
      claimable: '0.000001 USDL',
      claimed: '1 USDL',
    });
  }
}
test(reward_amounts_of_the_quote_token_read_in_usdl.name, reward_amounts_of_the_quote_token_read_in_usdl);

function reward_amounts_of_another_asset_stay_raw() {
  expect(anotherAsset.toLowerCase()).not.toBe(USDL_ADDRESS.toLowerCase());
  expect(rewardAmountWords(anotherAsset, '135000', '0')).toEqual({
    usdl: false,
    claimable: '135000',
    claimed: '0',
  });
  expect(rewardAmountWords(anotherAsset, '12345678901234567890', '42')).toEqual({
    usdl: false,
    claimable: '12345678901234567890',
    claimed: '42',
  });
}
test(reward_amounts_of_another_asset_stay_raw.name, reward_amounts_of_another_asset_stay_raw);

function reward_card_of_the_quote_token_reads_plain() {
  const claims: Reward[] = [];
  const onClaim = (row: Reward) => { claims.push(row); };
  const markup = renderToString(<RewardCard reward={reward} headingLevel={2} claimBlocked={false} onClaim={onClaim} />);
  const text = bodyText(markup);
  expect(markup).toMatch(/<h2\b[^>]*>Creator fees<\/h2>/);
  expect(markup).toContain('data-reward-id="creator:1"');
  expect(markup).toMatch(/<caption\b[^>]*>Reward amounts in USDL<\/caption>/);
  expect(markup).toContain('>0.135 USDL</td>');
  expect(markup).toContain('>0 USDL</td>');
  expect(markup).toMatch(/<dt\b[^>]*>Paid in<\/dt>/);
  expect(markup).toContain('>USDL</span>');
  expect(text).toContain('Claimable');
  expect(text).not.toMatch(/creator_fees|135000|smallest unit|\bCoin\b/);
  expect(markup).toMatch(/<button\b[^>]*><span\b[^>]*>Claim<\/span><\/button>/);
  expect(markup).not.toMatch(/<button\b[^>]*\sdisabled=/);

  const blocked = renderToString(<RewardCard reward={reward} headingLevel={3} claimBlocked onClaim={onClaim} />);
  expect(blocked).toMatch(/<h3\b[^>]*>Creator fees<\/h3>/);
  expect(blocked).toMatch(/<button\b[^>]*disabled=""[^>]*><span\b[^>]*>Claim<\/span><\/button>/);
  expect(claims).toEqual([]);
}
test(reward_card_of_the_quote_token_reads_plain.name, reward_card_of_the_quote_token_reads_plain);

function reward_card_of_another_asset_keeps_the_raw_form() {
  const claims: Reward[] = [];
  const onClaim = (row: Reward) => { claims.push(row); };
  const otherReward: Reward = { ...reward, assetAddress: anotherAsset };
  const markup = renderToString(<RewardCard reward={otherReward} headingLevel={3} claimBlocked={false} onClaim={onClaim} />);
  expect(markup).toMatch(/<h3\b[^>]*>Creator fees<\/h3>/);
  expect(markup).toContain('data-reward-id="creator:1"');
  expect(markup).toContain('>135000</td>');
  expect(markup).toContain('>0</td>');
  expect(bodyText(markup)).toContain("Reward amounts in the coin's smallest unit");
  expect(bodyText(markup)).toContain("Amounts are shown in the coin's smallest unit");
  expect(markup).toMatch(/<dt\b[^>]*>Coin<\/dt>/);
  expect(markup).toContain(`aria-label="${anotherAsset}"`);
  expect(bodyText(markup)).toContain(shortAddress(anotherAsset));
  expect(bodyText(markup)).not.toMatch(/creator_fees|USDL|Paid in/);
  expect(markup).not.toMatch(/<button\b[^>]*\sdisabled=/);

  const zero = renderToString(<RewardCard reward={{ ...otherReward, claimableRaw: '0' }} headingLevel={2} claimBlocked={false} onClaim={onClaim} />);
  expect(bodyText(zero)).toContain('Nothing to claim');
  expect(zero).toMatch(/<button\b[^>]*disabled=""[^>]*><span\b[^>]*>Claim<\/span><\/button>/);
  expect(claims).toEqual([]);
}
test(reward_card_of_another_asset_keeps_the_raw_form.name, reward_card_of_another_asset_keeps_the_raw_form);
