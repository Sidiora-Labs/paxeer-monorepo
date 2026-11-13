import { subtask } from "hardhat/config";
import { TASK_TYPECHAIN_GENERATE_TYPES } from "@typechain/hardhat/dist/constants";
import { runTypeChain } from "typechain";

subtask(TASK_TYPECHAIN_GENERATE_TYPES).setAction(
  async ({ compileSolOutput, quiet }, { artifacts, config }) => {
    const allFiles = (await artifacts.getArtifactPaths()).sort();
    const typechain = config.typechain;
    const result = await runTypeChain({
      cwd: config.paths.root,
      allFiles,
      filesToProcess: allFiles,
      outDir: typechain.outDir,
      target: typechain.target,
      flags: {
        alwaysGenerateOverloads: typechain.alwaysGenerateOverloads,
        discriminateTypes: typechain.discriminateTypes,
        tsNocheck: typechain.tsNocheck,
        node16Modules: typechain.node16Modules,
        environment: "hardhat",
      },
    });
    if (!quiet) {
      console.log(`Successfully generated ${result.filesGenerated} typings!`);
    }
    return compileSolOutput;
  }
);
