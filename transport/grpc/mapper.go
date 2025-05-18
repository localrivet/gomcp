package grpc

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	pb "github.com/localrivet/gomcp/transport/grpc/proto/gen"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrorMap defines mappings between JSON-RPC error codes and gRPC status codes
var ErrorMap = map[int]codes.Code{
	// JSON-RPC standard error codes
	-32700: codes.InvalidArgument, // Parse error
	-32600: codes.InvalidArgument, // Invalid Request
	-32601: codes.Unimplemented,   // Method not found
	-32602: codes.InvalidArgument, // Invalid params
	-32603: codes.Internal,        // Internal error
	-32000: codes.Internal,        // Server error (base)

	// MCP specific error codes
	1000: codes.Internal,           // Initialization failed
	1001: codes.Unauthenticated,    // Authentication failed
	1002: codes.PermissionDenied,   // Authorization failed
	1003: codes.Aborted,            // Session expired
	1004: codes.Internal,           // Function execution failed
	1005: codes.NotFound,           // Invalid session
	1006: codes.ResourceExhausted,  // Rate limited
	1007: codes.Unavailable,        // Stream closed
	1008: codes.DeadlineExceeded,   // Timeout
	1009: codes.Unavailable,        // Connection error
	1010: codes.FailedPrecondition, // Protocol error
}

// GRPCToJSONRPCError converts a gRPC status error to a JSON-RPC error
func GRPCToJSONRPCError(err error) *pb.ErrorInfo {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		// Not a gRPC status error, use internal error
		return &pb.ErrorInfo{
			Code:    -32603, // Internal error
			Message: err.Error(),
		}
	}

	// Find the JSON-RPC error code that matches the gRPC status code
	var code int32 = -32603 // Default to internal error
	for jsonRPCCode, grpcCode := range ErrorMap {
		if grpcCode == st.Code() {
			code = int32(jsonRPCCode)
			break
		}
	}

	return &pb.ErrorInfo{
		Code:    code,
		Message: st.Message(),
		Data:    fmt.Sprintf("gRPC status code: %s", st.Code().String()),
	}
}

// JSONRPCToGRPCError converts a JSON-RPC error to a gRPC status error
func JSONRPCToGRPCError(jsonError *pb.ErrorInfo) error {
	if jsonError == nil {
		return nil
	}

	// Find the gRPC status code that matches the JSON-RPC error code
	grpcCode, ok := ErrorMap[int(jsonError.Code)]
	if !ok {
		// Default to internal error if not found
		grpcCode = codes.Internal
	}

	return status.Error(grpcCode, jsonError.Message)
}

// ValueToProto converts a Go value to a Protocol Buffer Value
func ValueToProto(val interface{}) (*pb.Value, error) {
	if val == nil {
		return &pb.Value{Kind: &pb.Value_NullValue{NullValue: true}}, nil
	}

	switch v := val.(type) {
	case string:
		return &pb.Value{Kind: &pb.Value_StringValue{StringValue: v}}, nil
	case bool:
		return &pb.Value{Kind: &pb.Value_BoolValue{BoolValue: v}}, nil
	case int:
		return &pb.Value{Kind: &pb.Value_NumberValue{NumberValue: float64(v)}}, nil
	case int32:
		return &pb.Value{Kind: &pb.Value_NumberValue{NumberValue: float64(v)}}, nil
	case int64:
		return &pb.Value{Kind: &pb.Value_NumberValue{NumberValue: float64(v)}}, nil
	case float32:
		return &pb.Value{Kind: &pb.Value_NumberValue{NumberValue: float64(v)}}, nil
	case float64:
		return &pb.Value{Kind: &pb.Value_NumberValue{NumberValue: v}}, nil
	case []byte:
		return &pb.Value{Kind: &pb.Value_BinaryValue{BinaryValue: v}}, nil
	case []interface{}:
		arr := &pb.Array{Values: make([]*pb.Value, len(v))}
		for i, item := range v {
			arrVal, err := ValueToProto(item)
			if err != nil {
				return nil, err
			}
			arr.Values[i] = arrVal
		}
		return &pb.Value{Kind: &pb.Value_ArrayValue{ArrayValue: arr}}, nil
	case map[string]interface{}:
		obj := &pb.Object{Fields: make(map[string]*pb.Value)}
		for key, val := range v {
			objVal, err := ValueToProto(val)
			if err != nil {
				return nil, err
			}
			obj.Fields[key] = objVal
		}
		return &pb.Value{Kind: &pb.Value_ObjectValue{ObjectValue: obj}}, nil
	default:
		// For other types, try JSON marshaling and then convert to a string
		data, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal value: %w", err)
		}
		return &pb.Value{Kind: &pb.Value_StringValue{StringValue: string(data)}}, nil
	}
}

// ProtoToValue converts a Protocol Buffer Value to a Go value
func ProtoToValue(val *pb.Value) (interface{}, error) {
	if val == nil {
		return nil, nil
	}

	switch v := val.Kind.(type) {
	case *pb.Value_StringValue:
		return v.StringValue, nil
	case *pb.Value_BoolValue:
		return v.BoolValue, nil
	case *pb.Value_NumberValue:
		// Return as float64 by default, caller can convert as needed
		return v.NumberValue, nil
	case *pb.Value_BinaryValue:
		return v.BinaryValue, nil
	case *pb.Value_NullValue:
		return nil, nil
	case *pb.Value_ArrayValue:
		if v.ArrayValue == nil {
			return []interface{}{}, nil
		}
		arr := make([]interface{}, len(v.ArrayValue.Values))
		for i, item := range v.ArrayValue.Values {
			val, err := ProtoToValue(item)
			if err != nil {
				return nil, err
			}
			arr[i] = val
		}
		return arr, nil
	case *pb.Value_ObjectValue:
		if v.ObjectValue == nil {
			return map[string]interface{}{}, nil
		}
		obj := make(map[string]interface{})
		for key, item := range v.ObjectValue.Fields {
			val, err := ProtoToValue(item)
			if err != nil {
				return nil, err
			}
			obj[key] = val
		}
		return obj, nil
	default:
		return nil, fmt.Errorf("unknown value type: %T", v)
	}
}

// MapToJSONRPCRequest converts a Protocol Buffer FunctionRequest to a JSON-RPC request
func MapToJSONRPCRequest(req *pb.FunctionRequest) (map[string]interface{}, error) {
	params := make(map[string]interface{})
	for k, v := range req.Parameters {
		val, err := ProtoToValue(v)
		if err != nil {
			return nil, err
		}
		params[k] = val
	}

	// Create a JSON-RPC request
	jsonRPC := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  req.FunctionId,
		"params":  params,
		"id":      req.RequestId,
	}

	return jsonRPC, nil
}

// MapFromJSONRPCResponse converts a JSON-RPC response to a Protocol Buffer FunctionResponse
func MapFromJSONRPCResponse(jsonResp map[string]interface{}, functionID, requestID string) (*pb.FunctionResponse, error) {
	// Extract result or error from JSON-RPC response
	var response pb.FunctionResponse
	response.FunctionId = functionID
	response.RequestId = requestID

	// If there's an error in the JSON-RPC response
	if errObj, ok := jsonResp["error"].(map[string]interface{}); ok {
		code := int32(-32603) // Default to internal error
		message := "Unknown error"
		var data string

		if c, ok := errObj["code"]; ok {
			switch c := c.(type) {
			case float64:
				code = int32(c)
			case int:
				code = int32(c)
			case string:
				if intVal, err := strconv.Atoi(c); err == nil {
					code = int32(intVal)
				}
			}
		}

		if m, ok := errObj["message"].(string); ok {
			message = m
		}

		if d, ok := errObj["data"]; ok {
			dataBytes, err := json.Marshal(d)
			if err == nil {
				data = string(dataBytes)
			}
		}

		response.Result = &pb.FunctionResponse_Error{
			Error: &pb.ErrorInfo{
				Code:    code,
				Message: message,
				Data:    data,
			},
		}
	} else if result, ok := jsonResp["result"]; ok {
		// Convert to protocol buffer Value
		resultValue, err := ValueToProto(result)
		if err != nil {
			return nil, err
		}
		response.Result = &pb.FunctionResponse_ResultValue{
			ResultValue: resultValue,
		}
	} else {
		// Neither error nor result found
		return nil, fmt.Errorf("invalid JSON-RPC response: no result or error field")
	}

	response.IsFinal = true
	return &response, nil
}

// BuildFunctionRequest creates a FunctionRequest from parameters
func BuildFunctionRequest(functionID string, params map[string]interface{}) (*pb.FunctionRequest, error) {
	req := &pb.FunctionRequest{
		FunctionId:  functionID,
		RequestId:   fmt.Sprintf("req-%d", time.Now().UnixNano()),
		Parameters:  make(map[string]*pb.Value),
		IsStreaming: false,
	}

	// Convert parameters to protocol buffer Values
	for key, val := range params {
		pbVal, err := ValueToProto(val)
		if err != nil {
			return nil, fmt.Errorf("failed to convert parameter %s: %w", key, err)
		}
		req.Parameters[key] = pbVal
	}

	return req, nil
}
