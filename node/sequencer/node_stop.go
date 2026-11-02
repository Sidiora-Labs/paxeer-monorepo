package sequencer

func (n *Node) Stop() error {
	if n == nil {
		return nil
	}
	if n.Accepter != nil {
		n.Accepter.mu.Lock()
		defer n.Accepter.mu.Unlock()
		n.Accepter.halted = true
	}
	if n.cancel != nil {
		n.cancel()
	}
	if n.Replicator != nil {
		n.Replicator.Close()
	}
	if n.WAL != nil {
		n.WAL.Close()
	}
	return nil
}
