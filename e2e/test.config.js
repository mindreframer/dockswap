// E2E Test Configuration
export default {
    // Default timeout for e2e tests (30 seconds)
    timeout: 30000,

    // Test categories
    categories: {
        basic: "Basic deployment and functionality tests",
        error: "Error handling and edge case tests",
        integration: "Integration tests with external services"
    },

    // Test environment settings
    environment: {
        buildDockswap: true,
        pullImages: ["nginx:alpine"],
        cleanup: true
    },

    // Test apps and images
    apps: {
        nginx: {
            name: "nginx-test",
            image: "nginx:alpine",
            ports: {
                blue: 8080,
                green: 8081
            }
        }
    }
}; 