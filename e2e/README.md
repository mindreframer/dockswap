# End-to-End Tests

This directory contains comprehensive end-to-end tests for dockswap using Bun's test framework.

## Prerequisites

- [Bun](https://bun.sh/) installed
- Docker daemon running
- dockswap source code built

## Running Tests

### Using Bun Test Framework (Recommended)

The tests have been converted to use Bun's test framework with proper `describe`, `test`, `beforeEach`, and `afterEach` hooks.

#### Run All E2E Tests
```bash
bun test e2e/*.test.js --timeout=30000
```

#### Run Specific Test Files
```bash
# Basic flow tests
bun test e2e/01-basic-flow.test.js --timeout=30000

# Error scenario tests
bun test e2e/02-error-scenarios.test.js --timeout=30000
```

#### Run Tests with Pattern Matching
```bash
# Run only basic flow tests
bun test e2e/01-basic-flow.test.js

# Run only error tests
bun test e2e/02-error-scenarios.test.js

# Run tests matching a pattern
bun test e2e/ --test-name-pattern "deployment"
```

### Legacy Scripts (Deprecated)

The original manual scripts are still available for reference:

```bash
# Basic flow test (legacy)
bun run e2e/00-basic-flow.js

# Error scenarios test (legacy)
bun run e2e/01-error-scenarios.js
```

## Test Structure

### New Test Framework Files
- `01-basic-flow.test.js` - Complete basic deployment workflow using Bun test framework
- `02-error-scenarios.test.js` - Error handling and edge cases using Bun test framework

### Legacy Files (for reference)
- `00-basic-flow.js` - Original basic deployment workflow
- `01-error-scenarios.js` - Original error scenarios test
- `utils.js` - Shared utilities and helpers

## Test Categories

### Basic Flow Tests (`01-basic-flow.test.js`)
Tests the complete blue-green deployment workflow:

- ✅ **should perform complete blue-green deployment workflow** - Comprehensive test covering:
  - Initial deployment to first color
  - Container health verification
  - Status reporting and validation
  - Blue-green deployment (second color)
  - Traffic switching between colors
  - HTTP endpoint validation
  - Final verification

### Error Scenario Tests (`02-error-scenarios.test.js`)
Tests error handling and edge cases:

- ✅ **should handle all deployment-related errors** - Comprehensive deployment error testing:
  - Invalid app configuration rejection
  - Invalid image rejection
  - Malformed command arguments
- ✅ **should handle all traffic switching errors** - Comprehensive traffic switching error testing:
  - Switch without deployment
  - Invalid color rejection
  - Switch to non-existent container
- ✅ **should handle status and validation scenarios** - Status and validation testing:
  - Status for non-existent app
  - Valid deployment verification

## Test Features

### Bun Test Framework Benefits
- **Proper test isolation** - Each test runs in isolation with setup/teardown
- **Better test discovery** - Tests are automatically discovered and organized
- **Individual test timing** - Each test shows its execution time
- **Test filtering** - Run specific tests or test patterns
- **Better error reporting** - Clear test context and failure information
- **Parallel execution** - Tests can run in parallel when possible
- **Optimized performance** - Grouped tests share setup/teardown overhead (68% faster than individual tests)

### Colored Output
Tests use colored console output for better readability:
- 🔵 **Blue**: Step headers and info
- 🟢 **Green**: Success messages
- 🔴 **Red**: Error messages  
- 🟡 **Yellow**: Warnings and commands
- 🟣 **Cyan**: Info messages

### Error Handling
- Comprehensive error reporting with test context
- Automatic cleanup on failure
- Detailed logging of all operations
- Proper test isolation

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
- `logStep(message)` - Step headers with auto-incrementing numbers
- `logSuccess(message)` - Success messages
- `logError(message)` - Error messages
- `logWarning(message)` - Warning messages
- `logInfo(message)` - Info messages

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

### Using Bun Test Framework (Recommended)

1. Create a new `.test.js` file in the `e2e/` directory
2. Import Bun test functions and utilities
3. Use `describe` and `test` blocks for organization
4. Use `beforeEach` and `afterEach` for setup/teardown

Example:
```javascript
#!/usr/bin/env bun
import { describe, test, beforeEach, afterEach, expect } from "bun:test";
import { log, logStep, run, setupE2EEnvironment, teardownE2EEnvironment } from './utils.js';

describe("My Feature Tests", () => {
  beforeEach(async () => {
    await setupE2EEnvironment({ pullImages: ['nginx:alpine'] });
  });

  afterEach(async () => {
    await teardownE2EEnvironment();
  });

  test("should perform my test", async () => {
    logStep("Testing my feature");
    // Your test logic here
    expect(result).toBe(true);
  });
});
```

### Legacy Script Pattern (Deprecated)

For reference, the old pattern was:
```javascript
#!/usr/bin/env bun
import { log, logStep, run, setupE2EEnvironment, teardownE2EEnvironment } from './utils.js';

async function main() {
  try {
    await setupE2EEnvironment({ pullImages: ['nginx:alpine'] });
    
    logStep("Testing custom feature");
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
- Proper test isolation and parallel execution support