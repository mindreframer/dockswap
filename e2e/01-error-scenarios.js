#!/usr/bin/env bun
import {
  log, logStep, logSuccess, logError, logInfo,
  run, setupE2EEnvironment, teardownE2EEnvironment,
  dockswapDeploy, dockswapStatus,
  assertTrue, colors, resetStepCounter
} from './utils.js';

// Test configuration
const TEST_APP = "nginx-test"; // Use existing config
const INVALID_APP = "error-test"; // Non-existent app for error testing
const VALID_IMAGE = "nginx:alpine";

// Main test execution
async function main() {
  const startTime = Date.now();

  // Reset step counter for this test
  resetStepCounter();

  log(`${colors.bold}${colors.blue}🧪 Testing Error Scenarios${colors.reset}`);
  log(`${colors.yellow}Testing error handling and edge cases${colors.reset}\n`);

  try {
    // Setup
    logStep("Setting up test environment");
    await setupE2EEnvironment({
      pullImages: [VALID_IMAGE],
      cleanup: true
    });

    // Test invalid app configuration
    logStep("Testing invalid app configuration");
    logInfo("Attempting to deploy app with no configuration...");
    const invalidAppResult = await run(
      `./dockswap deploy ${INVALID_APP} ${VALID_IMAGE}`,
      { allowFailure: true, silent: true }
    );

    assertTrue(!invalidAppResult.success, "Expected deployment to fail for nonexistent app");
    assertTrue(
      invalidAppResult.stderr.includes("no configuration found") ||
      invalidAppResult.stdout.includes("no configuration found"),
      "Expected 'no configuration found' error message"
    );
    logSuccess("Invalid app configuration properly rejected");

    // Test switch without deployment
    logStep("Testing traffic switch without deployment");
    logInfo("Attempting to switch traffic with no containers...");
    const switchResult = await run(
      `./dockswap switch ${TEST_APP} blue`,
      { allowFailure: true, silent: true }
    );

    assertTrue(!switchResult.success, "Expected switch to fail with no containers");
    logSuccess("Switch without deployment properly rejected");

    // Test valid deployment
    logStep("Testing valid deployment for comparison");
    await dockswapDeploy(TEST_APP, VALID_IMAGE);

    const status = await dockswapStatus(TEST_APP);
    assertTrue(status.activeColor !== undefined, "Status should show active color");
    logSuccess("Valid deployment works as expected");

    // Test switching to invalid color
    logStep("Testing switch to invalid color");
    logInfo("Attempting to switch to invalid color...");
    const invalidColorResult = await run(
      `./dockswap switch ${TEST_APP} purple`,
      { allowFailure: true, silent: true }
    );

    assertTrue(!invalidColorResult.success, "Expected switch to fail for invalid color");
    assertTrue(
      invalidColorResult.stderr.includes("must be 'blue' or 'green'") ||
      invalidColorResult.stdout.includes("must be 'blue' or 'green'"),
      "Expected color validation error message"
    );
    logSuccess("Invalid color properly rejected");

    // Test switching to non-existent container
    logStep("Testing switch to non-existent container");
    const currentStatus = await dockswapStatus(TEST_APP);
    const otherColor = currentStatus.activeColor === 'blue' ? 'green' : 'blue';

    logInfo(`Attempting to switch to ${otherColor} (no container)...`);
    const noContainerResult = await run(
      `./dockswap switch ${TEST_APP} ${otherColor}`,
      { allowFailure: true, silent: true }
    );

    assertTrue(!noContainerResult.success, "Expected switch to fail for non-existent container");
    // Log actual error for debugging
    if (!noContainerResult.success) {
      logInfo(`Actual error: ${noContainerResult.stderr || noContainerResult.stdout}`);
    }
    logSuccess("Switch to non-existent container properly rejected");

    // Test status for non-existent app
    logStep("Testing status for non-existent app");
    logInfo("Checking status for non-existent app...");
    await run(
      `./dockswap status nonexistent-app`,
      { allowFailure: true, silent: true }
    );

    // This might succeed with empty results or fail - both are acceptable
    logSuccess("Status for non-existent app handled appropriately");

    // Success!
    const duration = ((Date.now() - startTime) / 1000).toFixed(2);
    log(`\n${colors.bold}${colors.green}🎉 All error scenario tests passed! (${duration}s)${colors.reset}`);
    log(`${colors.green}✓ Invalid app configuration rejected${colors.reset}`);
    log(`${colors.green}✓ Switch without deployment rejected${colors.reset}`);
    log(`${colors.green}✓ Valid deployment still works${colors.reset}`);
    log(`${colors.green}✓ Invalid color rejected${colors.reset}`);
    log(`${colors.green}✓ Switch to non-existent container rejected${colors.reset}`);
    log(`${colors.green}✓ Status for non-existent app handled${colors.reset}`);

  } catch (error) {
    const duration = ((Date.now() - startTime) / 1000).toFixed(2);
    log(`\n${colors.bold}${colors.red}💥 Error scenario test failed after ${duration}s${colors.reset}`);
    logError(error.message);
    process.exit(1);

  } finally {
    // Always cleanup
    await teardownE2EEnvironment();
  }
}

// Run the tests
if (import.meta.main) {
  await main();
}