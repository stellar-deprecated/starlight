package close

func (c *Channel) CloseStart(iterationNumber int) error {
	return nil
}

func (c *Channel) CloseCoordinated(id string) (newStatus string, err error) {
	return "", nil
}

func (c *Channel) CloseUncoordinated(id string) error {
	return nil
}
