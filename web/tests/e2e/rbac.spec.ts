import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';
import { SetupPage } from '../pages/SetupPage';

const API_BASE = 'http://localhost:9096';
const ADMIN_SECRET = 'warden-e2e-magic-key';
const ADMIN_USER = 'admin';
const ADMIN_PASS = 'password123!';

/**
 * Helper: Reset the database and set up a fresh admin user.
 * Returns the admin session cookie for API-level tests.
 */
async function resetAndSetupAdmin(request: import('@playwright/test').APIRequestContext) {
    // 1. Reset DB
    const resetRes = await request.post(`${API_BASE}/api/admin/reset`, {
        headers: { 'X-Admin-Secret': ADMIN_SECRET },
    });
    expect(resetRes.ok()).toBeTruthy();

    // 2. Create admin via setup
    const setupRes = await request.post(`${API_BASE}/api/setup`, {
        headers: { 'X-Admin-Secret': ADMIN_SECRET },
        data: { username: ADMIN_USER, password: ADMIN_PASS, timezone: 'UTC' },
    });
    expect(setupRes.ok()).toBeTruthy();

    // 3. Login as admin to get session cookie
    const loginRes = await request.post(`${API_BASE}/api/auth/login`, {
        data: { username: ADMIN_USER, password: ADMIN_PASS },
    });
    expect(loginRes.ok()).toBeTruthy();

    const loginBody = await loginRes.json();
    expect(loginBody.user.role).toBe('admin');

    // Extract the auth_token cookie from the login response
    const cookies = loginRes.headers()['set-cookie'] || '';
    const match = cookies.match(/auth_token=([^;]+)/);
    expect(match).toBeTruthy();
    const authToken = match![1];

    return authToken;
}

/**
 * Helper: Create an API key with a specific role using admin credentials.
 * Returns the raw API key string.
 */
async function createApiKeyWithRole(
    request: import('@playwright/test').APIRequestContext,
    adminToken: string,
    name: string,
    role: string,
): Promise<string> {
    const res = await request.post(`${API_BASE}/api/api-keys`, {
        headers: {
            'Content-Type': 'application/json',
            Cookie: `auth_token=${adminToken}`,
        },
        data: { name, role },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(body.key).toBeTruthy();
    return body.key;
}

// ─────────────────────────────────────────────────────────────────────────────
// Test Suite: RBAC (Role-Based Access Control)
// ─────────────────────────────────────────────────────────────────────────────

test.describe('RBAC - Role-Based Access Control', () => {
    test.describe.configure({ mode: 'serial' });

    // ─── Test 1: Admin has full access to all UI tabs ────────────────────────
    test('Admin user has full access to all settings tabs', async ({ page, request }) => {
        // Reset and setup
        await resetAndSetupAdmin(request);

        const login = new LoginPage(page);
        const dashboard = new DashboardPage(page);

        // Navigate to dashboard and login
        await dashboard.goto();
        await page.waitForLoadState('networkidle');

        if (page.url().includes('/setup')) {
            const setup = new SetupPage(page);
            await setup.completeSetup();
        }

        if (page.url().includes('/login')) {
            await login.login();
        }

        await dashboard.waitForLoad();

        // Verify admin can see create monitor and create group buttons
        await expect(dashboard.createMonitorTrigger).toBeVisible();
        await expect(dashboard.createGroupTrigger).toBeVisible();

        // Navigate to Settings
        await page.locator('button:has(span:text-is("Settings"))').click();
        await page.getByRole('link', { name: 'General' }).click();
        await expect(page).toHaveURL(/.*settings/);

        // Verify all tabs are visible for admin
        const tabsList = page.locator('[role="tablist"]');
        await expect(tabsList.getByText('General')).toBeVisible();
        await expect(tabsList.getByText('Notifications')).toBeVisible();
        await expect(tabsList.getByText('Security')).toBeVisible();
        await expect(tabsList.getByText('System')).toBeVisible();
        await expect(tabsList.getByText('Users')).toBeVisible();
    });

    // ─── Test 2: Users management tab shows admin user ───────────────────────
    test('Users management tab shows admin with correct role', async ({ page, request }) => {
        const login = new LoginPage(page);
        const dashboard = new DashboardPage(page);

        await dashboard.goto();
        await page.waitForLoadState('networkidle');

        if (page.url().includes('/login')) {
            await login.login();
        }

        await dashboard.waitForLoad();

        // Navigate to Settings > Users tab
        await page.goto('/settings?tab=users');
        await expect(page.getByTestId('loading-spinner')).toHaveCount(0, { timeout: 10000 });
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 10000 });

        // Verify User Management card is visible
        await expect(page.getByText('User Management')).toBeVisible({ timeout: 10000 });

        // Verify the admin user appears in the user table with "(you)" indicator
        const usersTable = page.locator('table');
        await expect(usersTable).toBeVisible({ timeout: 10000 });
        const adminRow = usersTable.locator('tr').filter({ hasText: '(you)' });
        await expect(adminRow).toBeVisible();

        // Verify the admin user has a non-interactive "Admin" badge (self cannot change own role)
        const adminBadge = adminRow.getByText('Admin', { exact: true });
        await expect(adminBadge).toBeVisible();

        // The admin's own row should NOT have a delete/action button
        const actionBtn = adminRow.locator('button');
        await expect(actionBtn).toHaveCount(0);
    });

    // ─── Test 3: API key creation with role selection ────────────────────────
    test('API key creation with role selection via UI', async ({ page, request }) => {
        const login = new LoginPage(page);
        const dashboard = new DashboardPage(page);

        await dashboard.goto();
        await page.waitForLoadState('networkidle');

        if (page.url().includes('/login')) {
            await login.login();
        }

        await dashboard.waitForLoad();

        // Navigate to Settings > Security tab
        await page.goto('/settings?tab=security');
        await expect(page.getByTestId('loading-spinner')).toHaveCount(0, { timeout: 10000 });
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 10000 });

        // Wait for API Keys section
        await expect(page.getByText('API Keys', { exact: true })).toBeVisible({ timeout: 10000 });

        // Click "Create API Key" trigger
        const createBtn = page.getByTestId('create-apikey-trigger');
        await expect(createBtn).toBeVisible({ timeout: 5000 });
        await createBtn.click();

        // Fill in key name
        const keyName = `RBAC Test Key ${Date.now()}`;
        await page.getByTestId('apikey-name-input').fill(keyName);

        // Select "Viewer (read-only)" role
        // Click the role select trigger and choose viewer
        const roleSelect = page.locator('[role="combobox"]').last();
        await roleSelect.click();
        await page.getByRole('option', { name: 'Viewer (read-only)' }).click();

        // Submit
        await page.getByTestId('apikey-create-submit').click();

        // Verify success
        await expect(page.getByText('Success!')).toBeVisible({ timeout: 10000 });

        // Close the sheet
        await page.getByRole('button', { name: 'Done' }).click();

        // Verify the key appears in the table with "viewer" role
        await expect(page.getByText(keyName)).toBeVisible({ timeout: 10000 });

        // The role column should show "viewer" (capitalized via CSS)
        const keyRow = page.locator('tr').filter({ hasText: keyName });
        await expect(keyRow).toBeVisible();
        await expect(keyRow.getByText('viewer')).toBeVisible();
    });

    // ─── Test 4: Login response includes role ────────────────────────────────
    test('Login response includes user role', async ({ request }) => {
        // Login via API and verify the response includes role
        const loginRes = await request.post(`${API_BASE}/api/auth/login`, {
            data: { username: ADMIN_USER, password: ADMIN_PASS },
        });
        expect(loginRes.ok()).toBeTruthy();

        const body = await loginRes.json();
        expect(body.user).toBeDefined();
        expect(body.user.role).toBe('admin');
        expect(body.user.username).toBe(ADMIN_USER);
    });

    // ─── Test 5: /api/auth/me includes role ──────────────────────────────────
    test('/api/auth/me response includes role', async ({ request }) => {
        // Login to get token
        const loginRes = await request.post(`${API_BASE}/api/auth/login`, {
            data: { username: ADMIN_USER, password: ADMIN_PASS },
        });
        const cookies = loginRes.headers()['set-cookie'] || '';
        const match = cookies.match(/auth_token=([^;]+)/);
        const authToken = match![1];

        // Call /api/auth/me
        const meRes = await request.get(`${API_BASE}/api/auth/me`, {
            headers: { Cookie: `auth_token=${authToken}` },
        });
        expect(meRes.ok()).toBeTruthy();

        const body = await meRes.json();
        expect(body.user).toBeDefined();
        expect(body.user.role).toBe('admin');
        expect(body.user.username).toBe(ADMIN_USER);
    });

    // ─── Test 6: Viewer API key can read but not write ───────────────────────
    test('Viewer API key can GET overview but cannot POST monitors', async ({ request }) => {
        // Get admin token
        const adminToken = await resetAndSetupAdmin(request);

        // Create a group first (needed for monitor creation test)
        const groupRes = await request.post(`${API_BASE}/api/groups`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${adminToken}`,
            },
            data: { name: 'RBAC Test Group' },
        });
        expect(groupRes.status()).toBe(201);

        // Create a viewer API key
        const viewerKey = await createApiKeyWithRole(request, adminToken, 'viewer-key', 'viewer');

        // Viewer CAN read overview
        const overviewRes = await request.get(`${API_BASE}/api/overview`, {
            headers: { Authorization: `Bearer ${viewerKey}` },
        });
        expect(overviewRes.ok()).toBeTruthy();

        // Viewer CAN read uptime
        const uptimeRes = await request.get(`${API_BASE}/api/uptime`, {
            headers: { Authorization: `Bearer ${viewerKey}` },
        });
        expect(uptimeRes.ok()).toBeTruthy();

        // Viewer CANNOT create monitors (requires editor)
        const createMonitorRes = await request.post(`${API_BASE}/api/monitors`, {
            headers: {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${viewerKey}`,
            },
            data: {
                name: 'Unauthorized Monitor',
                url: 'https://example.com',
                groupId: 'g-rbac-test-group',
                interval: 30,
            },
        });
        expect(createMonitorRes.status()).toBe(403);

        // Viewer CANNOT create groups (requires editor)
        const createGroupRes = await request.post(`${API_BASE}/api/groups`, {
            headers: {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${viewerKey}`,
            },
            data: { name: 'Unauthorized Group' },
        });
        expect(createGroupRes.status()).toBe(403);

        // Viewer CANNOT update settings (requires admin)
        const settingsRes = await request.patch(`${API_BASE}/api/settings`, {
            headers: {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${viewerKey}`,
            },
            data: { latency_threshold: '500' },
        });
        expect(settingsRes.status()).toBe(403);

        // Viewer CANNOT list API keys (requires admin)
        const apiKeysRes = await request.get(`${API_BASE}/api/api-keys`, {
            headers: { Authorization: `Bearer ${viewerKey}` },
        });
        expect(apiKeysRes.status()).toBe(403);

        // Viewer CANNOT list users (requires admin)
        const usersRes = await request.get(`${API_BASE}/api/users`, {
            headers: { Authorization: `Bearer ${viewerKey}` },
        });
        expect(usersRes.status()).toBe(403);
    });

    // ─── Test 7: Editor API key can read and write but not admin ─────────────
    test('Editor API key can manage monitors but not settings or users', async ({ request }) => {
        // Get admin token (reuse from previous reset)
        const loginRes = await request.post(`${API_BASE}/api/auth/login`, {
            data: { username: ADMIN_USER, password: ADMIN_PASS },
        });
        const cookies = loginRes.headers()['set-cookie'] || '';
        const match = cookies.match(/auth_token=([^;]+)/);
        const adminToken = match![1];

        // Create an editor API key
        const editorKey = await createApiKeyWithRole(request, adminToken, 'editor-key', 'editor');

        // Editor CAN read overview
        const overviewRes = await request.get(`${API_BASE}/api/overview`, {
            headers: { Authorization: `Bearer ${editorKey}` },
        });
        expect(overviewRes.ok()).toBeTruthy();

        // Editor CAN create groups
        const createGroupRes = await request.post(`${API_BASE}/api/groups`, {
            headers: {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${editorKey}`,
            },
            data: { name: 'Editor Group' },
        });
        expect(createGroupRes.status()).toBe(201);

        // Editor CAN create monitors
        const createMonitorRes = await request.post(`${API_BASE}/api/monitors`, {
            headers: {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${editorKey}`,
            },
            data: {
                name: 'Editor Monitor',
                url: 'https://example.com',
                groupId: 'g-editor-group',
                interval: 30,
            },
        });
        expect(createMonitorRes.status()).toBe(201);

        // Editor CANNOT update settings (requires admin)
        const settingsRes = await request.patch(`${API_BASE}/api/settings`, {
            headers: {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${editorKey}`,
            },
            data: { latency_threshold: '500' },
        });
        expect(settingsRes.status()).toBe(403);

        // Editor CANNOT list or create API keys (requires admin)
        const listKeysRes = await request.get(`${API_BASE}/api/api-keys`, {
            headers: { Authorization: `Bearer ${editorKey}` },
        });
        expect(listKeysRes.status()).toBe(403);

        const createKeyRes = await request.post(`${API_BASE}/api/api-keys`, {
            headers: {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${editorKey}`,
            },
            data: { name: 'sneaky-key', role: 'admin' },
        });
        expect(createKeyRes.status()).toBe(403);

        // Editor CANNOT list users (requires admin)
        const usersRes = await request.get(`${API_BASE}/api/users`, {
            headers: { Authorization: `Bearer ${editorKey}` },
        });
        expect(usersRes.status()).toBe(403);

        // Editor CANNOT change user roles (requires admin)
        const roleRes = await request.patch(`${API_BASE}/api/users/1/role`, {
            headers: {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${editorKey}`,
            },
            data: { role: 'viewer' },
        });
        expect(roleRes.status()).toBe(403);
    });

    // ─── Test 8: Admin API key has full access ───────────────────────────────
    test('Admin API key has full access including settings and users', async ({ request }) => {
        // Login as admin
        const loginRes = await request.post(`${API_BASE}/api/auth/login`, {
            data: { username: ADMIN_USER, password: ADMIN_PASS },
        });
        const cookies = loginRes.headers()['set-cookie'] || '';
        const match = cookies.match(/auth_token=([^;]+)/);
        const adminToken = match![1];

        // Create an admin API key
        const adminKey = await createApiKeyWithRole(request, adminToken, 'admin-key', 'admin');

        // Admin key CAN read overview
        const overviewRes = await request.get(`${API_BASE}/api/overview`, {
            headers: { Authorization: `Bearer ${adminKey}` },
        });
        expect(overviewRes.ok()).toBeTruthy();

        // Admin key CAN read settings
        const settingsGetRes = await request.get(`${API_BASE}/api/settings`, {
            headers: { Authorization: `Bearer ${adminKey}` },
        });
        expect(settingsGetRes.ok()).toBeTruthy();

        // Admin key CAN update settings
        const settingsPatchRes = await request.patch(`${API_BASE}/api/settings`, {
            headers: {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${adminKey}`,
            },
            data: { latency_threshold: '2000' },
        });
        expect(settingsPatchRes.ok()).toBeTruthy();

        // Admin key CAN list API keys
        const listKeysRes = await request.get(`${API_BASE}/api/api-keys`, {
            headers: { Authorization: `Bearer ${adminKey}` },
        });
        expect(listKeysRes.ok()).toBeTruthy();

        // Admin key CAN list users
        const usersRes = await request.get(`${API_BASE}/api/users`, {
            headers: { Authorization: `Bearer ${adminKey}` },
        });
        expect(usersRes.ok()).toBeTruthy();
        const usersBody = await usersRes.json();
        expect(usersBody.users).toBeDefined();
        expect(usersBody.users.length).toBeGreaterThanOrEqual(1);
    });

    // ─── Test 9: Role hierarchy is enforced correctly ────────────────────────
    test('Role hierarchy: admin > editor > viewer permissions', async ({ request }) => {
        // Login to get admin session
        const loginRes = await request.post(`${API_BASE}/api/auth/login`, {
            data: { username: ADMIN_USER, password: ADMIN_PASS },
        });
        const cookies = loginRes.headers()['set-cookie'] || '';
        const match = cookies.match(/auth_token=([^;]+)/);
        const adminToken = match![1];

        // Create keys for each role
        const adminKey = await createApiKeyWithRole(request, adminToken, 'hierarchy-admin', 'admin');
        const editorKey = await createApiKeyWithRole(request, adminToken, 'hierarchy-editor', 'editor');
        const viewerKey = await createApiKeyWithRole(request, adminToken, 'hierarchy-viewer', 'viewer');

        // All roles can read overview (viewer+)
        for (const key of [adminKey, editorKey, viewerKey]) {
            const res = await request.get(`${API_BASE}/api/overview`, {
                headers: { Authorization: `Bearer ${key}` },
            });
            expect(res.ok()).toBeTruthy();
        }

        // All roles can read settings (viewer+ can GET, only admin can PATCH)
        for (const key of [adminKey, editorKey, viewerKey]) {
            const res = await request.get(`${API_BASE}/api/settings`, {
                headers: { Authorization: `Bearer ${key}` },
            });
            expect(res.ok()).toBeTruthy();
        }

        // Only editor+ can create groups
        const viewerGroupRes = await request.post(`${API_BASE}/api/groups`, {
            headers: {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${viewerKey}`,
            },
            data: { name: 'Viewer Attempt' },
        });
        expect(viewerGroupRes.status()).toBe(403);

        const editorGroupRes = await request.post(`${API_BASE}/api/groups`, {
            headers: {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${editorKey}`,
            },
            data: { name: 'Hierarchy Editor Group' },
        });
        expect(editorGroupRes.status()).toBe(201);

        // Only admin can update settings
        const editorSettingsRes = await request.patch(`${API_BASE}/api/settings`, {
            headers: {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${editorKey}`,
            },
            data: { latency_threshold: '999' },
        });
        expect(editorSettingsRes.status()).toBe(403);

        const adminSettingsRes = await request.patch(`${API_BASE}/api/settings`, {
            headers: {
                'Content-Type': 'application/json',
                Authorization: `Bearer ${adminKey}`,
            },
            data: { latency_threshold: '999' },
        });
        expect(adminSettingsRes.ok()).toBeTruthy();
    });

    // ─── Test 10: API key cannot access /api/auth/me ─────────────────────────
    test('API key cannot access user-specific endpoints', async ({ request }) => {
        const loginRes = await request.post(`${API_BASE}/api/auth/login`, {
            data: { username: ADMIN_USER, password: ADMIN_PASS },
        });
        const cookies = loginRes.headers()['set-cookie'] || '';
        const match = cookies.match(/auth_token=([^;]+)/);
        const adminToken = match![1];

        const adminKey = await createApiKeyWithRole(request, adminToken, 'me-test-key', 'admin');

        // API keys CANNOT access /api/auth/me (user-specific endpoint)
        const meRes = await request.get(`${API_BASE}/api/auth/me`, {
            headers: { Authorization: `Bearer ${adminKey}` },
        });
        expect(meRes.status()).toBe(403);
        const meBody = await meRes.json();
        expect(meBody.error).toContain('API keys cannot access user profile');
    });

    // ─── Test 11: Cleanup - restore standard admin ───────────────────────────
    test('Cleanup: restore standard admin state', async ({ request }) => {
        // Reset DB
        const resetRes = await request.post(`${API_BASE}/api/admin/reset`, {
            headers: { 'X-Admin-Secret': ADMIN_SECRET },
        });
        expect(resetRes.ok()).toBeTruthy();

        // Restore standard admin
        const setupRes = await request.post(`${API_BASE}/api/setup`, {
            headers: { 'X-Admin-Secret': ADMIN_SECRET },
            data: { username: ADMIN_USER, password: ADMIN_PASS, timezone: 'UTC' },
        });
        expect(setupRes.ok()).toBeTruthy();
    });
});
