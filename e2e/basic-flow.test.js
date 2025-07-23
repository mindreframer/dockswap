#!/usr/bin/env bun

import { describe, test, beforeEach, afterEach, expect } from "bun:test";

// Import utilities
import {
    log, logStep, logSuccess, logError, run, setupE2EEnvironment, teardownE2EEnvironment,
    colors, resetStepCounter, checkEndpoint, assertTrue, assertEqual
} from './utils.js';

// Test configuration
const TEST_APP = "nginx-test";
const TEST_IMAGE = "nginx:alpine";
const BLUE_PORT = 8080;
const GREEN_PORT = 8081;
const DOCKSWAP_BIN = "./dockswap";

describe("Dockswap E2E - Basic Flow", () => {
    let testStartTime;

    beforeEach(async () => {
        testStartTime = Date.now();
        resetStepCounter();

        log(`${colors.bold}${colors.blue}🚀 Setting up test environment${colors.reset}`);
        await setupE2EEnvironment({
            pullImages: [TEST_IMAGE],
            cleanup: true
        });
    });

    afterEach(async () => {
        const duration = ((Date.now() - testStartTime) / 1000).toFixed(2);
        log(`${colors.dim}Test completed in ${duration}s${colors.reset}`);

        await teardownE2EEnvironment();
    });

    test("should perform complete blue-green deployment workflow", async () => {
        logStep("Testing complete blue-green deployment workflow");

        // 1. Initial deployment
        log(`Deploying ${TEST_APP} with ${TEST_IMAGE}...`);
        const deployResult = await run(`${DOCKSWAP_BIN} deploy ${TEST_APP} ${TEST_IMAGE}`);
        expect(deployResult.success).toBe(true);
        logSuccess("Initial deployment completed");

        // 2. Verify container is running
        logStep("Verifying container is running...");
        const containerCheck = await run(
            `docker ps --filter "label=dockswap.managed=true" --format "{{.Names}}\t{{.Status}}"`,
            { silent: true }
        );
        const runningContainers = containerCheck.stdout.trim().split('\n').filter(line => line.trim());
        expect(runningContainers.length).toBeGreaterThan(0);

        const containerName = runningContainers[0].split('\t')[0];
        const deployedColor = containerName.includes('blue') ? 'blue' : 'green';
        const port = deployedColor === 'blue' ? BLUE_PORT : GREEN_PORT;
        logSuccess(`Container running: ${runningContainers[0]}`);

        // 3. Test HTTP endpoint
        logStep(`Testing HTTP endpoint on port ${port}...`);
        const endpointResult = await checkEndpoint(`http://localhost:${port}`);
        expect(endpointResult.success).toBe(true);
        logSuccess(`HTTP endpoint responding correctly (${endpointResult.status})`);

        // 4. Test status reporting
        logStep("Checking deployment status...");
        const statusResult = await run(`${DOCKSWAP_BIN} status ${TEST_APP}`);
        expect(statusResult.success).toBe(true);

        const statusLines = statusResult.stdout.split('\n');
        const colorLine = statusLines.find(line => line.includes('Color:'));
        const statusLine = statusLines.find(line => line.includes('Status:'));
        expect(colorLine).toBeDefined();
        expect(statusLine).toBeDefined();

        const activeColor = colorLine.split('Color:')[1].trim();
        const deploymentStatus = statusLine.split('Status:')[1].trim();
        expect(activeColor).toMatch(/^(blue|green)$/);
        expect(deploymentStatus).toBeDefined();
        logSuccess(`Status check passed - Active: ${activeColor}, Status: ${deploymentStatus}`);

        // 5. Deploy to second color (blue-green deployment)
        const targetColor = activeColor === 'blue' ? 'green' : 'blue';
        const targetPort = targetColor === 'blue' ? BLUE_PORT : GREEN_PORT;

        logStep(`Deploying to ${targetColor} (current active: ${activeColor})...`);
        const secondDeployResult = await run(`${DOCKSWAP_BIN} deploy ${TEST_APP} ${TEST_IMAGE}`);
        expect(secondDeployResult.success).toBe(true);
        logSuccess(`Deployment to ${targetColor} completed`);

        // 6. Verify both containers are running
        logStep("Verifying both containers are running...");
        const secondContainerCheck = await run(
            `docker ps --filter "label=dockswap.managed=true" --format "{{.Names}}\t{{.Status}}"`,
            { silent: true }
        );
        const secondRunningContainers = secondContainerCheck.stdout.trim().split('\n').filter(line => line.trim());
        expect(secondRunningContainers.length).toBe(2);
        logSuccess(`Both containers running: ${secondRunningContainers.map(c => c.split('\t')[0]).join(', ')}`);

        // 7. Test both endpoints
        logStep("Testing both HTTP endpoints...");
        const blueResult = await checkEndpoint(`http://localhost:${BLUE_PORT}`);
        const greenResult = await checkEndpoint(`http://localhost:${GREEN_PORT}`);
        expect(blueResult.success).toBe(true);
        expect(greenResult.success).toBe(true);
        logSuccess(`Both endpoints responding - Blue: ${blueResult.status}, Green: ${greenResult.status}`);

        // 8. Test traffic switching
        logStep(`Switching traffic to ${targetColor}...`);
        const switchResult = await run(`${DOCKSWAP_BIN} switch ${TEST_APP} ${targetColor}`);
        expect(switchResult.success).toBe(true);
        logSuccess(`Traffic switched to ${targetColor}`);

        // 9. Verify status shows new active color
        logStep("Verifying status update...");
        const newStatusResult = await run(`${DOCKSWAP_BIN} status ${TEST_APP}`, { silent: true });
        expect(newStatusResult.success).toBe(true);

        const newColorLine = newStatusResult.stdout.split('\n').find(line => line.includes('Color:'));
        const newActiveColor = newColorLine.split('Color:')[1].trim();
        expect(newActiveColor).toBe(targetColor);
        logSuccess(`Status correctly shows active color: ${newActiveColor}`);

        // 10. Final verification
        logStep("Performing final verification...");
        const finalPort = newActiveColor === 'blue' ? BLUE_PORT : GREEN_PORT;
        const finalEndpointCheck = await checkEndpoint(`http://localhost:${finalPort}`);
        expect(finalEndpointCheck.success).toBe(true);
        logSuccess("Final endpoint verification passed");
    });
}); 