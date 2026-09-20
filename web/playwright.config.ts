import { defineConfig, devices } from '@playwright/test';

const baseURL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';
const backendURL = process.env.WARDEN_E2E_BACKEND_URL;
const frontendURL = backendURL ? baseURL : 'http://localhost:5173';
const webServerPort = new URL(frontendURL).port || '5173';

export default defineConfig({
    testDir: './tests',
    fullyParallel: false,
    workers: 1,
    forbidOnly: !!process.env.CI,
    retries: process.env.CI ? 2 : 0,
    reporter: 'html',
    use: {
        baseURL,
        trace: 'on-first-retry',
    },

    projects: [
        // Auth tests run first and alone (they log out which can affect other tests)
        {
            name: 'auth',
            testMatch: /auth\.spec\.ts/,
            use: { ...devices['Desktop Chrome'] },
        },
        // Custom setup tests also run isolated (they modify admin user)
        {
            name: 'custom-setup',
            testMatch: /custom_setup\.spec\.ts/,
            use: { ...devices['Desktop Chrome'] },
            dependencies: ['auth'],
        },
        // Status page tests share the "all" status page - run them serially
        {
            name: 'status-pages',
            testMatch: /status_pages\.spec\.ts/,
            use: { ...devices['Desktop Chrome'] },
            dependencies: ['custom-setup'],
        },
        {
            name: 'status-pages-full',
            testMatch: /status_pages_full\.spec\.ts/,
            use: { ...devices['Desktop Chrome'] },
            dependencies: ['status-pages'],
        },
        // RBAC tests run isolated (they reset the database)
        {
            name: 'rbac',
            testMatch: /rbac\.spec\.ts/,
            use: { ...devices['Desktop Chrome'] },
            dependencies: ['status-pages-full'],
        },
        // Status viewer tests run isolated (they reset the database)
        {
            name: 'status-viewer',
            testMatch: /status_viewer\.spec\.ts/,
            use: { ...devices['Desktop Chrome'] },
            dependencies: ['rbac'],
        },
        // Comprehensive RBAC role tests (reset DB, test all roles)
        {
            name: 'rbac-roles',
            testMatch: /rbac_roles\.spec\.ts/,
            use: { ...devices['Desktop Chrome'] },
            dependencies: ['status-viewer'],
        },
        // All other tests can run in parallel after auth tests complete
        {
            name: 'chromium',
            testIgnore: [
                /auth\.spec\.ts/,
                /custom_setup\.spec\.ts/,
                /status_pages\.spec\.ts/,
                /status_pages_full\.spec\.ts/,
                /rbac\.spec\.ts/,
                /status_viewer\.spec\.ts/,
                /rbac_roles\.spec\.ts/,
            ],
            use: { ...devices['Desktop Chrome'] },
            dependencies: ['custom-setup'],
        },
    ],

    // Run your local dev server before starting the tests
    webServer: [
        ...(backendURL ? [{
            command: 'go run ../cmd/dashboard',
            url: `${backendURL}/healthz`,
            reuseExistingServer: false,
            timeout: 120 * 1000,
        }] : []),
        {
            command: `npm run dev -- --host 127.0.0.1 --port ${webServerPort}`,
            url: frontendURL,
            reuseExistingServer: !process.env.CI,
            timeout: 120 * 1000,
        },
    ],
});
