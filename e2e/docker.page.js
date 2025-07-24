import { run } from './utils.js';

export class DockerTest {
    async cleanupContainers() {
        // Clean up all dockswap-managed containers
        const result = await run(
            `docker ps -aq --filter "label=dockswap.managed=true" | xargs -r docker stop 2>/dev/null | xargs -r docker rm 2>/dev/null`,
            { allowFailure: true, silent: true }
        );
        const count = result.stdout.split('\n').filter(line => line.trim()).length;
        return { cleaned: count };
    }

    async getContainers() {
        const result = await run(
            `docker ps --filter "label=dockswap.managed=true" --format "{{.Names}}\t{{.Status}}\t{{.Ports}}"`,
            { silent: true }
        );
        if (!result.success) return [];
        return result.stdout
            .trim()
            .split('\n')
            .filter(line => line.trim())
            .map(line => {
                const [name, status, ports] = line.split('\t');
                return { name, status, ports };
            });
    }

    async pullImage(image) {
        const result = await run(`docker pull ${image}`, { silent: true });
        if (!result.success) {
            throw new Error(`Failed to pull image: ${image}`);
        }
        return { pulled: image };
    }

    async validateContainerHealth(baseDir, appName, color, port) {
        // Check container is running
        const containers = await this.getContainers();
        const containerName = `${appName}-${color}`;
        const container = containers.find(c => c.name === containerName);
        if (!container) {
            const ps = await run('docker ps -a', { silent: false, allowFailure: true });
            throw new Error(`Container ${containerName} not found. docker ps -a:\n${ps.stdout}`);
        }
        if (!container.status.includes('Up')) {
            const logs = await run(`docker logs ${containerName}`, { silent: false, allowFailure: true });
            throw new Error(`Container ${containerName} is not running: ${container.status}\nLogs:\n${logs.stdout}`);
        }
        // Check HTTP endpoint
        const endpointCheck = await this.checkEndpoint(`http://localhost:${port}`, 200, 10000, 6);
        if (!endpointCheck.success) {
            const logs = await run(`docker logs ${containerName}`, { silent: false, allowFailure: true });
            throw new Error(`Container health check failed on port ${port}: ${endpointCheck.error || endpointCheck.status}\nLogs:\n${logs.stdout}`);
        }
        return true;
    }

    async checkEndpoint(url, expectedStatus = 200, timeout = 10000, retries = 6) {
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
                await new Promise(resolve => setTimeout(resolve, 1000));
            }
        }
    }

    assertContainerRunning(containers, appName, color) {
        const expectedName = `${appName}-${color}`;
        const container = containers.find(c => c.name === expectedName);
        if (!container) {
            throw new Error(`Container ${expectedName} not found. Running containers: ${containers.map(c => c.name).join(', ')}`);
        }
        if (!container.status.includes('Up')) {
            throw new Error(`Container ${expectedName} is not running: ${container.status}`);
        }
        return true;
    }

    async getContainerLogs(containerName) {
        const result = await run(`docker logs ${containerName}`, { silent: true, allowFailure: true });
        return result.stdout;
    }
} 