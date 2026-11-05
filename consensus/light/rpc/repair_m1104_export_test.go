package rpc

// RepairM1104Subscription exposes completion to the external integration tests
// without adding lifecycle inspection to the production client API.
type RepairM1104Subscription = wsSubscription

func (s *wsSubscription) RepairM1104Done() <-chan struct{} {
	return s.done
}

func RepairM1104LookupSubscription(c *Client, subscriber, query string) *RepairM1104Subscription {
	c.subscriptionsMu.Lock()
	defer c.subscriptionsMu.Unlock()
	return c.subscriptions[wsSubscriptionKey{subscriber: subscriber, query: query}]
}

func RepairM1104SubscriptionCount(c *Client) int {
	c.subscriptionsMu.Lock()
	defer c.subscriptionsMu.Unlock()
	return len(c.subscriptions)
}
