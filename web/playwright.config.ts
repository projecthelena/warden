import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
    testDir: './tests',
    fullyParallel: false,
    workers: 1,
    forbidOnly: !!process.env.CI,
    retries: process.env.CI ? 2 : 0,
    reporter: 'html',
    use: {
        baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173',
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
        // All other tests can run in parallel after auth tests complete
        {
            name: 'chromium',
            testIgnore: [
                /auth\.spec\.ts/,
                /custom_setup\.spec\.ts/,
                /status_pages\.spec\.ts/,
                /status_pages_full\.spec\.ts/,
            ],
            use: { ...devices['Desktop Chrome'] },
            dependencies: ['custom-setup'],
        },
    ],

    // Run your local dev server before starting the tests
    webServer: {
        command: 'npm run dev',
        url: 'http://localhost:5173',
        reuseExistingServer: !process.env.CI,
        timeout: 120 * 1000,
    },
});
