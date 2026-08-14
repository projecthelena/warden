import { API_BASE } from '../apiBase';
import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';

const API_BASE = API_BASE;
const ADMIN_SECRET = 'warden-e2e-magic-key';
const ADMIN_USER = 'admin';
const ADMIN_PASS = 'password123!';

const SV_USER = 'linda';
const SV_PASS = 'viewer1234!';

/**
 * Helper: Reset DB, create admin, login as admin, return auth token.
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
 * Helper: Create a status_viewer user via admin API.
 */
async function createStatusViewerUser(
    request: import('@playwright/test').APIRequestContext,
    adminToken: string,
) {
    const res = await request.post(`${API_BASE}/api/users`, {
        headers: {
            'Content-Type': 'application/json',
            Cookie: `auth_token=${adminToken}`,
        },
        data: { username: SV_USER, password: SV_PASS, role: 'status_viewer' },
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
 * Helper: Create a group, configure a status page, and assign it to a user.
 */
async function createAndAssignStatusPage(
    request: import('@playwright/test').APIRequestContext,
    adminToken: string,
    userId: number,
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

    return page.id;
}

// ─────────────────────────────────────────────────────────────────────────────
// Test Suite: Status Viewer Role
// ─────────────────────────────────────────────────────────────────────────────

test.describe('Status Viewer Role', () => {
    test.describe.configure({ mode: 'serial' });

    let adminToken: string;

    // ─── Setup: fresh DB + admin + status_viewer user + assigned page ────────
    test('Setup: create admin, status_viewer user, and assign a status page', async ({ request }) => {
        adminToken = await resetAndSetupAdmin(request);
        await createStatusViewerUser(request, adminToken);

        const userId = await getUserId(request, adminToken, SV_USER);

        // Create and assign a private status page
        await createAndAssignStatusPage(request, adminToken, userId, 'sv-test-page', 'SV Test Page', false);
    });

    // ─── Test 1: API - /api/my/status-pages returns assigned pages ──────────
    test('GET /api/my/status-pages returns assigned pages for status_viewer', async ({ request }) => {
        // Login as status_viewer
        const loginRes = await request.post(`${API_BASE}/api/auth/login`, {
            data: { username: SV_USER, password: SV_PASS },
        });
        expect(loginRes.ok()).toBeTruthy();
        const loginBody = await loginRes.json();
        expect(loginBody.user.role).toBe('status_viewer');

        const cookies = loginRes.headers()['set-cookie'] || '';
        const match = cookies.match(/auth_token=([^;]+)/);
        const svToken = match![1];

        // Fetch my status pages
        const res = await request.get(`${API_BASE}/api/my/status-pages`, {
            headers: { Cookie: `auth_token=${svToken}` },
        });
        expect(res.ok()).toBeTruthy();
        const body = await res.json();
        expect(body.pages).toBeDefined();
        expect(body.pages.length).toBe(1);
        expect(body.pages[0].slug).toBe('sv-test-page');
        expect(body.pages[0].title).toBe('SV Test Page');
    });

    // ─── Test 2: API - status_viewer blocked from dashboard endpoints ───────
    test('status_viewer is blocked from dashboard API endpoints', async ({ request }) => {
        const loginRes = await request.post(`${API_BASE}/api/auth/login`, {
            data: { username: SV_USER, password: SV_PASS },
        });
        const cookies = loginRes.headers()['set-cookie'] || '';
        const match = cookies.match(/auth_token=([^;]+)/);
        const svToken = match![1];

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

        // Blocked from /api/users
        const usersRes = await request.get(`${API_BASE}/api/users`, {
            headers: { Cookie: `auth_token=${svToken}` },
        });
        expect(usersRes.status()).toBe(403);

        // Can access /api/auth/me
        const meRes = await request.get(`${API_BASE}/api/auth/me`, {
            headers: { Cookie: `auth_token=${svToken}` },
        });
        expect(meRes.ok()).toBeTruthy();

        // Can access /api/my/status-pages
        const myPagesRes = await request.get(`${API_BASE}/api/my/status-pages`, {
            headers: { Cookie: `auth_token=${svToken}` },
        });
        expect(myPagesRes.ok()).toBeTruthy();
    });

    // ─── Test 3: API - status_viewer can view assigned private page ─────────
    test('status_viewer can view assigned private status page', async ({ request }) => {
        const loginRes = await request.post(`${API_BASE}/api/auth/login`, {
            data: { username: SV_USER, password: SV_PASS },
        });
        const cookies = loginRes.headers()['set-cookie'] || '';
        const match = cookies.match(/auth_token=([^;]+)/);
        const svToken = match![1];

        const res = await request.get(`${API_BASE}/api/s/sv-test-page`, {
            headers: { Cookie: `auth_token=${svToken}` },
        });
        expect(res.ok()).toBeTruthy();
        const body = await res.json();
        expect(body.title).toBe('SV Test Page');
    });

    // ─── Test 4: API - status_viewer denied unassigned private page ─────────
    test('status_viewer is denied access to unassigned private page', async ({ request }) => {
        // Create another private page without assigning it
        // Re-login as admin
        const adminLoginRes = await request.post(`${API_BASE}/api/auth/login`, {
            data: { username: ADMIN_USER, password: ADMIN_PASS },
        });
        const adminCookies = adminLoginRes.headers()['set-cookie'] || '';
        const adminMatch = adminCookies.match(/auth_token=([^;]+)/);
        const freshAdminToken = adminMatch![1];

        // Create group + page
        const groupRes = await request.post(`${API_BASE}/api/groups`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${freshAdminToken}`,
            },
            data: { name: 'Unassigned Group' },
        });
        expect(groupRes.status()).toBe(201);
        const group = await groupRes.json();

        await request.patch(`${API_BASE}/api/status-pages/unassigned-page`, {
            headers: {
                'Content-Type': 'application/json',
                Cookie: `auth_token=${freshAdminToken}`,
            },
            data: {
                public: false,
                enabled: true,
                title: 'Unassigned Page',
                groupId: group.id,
            },
        });

        // Login as status_viewer and try to access
        const svLoginRes = await request.post(`${API_BASE}/api/auth/login`, {
            data: { username: SV_USER, password: SV_PASS },
        });
        const svCookies = svLoginRes.headers()['set-cookie'] || '';
        const svMatch = svCookies.match(/auth_token=([^;]+)/);
        const svToken = svMatch![1];

        const res = await request.get(`${API_BASE}/api/s/unassigned-page`, {
            headers: { Cookie: `auth_token=${svToken}` },
        });
        expect(res.status()).toBe(403);
    });

    // ─── Test 5: UI - status_viewer login redirects to /my-pages ────────────
    test('status_viewer login redirects to /my-pages', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });

        // Fill credentials and submit manually (LoginPage.login expects /dashboard redirect)
        await page.getByLabel('Username').fill(SV_USER);
        await page.getByLabel('Password').fill(SV_PASS);
        await page.getByRole('button', { name: 'Sign In' }).click();

        // Should redirect to /my-pages
        await expect(page).toHaveURL(/.*my-pages/, { timeout: 15000 });

        // Should see the "My Status Pages" heading
        await expect(page.getByText('My Status Pages')).toBeVisible({ timeout: 10000 });

        // Should see the assigned page
        await expect(page.getByText('SV Test Page')).toBeVisible({ timeout: 10000 });
    });

    // ─── Test 6: UI - status_viewer can click through to view status page ───
    test('status_viewer can navigate to assigned status page', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });
        await page.getByLabel('Username').fill(SV_USER);
        await page.getByLabel('Password').fill(SV_PASS);
        await page.getByRole('button', { name: 'Sign In' }).click();

        await expect(page).toHaveURL(/.*my-pages/, { timeout: 15000 });
        await expect(page.getByText('SV Test Page')).toBeVisible({ timeout: 10000 });

        // Click the page card
        await page.getByText('SV Test Page').click();

        // Should navigate to /status/sv-test-page
        await expect(page).toHaveURL(/.*status\/sv-test-page/, { timeout: 15000 });

        // Should see the status page title
        await expect(page.getByText('SV Test Page').first()).toBeVisible({ timeout: 10000 });
    });

    // ─── Test 7: UI - status_viewer redirected from /dashboard ──────────────
    test('status_viewer is redirected from /dashboard to /my-pages', async ({ page }) => {
        await page.goto('/login');
        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });
        await page.getByLabel('Username').fill(SV_USER);
        await page.getByLabel('Password').fill(SV_PASS);
        await page.getByRole('button', { name: 'Sign In' }).click();
        await expect(page).toHaveURL(/.*my-pages/, { timeout: 15000 });

        // Try navigating to dashboard
        await page.goto('/dashboard');

        // Should be redirected to /my-pages
        await expect(page).toHaveURL(/.*my-pages/, { timeout: 15000 });
    });

    // ─── Test 8: UI - admin can manage pages for status_viewer ──────────────
    test('Admin can see and use Manage Pages dialog for status_viewer', async ({ page }) => {
        const login = new LoginPage(page);
        const dashboard = new DashboardPage(page);

        // Explicitly log out and log in as admin (previous tests logged in as status_viewer)
        await page.goto('/login');
        await page.waitForLoadState('networkidle');

        // If already logged in (as status_viewer), we need to log out first
        if (!page.url().includes('/login')) {
            // Navigate to login by clearing cookies
            await page.context().clearCookies();
            await page.goto('/login');
        }

        await expect(page.getByTestId('login-header')).toBeVisible({ timeout: 15000 });
        await login.login(ADMIN_USER, ADMIN_PASS);
        await dashboard.waitForLoad();

        // Navigate to Settings > Users
        await page.goto('/settings?tab=users');
        await expect(page.getByText('User Management').first()).toBeVisible({ timeout: 15000 });

        // Find the status_viewer user row
        const usersTable = page.locator('table');
        await expect(usersTable).toBeVisible({ timeout: 10000 });
        const svRow = usersTable.locator('tr').filter({ hasText: SV_USER });
        await expect(svRow).toBeVisible();

        // Should show page count indicator
        await expect(svRow.getByText(/\d+ page/)).toBeVisible({ timeout: 5000 });

        // Open the actions dropdown
        await svRow.locator('button').last().click();

        // Click "Manage Pages"
        await page.getByText('Manage Pages').click();

        // Dialog should appear
        await expect(page.getByText('Manage Status Pages')).toBeVisible({ timeout: 5000 });
        await expect(page.getByText('Assign status pages to')).toBeVisible();

        // Should see at least one status page checkbox
        const checkboxes = page.locator('[role="checkbox"]');
        await expect(checkboxes.first()).toBeVisible({ timeout: 5000 });

        // Close dialog
        await page.getByRole('button', { name: 'Cancel' }).click();
        await expect(page.getByText('Manage Status Pages')).toBeHidden({ timeout: 5000 });
    });

    // ─── Test 9: API - GetAll includes id field ─────────────────────────────
    test('GET /api/status-pages includes id field in response', async ({ request }) => {
        const loginRes = await request.post(`${API_BASE}/api/auth/login`, {
            data: { username: ADMIN_USER, password: ADMIN_PASS },
        });
        const cookies = loginRes.headers()['set-cookie'] || '';
        const match = cookies.match(/auth_token=([^;]+)/);
        const token = match![1];

        const res = await request.get(`${API_BASE}/api/status-pages`, {
            headers: { Cookie: `auth_token=${token}` },
        });
        expect(res.ok()).toBeTruthy();
        const body = await res.json();
        expect(body.pages).toBeDefined();

        // At least one saved page should have a positive id
        const savedPages = body.pages.filter((p: { id: number }) => p.id > 0);
        expect(savedPages.length).toBeGreaterThan(0);
    });

    // ─── Test 10: API - login response includes full user fields ────────────
    test('Login response includes avatar, displayName, timezone fields', async ({ request }) => {
        const loginRes = await request.post(`${API_BASE}/api/auth/login`, {
            data: { username: ADMIN_USER, password: ADMIN_PASS },
        });
        expect(loginRes.ok()).toBeTruthy();

        const body = await loginRes.json();
        expect(body.user).toBeDefined();
        expect(body.user.role).toBe('admin');
        expect(body.user.username).toBe(ADMIN_USER);
        // These fields should now be present (audit fix)
        expect(body.user.displayName).toBeDefined();
        expect(body.user.timezone).toBeDefined();
        expect(body.user.avatar).toBeDefined();
    });

    // ─── Cleanup ────────────────────────────────────────────────────────────
    test('Cleanup: restore standard admin state', async ({ request }) => {
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
