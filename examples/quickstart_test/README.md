# README Quickstart Test

This test verifies that [GitHub issue #11](https://github.com/localrivet/gomcp/issues/11) has been resolved.

## Issue Description

The original README quickstart example was missing transport configuration, causing users to encounter:

```
Failed to create client: failed to connect to MCP server: no transport configured, use WithTransport option
```

## Fix Applied

The README quickstart example now includes proper transport configuration:

```go
c, err := client.NewClient("stdio:///",
    client.WithStdio(),                    // ← Added transport configuration
    client.WithProtocolVersion("2025-03-26"),
    client.WithProtocolNegotiation(true),
)
```

## Test Verification

This test confirms that:

1. **✅ No transport error**: Client creation succeeds without "no transport configured" error
2. **✅ Transport works**: JSON-RPC initialization message is sent successfully  
3. **✅ Expected behavior**: Connection times out when no server is connected (expected)

## Running the Test

```bash
cd examples/quickstart_test
go run main.go
```

**Expected Output:**
- The test should show that transport configuration works
- It will timeout after attempting to connect (expected - no server connected to stdin)
- This proves the transport configuration issue is resolved

## Before vs After

**Before fix:** Failed immediately with transport configuration error  
**After fix:** Successfully configures transport and attempts connection

The README quickstart example now works correctly when users copy and paste it! 