#!/usr/bin/env bun

import { describe, test, beforeEach, afterEach, expect } from "bun:test";

// Import utilities
import {
    log, logStep, logSuccess, logWarning, logInfo, run, setupE2EEnvironment, teardownE2EEnvironment,
    colors, resetStepCounter, createTestAppConfig,
    createTestTempDir, cleanupTestTempDir
} from './utils.js';

import { DockSwap } from './dockswap.page.js';
import { DockerTest } from './docker.page.js';
import { Caddy } from './caddy.page.js';

// Test configuration
const TEST_APP = "caddy-integration-test";
const TEST_IMAGE = "nginx:alpine";
const BLUE_PORT = 8081;
const GREEN_PORT = 8082;
const PROXY_PORT = 8080;
const CADDY_CONFIG_PATH = (baseDir) => `${baseDir}/caddy/caddy.json`;

describe("Dockswap E2E - Complete Caddy Integration", () => {
    let testStartTime;
    let caddyStarted = false;
    let baseDir;
    let env;

    /**
     * Instance of DockSwap class for controlling the dockswap CLI in tests.
     * Provides methods for deployment, switching, status checks, and Caddy integration.
     * @type {DockSwap}
     */
    let dockSwap;

    /**
     * Instance of DockerTest class for Docker container assertions and health checks.
     * @type {DockerTest}
     */
    let docker;

    /**
     * Instance of Caddy page object for controlling and asserting Caddy server in tests.
     * @type {Caddy}
     */
    let caddy;

    beforeEach(async () => {
        testStartTime = Date.now();
        resetStepCounter();
        baseDir = createTestTempDir("caddy-integration");
        await run(`mkdir -p ${baseDir}`); // Ensure baseDir exists
        await run(`mkdir -p ${baseDir}/apps`);
        await run(`mkdir -p ${baseDir}/caddy`);
        env = { ...process.env, DOCKSWAP_CONFIG_DIR: baseDir };
        await createTestAppConfig(baseDir, TEST_APP, BLUE_PORT, GREEN_PORT, PROXY_PORT);

        dockSwap = new DockSwap({ binPath: "./dockswap", env, baseDir });
        docker = new DockerTest();
        caddy = new Caddy({ baseDir, env });

        log(`${colors.bold}${colors.blue}🚀 Setting up complete Caddy integration test environment${colors.reset}`);
        await setupE2EEnvironment({ pullImages: [TEST_IMAGE], cleanup: true, baseDir });
        try {
            await caddy.startComplete(CADDY_CONFIG_PATH(baseDir));
            caddyStarted = true;
            logSuccess("Caddy started for testing");
        } catch (error) {
            logWarning(`Failed to start Caddy: ${error.message}`);
            logInfo("Test will continue without Caddy integration");
            caddyStarted = false;
        }
        logInfo(`E2E baseDir: ${baseDir}`);
        await run(`ls -l ${baseDir}`);
    }, 30000);

    afterEach(async () => {
        const duration = ((Date.now() - testStartTime) / 1000).toFixed(2);
        log(`${colors.dim}Test completed in ${duration}s${colors.reset}`);

        // Stop Caddy if we started it
        if (caddyStarted && caddy) {
            await caddy.stop();
        }

        await teardownE2EEnvironment(baseDir);
        cleanupTestTempDir(baseDir);
    }, 30000); // 30 second timeout for teardown

    test("should perform complete blue-green deployment with Caddy integration", async () => {
        logStep("Testing complete blue-green deployment with Caddy integration");

        // Skip test if Caddy is not available
        if (!caddyStarted) {
            logWarning("Caddy not available - skipping integration test");
            return;
        }

        // 1. Initial deployment to green
        logStep("Deploying to green environment");
        const deployResult = await dockSwap.deploy(TEST_APP, TEST_IMAGE);
        expect(deployResult.success).toBe(true);
        logSuccess("Initial deployment completed");

        // 1b. Force Caddy config reload after first deploy
        await caddy.reload(CADDY_CONFIG_PATH(baseDir));

        // 2. Verify green container is running and healthy
        logStep("Verifying green container health");
        await docker.validateContainerHealth(baseDir, TEST_APP, "green", GREEN_PORT);

        // 3. Verify database state shows green as active
        logStep("Verifying database state");
        const greenStatus = await dockSwap.validateDatabaseState(TEST_APP, "green", TEST_IMAGE);

        // 4. Verify Caddy integration (should route to green)
        logStep("Verifying Caddy integration with green deployment");
        await caddy.validateIntegration(TEST_APP, GREEN_PORT, PROXY_PORT);

        // 5. Deploy to blue environment
        logStep("Deploying to blue environment");
        const secondDeployResult = await dockSwap.deploy(TEST_APP, TEST_IMAGE);
        expect(secondDeployResult.success).toBe(true);
        logSuccess("Blue deployment completed");

        // 6. Verify both containers are running
        logStep("Verifying both containers are running");
        await docker.validateContainerHealth(baseDir, TEST_APP, "green", GREEN_PORT);
        await docker.validateContainerHealth(baseDir, TEST_APP, "blue", BLUE_PORT);

        // 7. Verify database state still shows green as active (no switch yet)
        logStep("Verifying database state after blue deployment");
        const afterBlueStatus = await dockSwap.validateDatabaseState(TEST_APP, "green", TEST_IMAGE);

        // 8. Verify Caddy still routes to green (no switch yet)
        logStep("Verifying Caddy still routes to green");
        await caddy.validateIntegration(TEST_APP, GREEN_PORT, PROXY_PORT);

        // 9. Switch traffic to blue
        logStep("Switching traffic to blue");
        const switchResult = await dockSwap.switch(TEST_APP, "blue");
        expect(switchResult.success).toBe(true);
        logSuccess("Traffic switched to blue");

        // 10. Verify database state shows blue as active
        logStep("Verifying database state after switch");
        const blueStatus = await dockSwap.validateDatabaseState(TEST_APP, "blue", TEST_IMAGE);

        // 11. Verify Caddy now routes to blue
        logStep("Verifying Caddy routes to blue");
        await caddy.validateIntegration(TEST_APP, BLUE_PORT, PROXY_PORT);

        // 12. Final verification - both containers should still be running
        logStep("Final verification - both containers running");
        const containers = await docker.getContainers();
        const runningContainers = containers.filter(line => line);
        expect(runningContainers.length).toBe(2);
        logSuccess(`Both containers running: ${runningContainers.map(c => c.name).join(', ')}`);

        logSuccess("Complete blue-green deployment with Caddy integration test passed");
    }, 60000); // 60 second timeout for this test

    test("should handle Caddy configuration updates during deployment", async () => {
        logStep("Testing Caddy configuration updates during deployment");

        if (!caddyStarted) {
            logWarning("Caddy not available - skipping configuration test");
            return;
        }

        // 1. Deploy initial version (green)
        logStep("Deploying initial version");
        await dockSwap.deploy(TEST_APP, TEST_IMAGE);

        // 2. Verify Caddy config was generated
        logStep("Verifying Caddy configuration generation");
        const configCheck = await caddy.configExists(CADDY_CONFIG_PATH(baseDir));
        expect(configCheck).toBe(true);
        logSuccess("Caddy configuration file exists");

        // 3. Check Caddy template was created
        logStep("Verifying Caddy template creation");
        const templateCheck = await run(`test -f ${baseDir}/caddy/template.json`, { allowFailure: true, silent: true, env });
        if (!templateCheck.success) {
            // Create template if it doesn't exist
            logStep("Creating Caddy template");
            await dockSwap.caddyConfigCreate();
            logSuccess("Caddy template created");
        } else {
            logSuccess("Caddy template already exists");
        }

        // 4. Test Caddy reload functionality
        logStep("Testing Caddy reload functionality");
        const reloadResult = await caddy.reload(CADDY_CONFIG_PATH(baseDir));
        // caddy.reload throws if fails, so no need to check result
        logSuccess("Caddy reload successful");

        // 5. Verify proxy is working (should route to green)
        logStep("Verifying proxy functionality");
        await caddy.validateIntegration(TEST_APP, GREEN_PORT, PROXY_PORT);

        logSuccess("Caddy configuration updates test passed");
    }, 60000); // 60 second timeout for this test

    test("should validate complete workflow state consistency", async () => {
        logStep("Testing complete workflow state consistency");

        if (!caddyStarted) {
            logWarning("Caddy not available - skipping state consistency test");
            return;
        }

        // 1. Initial deployment (green)
        logStep("Performing initial deployment");
        await dockSwap.deploy(TEST_APP, TEST_IMAGE);
        // 1b. Force Caddy config reload after first deploy
        await caddy.reload(CADDY_CONFIG_PATH(baseDir));

        // 2. Verify all state components are consistent
        logStep("Verifying state consistency after deployment");

        // Database state
        const dbState = await dockSwap.validateDatabaseState(TEST_APP, "green", TEST_IMAGE);

        // Container state
        await docker.validateContainerHealth(baseDir, TEST_APP, "green", GREEN_PORT);

        // Caddy state
        await caddy.validateIntegration(TEST_APP, GREEN_PORT, PROXY_PORT);

        // CLI status
        const cliStatus = await dockSwap.status(TEST_APP);
        expect(cliStatus.activeColor).toBe("green");
        expect(cliStatus.image).toBe(TEST_IMAGE);

        // 3. Deploy to blue
        logStep("Deploying to blue");
        await dockSwap.deploy(TEST_APP, TEST_IMAGE);

        // 4. Verify state consistency after second deployment
        logStep("Verifying state consistency after second deployment");

        // Should still be on green (no switch yet)
        const afterSecondDeploy = await dockSwap.validateDatabaseState(TEST_APP, "green", TEST_IMAGE);
        await docker.validateContainerHealth(baseDir, TEST_APP, "green", GREEN_PORT);
        await docker.validateContainerHealth(baseDir, TEST_APP, "blue", BLUE_PORT);
        await caddy.validateIntegration(TEST_APP, GREEN_PORT, PROXY_PORT);

        // 5. Switch traffic
        logStep("Switching traffic");
        await dockSwap.switch(TEST_APP, "blue");

        // 6. Verify final state consistency
        logStep("Verifying final state consistency");

        const finalDbState = await dockSwap.validateDatabaseState(TEST_APP, "blue", TEST_IMAGE);
        await docker.validateContainerHealth(baseDir, TEST_APP, "green", GREEN_PORT);
        await docker.validateContainerHealth(baseDir, TEST_APP, "blue", BLUE_PORT);
        await caddy.validateIntegration(TEST_APP, BLUE_PORT, PROXY_PORT);

        const finalCliStatus = await dockSwap.status(TEST_APP);
        expect(finalCliStatus.activeColor).toBe("blue");
        expect(finalCliStatus.image).toBe(TEST_IMAGE);

        logSuccess("Complete workflow state consistency test passed");
    }, 60000); // 60 second timeout for this test

    test("should handle Caddy failures gracefully", async () => {
        logStep("Testing graceful handling of Caddy failures");

        // 1. Stop Caddy to simulate failure
        logStep("Stopping Caddy to simulate failure");
        if (caddy) await caddy.stop();
        caddyStarted = false;

        // 2. Deploy green
        logStep("Deploying without Caddy (green)");
        const deployResult = await dockSwap.deploy(TEST_APP, TEST_IMAGE);
        expect(deployResult.success).toBe(true);
        logSuccess("Deployment succeeded without Caddy");

        // 3. Deploy blue
        logStep("Deploying blue without Caddy");
        const deployBlueResult = await dockSwap.deploy(TEST_APP, TEST_IMAGE);
        expect(deployBlueResult.success).toBe(true);
        logSuccess("Blue deployment succeeded without Caddy");

        // 4. Verify green container is running
        logStep("Verifying green container is running");
        await docker.validateContainerHealth(baseDir, TEST_APP, "green", GREEN_PORT);

        // 5. Verify blue container is running
        logStep("Verifying blue container is running");
        await docker.validateContainerHealth(baseDir, TEST_APP, "blue", BLUE_PORT);

        // 6. Verify database state is correct (should still be green)
        logStep("Verifying database state");
        await dockSwap.validateDatabaseState(TEST_APP, "green", TEST_IMAGE);

        // 7. Switch should work (with warning about Caddy)
        logStep("Testing switch without Caddy");
        const switchResult = await dockSwap.switch(TEST_APP, "blue");
        expect(switchResult.success).toBe(true);
        logSuccess("Switch succeeded without Caddy");

        // 8. Verify final state
        logStep("Verifying final state");
        await dockSwap.validateDatabaseState(TEST_APP, "blue", TEST_IMAGE);

        logSuccess("Graceful Caddy failure handling test passed");
    }, 60000); // 60 second timeout for this test
}); 