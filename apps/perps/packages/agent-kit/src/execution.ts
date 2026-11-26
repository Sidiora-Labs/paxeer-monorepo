import {
  encodeFunctionData,
  getAbiItem,
  toFunctionSignature,
  type Hex,
} from "viem";
import { erc20Abi } from "@paxeer-network/paxeer-backend/core/perps/config";
import * as calls from "@paxeer-network/paxeer-backend/core/perps/calls";
import type { ApproveCall } from "@paxeer-network/paxeer-backend/core/network/approvals";
import { AgentGatewayClient, type WireTransaction } from "./gateway.js";
import { AgentKitError } from "./result.js";

export type TradingCall =
  | ReturnType<typeof calls.buildDeposit>
  | ReturnType<typeof calls.buildWithdraw>
  | ReturnType<typeof calls.buildOpenPosition>
  | ReturnType<typeof calls.buildClosePosition>
  | ReturnType<typeof calls.buildSetPositionTpSl>
  | ReturnType<typeof calls.buildAddCollateral>
  | ReturnType<typeof calls.buildRemoveCollateral>
  | ReturnType<typeof calls.buildCancelOrder>;

export function transaction(call: TradingCall | ApproveCall): WireTransaction {
  const data: Hex =
    call.functionName === "approve"
      ? encodeFunctionData({ ...call, abi: erc20Abi })
      : encodeFunctionData(call);
  return { to: call.address, data, value: "0" };
}

/** Executes one backend plan through the gateway's agent lane. */
export async function executePlan(
  gateway: AgentGatewayClient,
  plan: readonly (TradingCall | ApproveCall)[],
  idempotencyKey?: string,
): Promise<unknown> {
  const approval = plan.find(
    (call): call is ApproveCall => call.functionName === "approve",
  );
  const writes = plan.filter(
    (call): call is TradingCall => call.functionName !== "approve",
  );
  const call = writes[0];
  if (
    !call ||
    writes.length !== 1 ||
    plan.length !== (approval ? 2 : 1) ||
    (approval && plan[0] !== approval)
  ) {
    throw new AgentKitError(
      "INVALID_PLAN",
      "A trading plan must contain one call and at most one preceding approval",
    );
  }
  if (approval) {
    if (
      call.functionName !== "deposit" &&
      call.functionName !== "addCollateral"
    ) {
      throw new AgentKitError(
        "INVALID_PLAN",
        "Only deposit and collateral addition may request approval",
      );
    }
    if (
      !idempotencyKey ||
      approval.args[0].toLowerCase() !== call.address.toLowerCase()
    ) {
      throw new AgentKitError(
        "INVALID_PLAN",
        "Approval needs the call spender and an idempotency key",
      );
    }
    // The durable action simulates its call after confirming the approval.
    await gateway.simulate(transaction(approval));
    const method = getAbiItem({ abi: call.abi, name: call.functionName });
    return gateway.allowanceAndCall({
      token: approval.address,
      spender: approval.args[0],
      amount: approval.args[1].toString(),
      contract: call.address,
      method: toFunctionSignature(method),
      args: call.args.map((value) => value.toString()),
      idempotency_key: idempotencyKey,
    });
  }
  const tx = transaction(call);
  await gateway.simulate(tx);
  return gateway.send(tx);
}
