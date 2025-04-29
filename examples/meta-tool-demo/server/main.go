// Command meta-tool-demo spins up an MCP server that showcases composable tools
// and a higher‑level "meta‑tool" that orchestrates them.
//
// Hosted tools
// -----------
//
//	lookup_user              – returns a mock user profile
//	get_user_orders          – returns a mock list of recent orders
//	send_confirmation_email  – simulates sending an email
//
// Meta‑tool
// ---------
//
//	process_user_activity – Invokes the three atomic tools above, composes their
//	responses, and returns a single consolidated result.
//
// Build & run:
//
//	go run .
//
// Example request via stdin:
//
//	{"method":"process_user_activity","params":{"user_id":"alice"}}
//
// The example focuses on *clarity and composability*; the business logic is
// intentionally minimal.
package main

import (
	"fmt"
	"log"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
)

// ---------------------------------------------------------------------------
// Payload types
// ---------------------------------------------------------------------------

// UserLookupArgs is the request body for both lookup_user and get_user_orders.
// UserID must not be empty.
type UserLookupArgs struct {
	UserID string `json:"user_id" description:"ID of the user" required:"true"`
}

// EmailArgs is the request body for send_confirmation_email.
// Subject and Message are required so the meta‑tool can inject arbitrary
// content.
type EmailArgs struct {
	UserID  string `json:"user_id" description:"ID of the recipient" required:"true"`
	Subject string `json:"subject" description:"Email subject line" required:"true"`
	Message string `json:"message" description:"Email body" required:"true"`
}

// ProcessActivityArgs is the request body for the composite process_user_activity
// tool.
type ProcessActivityArgs struct {
	UserID string `json:"user_id" description:"ID of the user to process" required:"true"`
}

// ---------------------------------------------------------------------------
// Handler function stubs (populated in main)
// ---------------------------------------------------------------------------

var (
	lookupUserHandler    func(UserLookupArgs) (protocol.Content, error)
	getUserOrdersHandler func(UserLookupArgs) (protocol.Content, error)
	sendEmailHandler     func(EmailArgs) (protocol.Content, error)
)

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	// Instantiate the MCP server.
	srv := server.NewServer("meta-tool-demo")

	// ---------------------------------------------------------------------
	// Tool: lookup_user
	// ---------------------------------------------------------------------
	lookupUserHandler = func(args UserLookupArgs) (protocol.Content, error) {
		log.Printf("[lookup_user] user_id=%s", args.UserID)

		profile := map[string]any{
			"user_id": args.UserID,
			"name":    "Example User",
			"email":   fmt.Sprintf("%s@example.com", args.UserID),
		}

		return server.Text(fmt.Sprintf("profile=%v", profile)), nil
	}

	if err := server.AddTool(
		srv,
		"lookup_user",
		"Return basic profile information for the supplied user_id.",
		lookupUserHandler,
	); err != nil {
		log.Fatalf("register lookup_user: %v", err)
	}

	// ---------------------------------------------------------------------
	// Tool: get_user_orders
	// ---------------------------------------------------------------------
	getUserOrdersHandler = func(args UserLookupArgs) (protocol.Content, error) {
		log.Printf("[get_user_orders] user_id=%s", args.UserID)

		orders := []map[string]string{
			{"order_id": "order123", "item": "Widget", "status": "Shipped"},
			{"order_id": "order456", "item": "Gadget", "status": "Processing"},
		}

		return server.Text(fmt.Sprintf("orders=%v", orders)), nil
	}

	if err := server.AddTool(
		srv,
		"get_user_orders",
		"Return a list of recent orders for the supplied user_id.",
		getUserOrdersHandler,
	); err != nil {
		log.Fatalf("register get_user_orders: %v", err)
	}

	// ---------------------------------------------------------------------
	// Tool: send_confirmation_email
	// ---------------------------------------------------------------------
	sendEmailHandler = func(args EmailArgs) (protocol.Content, error) {
		log.Printf("[send_confirmation_email] to=%s subject=%q", args.UserID, args.Subject)
		return server.Text("email queued"), nil
	}

	if err := server.AddTool(
		srv,
		"send_confirmation_email",
		"Send an email to the supplied user_id.",
		sendEmailHandler,
	); err != nil {
		log.Fatalf("register send_confirmation_email: %v", err)
	}

	// ---------------------------------------------------------------------
	// Tool: process_user_activity (meta‑tool)
	// ---------------------------------------------------------------------
	if err := server.AddTool(
		srv,
		"process_user_activity",
		"Composite tool: look up the user, fetch their orders, and email a summary.",
		func(args ProcessActivityArgs) (protocol.Content, error) {
			log.Printf("[process_user_activity] user_id=%s", args.UserID)

			// 1. Look up the user.
			profile, err := lookupUserHandler(UserLookupArgs{UserID: args.UserID})
			if err != nil {
				return nil, fmt.Errorf("lookup_user: %w", err)
			}

			// 2. Fetch orders.
			orders, err := getUserOrdersHandler(UserLookupArgs{UserID: args.UserID})
			if err != nil {
				return nil, fmt.Errorf("get_user_orders: %w", err)
			}

			// 3. Send email with a short summary.
			email := EmailArgs{
				UserID:  args.UserID,
				Subject: "Your recent activity",
				Message: fmt.Sprintf("Hi!\n\nHere is your recent activity summary.\n\n%v\n%v", profile, orders),
			}
			if _, err := sendEmailHandler(email); err != nil {
				return nil, fmt.Errorf("send_confirmation_email: %w", err)
			}

			// 4. Return aggregate result to the client.
			return server.Text(fmt.Sprintf("processed user_id=%s", args.UserID)), nil
		},
	); err != nil {
		log.Fatalf("register process_user_activity: %v", err)
	}

	// ---------------------------------------------------------------------
	// Start the server (stdio transport)
	// ---------------------------------------------------------------------
	log.Println("meta-tool-demo ready (stdio mode)")
	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
