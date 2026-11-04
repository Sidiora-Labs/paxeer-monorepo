package sequencer

func (n *Node) Stop() error {
	if n == nil {
		return nil
	}
	n.lifecycle.Lock()
	defer n.lifecycle.Unlock()
	if n.cancel != nil {
		n.cancel()
	}
	if n.leaseDone != nil {
		<-n.leaseDone
	}
	if n.Standby != nil && n.Standby.serveDone != nil {
		<-n.Standby.serveDone
	}
	if n.Accepter != nil {
		n.Accepter.mu.Lock()
		defer n.Accepter.mu.Unlock()
		n.Accepter.halted = true
	}
	if n.Replicator != nil {
		n.Replicator.Close()
	}
	if n.WAL != nil {
		n.WAL.Close()
	}
	return nil
}
