module.exports = {
  testMatch: ["**/?(*.)+(spec|test).[jt]s?(x)"],
  setupFiles: [require.resolve("./jest.setup.cjs")],
  transform: {
    "^.+\\.[cm]?[jt]sx?$": [
      "babel-jest",
      { configFile: require.resolve("./babel.config.cjs"), babelrc: false },
    ],
  },
  moduleNameMapper: {
    "^(\\.{1,2}/.*)\\.js$": "$1",
  },
  transformIgnorePatterns: ["/node_modules/(?!.*(?:@noble|@scure|uuid)/)"],
  maxWorkers: 2,
};
