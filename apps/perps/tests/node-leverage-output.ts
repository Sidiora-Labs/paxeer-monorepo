import { decodeFunctionResult, type Hex } from "viem";
import { PerpsPrecompileABI } from "../src/core/network/abi/PerpsPrecompile";
import { decodeMarketConfig } from "../src/core/perps/marketConfig";
import output from "./fixtures/leverage-node-output.json";

// Unmodified eth_call bytes captured from the node identified in the fixture.
export const nodeLeverageOutput = output;
export const nodeMarketConfig = decodeMarketConfig(
  decodeFunctionResult({
    abi: PerpsPrecompileABI,
    functionName: "getMarketConfig",
    data: output.calls.getMarketConfig.response.result as Hex,
  }),
);
export const nodeMarginTiers = decodeFunctionResult({
  abi: PerpsPrecompileABI,
  functionName: "getMarginTiers",
  data: output.calls.getMarginTiers.response.result as Hex,
});
