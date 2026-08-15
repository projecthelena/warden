import { API_BASE } from '../apiBase';
import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';

const ADMIN_SECRET = 'warden-e2e-magic-key';
const ADMIN_USER = 'admin';
const ADMIN_PASS = 'password123!';

const TARGET_USER = 'bob';
const OLD_PASS = 'viewer1234!';
const NEW_PASS = 'BrandNew123!';

async function resetAndSetupAdmin(request: import('@playwright/test').APIRequestContext) {
    const reset = await request.post(`${API_BASE}/api/admin/reset`, { headers: { 'X-Admin-Secret': ADMIN_SECRET } });
    expect(reset.ok()).toBeTruthy();
    const setup = await request.post(`${API_BASE}/api/setup`, {
        headers: { 'X-Admin-Secret': ADMIN_SECRET },
        data: { username: ADMIN_USER, password: ADMIN_PASS, timezone: 'UTC' },
    });
    expect(setup.ok()).toBeTruthy();
    const login = await request.post(`${API_BASE}/api/auth/login`, { data: { username: ADMIN_USER, password: ADMIN_PASS } });
    expect(login.ok()).toBeTruthy();
    return (login.headers()['set-cookie'] || '').match(/auth_token=([^;]+)/)![1];
}

test('admin resets another user password from the UI', async ({ page, request }) => {
    const adminToken = await resetAndSetupAdmin(request);

    const created = await request.post(`${API_BASE}/api/users`, {
        headers: { 'Content-Type': 'application/json', Cookie: `auth_token=${adminToken}` },
        data: { username: TARGET_USER, password: OLD_PASS, role: 'viewer' },
    });
    expect(created.status()).toBe(201);

    // Sign in as admin and open the Users tab.
    const login = new LoginPage(page);
    const dashboard = new DashboardPage(page);
    await dashboard.goto();
    await page.waitForLoadState('networkidle');
    if (page.url().includes('/login')) {
        await login.login(ADMIN_USER, ADMIN_PASS);
    }
    await page.goto('/settings?tab=users');
    await expect(page.getByText('User Management').first()).toBeVisible({ timeout: 15000 });

    // Open bob's row menu and reset the password.
    const row = page.locator('tr').filter({ hasText: TARGET_USER });
    await row.getByRole('button').click();
    await page.getByTestId(`reset-password-${TARGET_USER}`).click();

    // A too-short password keeps submit disabled; a valid one enables it.
    await page.getByTestId('reset-password-input').fill('short');
    await expect(page.getByTestId('reset-password-submit')).toBeDisabled();
    await page.getByTestId('reset-password-input').fill(NEW_PASS);
    await expect(page.getByTestId('reset-password-submit')).toBeEnabled();
    await page.getByTestId('reset-password-submit').click();
    await expect(page.getByTestId('toast-title')).toContainText('Password Reset', { timeout: 10000 });

    // The new password works and the old one no longer does.
    const withNew = await request.post(`${API_BASE}/api/auth/login`, { data: { username: TARGET_USER, password: NEW_PASS } });
    expect(withNew.ok()).toBeTruthy();
    const withOld = await request.post(`${API_BASE}/api/auth/login`, { data: { username: TARGET_USER, password: OLD_PASS } });
    expect(withOld.status()).toBe(401);
});
