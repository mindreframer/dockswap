import { $ } from "bun";

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

// Logging utilities
export function log(message, color = colors.reset) {
  console.log(`${color}${message}${colors.reset}`);
}

export function logStep(step, message) {
  log(`${colors.bold}[STEP ${step}]${colors.reset} ${colors.blue}${message}${colors.reset}`);
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
export async function run(command, { silent = false, allowFailure = false, timeout = 30000 } = {}) {
  if (!silent) {
    log(`${colors.dim}$ ${command}${colors.reset}`);
  }
  
  try {
    const result = await $`sh -c ${command}`.quiet(silent);
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

export async function cleanupDockswapDatabase() {
  logInfo("Cleaning up dockswap database...");
  const result = await run("rm -f dockswap-cfg/dockswap.db", { allowFailure: true, silent: true });
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
export async function dockswapDeploy(appName, image, dockswapBin = "./dockswap") {
  logInfo(`Deploying ${appName} with ${image}...`);
  const result = await run(`${dockswapBin} deploy ${appName} ${image}`);
  
  if (!result.success) {
    throw new Error(`Deployment failed: ${result.stderr || result.stdout}`);
  }
  
  logSuccess(`Deployed ${appName} successfully`);
  return result;
}

export async function dockswapSwitch(appName, color, dockswapBin = "./dockswap") {
  logInfo(`Switching ${appName} to ${color}...`);
  const result = await run(`${dockswapBin} switch ${appName} ${color}`);
  
  if (!result.success) {
    throw new Error(`Switch failed: ${result.stderr || result.stdout}`);
  }
  
  logSuccess(`Switched ${appName} to ${color}`);
  return result;
}

export async function dockswapStatus(appName, dockswapBin = "./dockswap") {
  const result = await run(`${dockswapBin} status ${appName}`, { silent: true });
  
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
export async function setupE2EEnvironment({ buildDockswap = true, pullImages = [], cleanup = true } = {}) {
  logInfo("Setting up E2E test environment...");
  
  if (cleanup) {
    await cleanupDockswapContainers();
    await cleanupDockswapDatabase();
  }
  
  if (buildDockswap) {
    logInfo("Building dockswap binary...");
    await run("make build");
    logSuccess("Dockswap binary built");
  }
  
  for (const image of pullImages) {
    await pullDockerImage(image);
  }
  
  logSuccess("E2E environment setup complete");
}

export async function teardownE2EEnvironment() {
  logInfo("Tearing down E2E test environment...");
  await cleanupDockswapContainers();
  await cleanupDockswapDatabase();
  logSuccess("E2E environment torn down");
}