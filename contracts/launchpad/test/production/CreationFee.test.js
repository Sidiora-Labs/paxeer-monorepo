"use strict";

const { expect } = require("chai");
const { artifacts } = require("hardhat");
const { planCreationFee } = require("../../scripts/set-creation-fee");
const { NATIVE_SURFACE_RUNTIME } = require("../../scripts/lib/native-surface");

async function CreationFeeNativeRefusal() {
  const { deployedBytecode } = await artifacts.readArtifact("ProtocolConfig");
  expect(deployedBytecode).not.to.equal("0x");
  expect(planCreationFee({ code: deployedBytecode, fee: 1_000_000n })).to.deep.equal({
    kind: "contract",
  });

  const authority = "pax1authority";
  const params = Object.freeze({
    virtual_quote_default: "30000000000",
    virtual_token_default: "1000000000000000000000000",
    min_fee_bps: "10",
    max_fee_bps: "100",
    base_fee_bps: "30",
    protocol_fee_bps: "5",
    fee_decay_rate: "1",
    volatility_weight: "2",
    concentration_weight: "3",
    creation_fee: "5000000",
    quote_token: "0x0000000000000000000000000000000000000001",
  });
  const originalParams = { ...params };

  for (const code of [
    NATIVE_SURFACE_RUNTIME,
    NATIVE_SURFACE_RUNTIME.toUpperCase(),
    new String(NATIVE_SURFACE_RUNTIME),
  ]) {
    for (const fee of [1_000_000n, "9007199254740993123456789"]) {
      const plan = planCreationFee({ code, params, fee, authority });
      expect(plan).to.deep.equal({
        kind: "governance",
        msg: {
          "@type": "/paxprotocol.paxchain.launchpad.MsgUpdateParams",
          authority,
          params: { ...originalParams, creation_fee: String(fee) },
          role_grants: [],
          role_revokes: [],
        },
      });
      expect(JSON.parse(JSON.stringify(plan.msg))).to.deep.equal(plan.msg);
      expect(plan.msg.params).not.to.equal(params);
      expect(params).to.deep.equal(originalParams);
    }
  }

  const input = { code: NATIVE_SURFACE_RUNTIME, params, fee: 1_000_000n, authority };
  for (const missingAuthority of [undefined, null, "", "   "]) {
    expect(() => planCreationFee({ ...input, authority: missingAuthority })).to.throw(
      "LAUNCHPAD_AUTHORITY"
    );
  }
  for (const missingParams of [undefined, null, [], ""]) {
    expect(() => planCreationFee({ ...input, params: missingParams })).to.throw(
      "LAUNCHPAD_PARAMS_JSON"
    );
  }
}

it("CreationFeeNativeRefusal: native surfaces get a governance message", CreationFeeNativeRefusal);
