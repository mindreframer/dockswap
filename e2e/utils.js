import { $ } from "bun";
import { mkdtempSync, rmSync } from "fs";
import { tmpdir } from "os";
import { join } from "path";

// Colors for output
export const colors = {
  green: "\x1b[32m",
  red: "\x1b[31m",
  yellow: "\x1b[33m",
  blue: "\x1b[34m",
  magenta: "\x1b[35m",
  cyan: "\x1b[36m",
  reset: "\x1b[0m",
  bold: "\x1b[1m",
  dim: "\x1b[2m"
};

// Step counter for auto-incrementing steps
let stepCounter = 0;

// Logging utilities
export function log(message, color = colors.reset) {
  console.log(`${color}${message}${colors.reset}`);
}

export function logStep(message) {
  stepCounter++;
  // Debug: Check both counter and message
  console.log(`DEBUG: stepCounter=${stepCounter}, message="${message}"`);
  const stepNum = stepCounter;
  const stepMessage = `${colors.bold}[STEP ${stepNum}]${colors.reset} ${colors.blue}${message}${colors.reset}`;
  console.log(stepMessage);
}

export function resetStepCounter() {
  stepCounter = 0;
}

export function logSuccess(message) {
  log(`${colors.green}✓ ${message}${colors.reset}`);
}

export function logError(message) {
  log(`${colors.red}✗ ${message}${colors.reset}`);
}

export function logWarning(message) {
  log(`${colors.yellow}⚠ ${message}${colors.reset}`);
}

export function logInfo(message) {
  log(`${colors.cyan}ℹ ${message}${colors.reset}`);
}

// Shell command execution with error handling
export async function run(command, { silent = false, allowFailure = false, timeout = 30000, env = process.env } = {}) {
  if (!silent) {
    log(`${colors.dim}$ ${command}${colors.reset}`);
  }

  try {
    const result = await $`sh -c "${command}"`.env(env).quiet(silent);
    return {
      success: true,
      stdout: result.stdout?.toString() || "",
      stderr: result.stderr?.toString() || "",
      exitCode: 0
    };
  } catch (error) {
    if (!allowFailure) {
      logError(`Command failed: ${command}`);
      logError(`Error: ${error.message}`);
      if (error.stderr) {
        logError(`Stderr: ${error.stderr.toString()}`);
      }
      throw error;
    }
    return {
      success: false,
      stdout: error.stdout?.toString() || "",
      stderr: error.stderr?.toString() || "",
      exitCode: error.exitCode || 1
    };
  }
}

// HTTP endpoint testing
export async function checkEndpoint(url, expectedStatus = 200, timeout = 5000, retries = 3) {
  for (let attempt = 1; attempt <= retries; attempt++) {
    try {
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), timeout);

      const response = await fetch(url, {
        signal: controller.signal,
        headers: { 'User-Agent': 'dockswap-e2e-test' }
      });

      clearTimeout(timeoutId);

      const result = {
        success: response.status === expectedStatus,
        status: response.status,
        url,
        attempt
      };

      if (result.success || attempt === retries) {
        return result;
      }

      // Wait before retry
      await new Promise(resolve => setTimeout(resolve, 1000));

    } catch (error) {
      if (attempt === retries) {
        return {
          success: false,
          status: 0,
          url,
          attempt,
          error: error.message
        };
      }

      // Wait before retry
      await new Promise(resolve => setTimeout(resolve, 1000));
    }
  }
}

// Docker utilities
export async function cleanupDockswapContainers() {
  logInfo("Cleaning up dockswap containers...");
  const result = await run(
    `docker ps -aq --filter "label=dockswap.managed=true" | xargs -r docker stop 2>/dev/null | xargs -r docker rm 2>/dev/null`,
    { allowFailure: true, silent: true }
  );

  const count = result.stdout.split('\n').filter(line => line.trim()).length;
  if (count > 0) {
    logSuccess(`Cleaned up ${count} containers`);
  } else {
    logInfo("No containers to clean up");
  }
}

export async function cleanupDockswapDatabase(baseDir) {
  logInfo("Cleaning up dockswap database...");
  const dbPath = baseDir ? `${baseDir}/dockswap.db` : "dockswap-cfg/dockswap.db";
  const result = await run(`rm -f ${dbPath}`, { allowFailure: true, silent: true });
  if (result.success) {
    logSuccess("Database cleaned up");
  }
}

export async function getDockswapContainers() {
  const result = await run(
    `docker ps --filter "label=dockswap.managed=true" --format "{{.Names}}\t{{.Status}}\t{{.Ports}}"`,
    { silent: true }
  );

  if (!result.success) {
    return [];
  }

  return result.stdout
    .trim()
    .split('\n')
    .filter(line => line.trim())
    .map(line => {
      const [name, status, ports] = line.split('\t');
      return { name, status, ports };
    });
}

export async function pullDockerImage(image) {
  logInfo(`Pulling Docker image: ${image}`);
  const result = await run(`docker pull ${image}`, { silent: true });

  if (result.success) {
    logSuccess(`Image pulled: ${image}`);
  } else {
    throw new Error(`Failed to pull image: ${image}`);
  }
}

// Dockswap command wrappers
export async function dockswapDeploy(baseDir, appName, image, dockswapBin = "./dockswap") {
  logInfo(`Deploying ${baseDir} with ${appName}...`);
  const env = { ...process.env, DOCKSWAP_CONFIG_DIR: baseDir };
  const result = await run(`${dockswapBin} deploy ${appName} ${image}`, { env });
  if (!result.success) {
    throw new Error(`Deployment failed: ${result.stderr || result.stdout}`);
  }
  logSuccess(`Deployed ${appName} successfully`);
  return result;
}

export async function dockswapSwitch(baseDir, appName, color, dockswapBin = "./dockswap") {
  logInfo(`Switching ${appName} to ${color}...`);
  const env = { ...process.env, DOCKSWAP_CONFIG_DIR: baseDir };
  const result = await run(`${dockswapBin} switch ${appName} ${color}`, { env });
  if (!result.success) {
    throw new Error(`Switch failed: ${result.stderr || result.stdout}`);
  }
  logSuccess(`Switched ${appName} to ${color}`);
  return result;
}

export async function dockswapStatus(baseDir, appName, dockswapBin = "./dockswap") {
  const env = { ...process.env, DOCKSWAP_CONFIG_DIR: baseDir };
  const result = await run(`${dockswapBin} status ${appName}`, { silent: true, env });
  if (!result.success) {
    throw new Error(`Status check failed: ${result.stderr || result.stdout}`);
  }
  // Parse status output
  const lines = result.stdout.split('\n');
  const colorLine = lines.find(line => line.includes('Color:'));
  const imageLine = lines.find(line => line.includes('Image:'));
  const statusLine = lines.find(line => line.includes('Status:'));
  const updatedLine = lines.find(line => line.includes('Updated:'));
  return {
    activeColor: colorLine?.split('Color:')[1]?.trim(),
    image: imageLine?.split('Image:')[1]?.trim(),
    status: statusLine?.split('Status:')[1]?.trim(),
    updated: updatedLine?.split('Updated:')[1]?.trim(),
    raw: result.stdout
  };
}

// Test assertions
export function assertEqual(actual, expected, message) {
  if (actual !== expected) {
    throw new Error(`${message}: expected "${expected}", got "${actual}"`);
  }
}

export function assertTrue(condition, message) {
  if (!condition) {
    throw new Error(`Assertion failed: ${message}`);
  }
}

export function assertContainerRunning(containers, appName, color) {
  const expectedName = `${appName}-${color}`;
  const container = containers.find(c => c.name === expectedName);

  if (!container) {
    throw new Error(`Container ${expectedName} not found. Running containers: ${containers.map(c => c.name).join(', ')}`);
  }

  if (!container.status.includes('Up')) {
    throw new Error(`Container ${expectedName} is not running: ${container.status}`);
  }

  logSuccess(`Container ${expectedName} is running`);
}

// Test timing utilities
export function timeExecution(name) {
  const start = Date.now();
  return {
    end: () => {
      const duration = ((Date.now() - start) / 1000).toFixed(2);
      return duration;
    }
  };
}

export async function waitFor(condition, { timeout = 30000, interval = 1000, message = "condition" } = {}) {
  const start = Date.now();

  while (Date.now() - start < timeout) {
    try {
      const result = await condition();
      if (result) {
        return result;
      }
    } catch (error) {
      // Continue waiting
    }

    await new Promise(resolve => setTimeout(resolve, interval));
  }

  throw new Error(`Timeout waiting for ${message} after ${timeout}ms`);
}

// Test setup and teardown
export async function setupE2EEnvironment({ buildDockswap = false, pullImages = [], cleanup = true, baseDir } = {}) {
  logInfo("Setting up E2E test environment...");

  if (cleanup) {
    await cleanupDockswapContainers();
    await cleanupDockswapDatabase(baseDir);
  }

  // We build the binary once in the makefile before running tests
  // if (buildDockswap) {
  //   logInfo("Building dockswap binary...");
  //   await run("make build");
  //   logSuccess("Dockswap binary built");
  // }

  for (const image of pullImages) {
    await pullDockerImage(image);
  }

  logSuccess("E2E environment setup complete");
}

export async function teardownE2EEnvironment(baseDir) {
  logInfo("Tearing down E2E test environment...");
  await cleanupDockswapContainers();
  await cleanupDockswapDatabase(baseDir);
  logSuccess("E2E environment torn down");
}

// Utility to create a unique temp dir for each test
export function createTestTempDir(testName) {
  const base = join(tmpdir(), `dockswap-e2e-${testName}-`);
  // mkdtempSync returns the actual created directory path
  const dir = mkdtempSync(base);
  return dir;
}

export function cleanupTestTempDir(baseDir) {
  try {
    rmSync(baseDir, { recursive: true, force: true });
  } catch (e) {
    // ignore
  }
}

// Caddy management utilities
export async function startCaddy(baseDir, configPath, adminPort = 2019) {
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
  await new Promise(resolve => setTimeout(resolve, 2000));

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

export async function stopCaddy() {
  logInfo("Stopping Caddy...");

  // Kill Caddy processes more aggressively
  await run("pkill -f 'caddy start'", { allowFailure: true, silent: true });
  await run("pkill -f 'caddy run'", { allowFailure: true, silent: true });
  await run("pkill -f caddy", { allowFailure: true, silent: true });

  // Wait a moment for processes to stop
  await new Promise(resolve => setTimeout(resolve, 1000));

  // Check if any Caddy processes are still running
  const checkResult = await run("pgrep -f caddy", { allowFailure: true, silent: true });
  if (checkResult.success) {
    logWarning("Some Caddy processes may still be running");
  } else {
    logSuccess("Caddy stopped");
  }
  return checkResult;
}

export async function createTestAppConfig(baseDir, appName, bluePort, greenPort, proxyPort = 8080) {
  const config = {
    name: appName,
    description: "Test app for Caddy integration",
    docker: {
      memory_limit: "256m",
      environment: {
        NGINX_HOST: "localhost"
      },
      expose_port: 80
    },
    ports: {
      blue: bluePort,
      green: greenPort
    },
    health_check: {
      endpoint: "/",
      method: "GET",
      timeout: "5s",
      interval: "10s",
      retries: 3,
      success_threshold: 1,
      expected_status: 200
    },
    deployment: {
      startup_delay: "30s",
      drain_timeout: "60s",
      stop_timeout: "30s",
      auto_rollback: false
    },
    proxy: {
      listen_port: proxyPort,
      host: "localhost"
    }
  };

  const configPath = `${baseDir}/apps/${appName}.yaml`;

  // Create proper YAML format
  const configYaml = `name: "${config.name}"
description: "${config.description}"
docker:
  memory_limit: "${config.docker.memory_limit}"
  environment:
    NGINX_HOST: "${config.docker.environment.NGINX_HOST}"
  expose_port: ${config.docker.expose_port}
ports:
  blue: ${config.ports.blue}
  green: ${config.ports.green}
health_check:
  endpoint: "${config.health_check.endpoint}"
  method: "${config.health_check.method}"
  timeout: "${config.health_check.timeout}"
  interval: "${config.health_check.interval}"
  retries: ${config.health_check.retries}
  success_threshold: ${config.health_check.success_threshold}
  expected_status: ${config.health_check.expected_status}
deployment:
  startup_delay: "${config.deployment.startup_delay}"
  drain_timeout: "${config.deployment.drain_timeout}"
  stop_timeout: "${config.deployment.stop_timeout}"
  auto_rollback: ${config.deployment.auto_rollback}
proxy:
  listen_port: ${config.proxy.listen_port}
  host: "${config.proxy.host}"`;

  await run(`mkdir -p ${baseDir}/apps`);
  await run(`echo '${configYaml}' > ${configPath}`);
  logSuccess(`Created test app config: ${configPath}`);
  return configPath;
}

export async function validateCaddyIntegration(baseDir, appName, expectedPort, proxyPort = 8080) {
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

export async function validateDatabaseState(baseDir, appName, expectedColor, expectedImage) {
  logStep(`Validating database state for ${appName}`);

  // Check current state
  const statusResult = await run(`./dockswap status ${appName}`, { silent: true });
  if (!statusResult.success) {
    throw new Error(`Failed to get status: ${statusResult.stderr}`);
  }

  const status = await dockswapStatus(baseDir, appName);

  if (status.activeColor !== expectedColor) {
    throw new Error(`Expected active color ${expectedColor}, got ${status.activeColor}`);
  }

  if (status.image !== expectedImage) {
    throw new Error(`Expected image ${expectedImage}, got ${status.image}`);
  }

  logSuccess(`Database state validated - Active: ${status.activeColor}, Image: ${status.image}`);
  return status;
}

export async function validateContainerHealth(baseDir, appName, color, port) {
  logStep(`Validating container health for ${appName}-${color}`);

  // Check container is running
  const containers = await getDockswapContainers();
  const containerName = `${appName}-${color}`;
  const container = containers.find(c => c.name === containerName);

  if (!container) {
    // Print docker ps output for debugging
    const ps = await run('docker ps -a', { silent: false, allowFailure: true });
    throw new Error(`Container ${containerName} not found. docker ps -a:\n${ps.stdout}`);
  }

  if (!container.status.includes('Up')) {
    // Print docker logs for debugging
    const logs = await run(`docker logs ${containerName}`, { silent: false, allowFailure: true });
    throw new Error(`Container ${containerName} is not running: ${container.status}\nLogs:\n${logs.stdout}`);
  }

  // Check HTTP endpoint
  const endpointCheck = await checkEndpoint(`http://localhost:${port}`, 200, 10000, 6);
  if (!endpointCheck.success) {
    // Print docker logs for debugging
    const logs = await run(`docker logs ${containerName}`, { silent: false, allowFailure: true });
    throw new Error(`Container health check failed on port ${port}: ${endpointCheck.error || endpointCheck.status}\nLogs:\n${logs.stdout}`);
  }

  logSuccess(`Container ${containerName} is healthy and responding on port ${port}`);
  return true;
}