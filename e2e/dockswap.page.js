import { run } from './utils.js';

export class DockSwap {
    constructor({ binPath = './dockswap', env = process.env, baseDir = '' } = {}) {
        this.binPath = binPath;
        this.env = { ...env, DOCKSWAP_CONFIG_DIR: baseDir };
        this.baseDir = baseDir;
    }

    async deploy(app, image) {
        return await run(`${this.binPath} deploy ${app} ${image}`, { env: this.env });
    }

    async switch(app, color) {
        return await run(`${this.binPath} switch ${app} ${color}`, { env: this.env });
    }

    async status(app) {
        return await run(`${this.binPath} status ${app}`, { env: this.env });
    }

    async caddyReload() {
        return await run(`${this.binPath} caddy reload`, { env: this.env });
    }

    async caddyConfigCreate() {
        return await run(`${this.binPath} caddy config create`, { env: this.env });
    }

    async validateDatabaseState(appName, expectedColor, expectedImage) {
        // Run status command
        const result = await run(`${this.binPath} status ${appName}`, { silent: true, env: this.env });
        if (!result.success) {
            throw new Error(`Failed to get status: ${result.stderr}`);
        }
        // Parse status output
        const lines = result.stdout.split('\n');
        const colorLine = lines.find(line => line.includes('Color:'));
        const imageLine = lines.find(line => line.includes('Image:'));
        const statusLine = lines.find(line => line.includes('Status:'));
        const updatedLine = lines.find(line => line.includes('Updated:'));
        const status = {
            activeColor: colorLine?.split('Color:')[1]?.trim(),
            image: imageLine?.split('Image:')[1]?.trim(),
            status: statusLine?.split('Status:')[1]?.trim(),
            updated: updatedLine?.split('Updated:')[1]?.trim(),
            raw: result.stdout
        };
        if (status.activeColor !== expectedColor) {
            throw new Error(`Expected active color ${expectedColor}, got ${status.activeColor}`);
        }
        if (status.image !== expectedImage) {
            throw new Error(`Expected image ${expectedImage}, got ${status.image}`);
        }
        return status;
    }
} 