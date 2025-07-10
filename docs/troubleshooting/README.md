# Troubleshooting Guide

This guide helps you diagnose and fix common issues when using GoMCP.

## Common Issues

### Embedded Mode Timeout Errors

**Symptoms:**
- Repeated "context deadline exceeded" errors in logs
- Error message: `failed to handle roots/list request`
- Multiple agents or clients initializing simultaneously

**Root Cause:**
In embedded mode, when many clients initialize simultaneously (e.g., 34 agents), the `roots/list` requests can overwhelm the embedded transport, causing timeouts.

**Solutions:**

1. **Reduce Concurrent Initialization:**
   ```go
   // Stagger agent initialization
   for i, agent := range agents {
       go func(a Agent, delay time.Duration) {
           time.Sleep(delay)
           a.Initialize()
       }(agent, time.Duration(i)*100*time.Millisecond)
   }
   ```

2. **Increase Timeout Values:**
   ```go
   // Configure longer timeouts for embedded transport
   client, err := client.NewClient("embedded://",
       client.WithRequestTimeout(60*time.Second),
       client.WithConnectionTimeout(30*time.Second))
   ```

3. **Use Rate Limiting:**
   The server now automatically limits concurrent `roots/list` requests to 5 to prevent overwhelming the embedded transport.

4. **Monitor Resource Usage:**
   - Check CPU and memory usage during initialization
   - Consider reducing the number of concurrent agents if resources are constrained

**Prevention:**
- Initialize agents in batches rather than all at once
- Use proper backoff strategies for retry logic
- Monitor system resources during high-concurrency scenarios

### Other Issues

*Additional troubleshooting sections can be added here*
