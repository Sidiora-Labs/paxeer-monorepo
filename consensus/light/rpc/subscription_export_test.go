package rpc

// SubscriptionForTest exposes completion to the external integration tests
// without adding lifecycle inspection to the production client API.
type SubscriptionForTest = wsSubscription

func (s *wsSubscription) DoneForTest() <-chan struct{} {
	return s.done
}

func LookupTestSubscription(c *Client, subscriber, query string) *SubscriptionForTest {
	c.subscriptionsMu.Lock()
	defer c.subscriptionsMu.Unlock()
	return c.subscriptions[wsSubscriptionKey{subscriber: subscriber, query: query}]
}

func SubscriptionCountForTest(c *Client) int {
	c.subscriptionsMu.Lock()
	defer c.subscriptionsMu.Unlock()
	return len(c.subscriptions)
}
