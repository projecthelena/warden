import { API_BASE } from '../apiBase';
import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';

const API_BASE = API_BASE;
const ADMIN_SECRET = 'warden-e2e-magic-key';
const ADMIN_USER = 'admin';
const ADMIN_PASS = 'password123!';

const EDITOR_USER = 'editoruser';
const EDITOR_PASS = 'editor1234!';

/**
 * Helper: Reset the database and set up a fresh admin user.
 * Returns the admin session cookie for API-level tests.
 */
async function resetAndSetupAdmin(request: import('@playwright/test').APIRequestContext) {
    const resetRes = await request.post(`${API_BASE}/api/admin/reset`, {
        headers: { 'X-Admin-Secret': ADMIN_SECRET },
    });
    expect(resetRes.ok()).toBeTruthy();

    const setupRes = await request.post(`${API_BASE}/api/setup`, {
        headers: { 'X-Admin-Secret': ADMIN_SECRET },
        data: { username: ADMIN_USER, password: ADMIN_PASS, timezone: 'UTC' },
    });
    expect(setupRes.ok()).toBeTruthy();

    const loginRes = await request.post(`${API_BASE}/api/auth/login`, {
        data: { username: ADMIN_USER, password: ADMIN_PASS },
    });
    expect(loginRes.ok()).toBeTruthy();

    const cookies = loginRes.headers()['set-cookie'] || '';
    const match = cookies.match(/auth_token=([^;]+)/);
    expect(match).toBeTruthy();
    return match![1];
}

/**
 * Helper: Create a user with a given role via admin API.
 */
async function createUser(
    request: import('@playwright/test').APIRequestContext,
    adminToken: string,
    username: string,
    password: string,
    role: string,
) {
    const res = await request.post(`${API_BASE}/api/users`, {
        headers: {
            'Content-Type': 'application/json',
            Cookie: `auth_token=${adminToken}`,
        },
        data: { username, password, role },
    });
    expect(res.status()).toBe(201);
}

/**
 * Helper: Login as a user and return their auth token.
 */
async function loginAsUser(
    request: import('@playwright/test').APIRequestContext,
    username: string,
    password: string,
): Promise<string> {
    const loginRes = await request.post(`${API_BASE}/api/auth/login`, {
        data: { username, password },
    });
    expect(loginRes.ok()).toBeTruthy();

    const cookies = loginRes.headers()['set-cookie'] || '';
    const match = cookies.match(/auth_token=([^;]+)/);
    expect(match).toBeTruthy();
    return match![1];
}

// ─────────────────────────────────────────────────────────────────────────────
// Test Suite: Admin vs Editor Role Boundary
// ─────────────────────────────────────────────────────────────────────────────

test.describe('RBAC - Admin vs Editor Boundary', () => {
    test.describe.configure({ mode: 'serial' });

    let adminToken: string;
    let editorToken: string;

    // ─── Setup ─────────────────────────────────────────────────────────────
    test('Setup: reset DB, create admin and editor users', async ({ request }) => {
        adminToken = await resetAndSetupAdmin(request);

        // Create editor user
        await createUser(request, adminToken, EDITOR_USER, EDITOR_PASS, 'editor');

        // Login as editor
        editorToken = await loginAsUser(request, EDITOR_USER, EDITOR_PASS);

        // Verify editor role returned in /me
        const meRes = await request.get(`${API_BASE}/api/auth/me`, {
            headers: { Cookie: `auth_token=${editorToken}` },
        });
        expect(meRes.ok()).toBeTruthy();
        const meBody = await meRes.json();
        expect(meBody.user.role).toBe('editor');

        // Create a group and monitor for later tests
        const groupRes = await request.post(`${API_BASE}/api/groups`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${adminToken}`,
            },
            data: { name: 'Test Group' },
        });
        expect(groupRes.status()).toBe(201);
    });

    // ─────────────────────────────────────────────────────────────────────────
    // EDITOR API: things editor CAN do
    // ─────────────────────────────────────────────────────────────────────────

    test('Editor API: can create groups', async ({ request }) => {
        const res = await request.post(`${API_BASE}/api/groups`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${editorToken}`,
            },
            data: { name: 'Editor Group' },
        });
        expect(res.status()).toBe(201);
    });

    test('Editor API: can create monitors', async ({ request }) => {
        const res = await request.post(`${API_BASE}/api/monitors`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${editorToken}`,
            },
            data: {
                name: 'Editor Monitor',
                url: 'https://example.com',
                groupId: 'g-editor-group',
                interval: 30,
            },
        });
        expect(res.status()).toBe(201);
    });

    test('Editor API: can create incidents', async ({ request }) => {
        const res = await request.post(`${API_BASE}/api/incidents`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${editorToken}`,
            },
            data: {
                title: 'Editor Incident',
                description: 'Something happened',
                severity: 'major',
                status: 'investigating',
                startTime: new Date().toISOString(),
                affectedGroups: [],
                public: true,
            },
        });
        expect(res.status()).toBe(201);
    });

    test('Editor API: can create maintenance windows', async ({ request }) => {
        const start = new Date(Date.now() + 3600000).toISOString();
        const end = new Date(Date.now() + 7200000).toISOString();
        const res = await request.post(`${API_BASE}/api/maintenance`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${editorToken}`,
            },
            data: {
                title: 'Editor Maintenance',
                description: 'Scheduled work',
                status: 'scheduled',
                startTime: start,
                endTime: end,
                affectedGroups: [],
            },
        });
        expect(res.status()).toBe(201);
    });

    test('Editor API: can create notification channels', async ({ request }) => {
        const res = await request.post(`${API_BASE}/api/notifications/channels`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${editorToken}`,
            },
            data: {
                type: 'webhook',
                name: 'Editor Webhook',
                config: { webhook_url: 'https://example.com/hook' },
                enabled: true,
            },
        });
        expect(res.status()).toBe(201);
    });

    test('Editor API: can read overview, uptime, settings, status pages', async ({ request }) => {
        const endpoints = ['/api/overview', '/api/uptime', '/api/settings', '/api/status-pages'];
        for (const endpoint of endpoints) {
            const res = await request.get(`${API_BASE}${endpoint}`, {
                headers: { Cookie: `auth_token=${editorToken}` },
            });
            expect(res.ok(), `Editor should access GET ${endpoint}`).toBeTruthy();
        }
    });

    test('Editor API: can toggle status page', async ({ request }) => {
        const res = await request.patch(`${API_BASE}/api/status-pages/all`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${editorToken}`,
            },
            data: { enabled: true, public: true, title: 'All Systems' },
        });
        expect(res.ok()).toBeTruthy();
    });

    // ─────────────────────────────────────────────────────────────────────────
    // EDITOR API: things editor CANNOT do (admin-only)
    // ─────────────────────────────────────────────────────────────────────────

    test('Editor API: cannot update system settings', async ({ request }) => {
        const res = await request.patch(`${API_BASE}/api/settings`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${editorToken}`,
            },
            data: { latency_threshold: '999' },
        });
        expect(res.status()).toBe(403);
    });

    test('Editor API: cannot manage users (create, list, update role, delete)', async ({ request }) => {
        // Cannot create users
        const createRes = await request.post(`${API_BASE}/api/users`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${editorToken}`,
            },
            data: { username: 'hacker', password: 'longpassword', role: 'admin' },
        });
        expect(createRes.status()).toBe(403);

        // Cannot list users
        const listRes = await request.get(`${API_BASE}/api/users`, {
            headers: { Cookie: `auth_token=${editorToken}` },
        });
        expect(listRes.status()).toBe(403);

        // Cannot update roles
        const roleRes = await request.patch(`${API_BASE}/api/users/1/role`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${editorToken}`,
            },
            data: { role: 'admin' },
        });
        expect(roleRes.status()).toBe(403);

        // Cannot delete users
        const deleteRes = await request.delete(`${API_BASE}/api/users/1`, {
            headers: { Cookie: `auth_token=${editorToken}` },
        });
        expect(deleteRes.status()).toBe(403);
    });

    test('Editor API: cannot manage API keys (list, create, delete)', async ({ request }) => {
        // Cannot list API keys
        const listRes = await request.get(`${API_BASE}/api/api-keys`, {
            headers: { Cookie: `auth_token=${editorToken}` },
        });
        expect(listRes.status()).toBe(403);

        // Cannot create API keys
        const createRes = await request.post(`${API_BASE}/api/api-keys`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${editorToken}`,
            },
            data: { name: 'sneaky-key', role: 'admin' },
        });
        expect(createRes.status()).toBe(403);

        // Cannot delete API keys
        const deleteRes = await request.delete(`${API_BASE}/api/api-keys/1`, {
            headers: { Cookie: `auth_token=${editorToken}` },
        });
        expect(deleteRes.status()).toBe(403);
    });

    test('Editor API: cannot access user status page assignments', async ({ request }) => {
        // Cannot get assignments
        const getRes = await request.get(`${API_BASE}/api/users/1/status-pages`, {
            headers: { Cookie: `auth_token=${editorToken}` },
        });
        expect(getRes.status()).toBe(403);

        // Cannot set assignments
        const setRes = await request.put(`${API_BASE}/api/users/1/status-pages`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${editorToken}`,
            },
            data: { statusPageIds: [1] },
        });
        expect(setRes.status()).toBe(403);
    });

    // ─────────────────────────────────────────────────────────────────────────
    // ADMIN API: can do everything editor can + admin-only
    // ─────────────────────────────────────────────────────────────────────────

    test('Admin API: can do everything editor can', async ({ request }) => {
        // Create group
        const groupRes = await request.post(`${API_BASE}/api/groups`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${adminToken}`,
            },
            data: { name: 'Admin Group' },
        });
        expect(groupRes.status()).toBe(201);

        // Create monitor
        const monitorRes = await request.post(`${API_BASE}/api/monitors`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${adminToken}`,
            },
            data: {
                name: 'Admin Monitor',
                url: 'https://example.com',
                groupId: 'g-admin-group',
                interval: 30,
            },
        });
        expect(monitorRes.status()).toBe(201);

        // Create incident
        const incidentRes = await request.post(`${API_BASE}/api/incidents`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${adminToken}`,
            },
            data: {
                title: 'Admin Incident',
                description: 'Admin created',
                severity: 'minor',
                status: 'investigating',
                startTime: new Date().toISOString(),
                affectedGroups: [],
                public: false,
            },
        });
        expect(incidentRes.status()).toBe(201);

        // Create maintenance
        const start = new Date(Date.now() + 3600000).toISOString();
        const end = new Date(Date.now() + 7200000).toISOString();
        const maintRes = await request.post(`${API_BASE}/api/maintenance`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${adminToken}`,
            },
            data: {
                title: 'Admin Maintenance',
                description: 'Admin scheduled',
                status: 'scheduled',
                startTime: start,
                endTime: end,
                affectedGroups: [],
            },
        });
        expect(maintRes.status()).toBe(201);

        // Create notification channel
        const notifRes = await request.post(`${API_BASE}/api/notifications/channels`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${adminToken}`,
            },
            data: {
                type: 'webhook',
                name: 'Admin Webhook',
                config: { webhook_url: 'https://example.com/admin-hook' },
                enabled: true,
            },
        });
        expect(notifRes.status()).toBe(201);

        // Toggle status page
        const toggleRes = await request.patch(`${API_BASE}/api/status-pages/all`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${adminToken}`,
            },
            data: { enabled: true, public: false, title: 'Admin Status' },
        });
        expect(toggleRes.ok()).toBeTruthy();
    });

    test('Admin API: can update system settings (editor cannot)', async ({ request }) => {
        const res = await request.patch(`${API_BASE}/api/settings`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${adminToken}`,
            },
            data: { latency_threshold: '2000' },
        });
        expect(res.ok()).toBeTruthy();
    });

    test('Admin API: can manage users (editor cannot)', async ({ request }) => {
        // List users
        const listRes = await request.get(`${API_BASE}/api/users`, {
            headers: { Cookie: `auth_token=${adminToken}` },
        });
        expect(listRes.ok()).toBeTruthy();
        const listBody = await listRes.json();
        expect(listBody.users.length).toBeGreaterThanOrEqual(2);

        // Create user
        const createRes = await request.post(`${API_BASE}/api/users`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${adminToken}`,
            },
            data: { username: 'tempuser', password: 'longpassword', role: 'viewer' },
        });
        expect(createRes.status()).toBe(201);

        // Get the temp user's ID
        const usersRes = await request.get(`${API_BASE}/api/users`, {
            headers: { Cookie: `auth_token=${adminToken}` },
        });
        const usersBody = await usersRes.json();
        const tempUser = usersBody.users.find((u: { username: string }) => u.username === 'tempuser');
        expect(tempUser).toBeTruthy();

        // Update role
        const roleRes = await request.patch(`${API_BASE}/api/users/${tempUser.id}/role`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${adminToken}`,
            },
            data: { role: 'editor' },
        });
        expect(roleRes.ok()).toBeTruthy();

        // Delete user
        const deleteRes = await request.delete(`${API_BASE}/api/users/${tempUser.id}`, {
            headers: { Cookie: `auth_token=${adminToken}` },
        });
        expect(deleteRes.ok()).toBeTruthy();
    });

    test('Admin API: can manage API keys (editor cannot)', async ({ request }) => {
        // Create API key
        const createRes = await request.post(`${API_BASE}/api/api-keys`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${adminToken}`,
            },
            data: { name: 'admin-test-key', role: 'editor' },
        });
        expect(createRes.ok()).toBeTruthy();

        // List API keys
        const listRes = await request.get(`${API_BASE}/api/api-keys`, {
            headers: { Cookie: `auth_token=${adminToken}` },
        });
        expect(listRes.ok()).toBeTruthy();
        const listBody = await listRes.json();
        expect(listBody.keys.length).toBeGreaterThanOrEqual(1);

        // Delete API key
        const keyId = listBody.keys[0].id;
        const deleteRes = await request.delete(`${API_BASE}/api/api-keys/${keyId}`, {
            headers: { Cookie: `auth_token=${adminToken}` },
        });
        expect(deleteRes.ok()).toBeTruthy();
    });

    // ─────────────────────────────────────────────────────────────────────────
    // EDITOR UI: things editor CAN see/do
    // ─────────────────────────────────────────────────────────────────────────

    test('Editor UI: can see dashboard with create buttons', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });
        await page.getByLabel('Username').fill(EDITOR_USER);
        await page.getByLabel('Password').fill(EDITOR_PASS);
        await page.getByRole('button', { name: 'Sign In' }).click();

        await expect(page).toHaveURL(/.*dashboard/, { timeout: 15000 });
        await expect(page.getByTestId('loading-spinner')).toHaveCount(0, { timeout: 15000 });
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 15000 });

        // Editor should see create monitor and create group buttons
        await expect(page.getByRole('button', { name: 'New Monitor' })).toBeVisible({ timeout: 10000 });
        await expect(page.getByTestId('create-group-trigger')).toBeVisible({ timeout: 10000 });
    });

    test('Editor UI: can see Settings tab in monitor details sheet', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });
        await page.getByLabel('Username').fill(EDITOR_USER);
        await page.getByLabel('Password').fill(EDITOR_PASS);
        await page.getByRole('button', { name: 'Sign In' }).click();

        await expect(page).toHaveURL(/.*dashboard/, { timeout: 15000 });
        await expect(page.getByTestId('loading-spinner')).toHaveCount(0, { timeout: 15000 });
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 15000 });

        // Navigate to group with monitors
        await page.getByText('Editor Group').first().click();
        await expect(page).toHaveURL(/.*groups\//, { timeout: 15000 });

        // Click on the monitor
        await page.getByText('Editor Monitor').first().click();
        await expect(page.locator('[data-state="open"].fixed.inset-0')).toBeVisible({ timeout: 10000 });

        // Editor SHOULD see Settings tab (unlike viewer)
        await expect(page.getByTestId('monitor-settings-tab')).toBeVisible({ timeout: 10000 });
    });

    test('Editor UI: Settings page shows General + Notifications tabs only', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });
        await page.getByLabel('Username').fill(EDITOR_USER);
        await page.getByLabel('Password').fill(EDITOR_PASS);
        await page.getByRole('button', { name: 'Sign In' }).click();

        await expect(page).toHaveURL(/.*dashboard/, { timeout: 15000 });
        await expect(page.getByTestId('loading-spinner')).toHaveCount(0, { timeout: 15000 });
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 15000 });

        await page.goto('/settings');
        await expect(page.getByText('Settings').first()).toBeVisible({ timeout: 10000 });

        const tabsList = page.locator('[role="tablist"]');

        // Editor sees General + Notifications
        await expect(tabsList.getByText('General')).toBeVisible({ timeout: 10000 });
        await expect(tabsList.getByText('Notifications')).toBeVisible({ timeout: 10000 });

        // Editor does NOT see Security, System, Users (admin-only tabs)
        await expect(tabsList.getByText('Security')).toHaveCount(0);
        await expect(tabsList.getByText('System')).toHaveCount(0);
        await expect(tabsList.getByText('Users')).toHaveCount(0);
    });

    test('Editor UI: sidebar Settings shows General + Notifications sub-items only', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });
        await page.getByLabel('Username').fill(EDITOR_USER);
        await page.getByLabel('Password').fill(EDITOR_PASS);
        await page.getByRole('button', { name: 'Sign In' }).click();

        await expect(page).toHaveURL(/.*dashboard/, { timeout: 15000 });
        await expect(page.getByTestId('loading-spinner')).toHaveCount(0, { timeout: 15000 });
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 15000 });

        // Expand Settings in sidebar
        const sidebar = page.locator('[data-sidebar="sidebar"]');
        await sidebar.getByText('Settings').first().click();

        // Editor sees General + Notifications
        await expect(sidebar.getByText('General')).toBeVisible({ timeout: 10000 });
        await expect(sidebar.getByText('Notifications')).toBeVisible({ timeout: 10000 });

        // Editor does NOT see Security, System, Users
        await expect(sidebar.getByText('Security')).toHaveCount(0);
        await expect(sidebar.getByText('System')).toHaveCount(0);
        await expect(sidebar.getByText('Users')).toHaveCount(0);
    });

    test('Editor UI: can toggle status page switches', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });
        await page.getByLabel('Username').fill(EDITOR_USER);
        await page.getByLabel('Password').fill(EDITOR_PASS);
        await page.getByRole('button', { name: 'Sign In' }).click();

        await expect(page).toHaveURL(/.*dashboard/, { timeout: 15000 });
        await expect(page.getByTestId('loading-spinner')).toHaveCount(0, { timeout: 15000 });
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 15000 });

        await page.goto('/status-pages');
        await page.waitForLoadState('networkidle');
        await expect(page.getByRole('heading', { name: 'Status Pages' })).toBeVisible({ timeout: 10000 });

        // Editor should see enabled toggle and it should be enabled (interactive)
        const enabledToggle = page.getByTestId('status-page-enabled-toggle-all');
        await expect(enabledToggle).toBeVisible({ timeout: 10000 });
        await expect(enabledToggle).toBeEnabled();

        // Editor should see config gear icon
        await expect(page.getByTestId('status-page-config-all')).toBeVisible({ timeout: 10000 });
    });

    // ─────────────────────────────────────────────────────────────────────────
    // ADMIN UI: can see everything editor can + admin-only tabs
    // ─────────────────────────────────────────────────────────────────────────

    test('Admin UI: Settings page shows ALL tabs (General, Notifications, Security, System, Users)', async ({ page }) => {
        await page.context().clearCookies();

        const login = new LoginPage(page);
        const dashboard = new DashboardPage(page);

        await dashboard.goto();
        await page.waitForLoadState('networkidle');

        if (page.url().includes('/login')) {
            await login.login(ADMIN_USER, ADMIN_PASS);
        }

        await dashboard.waitForLoad();

        await page.goto('/settings');
        await expect(page.getByText('Settings').first()).toBeVisible({ timeout: 10000 });

        const tabsList = page.locator('[role="tablist"]');
        await expect(tabsList.getByText('General')).toBeVisible({ timeout: 10000 });
        await expect(tabsList.getByText('Notifications')).toBeVisible({ timeout: 10000 });
        await expect(tabsList.getByText('Security')).toBeVisible({ timeout: 10000 });
        await expect(tabsList.getByText('System')).toBeVisible({ timeout: 10000 });
        await expect(tabsList.getByText('Users')).toBeVisible({ timeout: 10000 });
    });

    test('Admin UI: sidebar Settings shows ALL sub-items', async ({ page }) => {
        await page.context().clearCookies();

        const login = new LoginPage(page);
        const dashboard = new DashboardPage(page);

        await dashboard.goto();
        await page.waitForLoadState('networkidle');

        if (page.url().includes('/login')) {
            await login.login(ADMIN_USER, ADMIN_PASS);
        }

        await dashboard.waitForLoad();

        const sidebar = page.locator('[data-sidebar="sidebar"]');
        await sidebar.getByText('Settings').first().click();

        await expect(sidebar.getByText('General')).toBeVisible({ timeout: 10000 });
        await expect(sidebar.getByText('Notifications')).toBeVisible({ timeout: 10000 });
        await expect(sidebar.getByText('Security')).toBeVisible({ timeout: 10000 });
        await expect(sidebar.getByText('System')).toBeVisible({ timeout: 10000 });
        await expect(sidebar.getByText('Users')).toBeVisible({ timeout: 10000 });
    });

    // ─── Cleanup ───────────────────────────────────────────────────────────
    test('Cleanup: reset DB and restore admin', async ({ request }) => {
        const resetRes = await request.post(`${API_BASE}/api/admin/reset`, {
            headers: { 'X-Admin-Secret': ADMIN_SECRET },
        });
        expect(resetRes.ok()).toBeTruthy();

        const setupRes = await request.post(`${API_BASE}/api/setup`, {
            headers: { 'X-Admin-Secret': ADMIN_SECRET },
            data: { username: ADMIN_USER, password: ADMIN_PASS, timezone: 'UTC' },
        });
        expect(setupRes.ok()).toBeTruthy();
    });
});
