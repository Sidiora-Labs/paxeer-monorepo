const hre = require("hardhat");
const lib = require("../test/lib");
const deployWasm = lib.deployWasm;
const deployErc20PointerForCw20 = lib.deployErc20PointerForCw20;
const getAdmin = lib.getAdmin;
const { ABI, WASM } = lib;

async function main() {
    const admin = await getAdmin();
    console.log(`adminAddr: ${admin.paxAddress}`);
    const args = {
        name: "BTOK",
        symbol: "BTOK",
        decimals: 6,
        initial_balances: [
            { address: admin.paxAddress, amount: "1000000" }
        ],
        mint: { minter: admin.paxAddress, cap: "99900000000" }
    };
    const cw20Address = await deployWasm(WASM.CW20, admin.paxAddress, "cw20", args);
    console.log(`contractAddress: ${cw20Address}`);

    const [deployer] = await hre.ethers.getSigners();
    const addressToCheck = await deployer.getAddress();
    console.log("Deploying contracts with the account:", addressToCheck);
    const pointerAddress = await deployErc20PointerForCw20(hre.ethers.provider, cw20Address);
    console.log(`pointerAddress: ${pointerAddress}`);
    const pointer = new hre.ethers.Contract(pointerAddress, ABI.ERC20, hre.ethers.provider);
    const balance = await pointer.balanceOf(addressToCheck);
    console.log(`Balance of ${addressToCheck}: ${balance}`);
}

main().catch((error) => {
    console.error(error);
    process.exitCode = 1;
});
