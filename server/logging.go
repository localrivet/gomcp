package server

import (
	"encoding/json"
	"fmt"
)

// ProcessLoggingSetLevel processes a logging set level request.
func (s *serverImpl) ProcessLoggingSetLevel(ctx *Context) (interface{}, error) {
	// Parse the request
	var params struct {
		Level string `json:"level"`
	}
	if err := json.Unmarshal(ctx.Request.Params, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	// Update the logger level
	// TODO: Implement proper level setting
	s.logger.Debug("setting log level", "level", params.Level)

	return map[string]interface{}{"success": true}, nil
}
