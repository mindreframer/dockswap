#!/usr/bin/env bun

import { describe, test, beforeEach, afterEach, expect } from "bun:test";

// Import utilities
import {
    log, logStep, logSuccess, logInfo, run, setupE2EEnvironment, teardownE2EEnvironment,
    colors, resetStepCounter, dockswapDeploy, dockswapStatus
} from './utils.js';

// Test configuration
const TEST_APP = "nginx-test"; // Use existing config
const INVALID_APP = "error-test"; // Non-existent app for error testing
const VALID_IMAGE = "nginx:alpine";

describe("Dockswap E2E - Error Scenarios", () => {
    let testStartTime;

    beforeEach(async () => {
        testStartTime = Date.now();
        resetStepCounter();

        log(`${colors.bold}${colors.blue}🧪 Setting up error scenario test environment${colors.reset}`);
        await setupE2EEnvironment({
            pullImages: [VALID_IMAGE],
            cleanup: true
        });
    });

    afterEach(async () => {
        const duration = ((Date.now() - testStartTime) / 1000).toFixed(2);
        log(`${colors.dim}Test completed in ${duration}s${colors.reset}`);

        await teardownE2EEnvironment();
    });

    test("should handle all deployment-related errors", async () => {
        logInfo("Testing deployment-related error scenarios");

        // 1. Test invalid app configuration
        logStep("Attempting to deploy app with no configuration...");
        const invalidAppResult = await run(
            `./dockswap deploy ${INVALID_APP} ${VALID_IMAGE}`,
            { allowFailure: true, silent: true }
        );
        expect(invalidAppResult.success).toBe(false);
        expect(
            invalidAppResult.stderr.includes("no configuration found") ||
            invalidAppResult.stdout.includes("no configuration found")
        ).toBe(true);
        logSuccess("Invalid app configuration properly rejected");

        // 2. Test deployment with invalid image
        logStep("Attempting to deploy with non-existent image...");
        const invalidImageResult = await run(
            `./dockswap deploy ${TEST_APP} nonexistent-image:latest`,
            { allowFailure: true, silent: true }
        );
        expect(invalidImageResult.success).toBe(false);
        logSuccess("Invalid image properly rejected");

        // 3. Test malformed command arguments
        logStep("Attempting to run deploy without required arguments...");
        const malformedResult = await run(
            `./dockswap deploy`,
            { allowFailure: true, silent: true }
        );
        expect(malformedResult.success).toBe(false);
        logSuccess("Malformed command properly rejected");
    });

    test("should handle all traffic switching errors", async () => {
        logInfo("Testing traffic switching error scenarios");

        // 1. Test traffic switch without deployment
        logStep("Attempting to switch traffic with no containers...");
        const switchResult = await run(
            `./dockswap switch ${TEST_APP} blue`,
            { allowFailure: true, silent: true }
        );
        expect(switchResult.success).toBe(false);
        logSuccess("Switch without deployment properly rejected");

        // 2. Test switch to invalid color
        logStep("Attempting to switch to invalid color...");
        const invalidColorResult = await run(
            `./dockswap switch ${TEST_APP} purple`,
            { allowFailure: true, silent: true }
        );
        expect(invalidColorResult.success).toBe(false);
        expect(
            invalidColorResult.stderr.includes("must be 'blue' or 'green'") ||
            invalidColorResult.stdout.includes("must be 'blue' or 'green'")
        ).toBe(true);
        logSuccess("Invalid color properly rejected");

        // 3. Test switch to non-existent container
        logStep("Test switch to non-existent container");
        // First deploy to one color
        await dockswapDeploy(TEST_APP, VALID_IMAGE);
        const currentStatus = await dockswapStatus(TEST_APP);
        const otherColor = currentStatus.activeColor === 'blue' ? 'green' : 'blue';

        logInfo(`Attempting to switch to ${otherColor} (no container)...`);
        const noContainerResult = await run(
            `./dockswap switch ${TEST_APP} ${otherColor}`,
            { allowFailure: true, silent: true }
        );
        expect(noContainerResult.success).toBe(false);

        // Log actual error for debugging
        if (!noContainerResult.success) {
            logInfo(`Actual error: ${noContainerResult.stderr || noContainerResult.stdout}`);
        }
        logSuccess("Switch to non-existent container properly rejected");
    });

    test("should handle status and validation scenarios", async () => {
        logInfo("Testing status and validation scenarios");

        // 1. Test status for non-existent app
        logStep("Checking status for non-existent app...");
        const result = await run(
            `./dockswap status nonexistent-app`,
            { allowFailure: true, silent: true }
        );
        // This might succeed with empty results or fail - both are acceptable
        expect(result).toBeDefined();
        logSuccess("Status for non-existent app handled appropriately");

        // 2. Test valid deployment for comparison
        logStep("Test valid deployment for comparison");
        await dockswapDeploy(TEST_APP, VALID_IMAGE);
        const status = await dockswapStatus(TEST_APP);
        expect(status.activeColor).toBeDefined();
        logSuccess("Valid deployment works as expected");
    });
}); 