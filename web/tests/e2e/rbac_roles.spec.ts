import { API_BASE } from '../apiBase';
import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';

const API_BASE = API_BASE;
const ADMIN_SECRET = 'warden-e2e-magic-key';
const ADMIN_USER = 'admin';
const ADMIN_PASS = 'password123!';

const VIEWER_USER = 'bob';
const VIEWER_PASS = 'viewer1234!';

const SV_USER = 'carol';
const SV_PASS = 'viewer1234!';

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
 * Helper: Get user ID by listing users.
 */
async function getUserId(
    request: import('@playwright/test').APIRequestContext,
    adminToken: string,
    username: string,
): Promise<number> {
    const res = await request.get(`${API_BASE}/api/users`, {
        headers: { Cookie: `auth_token=${adminToken}` },
    });
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    const user = body.users.find((u: { username: string }) => u.username === username);
    expect(user).toBeTruthy();
    return user.id;
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

/**
 * Helper: Create a group, configure a status page, and optionally assign it to a user.
 */
async function createAndAssignStatusPage(
    request: import('@playwright/test').APIRequestContext,
    adminToken: string,
    userId: number | null,
    slug: string,
    title: string,
    isPublic: boolean,
) {
    // Create a group for the page
    const groupRes = await request.post(`${API_BASE}/api/groups`, {
        headers: {
            'Content-Type': 'application/json',
            Cookie: `auth_token=${adminToken}`,
        },
        data: { name: `${title} Group` },
    });
    expect(groupRes.status()).toBe(201);
    const group = await groupRes.json();

    // Enable and configure the status page
    await request.patch(`${API_BASE}/api/status-pages/${slug}`, {
        headers: {
            'Content-Type': 'application/json',
            Cookie: `auth_token=${adminToken}`,
        },
        data: {
            public: isPublic,
            enabled: true,
            title,
            groupId: group.id,
        },
    });

    if (userId !== null) {
        // Get the page ID
        const pagesRes = await request.get(`${API_BASE}/api/status-pages`, {
            headers: { Cookie: `auth_token=${adminToken}` },
        });
        const pagesBody = await pagesRes.json();
        const page = pagesBody.pages.find((p: { slug: string }) => p.slug === slug);
        expect(page).toBeTruthy();
        expect(page.id).toBeGreaterThan(0);

        // Assign to user
        await request.put(`${API_BASE}/api/users/${userId}/status-pages`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${adminToken}`,
            },
            data: { statusPageIds: [page.id] },
        });
    }
}

// ─────────────────────────────────────────────────────────────────────────────
// Test Suite: RBAC Roles - Comprehensive UI + API permission tests
// ─────────────────────────────────────────────────────────────────────────────

test.describe('RBAC Roles - Viewer, Status Viewer, and Admin Permissions', () => {
    test.describe.configure({ mode: 'serial' });

    let adminToken: string;

    // ─── Setup: fresh DB + admin + viewer + status_viewer users ──────────
    test('Setup: reset DB, create admin, viewer, status_viewer, and assign status page', async ({ request }) => {
        adminToken = await resetAndSetupAdmin(request);

        // Create viewer user (bob)
        await createUser(request, adminToken, VIEWER_USER, VIEWER_PASS, 'viewer');

        // Create status_viewer user (carol)
        await createUser(request, adminToken, SV_USER, SV_PASS, 'status_viewer');

        // Create a group + monitor so viewer has something to browse
        const groupRes = await request.post(`${API_BASE}/api/groups`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${adminToken}`,
            },
            data: { name: 'Test Group' },
        });
        expect(groupRes.status()).toBe(201);

        const monitorRes = await request.post(`${API_BASE}/api/monitors`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${adminToken}`,
            },
            data: {
                name: 'Test Monitor',
                url: 'https://example.com',
                groupId: 'g-test-group',
                interval: 30,
            },
        });
        expect(monitorRes.status()).toBe(201);

        // Create and assign a status page to the status_viewer user
        const svUserId = await getUserId(request, adminToken, SV_USER);
        await createAndAssignStatusPage(request, adminToken, svUserId, 'carol-page', 'Carol Page', false);

        // Also create an unassigned private page for testing denied access
        await createAndAssignStatusPage(request, adminToken, null, 'secret-page', 'Secret Page', false);
    });

    // ─────────────────────────────────────────────────────────────────────
    // VIEWER ROLE TESTS (bob / viewer1234!)
    // ─────────────────────────────────────────────────────────────────────

    test('Viewer can login and sees dashboard', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });

        // Fill credentials manually (LoginPage.login expects /dashboard redirect which works for viewer)
        await page.getByLabel('Username').fill(VIEWER_USER);
        await page.getByLabel('Password').fill(VIEWER_PASS);
        await page.getByRole('button', { name: 'Sign In' }).click();

        // Viewer should land on /dashboard
        await expect(page).toHaveURL(/.*dashboard/, { timeout: 15000 });

        // Should see the dashboard content
        await expect(page.getByTestId('loading-spinner')).toHaveCount(0, { timeout: 15000 });
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 15000 });
    });

    test('Viewer can browse monitors and reach the full monitor view', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });
        await page.getByLabel('Username').fill(VIEWER_USER);
        await page.getByLabel('Password').fill(VIEWER_PASS);
        await page.getByRole('button', { name: 'Sign In' }).click();
        await expect(page).toHaveURL(/.*dashboard/, { timeout: 15000 });
        await expect(page.getByTestId('loading-spinner')).toHaveCount(0, { timeout: 15000 });
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 15000 });

        // Navigate to the group containing the monitor
        await page.getByText('Test Group').first().click();
        await expect(page).toHaveURL(/.*groups\//, { timeout: 15000 });

        // Click on the monitor to open details sheet
        await page.getByText('Test Monitor').first().click();

        // Wait for sheet to open
        await expect(page.locator('[data-state="open"].fixed.inset-0')).toBeVisible({ timeout: 10000 });

        // Metrics is the only tab a viewer gets — Settings is editor-only, and the
        // activity log moved out of the sheet onto the dedicated monitor page.
        await expect(page.getByRole('tab', { name: 'Metrics' })).toBeVisible({ timeout: 10000 });

        // Follow "Open full view" and check the viewer can read the incident rollups there
        await page.getByTestId('monitor-open-full-view').click();
        await expect(page).toHaveURL(/.*monitors\//, { timeout: 15000 });
        await expect(page.getByRole('heading', { name: /Incidents/ })).toBeVisible({ timeout: 15000 });
    });

    test('Viewer does NOT see Settings tab in monitor details sheet', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });
        await page.getByLabel('Username').fill(VIEWER_USER);
        await page.getByLabel('Password').fill(VIEWER_PASS);
        await page.getByRole('button', { name: 'Sign In' }).click();
        await expect(page).toHaveURL(/.*dashboard/, { timeout: 15000 });
        await expect(page.getByTestId('loading-spinner')).toHaveCount(0, { timeout: 15000 });
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 15000 });

        // Navigate to group
        await page.getByText('Test Group').first().click();
        await expect(page).toHaveURL(/.*groups\//, { timeout: 15000 });

        // Click on monitor
        await page.getByText('Test Monitor').first().click();
        await expect(page.locator('[data-state="open"].fixed.inset-0')).toBeVisible({ timeout: 10000 });

        // Settings tab should NOT be visible (only editors+ see it)
        await expect(page.getByTestId('monitor-settings-tab')).toHaveCount(0);
    });

    test('Viewer sidebar Settings shows only "Settings" (no sub-items)', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });
        await page.getByLabel('Username').fill(VIEWER_USER);
        await page.getByLabel('Password').fill(VIEWER_PASS);
        await page.getByRole('button', { name: 'Sign In' }).click();
        await expect(page).toHaveURL(/.*dashboard/, { timeout: 15000 });
        await expect(page.getByTestId('loading-spinner')).toHaveCount(0, { timeout: 15000 });
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 15000 });

        // For viewer with only 1 sub-item (General), the sidebar renders Settings as a direct link
        // (no collapsible sub-items). Verify Settings link exists.
        const settingsLink = page.locator('a').filter({ hasText: 'Settings' });
        await expect(settingsLink.first()).toBeVisible({ timeout: 10000 });

        // There should be no sub-items like Notifications, Security, System, Users in the sidebar
        // The sidebar SidebarMenuSub for settings should not exist (it renders as direct link)
        const sidebar = page.locator('[data-sidebar="sidebar"]');
        await expect(sidebar.getByText('Notifications')).toHaveCount(0);
        await expect(sidebar.getByText('Security')).toHaveCount(0);
        await expect(sidebar.getByText('System')).toHaveCount(0);
        await expect(sidebar.getByText('Users')).toHaveCount(0);
    });

    test('Viewer Settings page only shows General tab', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });
        await page.getByLabel('Username').fill(VIEWER_USER);
        await page.getByLabel('Password').fill(VIEWER_PASS);
        await page.getByRole('button', { name: 'Sign In' }).click();
        await expect(page).toHaveURL(/.*dashboard/, { timeout: 15000 });
        await expect(page.getByTestId('loading-spinner')).toHaveCount(0, { timeout: 15000 });
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 15000 });

        // Navigate to settings
        await page.goto('/settings');
        await expect(page.getByText('Settings').first()).toBeVisible({ timeout: 10000 });

        // Verify only General tab is visible in the tablist
        const tabsList = page.locator('[role="tablist"]');
        await expect(tabsList.getByText('General')).toBeVisible({ timeout: 10000 });

        // Other tabs should NOT be present
        await expect(tabsList.getByText('Notifications')).toHaveCount(0);
        await expect(tabsList.getByText('Security')).toHaveCount(0);
        await expect(tabsList.getByText('System')).toHaveCount(0);
        await expect(tabsList.getByText('Users')).toHaveCount(0);
    });

    test('Viewer cannot toggle status page switches (they are disabled)', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });
        await page.getByLabel('Username').fill(VIEWER_USER);
        await page.getByLabel('Password').fill(VIEWER_PASS);
        await page.getByRole('button', { name: 'Sign In' }).click();
        await expect(page).toHaveURL(/.*dashboard/, { timeout: 15000 });
        await expect(page.getByTestId('loading-spinner')).toHaveCount(0, { timeout: 15000 });
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 15000 });

        // Navigate to Status Pages
        await page.goto('/status-pages');
        await page.waitForLoadState('networkidle');
        await expect(page.getByRole('heading', { name: 'Status Pages' })).toBeVisible({ timeout: 10000 });

        // The enabled toggle for the "all" status page should be disabled for viewers
        const enabledToggle = page.getByTestId('status-page-enabled-toggle-all');
        await expect(enabledToggle).toBeVisible({ timeout: 10000 });
        await expect(enabledToggle).toBeDisabled();

        // The public toggle should also be disabled
        const publicToggle = page.getByTestId('status-page-public-toggle-all');
        await expect(publicToggle).toBeVisible({ timeout: 10000 });
        await expect(publicToggle).toBeDisabled();
    });

    test('Viewer does not see config gear icon on status pages', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });
        await page.getByLabel('Username').fill(VIEWER_USER);
        await page.getByLabel('Password').fill(VIEWER_PASS);
        await page.getByRole('button', { name: 'Sign In' }).click();
        await expect(page).toHaveURL(/.*dashboard/, { timeout: 15000 });
        await expect(page.getByTestId('loading-spinner')).toHaveCount(0, { timeout: 15000 });
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 15000 });

        // Navigate to Status Pages
        await page.goto('/status-pages');
        await page.waitForLoadState('networkidle');
        await expect(page.getByRole('heading', { name: 'Status Pages' })).toBeVisible({ timeout: 10000 });

        // Config gear icon should NOT be visible (only canEdit users see it)
        await expect(page.getByTestId('status-page-config-all')).toHaveCount(0);
    });

    test('Viewer API: can GET /api/overview, /api/uptime, /api/settings, /api/status-pages', async ({ request }) => {
        const viewerToken = await loginAsUser(request, VIEWER_USER, VIEWER_PASS);

        // Viewer CAN read overview
        const overviewRes = await request.get(`${API_BASE}/api/overview`, {
            headers: { Cookie: `auth_token=${viewerToken}` },
        });
        expect(overviewRes.ok()).toBeTruthy();

        // Viewer CAN read uptime
        const uptimeRes = await request.get(`${API_BASE}/api/uptime`, {
            headers: { Cookie: `auth_token=${viewerToken}` },
        });
        expect(uptimeRes.ok()).toBeTruthy();

        // Viewer CAN read settings
        const settingsRes = await request.get(`${API_BASE}/api/settings`, {
            headers: { Cookie: `auth_token=${viewerToken}` },
        });
        expect(settingsRes.ok()).toBeTruthy();

        // Viewer CAN read status pages
        const statusPagesRes = await request.get(`${API_BASE}/api/status-pages`, {
            headers: { Cookie: `auth_token=${viewerToken}` },
        });
        expect(statusPagesRes.ok()).toBeTruthy();
    });

    test('Viewer API: cannot PATCH /api/settings, POST monitors/groups, DELETE monitors, PATCH status-pages', async ({ request }) => {
        const viewerToken = await loginAsUser(request, VIEWER_USER, VIEWER_PASS);

        // Viewer CANNOT update settings
        const settingsRes = await request.patch(`${API_BASE}/api/settings`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${viewerToken}`,
            },
            data: { latency_threshold: '500' },
        });
        expect(settingsRes.status()).toBe(403);

        // Viewer CANNOT create monitors
        const monitorRes = await request.post(`${API_BASE}/api/monitors`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${viewerToken}`,
            },
            data: {
                name: 'Unauthorized Monitor',
                url: 'https://example.com',
                groupId: 'g-test-group',
                interval: 30,
            },
        });
        expect(monitorRes.status()).toBe(403);

        // Viewer CANNOT create groups
        const groupRes = await request.post(`${API_BASE}/api/groups`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${viewerToken}`,
            },
            data: { name: 'Unauthorized Group' },
        });
        expect(groupRes.status()).toBe(403);

        // Viewer CANNOT delete monitors
        const deleteRes = await request.delete(`${API_BASE}/api/monitors/m-test-monitor`, {
            headers: { Cookie: `auth_token=${viewerToken}` },
        });
        expect(deleteRes.status()).toBe(403);

        // Viewer CANNOT update status pages
        const statusPageRes = await request.patch(`${API_BASE}/api/status-pages/all`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${viewerToken}`,
            },
            data: { enabled: true, public: true, title: 'Hacked' },
        });
        expect(statusPageRes.status()).toBe(403);
    });

    // ─────────────────────────────────────────────────────────────────────
    // STATUS_VIEWER ROLE TESTS (carol / viewer1234!)
    // ─────────────────────────────────────────────────────────────────────

    test('Status viewer login redirects to /my-pages (NOT /dashboard)', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });

        // Fill credentials manually (LoginPage.login expects /dashboard redirect)
        await page.getByLabel('Username').fill(SV_USER);
        await page.getByLabel('Password').fill(SV_PASS);
        await page.getByRole('button', { name: 'Sign In' }).click();

        // Should redirect to /my-pages
        await expect(page).toHaveURL(/.*my-pages/, { timeout: 15000 });

        // Should see "My Status Pages" heading
        await expect(page.getByText('My Status Pages')).toBeVisible({ timeout: 10000 });
    });

    test('Status viewer sees assigned pages on /my-pages', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });
        await page.getByLabel('Username').fill(SV_USER);
        await page.getByLabel('Password').fill(SV_PASS);
        await page.getByRole('button', { name: 'Sign In' }).click();
        await expect(page).toHaveURL(/.*my-pages/, { timeout: 15000 });

        // Should see the assigned page
        await expect(page.getByText('Carol Page')).toBeVisible({ timeout: 10000 });
    });

    test('Status viewer is redirected from /dashboard to /my-pages', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });
        await page.getByLabel('Username').fill(SV_USER);
        await page.getByLabel('Password').fill(SV_PASS);
        await page.getByRole('button', { name: 'Sign In' }).click();
        await expect(page).toHaveURL(/.*my-pages/, { timeout: 15000 });

        // Try navigating to /dashboard
        await page.goto('/dashboard');

        // Should be redirected back to /my-pages
        await expect(page).toHaveURL(/.*my-pages/, { timeout: 15000 });
    });

    test('Status viewer API: blocked from /api/overview, /api/uptime', async ({ request }) => {
        const svToken = await loginAsUser(request, SV_USER, SV_PASS);

        // Blocked from /api/overview
        const overviewRes = await request.get(`${API_BASE}/api/overview`, {
            headers: { Cookie: `auth_token=${svToken}` },
        });
        expect(overviewRes.status()).toBe(403);

        // Blocked from /api/uptime
        const uptimeRes = await request.get(`${API_BASE}/api/uptime`, {
            headers: { Cookie: `auth_token=${svToken}` },
        });
        expect(uptimeRes.status()).toBe(403);
    });

    test('Status viewer API: can access /api/my/status-pages, /api/auth/me, assigned page via /api/s/{slug}', async ({ request }) => {
        const svToken = await loginAsUser(request, SV_USER, SV_PASS);

        // Can access /api/my/status-pages
        const myPagesRes = await request.get(`${API_BASE}/api/my/status-pages`, {
            headers: { Cookie: `auth_token=${svToken}` },
        });
        expect(myPagesRes.ok()).toBeTruthy();
        const myPagesBody = await myPagesRes.json();
        expect(myPagesBody.pages).toBeDefined();
        expect(myPagesBody.pages.length).toBeGreaterThanOrEqual(1);
        expect(myPagesBody.pages[0].slug).toBe('carol-page');

        // Can access /api/auth/me
        const meRes = await request.get(`${API_BASE}/api/auth/me`, {
            headers: { Cookie: `auth_token=${svToken}` },
        });
        expect(meRes.ok()).toBeTruthy();
        const meBody = await meRes.json();
        expect(meBody.user.role).toBe('status_viewer');

        // Can access assigned page via /api/s/{slug}
        const pageRes = await request.get(`${API_BASE}/api/s/carol-page`, {
            headers: { Cookie: `auth_token=${svToken}` },
        });
        expect(pageRes.ok()).toBeTruthy();
        const pageBody = await pageRes.json();
        expect(pageBody.title).toBe('Carol Page');
    });

    test('Status viewer API: denied unassigned private page', async ({ request }) => {
        const svToken = await loginAsUser(request, SV_USER, SV_PASS);

        // Should be denied access to unassigned private page
        const res = await request.get(`${API_BASE}/api/s/secret-page`, {
            headers: { Cookie: `auth_token=${svToken}` },
        });
        expect(res.status()).toBe(403);
    });

    // ─────────────────────────────────────────────────────────────────────
    // ADMIN ROLE TESTS
    // ─────────────────────────────────────────────────────────────────────

    test('Admin can see all Settings tabs (General, Notifications, Security, System, Users)', async ({ page }) => {
        // Clear cookies from previous status_viewer sessions
        await page.context().clearCookies();

        const login = new LoginPage(page);
        const dashboard = new DashboardPage(page);

        await dashboard.goto();
        await page.waitForLoadState('networkidle');

        if (page.url().includes('/login')) {
            await login.login(ADMIN_USER, ADMIN_PASS);
        }

        await dashboard.waitForLoad();

        // Navigate to Settings
        await page.goto('/settings');
        await expect(page.getByText('Settings').first()).toBeVisible({ timeout: 10000 });

        // Verify all tabs are visible
        const tabsList = page.locator('[role="tablist"]');
        await expect(tabsList.getByText('General')).toBeVisible({ timeout: 10000 });
        await expect(tabsList.getByText('Notifications')).toBeVisible({ timeout: 10000 });
        await expect(tabsList.getByText('Security')).toBeVisible({ timeout: 10000 });
        await expect(tabsList.getByText('System')).toBeVisible({ timeout: 10000 });
        await expect(tabsList.getByText('Users')).toBeVisible({ timeout: 10000 });
    });

    test('Admin sidebar Settings shows all sub-items', async ({ page }) => {
        await page.context().clearCookies();

        const login = new LoginPage(page);
        const dashboard = new DashboardPage(page);

        await dashboard.goto();
        await page.waitForLoadState('networkidle');

        if (page.url().includes('/login')) {
            await login.login(ADMIN_USER, ADMIN_PASS);
        }

        await dashboard.waitForLoad();

        // Expand Settings collapsible in sidebar
        const sidebar = page.locator('[data-sidebar="sidebar"]');
        await sidebar.getByText('Settings').first().click();

        // Verify all sub-items are visible
        await expect(sidebar.getByText('General')).toBeVisible({ timeout: 10000 });
        await expect(sidebar.getByText('Notifications')).toBeVisible({ timeout: 10000 });
        await expect(sidebar.getByText('Security')).toBeVisible({ timeout: 10000 });
        await expect(sidebar.getByText('System')).toBeVisible({ timeout: 10000 });
        await expect(sidebar.getByText('Users')).toBeVisible({ timeout: 10000 });
    });

    test('Admin can toggle status page switches', async ({ page }) => {
        await page.context().clearCookies();

        const login = new LoginPage(page);
        const dashboard = new DashboardPage(page);

        await dashboard.goto();
        await page.waitForLoadState('networkidle');

        if (page.url().includes('/login')) {
            await login.login(ADMIN_USER, ADMIN_PASS);
        }

        await dashboard.waitForLoad();

        // Navigate to Status Pages
        await page.goto('/status-pages');
        await page.waitForLoadState('networkidle');
        await expect(page.getByRole('heading', { name: 'Status Pages' })).toBeVisible({ timeout: 10000 });

        // Wait for rows to load
        await expect(page.getByTestId('status-page-row-all')).toBeVisible({ timeout: 10000 });

        // Verify the enabled toggle is NOT disabled (admin can interact)
        const enabledToggle = page.getByTestId('status-page-enabled-toggle-all');
        await expect(enabledToggle).toBeVisible({ timeout: 10000 });
        await expect(enabledToggle).toBeEnabled();

        // Verify the config gear icon is visible
        await expect(page.getByTestId('status-page-config-all')).toBeVisible({ timeout: 10000 });
    });

    // ─── Cleanup ────────────────────────────────────────────────────────
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
