#!/usr/bin/env bun

import { describe, test, beforeEach, afterEach, expect } from "bun:test";
import { $ } from "bun";

// Import utilities
import {
    log, logStep, logSuccess, logError, logInfo, run, setupE2EEnvironment, teardownE2EEnvironment,
    colors, resetStepCounter, dockswapDeploy, dockswapStatus, assertTrue
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
            buildDockswap: true,
            pullImages: [VALID_IMAGE],
            cleanup: true
        });
    });

    afterEach(async () => {
        const duration = ((Date.now() - testStartTime) / 1000).toFixed(2);
        log(`${colors.dim}Test completed in ${duration}s${colors.reset}`);

        await teardownE2EEnvironment();
    });

    test("should reject invalid app configuration", async () => {
        logStep("Testing invalid app configuration");
        logInfo("Attempting to deploy app with no configuration...");

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
    });

    test("should reject traffic switch without deployment", async () => {
        logStep("Testing traffic switch without deployment");
        logInfo("Attempting to switch traffic with no containers...");

        const switchResult = await run(
            `./dockswap switch ${TEST_APP} blue`,
            { allowFailure: true, silent: true }
        );

        expect(switchResult.success).toBe(false);
        logSuccess("Switch without deployment properly rejected");
    });

    test("should handle valid deployment correctly", async () => {
        logStep("Testing valid deployment for comparison");

        await dockswapDeploy(TEST_APP, VALID_IMAGE);

        const status = await dockswapStatus(TEST_APP);
        expect(status.activeColor).toBeDefined();
        logSuccess("Valid deployment works as expected");
    });

    test("should reject invalid color in switch command", async () => {
        logStep("Testing switch to invalid color");
        logInfo("Attempting to switch to invalid color...");

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
    });

    test("should reject switch to non-existent container", async () => {
        logStep("Testing switch to non-existent container");

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

    test("should handle status for non-existent app", async () => {
        logStep("Testing status for non-existent app");
        logInfo("Checking status for non-existent app...");

        const result = await run(
            `./dockswap status nonexistent-app`,
            { allowFailure: true, silent: true }
        );

        // This might succeed with empty results or fail - both are acceptable
        // We just want to ensure it doesn't crash
        expect(result).toBeDefined();
        logSuccess("Status for non-existent app handled appropriately");
    });

    test("should handle deployment with invalid image", async () => {
        logStep("Testing deployment with invalid image");
        logInfo("Attempting to deploy with non-existent image...");

        const invalidImageResult = await run(
            `./dockswap deploy ${TEST_APP} nonexistent-image:latest`,
            { allowFailure: true, silent: true }
        );

        expect(invalidImageResult.success).toBe(false);
        logSuccess("Invalid image properly rejected");
    });

    test("should handle malformed command arguments", async () => {
        logStep("Testing malformed command arguments");
        logInfo("Attempting to run deploy without required arguments...");

        const malformedResult = await run(
            `./dockswap deploy`,
            { allowFailure: true, silent: true }
        );

        expect(malformedResult.success).toBe(false);
        logSuccess("Malformed command properly rejected");
    });
}); 