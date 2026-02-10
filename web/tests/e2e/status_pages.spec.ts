import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';
import { StatusPagesPage } from '../pages/StatusPagesPage';

test.describe('Status Pages - Enabled/Public Controls', () => {

    test.beforeEach(async ({ page }) => {
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);

        await dashboard.goto();
        await page.waitForLoadState('networkidle');

        if (page.url().includes('/login')) {
            await login.login();
        }

        await dashboard.waitForLoad();

        // Ensure status page starts in a clean disabled+private state
        const statusPages = new StatusPagesPage(page);
        await statusPages.resetToDefaults();
    });

    test.afterEach(async ({ page }) => {
        // Always reset after each test to avoid state leaking
        const statusPages = new StatusPagesPage(page);
        await statusPages.resetToDefaults();
    });

    test('Default Global Status page starts as Disabled', async ({ page }) => {
        const statusPages = new StatusPagesPage(page);
        await statusPages.navigateViaSidebar();
        await statusPages.waitForLoad();

        // Global page should exist and show "Disabled" badge by default
        await statusPages.expectBadge('all', 'Disabled');

        // Visit link should NOT be visible when disabled
        await statusPages.expectVisitLinkHidden('all');

        // Public toggle should be disabled when page is disabled
        await statusPages.expectPublicToggleDisabled('all');
    });

    test('Enable Global Status page and verify Public badge', async ({ page }) => {
        const statusPages = new StatusPagesPage(page);
        await statusPages.navigateViaSidebar();
        await statusPages.waitForLoad();

        // Enable the page (toggleEnabled waits for toast internally)
        await statusPages.toggleEnabled('all');

        // Badge should now show "Private" (enabled but not yet public)
        await statusPages.expectBadge('all', 'Private');

        // Visit link should now be visible
        await statusPages.expectVisitLinkVisible('all');

        // Public toggle should be enabled
        await statusPages.expectPublicToggleEnabled('all');

        // Make it public
        await statusPages.togglePublic('all');

        // Badge should now show "Public"
        await statusPages.expectBadge('all', 'Public');
    });

    test('Disable an enabled page returns to Disabled state', async ({ page }) => {
        const statusPages = new StatusPagesPage(page);
        await statusPages.navigateViaSidebar();
        await statusPages.waitForLoad();

        // Enable the page first
        await statusPages.toggleEnabled('all');
        await statusPages.expectBadge('all', 'Private');

        // Now disable it
        await statusPages.toggleEnabled('all');

        // Badge should be "Disabled"
        await statusPages.expectBadge('all', 'Disabled');

        // Visit link should be hidden
        await statusPages.expectVisitLinkHidden('all');

        // Public toggle should be disabled
        await statusPages.expectPublicToggleDisabled('all');
    });

    test('Public toggle is independent from Enabled toggle', async ({ page }) => {
        const statusPages = new StatusPagesPage(page);
        await statusPages.navigateViaSidebar();
        await statusPages.waitForLoad();

        // Enable the page
        await statusPages.toggleEnabled('all');

        // It should be Private (enabled, not public)
        await statusPages.expectBadge('all', 'Private');

        // Make it public
        await statusPages.togglePublic('all');
        await statusPages.expectBadge('all', 'Public');

        // Make it private again (toggle public off, keep enabled)
        await statusPages.togglePublic('all');
        await statusPages.expectBadge('all', 'Private');

        // Page should still be enabled — visit link still visible
        await statusPages.expectVisitLinkVisible('all');
    });

    test('Enabled + Public page is accessible at /status/all', async ({ page }) => {
        const statusPages = new StatusPagesPage(page);
        await statusPages.navigateViaSidebar();
        await statusPages.waitForLoad();

        // Enable + make public
        await statusPages.toggleEnabled('all');
        await statusPages.togglePublic('all');
        await statusPages.expectBadge('all', 'Public');

        // Visit the public status page
        await page.goto('/status/all');
        await page.waitForLoadState('networkidle');

        // Should NOT see error page
        await expect(page.getByText('Status Page Unavailable')).toHaveCount(0, { timeout: 10000 });

        // Should see the page title "Global Status"
        await expect(page.getByText('Global Status')).toBeVisible({ timeout: 10000 });
    });

    test('Disabled page returns 404 at /status/all', async ({ page, context }) => {
        const statusPages = new StatusPagesPage(page);
        await statusPages.navigateViaSidebar();
        await statusPages.waitForLoad();

        // Default is disabled — verify badge
        await statusPages.expectBadge('all', 'Disabled');

        // Open a new page in the same context to check the public endpoint
        const publicPage = await context.newPage();
        await publicPage.goto('/status/all');
        await publicPage.waitForLoadState('networkidle');

        // Should see error/unavailable since page is disabled
        await expect(publicPage.getByText('Status Page Unavailable')).toBeVisible({ timeout: 10000 });

        await publicPage.close();
    });

    test('Private page requires authentication', async ({ page }) => {
        const statusPages = new StatusPagesPage(page);
        await statusPages.navigateViaSidebar();
        await statusPages.waitForLoad();

        // Enable but keep private (don't toggle public)
        await statusPages.toggleEnabled('all');
        await statusPages.expectBadge('all', 'Private');

        // Authenticated user (current page) should see it
        await page.goto('/status/all');
        await page.waitForLoadState('networkidle');
        await expect(page.getByText('Status Page Unavailable')).toHaveCount(0, { timeout: 10000 });
        await expect(page.getByText('Global Status')).toBeVisible({ timeout: 10000 });

        // Unauthenticated user (fresh browser context) should NOT see it
        const freshContext = await page.context().browser()!.newContext();
        const publicPage = await freshContext.newPage();
        await publicPage.goto(`${page.url().split('/status')[0]}/status/all`);
        await publicPage.waitForLoadState('networkidle');

        // Should see error since user is not authenticated
        await expect(publicPage.getByText('Status Page Unavailable')).toBeVisible({ timeout: 10000 });

        await publicPage.close();
        await freshContext.close();
    });

    test('State persists after page refresh', async ({ page }) => {
        const statusPages = new StatusPagesPage(page);
        await statusPages.navigateViaSidebar();
        await statusPages.waitForLoad();

        // Enable + make public
        await statusPages.toggleEnabled('all');
        await statusPages.togglePublic('all');
        await statusPages.expectBadge('all', 'Public');

        // Refresh
        await page.reload();
        await statusPages.waitForLoad();

        // State should persist
        await statusPages.expectBadge('all', 'Public');
        await statusPages.expectVisitLinkVisible('all');
    });

});
