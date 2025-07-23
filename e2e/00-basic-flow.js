#!/usr/bin/env bun

// Test configuration
const TEST_APP = "nginx-test";
const TEST_IMAGE = "nginx:alpine";
const BLUE_PORT = 8080;
const GREEN_PORT = 8081;
const DOCKSWAP_BIN = "./dockswap";


// Import utilities after they're defined
import {
  log, logStep, logSuccess, logError, run, setupE2EEnvironment, teardownE2EEnvironment, colors, resetStepCounter, checkEndpoint
} from './utils.js';

// Setup functions
async function setup() {
  logStep("Setting up test environment");

  await setupE2EEnvironment({
    pullImages: [TEST_IMAGE],
    cleanup: true
  });
}

// Test the basic deployment flow
async function testBasicDeployment() {
  logStep("Testing basic deployment flow");

  // Deploy to first color (should be green since no current state)
  log(`Deploying ${TEST_APP} with ${TEST_IMAGE}...`);
  const deployResult = await run(`${DOCKSWAP_BIN} deploy ${TEST_APP} ${TEST_IMAGE}`);

  if (!deployResult.success) {
    throw new Error("Initial deployment failed");
  }

  logSuccess("Initial deployment completed");

  // Check that container is running
  log("Verifying container is running...");
  const containerCheck = await run(
    `docker ps --filter "label=dockswap.managed=true" --format "{{.Names}}\t{{.Status}}"`,
    { silent: true }
  );

  const runningContainers = containerCheck.stdout.trim().split('\n').filter(line => line.trim());
  if (runningContainers.length === 0) {
    throw new Error("No containers are running after deployment");
  }

  logSuccess(`Container running: ${runningContainers[0]}`);

  // Determine which port to check based on container name
  const containerName = runningContainers[0].split('\t')[0];
  const port = containerName.includes('blue') ? BLUE_PORT : GREEN_PORT;

  // Test HTTP endpoint
  log(`Testing HTTP endpoint on port ${port}...`);
  const endpointResult = await checkEndpoint(`http://localhost:${port}`);

  if (!endpointResult.success) {
    throw new Error(`HTTP endpoint test failed: ${endpointResult.error || `Status ${endpointResult.status}`}`);
  }

  logSuccess(`HTTP endpoint responding correctly (${endpointResult.status})`);

  return { deployedColor: containerName.includes('blue') ? 'blue' : 'green', port };
}

// Test deployment status
async function testStatus() {
  logStep("Testing status command");

  log("Checking deployment status...");
  const statusResult = await run(`${DOCKSWAP_BIN} status ${TEST_APP}`);

  if (!statusResult.success) {
    throw new Error("Status command failed");
  }

  // Parse status output
  const statusLines = statusResult.stdout.split('\n');
  const colorLine = statusLines.find(line => line.includes('Color:'));
  const statusLine = statusLines.find(line => line.includes('Status:'));

  if (!colorLine || !statusLine) {
    throw new Error("Could not parse status output");
  }

  const activeColor = colorLine.split('Color:')[1].trim();
  const deploymentStatus = statusLine.split('Status:')[1].trim();

  logSuccess(`Status check passed - Active: ${activeColor}, Status: ${deploymentStatus}`);

  return { activeColor, deploymentStatus };
}

// Test blue-green deployment
async function testBlueGreenDeployment(currentColor) {
  logStep("Testing blue-green deployment");

  const targetColor = currentColor === 'blue' ? 'green' : 'blue';
  const targetPort = targetColor === 'blue' ? BLUE_PORT : GREEN_PORT;

  log(`Deploying to ${targetColor} (current active: ${currentColor})...`);
  const deployResult = await run(`${DOCKSWAP_BIN} deploy ${TEST_APP} ${TEST_IMAGE}`);

  if (!deployResult.success) {
    throw new Error(`Deployment to ${targetColor} failed`);
  }

  logSuccess(`Deployment to ${targetColor} completed`);

  // Verify both containers are running
  log("Verifying both containers are running...");
  const containerCheck = await run(
    `docker ps --filter "label=dockswap.managed=true" --format "{{.Names}}\t{{.Status}}"`,
    { silent: true }
  );

  const runningContainers = containerCheck.stdout.trim().split('\n').filter(line => line.trim());
  if (runningContainers.length !== 2) {
    throw new Error(`Expected 2 containers, found ${runningContainers.length}`);
  }

  logSuccess(`Both containers running: ${runningContainers.map(c => c.split('\t')[0]).join(', ')}`);

  // Test both endpoints
  log("Testing both HTTP endpoints...");
  const blueResult = await checkEndpoint(`http://localhost:${BLUE_PORT}`);
  const greenResult = await checkEndpoint(`http://localhost:${GREEN_PORT}`);

  if (!blueResult.success || !greenResult.success) {
    throw new Error("One or both HTTP endpoints failed");
  }

  logSuccess(`Both endpoints responding - Blue: ${blueResult.status}, Green: ${greenResult.status}`);

  return { targetColor, targetPort };
}

// Test traffic switching
async function testTrafficSwitching(targetColor) {
  logStep("Testing traffic switching");

  log(`Switching traffic to ${targetColor}...`);
  const switchResult = await run(`${DOCKSWAP_BIN} switch ${TEST_APP} ${targetColor}`);

  if (!switchResult.success) {
    throw new Error(`Traffic switching to ${targetColor} failed`);
  }

  logSuccess(`Traffic switched to ${targetColor}`);

  // Verify status shows new active color
  log("Verifying status update...");
  const statusResult = await run(`${DOCKSWAP_BIN} status ${TEST_APP}`, { silent: true });

  if (!statusResult.success) {
    throw new Error("Status check after switch failed");
  }

  const colorLine = statusResult.stdout.split('\n').find(line => line.includes('Color:'));
  const newActiveColor = colorLine.split('Color:')[1].trim();

  if (newActiveColor !== targetColor) {
    throw new Error(`Expected active color ${targetColor}, got ${newActiveColor}`);
  }

  logSuccess(`Status correctly shows active color: ${newActiveColor}`);

  return { newActiveColor };
}

// Cleanup function
async function cleanup() {
  logStep("Cleaning up test environment");
  await teardownE2EEnvironment();
}

// Main test execution
async function main() {
  const startTime = Date.now();

  // Reset step counter for this test
  resetStepCounter();

  log(`${colors.bold}${colors.blue}🚀 Starting Dockswap E2E Test Suite${colors.reset}`);
  log(`${colors.yellow}Testing app: ${TEST_APP} with image: ${TEST_IMAGE}${colors.reset}\n`);

  try {
    // Setup
    await setup();

    // Test basic deployment
    const { deployedColor } = await testBasicDeployment();

    // Test status
    const { activeColor } = await testStatus();

    // Verify consistency
    if (deployedColor !== activeColor) {
      logWarning(`Deployed color (${deployedColor}) differs from status color (${activeColor})`);
    }

    // Test blue-green deployment
    const { targetColor } = await testBlueGreenDeployment(activeColor);

    // Test traffic switching
    const { newActiveColor } = await testTrafficSwitching(targetColor);

    // Final verification
    log("\nPerforming final verification...");
    const finalPort = newActiveColor === 'blue' ? BLUE_PORT : GREEN_PORT;
    const finalEndpointCheck = await checkEndpoint(`http://localhost:${finalPort}`);

    if (!finalEndpointCheck.success) {
      throw new Error("Final endpoint verification failed");
    }

    logSuccess("Final endpoint verification passed");

    // Success!
    const duration = ((Date.now() - startTime) / 1000).toFixed(2);
    log(`\n${colors.bold}${colors.green}🎉 All tests passed! (${duration}s)${colors.reset}`);
    log(`${colors.green}✓ Basic deployment works${colors.reset}`);
    log(`${colors.green}✓ Status reporting works${colors.reset}`);
    log(`${colors.green}✓ Blue-green deployment works${colors.reset}`);
    log(`${colors.green}✓ Traffic switching works${colors.reset}`);
    log(`${colors.green}✓ HTTP endpoints respond correctly${colors.reset}`);

  } catch (error) {
    const duration = ((Date.now() - startTime) / 1000).toFixed(2);
    log(`\n${colors.bold}${colors.red}💥 Test failed after ${duration}s${colors.reset}`);
    logError(error.message);
    process.exit(1);

  } finally {
    // Always cleanup
    await cleanup();
  }
}

// Run the tests
if (import.meta.main) {
  await main();
}