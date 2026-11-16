module.exports = {
  ...require("../../jest.config.cjs"),
  rootDir: __dirname,
  testEnvironment: "jsdom",
  setupFilesAfterEnv: ["<rootDir>/src/setupTests.ts"],
};
