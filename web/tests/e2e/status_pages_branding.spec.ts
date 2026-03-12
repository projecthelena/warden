import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';
import { StatusPagesPage } from '../pages/StatusPagesPage';

// Tiny 1×1 transparent PNG encoded as a data URI — no external HTTP dependency.
const TINY_PNG_DATA_URI =
    'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==';

// Run tests serially — all modify the shared "all" status page.
test.describe.configure({ mode: 'serial' });

test.describe('Status Pages - Branding Config', () => {

    test.beforeEach(async ({ page }) => {
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);

        await dashboard.goto();
        await page.waitForLoadState('networkidle');

        if (page.url().includes('/login')) {
            await login.login();
        }

        await dashboard.waitForLoad();

        // Start each test with a clean slate.
        const statusPages = new StatusPagesPage(page);
        await statusPages.resetToDefaults();
    });

    test.afterEach(async ({ page }) => {
        const statusPages = new StatusPagesPage(page);
        await statusPages.resetToDefaults();
    });

    // ------------------------------------------------------------------
    // Dialog pre-population
    // ------------------------------------------------------------------

    test('Config dialog shows pre-configured values', async ({ page }) => {
        const statusPages = new StatusPagesPage(page);

        // Pre-configure all branding fields via API.
        await statusPages.configureViaAPI('all', {
            enabled: false,
            public: false,
            title: 'My Custom Title',
            description: 'A custom tagline',
            logoUrl: 'https://example.com/logo.png',
            faviconUrl: 'https://example.com/favicon.ico',
            accentColor: '#FF5500',
            theme: 'dark',
        });

        await statusPages.navigateViaSidebar();
        await statusPages.waitForLoad();
        await statusPages.openConfigDialog('all');

        const dialog = statusPages.getConfigDialog();
        await expect(dialog.getByLabel('Title')).toHaveValue('My Custom Title');
        await expect(dialog.getByLabel('Description')).toHaveValue('A custom tagline');
        await expect(dialog.locator('#logoUrl')).toHaveValue('https://example.com/logo.png');
        await expect(dialog.locator('#faviconUrl')).toHaveValue('https://example.com/favicon.ico');

        await statusPages.cancelConfigDialog();
    });

    // ------------------------------------------------------------------
    // Logo URL — set, persist, clear, persist clear (regression for the
    // "|| undefined" frontend bug where clearing was silently ignored).
    // ------------------------------------------------------------------

    test('Logo URL can be set via the config dialog and persists after re-open', async ({ page }) => {
        const statusPages = new StatusPagesPage(page);

        await statusPages.navigateViaSidebar();
        await statusPages.waitForLoad();

        // Open dialog and set the logo URL.
        await statusPages.openConfigDialog('all');
        await statusPages.fillConfigLogoUrl(TINY_PNG_DATA_URI);
        await statusPages.saveConfigDialog();

        // Re-open the dialog — the saved value must appear.
        await statusPages.openConfigDialog('all');
        await expect(statusPages.getConfigDialog().locator('#logoUrl')).toHaveValue(TINY_PNG_DATA_URI);
        await statusPages.cancelConfigDialog();
    });

    test('Logo URL can be cleared via the Remove button and stays cleared', async ({ page }) => {
        const statusPages = new StatusPagesPage(page);

        // Pre-set a logo URL via API.
        await statusPages.configureViaAPI('all', {
            enabled: false,
            public: false,
            title: 'Global Status',
            logoUrl: TINY_PNG_DATA_URI,
        });

        await statusPages.navigateViaSidebar();
        await statusPages.waitForLoad();

        // Open dialog, verify the URL is present, then clear it.
        await statusPages.openConfigDialog('all');
        await expect(statusPages.getConfigDialog().locator('#logoUrl')).not.toHaveValue('');
        await statusPages.clearConfigLogo();
        await expect(statusPages.getConfigDialog().locator('#logoUrl')).toHaveValue('');
        await statusPages.saveConfigDialog();

        // Re-open: field must still be empty (regression: old bug preserved the old URL).
        await statusPages.openConfigDialog('all');
        await expect(statusPages.getConfigDialog().locator('#logoUrl')).toHaveValue('');
        await statusPages.cancelConfigDialog();
    });

    // ------------------------------------------------------------------
    // Favicon URL — same set/clear round-trip as logo.
    // ------------------------------------------------------------------

    test('Favicon URL can be set via the config dialog and persists after re-open', async ({ page }) => {
        const statusPages = new StatusPagesPage(page);

        await statusPages.navigateViaSidebar();
        await statusPages.waitForLoad();

        await statusPages.openConfigDialog('all');
        await statusPages.fillConfigFaviconUrl('https://example.com/favicon.ico');
        await statusPages.saveConfigDialog();

        await statusPages.openConfigDialog('all');
        await expect(statusPages.getConfigDialog().locator('#faviconUrl')).toHaveValue('https://example.com/favicon.ico');
        await statusPages.cancelConfigDialog();
    });

    test('Favicon URL can be cleared via the Remove button and stays cleared', async ({ page }) => {
        const statusPages = new StatusPagesPage(page);

        // Pre-set a favicon URL via API.
        await statusPages.configureViaAPI('all', {
            enabled: false,
            public: false,
            title: 'Global Status',
            faviconUrl: 'https://example.com/favicon.ico',
        });

        await statusPages.navigateViaSidebar();
        await statusPages.waitForLoad();

        await statusPages.openConfigDialog('all');
        await expect(statusPages.getConfigDialog().locator('#faviconUrl')).not.toHaveValue('');
        await statusPages.clearConfigFavicon();
        await expect(statusPages.getConfigDialog().locator('#faviconUrl')).toHaveValue('');
        await statusPages.saveConfigDialog();

        // Re-open: still empty.
        await statusPages.openConfigDialog('all');
        await expect(statusPages.getConfigDialog().locator('#faviconUrl')).toHaveValue('');
        await statusPages.cancelConfigDialog();
    });

    // ------------------------------------------------------------------
    // Accent color + theme
    // ------------------------------------------------------------------

    test('Theme is saved via the config dialog', async ({ page }) => {
        const statusPages = new StatusPagesPage(page);

        await statusPages.navigateViaSidebar();
        await statusPages.waitForLoad();

        await statusPages.openConfigDialog('all');
        await statusPages.selectConfigTheme('dark');
        await statusPages.saveConfigDialog();

        // Verify theme via the public API (enable page first).
        await statusPages.configureViaAPI('all', { enabled: true, public: false, title: 'Global Status' });
        const listResp = await page.request.get('/api/status-pages');
        const listData = await listResp.json();
        const allPage = listData.pages?.find((p: { slug: string }) => p.slug === 'all');
        expect(allPage?.theme).toBe('dark');
    });

    // ------------------------------------------------------------------
    // Public page rendering
    // ------------------------------------------------------------------

    test('Logo renders on the public status page when set', async ({ page }) => {
        const statusPages = new StatusPagesPage(page);

        // Enable + make public + set a logo via API.
        await statusPages.configureViaAPI('all', {
            enabled: true,
            public: true,
            title: 'Global Status',
            logoUrl: TINY_PNG_DATA_URI,
        });

        await page.goto('/status/all');
        await page.waitForLoadState('networkidle');

        // The logo <img alt="Logo"> should be visible.
        await expect(page.getByRole('img', { name: 'Logo' })).toBeVisible({ timeout: 10000 });
    });

    test('Logo is absent from the public status page when not set', async ({ page }) => {
        const statusPages = new StatusPagesPage(page);

        // Enable + make public, no logo.
        await statusPages.configureViaAPI('all', {
            enabled: true,
            public: true,
            title: 'Global Status',
            logoUrl: '',
        });

        await page.goto('/status/all');
        await page.waitForLoadState('networkidle');

        // Page loads correctly.
        await expect(page.getByText('Global Status')).toBeVisible({ timeout: 10000 });

        // But the logo img must not be present.
        await expect(page.getByRole('img', { name: 'Logo' })).toHaveCount(0, { timeout: 5000 });
    });

    // ------------------------------------------------------------------
    // Description
    // ------------------------------------------------------------------

    test('Description is saved via the config dialog and shown on public page', async ({ page }) => {
        const statusPages = new StatusPagesPage(page);

        // Enable + public via API, then set description via dialog.
        await statusPages.configureViaAPI('all', {
            enabled: true,
            public: true,
            title: 'Global Status',
        });

        await statusPages.navigateViaSidebar();
        await statusPages.waitForLoad();

        await statusPages.openConfigDialog('all');
        await statusPages.getConfigDialog().getByLabel('Description').fill('My custom tagline');
        await statusPages.saveConfigDialog();

        // Navigate to public page and verify description text.
        await page.goto('/status/all');
        await page.waitForLoadState('networkidle');
        await expect(page.getByText('My custom tagline')).toBeVisible({ timeout: 10000 });
    });

});
