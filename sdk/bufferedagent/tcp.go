package bufferedagent

func (a *Agent) ServeTCP(addr string) error {
	go a.eventLoop()
	return a.agent.ServeTCP(addr)
}

func (a *Agent) ConnectTCP(addr string) error {
	go a.eventLoop()
	return a.agent.ConnectTCP(addr)
}
