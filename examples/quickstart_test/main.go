// Package main tests the README quickstart example to ensure it doesn't fail with transport configuration errors
// This test verifies that GitHub issue #11 has been resolved.
//
// Expected behavior:
// - ✅ Client creation should succeed (no "no transport configured" error)
// - ❌ Connection/tool call should fail (no server connected to stdin - this is expected)
//
// Before the fix: Would fail immediately with "no transport configured, use WithTransport option"
// After the fix: Client creation succeeds, but connection times out (expected when no server is connected)
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/localrivet/gomcp/client"
)

func main() {
	fmt.Println("🧪 Testing README quickstart example (GitHub issue #11)...")
	fmt.Println("📋 Verifying transport configuration fixes...")
	fmt.Println()

	// Test 1: README example with explicit transport (should work)
	fmt.Println("✅ Test 1: README example with explicit transport")
	testExplicitTransport()
	fmt.Println()

	// Test 2: Generic client name (should now default to stdio)
	fmt.Println("✅ Test 2: Generic client name (auto-defaults to stdio)")
	testGenericClientName()
	fmt.Println()

	// Test 3: Empty string URL (should default to stdio)
	fmt.Println("✅ Test 3: Empty string URL (auto-defaults to stdio)")
	testEmptyStringURL()
	fmt.Println()

	fmt.Println("🎉 All tests passed! GitHub issue #11 is resolved.")
	fmt.Println("✅ Users can now create clients without explicit transport configuration.")
}

func testExplicitTransport() {
	// This is the exact code from the README quickstart
	c, err := client.NewClient("stdio:///",
		client.WithStdio(),
		client.WithProtocolVersion("2025-03-26"),
		client.WithProtocolNegotiation(true),
	)
	if err != nil {
		log.Fatalf("❌ Failed to create client with explicit transport: %v", err)
	}
	defer c.Close()

	fmt.Println("   ✅ Client created successfully with explicit transport")

	// Try to call a tool (should fail gracefully since no server is connected)
	_, err = c.CallTool("say_hello", map[string]interface{}{
		"name": "World",
	})
	if err != nil {
		if strings.Contains(err.Error(), "no transport configured") {
			log.Fatalf("❌ Still getting transport configuration error: %v", err)
		}
		fmt.Printf("   ✅ Tool call failed as expected (no server connected): %v\n", err)
	} else {
		fmt.Println("   ⚠️  Tool call succeeded unexpectedly")
	}
}

func testGenericClientName() {
	// This should now work by defaulting to stdio transport
	c, err := client.NewClient("my-client",
		client.WithProtocolVersion("2025-03-26"),
		client.WithProtocolNegotiation(true),
	)
	if err != nil {
		log.Fatalf("❌ Failed to create client with generic name: %v", err)
	}
	defer c.Close()

	fmt.Println("   ✅ Client created successfully with generic name (auto-defaulted to stdio)")

	// Try to call a tool (should fail gracefully since no server is connected)
	_, err = c.CallTool("say_hello", map[string]interface{}{
		"name": "World",
	})
	if err != nil {
		if strings.Contains(err.Error(), "no transport configured") {
			log.Fatalf("❌ Still getting transport configuration error: %v", err)
		}
		fmt.Printf("   ✅ Tool call failed as expected (no server connected): %v\n", err)
	} else {
		fmt.Println("   ⚠️  Tool call succeeded unexpectedly")
	}
}

func testEmptyStringURL() {
	// This should also work by defaulting to stdio transport
	c, err := client.NewClient("",
		client.WithProtocolVersion("2025-03-26"),
		client.WithProtocolNegotiation(true),
	)
	if err != nil {
		log.Fatalf("❌ Failed to create client with empty string URL: %v", err)
	}
	defer c.Close()

	fmt.Println("   ✅ Client created successfully with empty string URL (auto-defaulted to stdio)")

	// Try to call a tool (should fail gracefully since no server is connected)
	_, err = c.CallTool("say_hello", map[string]interface{}{
		"name": "World",
	})
	if err != nil {
		if strings.Contains(err.Error(), "no transport configured") {
			log.Fatalf("❌ Still getting transport configuration error: %v", err)
		}
		fmt.Printf("   ✅ Tool call failed as expected (no server connected): %v\n", err)
	} else {
		fmt.Println("   ⚠️  Tool call succeeded unexpectedly")
	}
}
