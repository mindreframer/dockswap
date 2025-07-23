# End-to-End Tests

This directory contains comprehensive end-to-end tests for dockswap using Bun.sh.

## Prerequisites

- [Bun](https://bun.sh/) installed
- Docker daemon running
- dockswap source code built

## Running Tests

### Basic Flow Test
Tests the complete blue-green deployment workflow:

```bash
bun run e2e/00-basic-flow.js
```

This test covers:
- ✅ Initial deployment to first color
- ✅ Container health verification
- ✅ Status reporting
- ✅ Blue-green deployment (second color)
- ✅ Traffic switching between colors
- ✅ HTTP endpoint validation
- ✅ Cleanup

### Error Scenarios Test
Tests error handling and edge cases:

```bash
bun run e2e/01-error-scenarios.js
```

This test covers:
- ✅ Invalid app configuration rejection
- ✅ Traffic switch without deployment
- ✅ Switch to invalid color rejection
- ✅ Switch to non-existent container rejection  
- ✅ Status handling for non-existent apps
- ✅ Proper error messages and codes

### Run All Tests
```bash
bun run e2e/00-basic-flow.js && bun run e2e/01-error-scenarios.js
```

### Test Structure

- `00-basic-flow.js` - Complete basic deployment workflow
- `utils.js` - Shared utilities and helpers

## Test Features

### Colored Output
Tests use colored console output for better readability:
- 🔵 **Blue**: Step headers and info
- 🟢 **Green**: Success messages
- 🔴 **Red**: Error messages  
- 🟡 **Yellow**: Warnings and commands
- 🟣 **Cyan**: Info messages

### Error Handling
- Comprehensive error reporting
- Automatic cleanup on failure
- Detailed logging of all operations

### HTTP Validation
- Endpoint health checking with retries
- Status code validation
- Timeout handling

### Docker Integration
- Container lifecycle management
- Automatic cleanup of test containers
- Image pulling and verification

## Utilities (utils.js)

### Logging Functions
- `log(message, color)` - Basic logging with color
- `logStep(step, message)` - Step headers
- `logSuccess(message)` - Success messages
- `logError(message)` - Error messages
- `logWarning(message)` - Warning messages

### Shell Execution
- `run(command, options)` - Execute shell commands with error handling
- Support for silent execution, failure tolerance, and timeouts

### HTTP Testing
- `checkEndpoint(url, expectedStatus, timeout)` - HTTP endpoint validation
- Automatic retries and timeout handling

### Docker Utilities
- `cleanupDockswapContainers()` - Clean up test containers
- `getDockswapContainers()` - List running dockswap containers
- `pullDockerImage(image)` - Pull Docker images

### Dockswap Commands
- `dockswapDeploy(appName, image)` - Deploy application
- `dockswapSwitch(appName, color)` - Switch traffic
- `dockswapStatus(appName)` - Get deployment status

### Test Assertions
- `assertEqual(actual, expected, message)` - Value equality
- `assertTrue(condition, message)` - Boolean assertions
- `assertContainerRunning(containers, appName, color)` - Container validation

### Test Environment
- `setupE2EEnvironment(options)` - Setup test environment
- `teardownE2EEnvironment()` - Cleanup test environment

## Writing New Tests

1. Create a new `.js` file in the `e2e/` directory
2. Import utilities: `import { log, run, checkEndpoint } from './utils.js'`
3. Use the helper functions for consistent testing
4. Follow the pattern of setup → test → cleanup

Example:
```javascript
#!/usr/bin/env bun
import { log, logStep, run, setupE2EEnvironment, teardownE2EEnvironment } from './utils.js';

async function main() {
  try {
    await setupE2EEnvironment({ pullImages: ['nginx:alpine'] });
    
    logStep(1, "Testing custom feature");
    // Your test logic here
    
    log("✅ Test passed!");
  } catch (error) {
    log(`❌ Test failed: ${error.message}`);
    process.exit(1);
  } finally {
    await teardownE2EEnvironment();
  }
}

if (import.meta.main) {
  await main();
}
```

## CI/CD Integration

Tests are designed to be CI/CD friendly:
- Clean exit codes (0 for success, 1 for failure)
- Structured output for parsing
- Automatic cleanup regardless of test outcome
- Configurable timeouts and retries