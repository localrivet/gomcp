package server

// ProcessCompletionComplete processes a completion request.
func (s *serverImpl) ProcessCompletionComplete(ctx *Context) (interface{}, error) {
	// TODO: Implement completion
	return map[string]interface{}{"completions": []interface{}{}}, nil
}
