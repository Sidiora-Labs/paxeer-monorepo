const NativePerformanceObserver = window.PerformanceObserver;
window.shiftDelivery = {
  held: new URL(location.href).searchParams.has("hold"),
  pending: [],
  delivered: [],
  release() {
    this.held = false;
    for (const { list, observer, callback } of this.pending.splice(0)) {
      this.delivered.push(...list.getEntries().map((entry) => ({ time: entry.startTime, value: entry.value })));
      callback(list, observer);
    }
  },
};
window.PerformanceObserver = class DelayedPerformanceObserver extends NativePerformanceObserver {
  constructor(callback) {
    super((list, observer) => {
      if (window.shiftDelivery.held && list.getEntries().some((entry) => entry.entryType === "layout-shift")) {
        window.shiftDelivery.pending.push({ list, observer, callback });
      } else callback(list, observer);
    });
  }
  takeRecords() {
    return window.shiftDelivery.held ? [] : super.takeRecords();
  }
};
