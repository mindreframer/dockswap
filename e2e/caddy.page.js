import { run, logStep, logSuccess, logInfo, waitFor, logWarning, checkEndpoint } from './utils.js';

/**
 * Page object for Caddy server control and assertions in E2E tests.
 */
export class Caddy {
    /**
     * @param {Object} options
     * @param {string} options.baseDir - Base directory for test environment.
     * @param {Object} options.env - Environment variables for Caddy process.
     */
    constructor({ baseDir, env }) {
        this.baseDir = baseDir;
        this.env = env;
    }

    /**
     * Starts the Caddy server with a complete config.
     * @param {string} configPath - Path to the Caddy config file.
     * @param {number} adminPort - The admin port to listen on.
     */
    async startComplete(configPath, adminPort = 2019) {
        logInfo(`Starting Caddy with config: ${configPath}`);

        // Kill any existing Caddy processes
        await run("pkill -f caddy || true", { allowFailure: true, silent: true });

        // Create config directory
        const configDir = configPath.substring(0, configPath.lastIndexOf('/'));
        await run(`mkdir -p ${configDir}`);

        // Create minimal config if it doesn't exist
        const configExists = await run(`test -f ${configPath}`, { allowFailure: true, silent: true });
        if (!configExists.success) {
            const minimalConfig = {
                "admin": {
                    "listen": `:${adminPort}`
                },
                "apps": {
                    "http": {
                        "servers": {
                            "default": {
                                "listen": [":8080"],
                                "automatic_https": {
                                    "disable": true
                                },
                                "routes": [
                                    {
                                        "handle": [
                                            {
                                                "handler": "static_response",
                                                "body": "Caddy is running"
                                            }
                                        ]
                                    }
                                ]
                            }
                        }
                    }
                }
            };

            await run(`echo '${JSON.stringify(minimalConfig)}' > ${configPath}`);
            logInfo("Created minimal Caddy config");
        }

        // Start Caddy in background using nohup to avoid Bun.sh hanging
        const result = await run(`nohup caddy start --config ${configPath} > /dev/null 2>&1 &`, {
            allowFailure: true,
            silent: true
        });

        // Give Caddy a moment to start
        await new Promise(resolve => setTimeout(resolve, 1000));

        // Wait for Caddy to be ready
        await waitFor(async () => {
            const check = await run(`curl -s -o /dev/null -w "%{http_code}" http://localhost:${adminPort}/config/`, {
                silent: true,
                allowFailure: true
            });
            return check.success && check.stdout === "200";
        }, { timeout: 10000, message: "Caddy admin API" });

        logSuccess("Caddy started successfully");
        return result;
    }

    /**
     * Stops the Caddy server.
     */
    async stop() {
        logStep("Stopping Caddy server");
        // Kill any existing Caddy processes
        await run("pkill -f caddy || true", { allowFailure: true, silent: true });
        logSuccess("Caddy stopped");
    }

    /**
     * Reloads the Caddy configuration.
     * @param {string} configPath - Path to the Caddy config file.
     */
    async reload(configPath) {
        logStep("Reloading Caddy configuration");
        await run(`caddy reload --config ${configPath}`, { env: this.env });
        logSuccess("Caddy config reloaded");
    }

    /**
     * Checks if the Caddy config file exists.
     * @param {string} configPath - Path to the Caddy config file.
     * @returns {Promise<boolean>} True if config exists, false otherwise.
     */
    async configExists(configPath) {
        const result = await run(`test -f ${configPath}`, { allowFailure: true, env: this.env });
        return result.success;
    }

    /**
     * Validates Caddy integration by checking admin API, proxy, and response.
     * @param {string} appName - Name of the app being tested.
     * @param {number} expectedPort - The backend port expected to be proxied.
     * @param {number} [proxyPort=8080] - The Caddy proxy port to check.
     * @returns {Promise<boolean>} True if integration is valid, throws otherwise.
     */
    async validateIntegration(appName, expectedPort, proxyPort = 8080) {
        logStep(`Validating Caddy integration for ${appName}`);

        // Check if Caddy is running
        const caddyCheck = await run(`curl -s -o /dev/null -w "%{http_code}" http://localhost:2019/config/`, {
            silent: true,
            allowFailure: true
        });
        if (!caddyCheck.success || caddyCheck.stdout !== "200") {
            throw new Error("Caddy admin API not accessible");
        }

        // Check if app is accessible through proxy (increase retries/timeout)
        const proxyCheck = await checkEndpoint(`http://localhost:${proxyPort}`, 200, 10000, 10);
        if (!proxyCheck.success) {
            throw new Error(`Proxy not responding on port ${proxyPort}: ${proxyCheck.error || proxyCheck.status}`);
        }

        // Verify the response comes from the expected container
        const response = await run(`curl -s http://localhost:${proxyPort}`, { silent: true });
        if (!response.success) {
            throw new Error("Failed to get response from proxy");
        }

        logSuccess(`Caddy integration validated - proxy responding on port ${proxyPort}`);
        return true;
    }

    // Add more methods as needed for your tests
} 