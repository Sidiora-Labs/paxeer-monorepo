package evmrpc

func SetReceiptLogFeedForTest(api *FilterAPI, feed *ReceiptLogFeed) {
	api.setReceiptLogFeed(feed)
}

func ShutdownFilterAPIForTest(api *FilterAPI) {
	api.shutdown()
}
