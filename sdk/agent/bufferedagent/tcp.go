package bufferedagent

// ServeTCP listens on the given address for a single incoming connection to
// start a payment channel.
func (a *Agent) ServeTCP(addr string) error {
	go a.eventLoop()
	return a.agent.ServeTCP(addr)
}

// ConnectTCP connects to the given address for establishing a single payment
// channel.
func (a *Agent) ConnectTCP(addr string) error {
	go a.eventLoop()
	return a.agent.ConnectTCP(addr)
}
