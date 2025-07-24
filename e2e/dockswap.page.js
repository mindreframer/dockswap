import { run } from './utils.js';

export class DockSwap {
    constructor({ binPath = './dockswap', env = process.env, baseDir = '' } = {}) {
        this.binPath = binPath;
        this.env = { ...env, DOCKSWAP_CONFIG_DIR: baseDir };
        this.baseDir = baseDir;
    }

    async deploy(app, image) {
        // Match dockswapDeploy: deploy <app> <image>
        return await run(`${this.binPath} deploy ${app} ${image}`, { env: this.env });
    }

    async switch(app, color) {
        // Match dockswapSwitch: switch <app> <color>
        return await run(`${this.binPath} switch ${app} ${color}`, { env: this.env });
    }

    async status(app) {
        // Match dockswapStatus: status <app>
        return await run(`${this.binPath} status ${app}`, { env: this.env });
    }

    async caddyReload() {
        return await run(`${this.binPath} caddy reload`, { env: this.env });
    }

    async caddyConfigCreate() {
        return await run(`${this.binPath} caddy config create`, { env: this.env });
    }
} 