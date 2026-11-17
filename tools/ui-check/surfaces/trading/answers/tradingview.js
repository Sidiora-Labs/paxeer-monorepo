globalThis.TradingView = {
  widget: function TradingViewWidget(options) {
    const chart = {
      symbol: () => options.symbol,
      setSymbol: () => {},
      setResolution: () => {},
      setChartType: () => {},
      executeActionById: () => {},
      onVisibleRangeChanged: () => ({ subscribe: () => {} }),
    };
    const instance = {
      _iFrame: { contentDocument: document, contentWindow: window },
      activeChart: () => chart,
      symbolInterval: () => ({ interval: options.interval }),
      setSymbol: () => {},
      onChartReady(callback) {
        queueMicrotask(callback);
      },
      subscribe: () => {},
      save(callback) {
        callback({});
      },
      applyOverrides: () => {},
      remove: () => {},
    };
    return instance;
  },
};
