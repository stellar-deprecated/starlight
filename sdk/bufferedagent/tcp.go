package bufferedagent

func (a *Agent) ServeTCP(addr string) error {
	return a.agent.ServeTCP(addr)
}

func (a *Agent) ConnectTCP(addr string) error {
	return a.agent.ConnectTCP(addr)
}
