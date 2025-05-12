package protocol

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONRPCRequestSerialization(t *testing.T) {
	// Test case 1: Basic request with string ID
	req1 := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "req-123",
		Method:  "test.method",
		Params:  map[string]interface{}{"key": "value"},
	}

	data1, err := json.Marshal(req1)
	require.NoError(t, err)

	var parsed1 map[string]interface{}
	err = json.Unmarshal(data1, &parsed1)
	require.NoError(t, err)

	assert.Equal(t, "2.0", parsed1["jsonrpc"])
	assert.Equal(t, "req-123", parsed1["id"])
	assert.Equal(t, "test.method", parsed1["method"])
	assert.NotNil(t, parsed1["params"])

	// Test case 2: Request with numeric ID
	req2 := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      42,
		Method:  "another.method",
		Params:  []string{"param1", "param2"},
	}

	data2, err := json.Marshal(req2)
	require.NoError(t, err)

	var parsed2 map[string]interface{}
	err = json.Unmarshal(data2, &parsed2)
	require.NoError(t, err)

	assert.Equal(t, "2.0", parsed2["jsonrpc"])
	assert.Equal(t, float64(42), parsed2["id"]) // JSON numbers are float64
	assert.Equal(t, "another.method", parsed2["method"])

	// Test case 3: Request with nil ID (should be allowed)
	req3 := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      nil,
		Method:  "test.method",
		Params:  nil, // No params
	}

	data3, err := json.Marshal(req3)
	require.NoError(t, err)

	var parsed3 map[string]interface{}
	err = json.Unmarshal(data3, &parsed3)
	require.NoError(t, err)

	assert.Equal(t, "2.0", parsed3["jsonrpc"])
	assert.Nil(t, parsed3["id"])
	assert.Equal(t, "test.method", parsed3["method"])
	_, hasParams := parsed3["params"]
	assert.False(t, hasParams, "params field should be omitted when nil")
}

func TestJSONRPCResponseSerialization(t *testing.T) {
	// Test case 1: Success response
	resp1 := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      "resp-123",
		Result:  map[string]interface{}{"status": "success"},
	}

	data1, err := json.Marshal(resp1)
	require.NoError(t, err)

	var parsed1 map[string]interface{}
	err = json.Unmarshal(data1, &parsed1)
	require.NoError(t, err)

	assert.Equal(t, "2.0", parsed1["jsonrpc"])
	assert.Equal(t, "resp-123", parsed1["id"])
	assert.NotNil(t, parsed1["result"])
	_, hasError := parsed1["error"]
	assert.False(t, hasError, "error field should be omitted in success response")

	// Test case 2: Error response
	resp2 := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      "err-456",
		Error: &ErrorPayload{
			Code:    CodeInvalidParams,
			Message: "Invalid parameters",
			Data:    map[string]string{"field": "username"},
		},
	}

	data2, err := json.Marshal(resp2)
	require.NoError(t, err)

	var parsed2 map[string]interface{}
	err = json.Unmarshal(data2, &parsed2)
	require.NoError(t, err)

	assert.Equal(t, "2.0", parsed2["jsonrpc"])
	assert.Equal(t, "err-456", parsed2["id"])
	_, hasResult := parsed2["result"]
	assert.False(t, hasResult, "result field should be omitted in error response")

	errorObj, hasError := parsed2["error"].(map[string]interface{})
	assert.True(t, hasError, "error field should be present")
	assert.Equal(t, float64(CodeInvalidParams), errorObj["code"])
	assert.Equal(t, "Invalid parameters", errorObj["message"])
	assert.NotNil(t, errorObj["data"])

	// Test case 3: Response with null ID (pre-parsing error)
	resp3 := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      nil,
		Error: &ErrorPayload{
			Code:    CodeParseError,
			Message: "Parse error",
		},
	}

	data3, err := json.Marshal(resp3)
	require.NoError(t, err)

	var parsed3 map[string]interface{}
	err = json.Unmarshal(data3, &parsed3)
	require.NoError(t, err)

	assert.Equal(t, "2.0", parsed3["jsonrpc"])
	assert.Nil(t, parsed3["id"])
	var hasResult3 bool
	_, hasResult3 = parsed3["result"]
	assert.False(t, hasResult3, "result field should be omitted in error response")
	assert.NotNil(t, parsed3["error"])
}

func TestJSONRPCNotificationSerialization(t *testing.T) {
	// Test case 1: Basic notification
	notif1 := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  "system.notify",
		Params:  map[string]interface{}{"event": "update", "data": 42},
	}

	data1, err := json.Marshal(notif1)
	require.NoError(t, err)

	var parsed1 map[string]interface{}
	err = json.Unmarshal(data1, &parsed1)
	require.NoError(t, err)

	assert.Equal(t, "2.0", parsed1["jsonrpc"])
	assert.Equal(t, "system.notify", parsed1["method"])
	assert.NotNil(t, parsed1["params"])
	var hasID1 bool
	_, hasID1 = parsed1["id"]
	assert.False(t, hasID1, "id field should not be present in notifications")

	// Test case 2: Notification without params
	notif2 := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  "heartbeat",
	}

	data2, err := json.Marshal(notif2)
	require.NoError(t, err)

	var parsed2 map[string]interface{}
	err = json.Unmarshal(data2, &parsed2)
	require.NoError(t, err)

	assert.Equal(t, "2.0", parsed2["jsonrpc"])
	assert.Equal(t, "heartbeat", parsed2["method"])
	_, hasParams := parsed2["params"]
	assert.False(t, hasParams, "params field should be omitted when nil")
	var hasID2 bool
	_, hasID2 = parsed2["id"]
	assert.False(t, hasID2, "id field should not be present in notifications")
}

func TestJSONRPCHelperFunctions(t *testing.T) {
	// Test NewSuccessResponse
	successResp := NewSuccessResponse("req-id", map[string]string{"status": "ok"})
	assert.Equal(t, "2.0", successResp.JSONRPC)
	assert.Equal(t, "req-id", successResp.ID)
	assert.NotNil(t, successResp.Result)
	assert.Nil(t, successResp.Error)

	// Test NewErrorResponse
	errorResp := NewErrorResponse("err-id", CodeInternalError, "Internal error", nil)
	assert.Equal(t, "2.0", errorResp.JSONRPC)
	assert.Equal(t, "err-id", errorResp.ID)
	assert.Nil(t, errorResp.Result)
	assert.NotNil(t, errorResp.Error)
	assert.Equal(t, CodeInternalError, errorResp.Error.Code)
	assert.Equal(t, "Internal error", errorResp.Error.Message)

	// Test NewNotification
	notif := NewNotification("test.event", map[string]bool{"success": true})
	assert.Equal(t, "2.0", notif.JSONRPC)
	assert.Equal(t, "test.event", notif.Method)
	assert.NotNil(t, notif.Params)
}

func TestUnmarshalPayload(t *testing.T) {
	// Test case 1: Valid unmarshal
	type TestStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	sourceData := map[string]interface{}{
		"name":  "test",
		"value": 42,
	}

	var target TestStruct
	err := UnmarshalPayload(sourceData, &target)
	require.NoError(t, err)
	assert.Equal(t, "test", target.Name)
	assert.Equal(t, 42, target.Value)

	// Test case 2: Nil payload
	var target2 TestStruct
	err = UnmarshalPayload(nil, &target2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "payload is nil")

	// Test case 3: Type mismatch
	sourceData3 := map[string]interface{}{
		"name":  "test",
		"value": "not-a-number", // Should be int
	}

	var target3 TestStruct
	err = UnmarshalPayload(sourceData3, &target3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal payload")
}

func TestJSONRPCRequestDeserialization(t *testing.T) {
	// Test valid JSON-RPC request
	reqJSON := `{
		"jsonrpc": "2.0",
		"id": "req-789",
		"method": "example.method",
		"params": {"foo": "bar", "baz": 123}
	}`

	var req JSONRPCRequest
	err := json.Unmarshal([]byte(reqJSON), &req)
	require.NoError(t, err)

	assert.Equal(t, "2.0", req.JSONRPC)
	assert.Equal(t, "req-789", req.ID)
	assert.Equal(t, "example.method", req.Method)

	// Extract params
	params, ok := req.Params.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "bar", params["foo"])
	assert.Equal(t, float64(123), params["baz"])

	// Test with invalid version
	wrongVersionJSON := `{
		"jsonrpc": "1.0", 
		"id": "invalid", 
		"method": "test"
	}`

	var wrongReq JSONRPCRequest
	err = json.Unmarshal([]byte(wrongVersionJSON), &wrongReq)
	require.NoError(t, err)                  // Parsing succeeds, but JSONRPC will be "1.0"
	assert.Equal(t, "1.0", wrongReq.JSONRPC) // We don't validate in Unmarshal
}

func TestBatchRequest(t *testing.T) {
	// While the JSON-RPC 2.0 spec allows for batch requests, confirm that we can parse them
	batchJSON := `[
		{"jsonrpc": "2.0", "id": "1", "method": "method1", "params": {"key": "value1"}},
		{"jsonrpc": "2.0", "id": "2", "method": "method2", "params": {"key": "value2"}},
		{"jsonrpc": "2.0", "method": "notification", "params": {"event": "something"}}
	]`

	var batch []map[string]interface{}
	err := json.Unmarshal([]byte(batchJSON), &batch)
	require.NoError(t, err)
	assert.Len(t, batch, 3)

	// Check first request
	assert.Equal(t, "2.0", batch[0]["jsonrpc"])
	assert.Equal(t, "1", batch[0]["id"])
	assert.Equal(t, "method1", batch[0]["method"])

	// Check second request
	assert.Equal(t, "2.0", batch[1]["jsonrpc"])
	assert.Equal(t, "2", batch[1]["id"])
	assert.Equal(t, "method2", batch[1]["method"])

	// Check notification (no id)
	assert.Equal(t, "2.0", batch[2]["jsonrpc"])
	assert.Equal(t, "notification", batch[2]["method"])
	var hasIDBatch bool
	_, hasIDBatch = batch[2]["id"]
	assert.False(t, hasIDBatch)
}
