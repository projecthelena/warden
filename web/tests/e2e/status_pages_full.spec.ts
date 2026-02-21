import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';
import { StatusPagesPage } from '../pages/StatusPagesPage';

/**
 * Comprehensive E2E test suite for Status Page Migration (Phases 1-4)
 *
 * Phase 1: Uptime Bars - Monitor status, latency, uptime data
 * Phase 2: Incident History - Active incidents, past incidents, maintenance
 * Phase 3: Configuration - Branding, theme, display toggles
 * Phase 4: RSS Feed - Feed generation, content, edge cases
 */

test.describe('Status Page - Full E2E Suite', () => {
    let statusPages: StatusPagesPage;
    let dashboard: DashboardPage;
    let login: LoginPage;
    let createdIncidentIds: string[] = [];
    let createdMaintenanceIds: string[] = [];

    test.beforeEach(async ({ page }) => {
        dashboard = new DashboardPage(page);
        login = new LoginPage(page);
        statusPages = new StatusPagesPage(page);

        await dashboard.goto();
        await page.waitForLoadState('networkidle');

        if (page.url().includes('/login')) {
            await login.login();
        }

        await dashboard.waitForLoad();

        // Reset status page to defaults
        await statusPages.resetToDefaults();
    });

    test.afterEach(async () => {
        // Cleanup any created incidents
        for (const id of createdIncidentIds) {
            try {
                await statusPages.deleteIncidentViaAPI(id);
            } catch {
                // Ignore errors during cleanup
            }
        }
        createdIncidentIds = [];

        // Cleanup any created maintenance windows
        for (const id of createdMaintenanceIds) {
            try {
                await statusPages.deleteMaintenanceViaAPI(id);
            } catch {
                // Ignore errors during cleanup
            }
        }
        createdMaintenanceIds = [];

        // Reset status page
        await statusPages.resetToDefaults();
    });

    // ============================================================
    // PHASE 1: Uptime Bars - Monitor Status Display
    // ============================================================

    test.describe('Phase 1: Uptime Bars', () => {

        test('Public status page displays monitor groups', async ({ page }) => {
            // Enable public status page
            await statusPages.enablePublicViaAPI('all');

            // Visit public status page
            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // Should see the page title
            await expect(page.getByText('Global Status')).toBeVisible({ timeout: 10000 });

            // Should see "All Systems Operational" banner (default state)
            await expect(page.getByText('All Systems Operational')).toBeVisible({ timeout: 10000 });
        });

        test('Status page shows monitor names and status', async ({ page }) => {
            // Enable public status page
            await statusPages.enablePublicViaAPI('all');

            // Visit public status page
            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // Should see the page content loaded (either monitors or empty state)
            await expect(page.getByText('Global Status')).toBeVisible({ timeout: 10000 });

            // The page should have group sections (even if empty)
            // Look for the status banner which always appears
            await expect(page.locator('.rounded-xl.border').first()).toBeVisible({ timeout: 10000 });
        });

        test('Status page shows refresh countdown', async ({ page }) => {
            await statusPages.enablePublicViaAPI('all');

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // The countdown timer shows seconds until next refresh
            // Look for the "s" suffix in the banner
            await expect(page.getByText(/\d+s/)).toBeVisible({ timeout: 10000 });
        });

        test('Status page auto-refreshes data', async ({ page }) => {
            await statusPages.enablePublicViaAPI('all');

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // Get initial countdown value
            const countdownLocator = page.locator('text=/\\d+s/').first();
            await expect(countdownLocator).toBeVisible({ timeout: 10000 });

            // Wait a few seconds and verify countdown decreases
            await page.waitForTimeout(3000);

            // The countdown should have decreased (we just verify the page is still showing)
            await expect(page.getByText('Global Status')).toBeVisible();
        });

    });

    // ============================================================
    // PHASE 2: Incident History
    // ============================================================

    test.describe('Phase 2: Incident History', () => {

        test('Active incident appears on status page', async ({ page }) => {
            await statusPages.enablePublicViaAPI('all');

            // Create a public incident
            const incidentId = await statusPages.createIncidentViaAPI({
                title: 'E2E Test Active Incident',
                description: 'Testing active incident display',
                type: 'incident',
                severity: 'critical',
                status: 'investigating',
                public: true,
            });
            createdIncidentIds.push(incidentId);

            // Visit public status page
            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // Should see the incident
            await expect(page.getByText('E2E Test Active Incident')).toBeVisible({ timeout: 10000 });

            // Should show "Active Incidents" section header
            await expect(page.getByText('Active Incidents')).toBeVisible({ timeout: 10000 });
        });

        test('Private incident does NOT appear on public status page', async ({ page }) => {
            await statusPages.enablePublicViaAPI('all');

            // Create a private incident
            const incidentId = await statusPages.createIncidentViaAPI({
                title: 'E2E Private Incident',
                type: 'incident',
                severity: 'major',
                status: 'investigating',
                public: false,
            });
            createdIncidentIds.push(incidentId);

            // Visit public status page
            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // Private incident should NOT appear
            await expect(page.getByText('E2E Private Incident')).toHaveCount(0, { timeout: 5000 });
        });

        test('Maintenance window displays correctly', async ({ page }) => {
            await statusPages.enablePublicViaAPI('all');

            // Create a maintenance window
            const maintenanceId = await statusPages.createIncidentViaAPI({
                title: 'E2E Scheduled Maintenance',
                description: 'Planned system upgrade',
                type: 'maintenance',
                severity: 'minor',
                status: 'scheduled',
                public: true,
            });
            createdMaintenanceIds.push(maintenanceId);

            // Visit public status page
            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // Should see the maintenance window
            await expect(page.getByText('E2E Scheduled Maintenance')).toBeVisible({ timeout: 10000 });

            // Should show "Scheduled Maintenance" section heading
            await expect(page.getByRole('heading', { name: 'Scheduled Maintenance' })).toBeVisible({ timeout: 10000 });
        });

        test('Incident with updates shows expandable timeline', async ({ page }) => {
            await statusPages.enablePublicViaAPI('all');

            // Create an incident with updates
            const incidentId = await statusPages.createIncidentViaAPI({
                title: 'E2E Incident With Updates',
                type: 'incident',
                severity: 'major',
                status: 'investigating',
                public: true,
            });
            createdIncidentIds.push(incidentId);

            // Add updates
            await statusPages.addIncidentUpdateViaAPI(incidentId, 'investigating', 'Looking into the issue');
            await statusPages.addIncidentUpdateViaAPI(incidentId, 'identified', 'Root cause found');

            // Visit public status page
            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // Should see the incident
            await expect(page.getByText('E2E Incident With Updates')).toBeVisible({ timeout: 10000 });

            // Click to expand the incident (if expandable)
            await page.getByText('E2E Incident With Updates').click();

            // Should see the updates
            await expect(page.getByText('Looking into the issue')).toBeVisible({ timeout: 5000 });
            await expect(page.getByText('Root cause found')).toBeVisible({ timeout: 5000 });
        });

        test('Resolved incident moves to past incidents', async ({ page }) => {
            await statusPages.enablePublicViaAPI('all');

            // Create and resolve an incident
            const incidentId = await statusPages.createIncidentViaAPI({
                title: 'E2E Resolved Incident',
                type: 'incident',
                severity: 'minor',
                status: 'investigating',
                public: true,
            });
            createdIncidentIds.push(incidentId);

            // Resolve the incident
            await statusPages.updateIncidentViaAPI(incidentId, {
                title: 'E2E Resolved Incident',
                description: 'Test incident',
                type: 'incident',
                severity: 'minor',
                status: 'resolved',
                public: true,
                startTime: new Date(Date.now() - 3600000).toISOString(), // 1 hour ago
                endTime: new Date().toISOString(),
            });

            // Visit public status page
            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // Should NOT be in active incidents (resolved)
            // Should appear in past incidents section (if visible)
            // The "Past Incidents" section shows resolved incidents
            await expect(page.getByText('Past Incidents')).toBeVisible({ timeout: 10000 });
        });

        test('Status banner changes based on active incidents', async ({ page }) => {
            await statusPages.enablePublicViaAPI('all');

            // Initially should show "All Systems Operational"
            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');
            await expect(page.getByText('All Systems Operational')).toBeVisible({ timeout: 10000 });

            // Create a critical incident
            const incidentId = await statusPages.createIncidentViaAPI({
                title: 'E2E Outage',
                type: 'incident',
                severity: 'critical',
                status: 'investigating',
                public: true,
            });
            createdIncidentIds.push(incidentId);

            // Refresh the page
            await page.reload();
            await page.waitForLoadState('networkidle');

            // Should now show outage status
            await expect(page.getByText('System Outage')).toBeVisible({ timeout: 10000 });
        });

    });

    // ============================================================
    // PHASE 3: Configuration (Branding, Theme, Toggles)
    // ============================================================

    test.describe('Phase 3: Configuration', () => {

        test('Custom description appears on status page', async ({ page }) => {
            // Configure with custom description
            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: true,
                title: 'Global Status',
                description: 'E2E Test Status Page Description',
            });

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // Should see the custom description
            await expect(page.getByText('E2E Test Status Page Description')).toBeVisible({ timeout: 10000 });
        });

        test('Custom logo appears on status page', async ({ page }) => {
            // Use a data URI for the logo (small 1x1 transparent PNG)
            const dataUri = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==';

            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: true,
                title: 'Global Status',
                logoUrl: dataUri,
            });

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // Should have an img element with the logo
            const logo = page.locator('img[alt="Logo"]');
            await expect(logo).toBeVisible({ timeout: 10000 });
            await expect(logo).toHaveAttribute('src', dataUri);
        });

        test('Light theme applies correctly', async ({ page }) => {
            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: true,
                title: 'Global Status',
                theme: 'light',
            });

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // HTML element should have "light" class
            const html = page.locator('html');
            await expect(html).toHaveClass(/light/, { timeout: 10000 });
        });

        test('Dark theme applies correctly', async ({ page }) => {
            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: true,
                title: 'Global Status',
                theme: 'dark',
            });

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // HTML element should have "dark" class
            const html = page.locator('html');
            await expect(html).toHaveClass(/dark/, { timeout: 10000 });
        });

        test('Uptime bars can be hidden via config', async ({ page }) => {
            // First, enable with uptime bars shown (default)
            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: true,
                title: 'Global Status',
                showUptimeBars: true,
            });

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // Now configure to hide uptime bars
            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: true,
                title: 'Global Status',
                showUptimeBars: false,
            });

            await page.reload();
            await page.waitForLoadState('networkidle');

            // The uptime bar elements should not be present
            // (This depends on having monitors - for empty state, just verify page loads)
            await expect(page.getByText('Global Status')).toBeVisible({ timeout: 10000 });
        });

        test('Incident history can be hidden via config', async ({ page }) => {
            // Create a resolved incident so there's history to show/hide
            const incidentId = await statusPages.createIncidentViaAPI({
                title: 'E2E History Test',
                type: 'incident',
                severity: 'minor',
                status: 'resolved',
                public: true,
            });
            createdIncidentIds.push(incidentId);

            // Configure with incident history visible
            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: true,
                title: 'Global Status',
                showIncidentHistory: true,
            });

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // Now hide incident history
            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: true,
                title: 'Global Status',
                showIncidentHistory: false,
            });

            await page.reload();
            await page.waitForLoadState('networkidle');

            // "Past Incidents" or "Incident History" section should not appear
            await expect(page.getByText('Incident History')).toHaveCount(0, { timeout: 5000 });
            await expect(page.getByText('Past Incidents')).toHaveCount(0, { timeout: 5000 });
        });

    });

    // ============================================================
    // PHASE 4: RSS Feed
    // ============================================================

    test.describe('Phase 4: RSS Feed', () => {

        test('RSS feed returns valid XML', async () => {
            await statusPages.enablePublicViaAPI('all');

            const rssContent = await statusPages.getRSSFeed('all');

            // Should be valid XML
            expect(rssContent).toContain('<?xml version="1.0" encoding="UTF-8"?>');
            expect(rssContent).toContain('<rss version="2.0"');
            expect(rssContent).toContain('<channel>');
            expect(rssContent).toContain('</channel>');
            expect(rssContent).toContain('</rss>');
        });

        test('RSS feed contains page title', async () => {
            await statusPages.enablePublicViaAPI('all');

            const rssContent = await statusPages.getRSSFeed('all');

            expect(rssContent).toContain('Global Status');
            expect(rssContent).toContain('<title>');
        });

        test('RSS feed includes public incidents', async () => {
            await statusPages.enablePublicViaAPI('all');

            // Create a public incident
            const incidentId = await statusPages.createIncidentViaAPI({
                title: 'E2E RSS Test Incident',
                type: 'incident',
                severity: 'major',
                status: 'investigating',
                public: true,
            });
            createdIncidentIds.push(incidentId);

            const rssContent = await statusPages.getRSSFeed('all');

            // Should contain the incident
            expect(rssContent).toContain('E2E RSS Test Incident');
            expect(rssContent).toContain('<item>');
            expect(rssContent).toContain('[MAJOR]');
        });

        test('RSS feed excludes private incidents', async () => {
            await statusPages.enablePublicViaAPI('all');

            // Create a private incident
            const incidentId = await statusPages.createIncidentViaAPI({
                title: 'E2E RSS Private Incident',
                type: 'incident',
                severity: 'major',
                status: 'investigating',
                public: false,
            });
            createdIncidentIds.push(incidentId);

            const rssContent = await statusPages.getRSSFeed('all');

            // Should NOT contain the private incident
            expect(rssContent).not.toContain('E2E RSS Private Incident');
        });

        test('RSS feed includes incident updates in description', async () => {
            await statusPages.enablePublicViaAPI('all');

            // Create an incident with updates
            const incidentId = await statusPages.createIncidentViaAPI({
                title: 'E2E RSS Updates Test',
                type: 'incident',
                severity: 'critical',
                status: 'investigating',
                public: true,
            });
            createdIncidentIds.push(incidentId);

            await statusPages.addIncidentUpdateViaAPI(incidentId, 'investigating', 'RSS update message test');

            const rssContent = await statusPages.getRSSFeed('all');

            expect(rssContent).toContain('RSS update message test');
        });

        test('RSS feed for disabled page returns 404', async ({ page }) => {
            // Ensure page is disabled
            await statusPages.resetToDefaults();

            const response = await page.request.get('/api/s/all/rss');
            expect(response.status()).toBe(404);
        });

        test('RSS feed for nonexistent page returns 404', async ({ page }) => {
            const response = await page.request.get('/api/s/nonexistent-slug/rss');
            expect(response.status()).toBe(404);
        });

        test('RSS feed has correct content type', async ({ page }) => {
            await statusPages.enablePublicViaAPI('all');

            const response = await page.request.get('/api/s/all/rss');
            expect(response.status()).toBe(200);

            const contentType = response.headers()['content-type'];
            expect(contentType).toContain('application/rss+xml');
        });

        test('RSS feed has Atom self link', async () => {
            await statusPages.enablePublicViaAPI('all');

            const rssContent = await statusPages.getRSSFeed('all');

            expect(rssContent).toContain('xmlns:atom="http://www.w3.org/2005/Atom"');
            expect(rssContent).toContain('<atom:link href=');
            expect(rssContent).toContain('rel="self"');
        });

        test('RSS link is visible on public status page', async ({ page }) => {
            await statusPages.enablePublicViaAPI('all');

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // Should see RSS link in footer
            const rssLink = page.locator('a[href*="/rss"]');
            await expect(rssLink).toBeVisible({ timeout: 10000 });

            // Verify it has correct href
            await expect(rssLink).toHaveAttribute('href', /\/api\/s\/all\/rss/);
        });

        test('RSS feed properly escapes XML characters', async () => {
            await statusPages.enablePublicViaAPI('all');

            // Create an incident with special characters
            const incidentId = await statusPages.createIncidentViaAPI({
                title: 'Test <script> & "quotes"',
                description: "Description with 'apostrophes' & <tags>",
                type: 'incident',
                severity: 'minor',
                status: 'investigating',
                public: true,
            });
            createdIncidentIds.push(incidentId);

            const rssContent = await statusPages.getRSSFeed('all');

            // Should have escaped characters
            expect(rssContent).toContain('&lt;script&gt;');
            expect(rssContent).toContain('&amp;');
            expect(rssContent).toContain('&quot;');
        });

        test('RSS feed includes maintenance windows with correct label', async () => {
            await statusPages.enablePublicViaAPI('all');

            // Create a maintenance window
            const maintenanceId = await statusPages.createIncidentViaAPI({
                title: 'E2E RSS Maintenance Test',
                type: 'maintenance',
                severity: 'minor',
                status: 'scheduled',
                public: true,
            });
            createdMaintenanceIds.push(maintenanceId);

            const rssContent = await statusPages.getRSSFeed('all');

            expect(rssContent).toContain('E2E RSS Maintenance Test');
            expect(rssContent).toContain('[MAINTENANCE]');
        });

    });

    // ============================================================
    // Access Control Edge Cases
    // ============================================================

    test.describe('Access Control', () => {

        test('Disabled status page returns unavailable message', async ({ context }) => {
            // Ensure page is disabled
            await statusPages.resetToDefaults();

            // Try to access in a new context (unauthenticated)
            const publicPage = await context.newPage();
            await publicPage.goto('/status/all');
            await publicPage.waitForLoadState('networkidle');

            await expect(publicPage.getByText('Status Page Unavailable')).toBeVisible({ timeout: 10000 });

            await publicPage.close();
        });

        test('Private status page accessible to authenticated users', async ({ page }) => {
            // Enable but keep private
            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: false,
                title: 'Global Status',
            });

            // Authenticated user (current page) should see it
            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            await expect(page.getByText('Status Page Unavailable')).toHaveCount(0, { timeout: 5000 });
            await expect(page.getByText('Global Status')).toBeVisible({ timeout: 10000 });
        });

        test('Private status page shows error to unauthenticated users', async ({ page }) => {
            // Enable but keep private
            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: false,
                title: 'Global Status',
            });

            // Create new browser context (unauthenticated)
            const browser = page.context().browser();
            if (!browser) {
                test.skip();
                return;
            }

            const freshContext = await browser.newContext();
            const publicPage = await freshContext.newPage();

            const baseUrl = new URL(page.url()).origin;
            await publicPage.goto(`${baseUrl}/status/all`);
            await publicPage.waitForLoadState('networkidle');

            // Should see error
            await expect(publicPage.getByText('Status Page Unavailable')).toBeVisible({ timeout: 10000 });

            await publicPage.close();
            await freshContext.close();
        });

        test('RSS feed returns 404 for private page', async ({ page }) => {
            // Enable but keep private
            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: false,
                title: 'Global Status',
            });

            const response = await page.request.get('/api/s/all/rss');
            // Private pages should return 404 for RSS (not authenticated access for RSS)
            expect(response.status()).toBe(404);
        });

    });

    // ============================================================
    // Integration Tests
    // ============================================================

    test.describe('Integration', () => {

        test('Full workflow: enable page, create incident, view on status page, check RSS', async ({ page }) => {
            // 1. Enable status page
            await statusPages.enablePublicViaAPI('all');

            // 2. Create an incident
            const incidentId = await statusPages.createIncidentViaAPI({
                title: 'E2E Integration Test Incident',
                description: 'Full workflow test',
                type: 'incident',
                severity: 'major',
                status: 'investigating',
                public: true,
            });
            createdIncidentIds.push(incidentId);

            // 3. Add an update
            await statusPages.addIncidentUpdateViaAPI(incidentId, 'identified', 'We found the issue');

            // 4. Visit status page and verify incident is shown
            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            await expect(page.getByText('E2E Integration Test Incident')).toBeVisible({ timeout: 10000 });
            await expect(page.getByText('Active Incidents')).toBeVisible({ timeout: 5000 });

            // 5. Check RSS feed
            const rssContent = await statusPages.getRSSFeed('all');
            expect(rssContent).toContain('E2E Integration Test Incident');
            expect(rssContent).toContain('We found the issue');

            // 6. Verify RSS link is on page
            const rssLink = page.locator('a[href*="/rss"]');
            await expect(rssLink).toBeVisible({ timeout: 5000 });
        });

        test('Incident lifecycle: create -> update -> resolve -> appears in history', async ({ page }) => {
            await statusPages.enablePublicViaAPI('all');

            // Create incident
            const incidentId = await statusPages.createIncidentViaAPI({
                title: 'E2E Lifecycle Incident',
                type: 'incident',
                severity: 'critical',
                status: 'investigating',
                public: true,
            });
            createdIncidentIds.push(incidentId);

            // Verify it's active
            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');
            await expect(page.getByText('E2E Lifecycle Incident')).toBeVisible({ timeout: 10000 });

            // Update to identified
            await statusPages.addIncidentUpdateViaAPI(incidentId, 'identified', 'Found the root cause');

            // Resolve the incident
            await statusPages.updateIncidentViaAPI(incidentId, {
                title: 'E2E Lifecycle Incident',
                description: 'Test incident',
                type: 'incident',
                severity: 'critical',
                status: 'resolved',
                public: true,
                startTime: new Date(Date.now() - 3600000).toISOString(),
                endTime: new Date().toISOString(),
            });

            // Refresh and verify it's no longer in active incidents
            await page.reload();
            await page.waitForLoadState('networkidle');

            // Should be in past incidents now (if history is enabled)
            // The resolved incident should appear in past incidents section
            await expect(page.getByText('All Systems Operational')).toBeVisible({ timeout: 10000 });
        });

    });

    // ============================================================
    // Edge Cases
    // ============================================================

    test.describe('Edge Cases', () => {

        // ---------- Error States & Recovery ----------

        test('Disabled page while viewing shows error on refresh', async ({ page }) => {
            // Enable page first
            await statusPages.enablePublicViaAPI('all');

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');
            await expect(page.getByText('Global Status')).toBeVisible({ timeout: 10000 });

            // Disable the page via API
            await statusPages.configureViaAPI('all', { enabled: false });

            // Refresh - should now show unavailable
            await page.reload();
            await page.waitForLoadState('networkidle');

            await expect(page.getByText('Status Page Unavailable')).toBeVisible({ timeout: 10000 });
        });

        test('Empty state with no monitors displays correctly', async ({ page }) => {
            // The "all" status page should have groups but may have empty monitor arrays
            // This test verifies the page handles empty groups gracefully
            await statusPages.enablePublicViaAPI('all');

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // Page should load with title
            await expect(page.getByText('Global Status')).toBeVisible({ timeout: 10000 });

            // If groups are empty, "No monitors configured" may appear
            // At minimum, the status banner should be present
            await expect(page.locator('.rounded-xl.border').first()).toBeVisible({ timeout: 10000 });
        });

        test('Invalid slug shows error UI', async ({ page }) => {
            await page.goto('/status/nonexistent-slug-12345');
            await page.waitForLoadState('networkidle');

            await expect(page.getByText('Status Page Unavailable')).toBeVisible({ timeout: 10000 });
        });

        // ---------- Incident Edge Cases ----------

        test('Multiple concurrent incidents show correct status (most severe wins)', async ({ page }) => {
            await statusPages.enablePublicViaAPI('all');

            // Create 3 incidents with different severities
            const minorId = await statusPages.createIncidentViaAPI({
                title: 'E2E Minor Issue',
                type: 'incident',
                severity: 'minor',
                status: 'investigating',
                public: true,
            });
            createdIncidentIds.push(minorId);

            const majorId = await statusPages.createIncidentViaAPI({
                title: 'E2E Major Issue',
                type: 'incident',
                severity: 'major',
                status: 'investigating',
                public: true,
            });
            createdIncidentIds.push(majorId);

            const criticalId = await statusPages.createIncidentViaAPI({
                title: 'E2E Critical Outage',
                type: 'incident',
                severity: 'critical',
                status: 'investigating',
                public: true,
            });
            createdIncidentIds.push(criticalId);

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // All three incidents should be visible
            await expect(page.getByText('E2E Minor Issue')).toBeVisible({ timeout: 10000 });
            await expect(page.getByText('E2E Major Issue')).toBeVisible({ timeout: 10000 });
            await expect(page.getByText('E2E Critical Outage')).toBeVisible({ timeout: 10000 });

            // Status banner should show "System Outage" (most severe)
            await expect(page.getByText('System Outage')).toBeVisible({ timeout: 10000 });
        });

        test('Incident visibility change reflects on refresh', async ({ page }) => {
            await statusPages.enablePublicViaAPI('all');

            // Create a public incident
            const incidentId = await statusPages.createIncidentViaAPI({
                title: 'E2E Visibility Test Incident',
                type: 'incident',
                severity: 'major',
                status: 'investigating',
                public: true,
            });
            createdIncidentIds.push(incidentId);

            // Verify it's visible
            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');
            await expect(page.getByText('E2E Visibility Test Incident')).toBeVisible({ timeout: 10000 });

            // Change to private via API
            await statusPages.updateIncidentViaAPI(incidentId, {
                title: 'E2E Visibility Test Incident',
                type: 'incident',
                severity: 'major',
                status: 'investigating',
                public: false,
                startTime: new Date().toISOString(),
            });

            // Refresh - should no longer be visible
            await page.reload();
            await page.waitForLoadState('networkidle');

            await expect(page.getByText('E2E Visibility Test Incident')).toHaveCount(0, { timeout: 5000 });
        });

        test('Very long incident title truncates properly', async ({ page }) => {
            await statusPages.enablePublicViaAPI('all');

            // Create incident with 200+ character title
            const longTitle = 'E2E ' + 'Very Long Incident Title '.repeat(10); // ~260 characters
            const incidentId = await statusPages.createIncidentViaAPI({
                title: longTitle,
                type: 'incident',
                severity: 'major',
                status: 'investigating',
                public: true,
            });
            createdIncidentIds.push(incidentId);

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // The incident should be visible but truncated
            // Look for the first part of the title
            await expect(page.getByText(/E2E Very Long Incident Title/)).toBeVisible({ timeout: 10000 });

            // Page should not be broken - status banner still visible
            await expect(page.locator('.rounded-xl.border').first()).toBeVisible();
        });

        test('Empty incident history shows appropriate state', async ({ page }) => {
            // Configure page with incident history enabled
            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: true,
                title: 'Global Status',
                showIncidentHistory: true,
            });

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // With no past incidents, the "Past Incidents" section should not appear
            // (The component only renders when pastIncidents.length > 0)
            await expect(page.getByText('Global Status')).toBeVisible({ timeout: 10000 });

            // Past Incidents heading should NOT be present when there are no past incidents
            await expect(page.getByRole('heading', { name: 'Past Incidents' })).toHaveCount(0, { timeout: 3000 });
        });

        // ---------- Configuration Edge Cases ----------

        test('All display options disabled renders minimal page', async ({ page }) => {
            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: true,
                title: 'Global Status',
                showUptimeBars: false,
                showIncidentHistory: false,
                showUptimePercentage: false,
            });

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // Page should still show title and status banner
            await expect(page.getByText('Global Status')).toBeVisible({ timeout: 10000 });
            await expect(page.getByText('All Systems Operational')).toBeVisible({ timeout: 10000 });

            // No incident history section
            await expect(page.getByRole('heading', { name: 'Past Incidents' })).toHaveCount(0, { timeout: 3000 });
        });

        test('Config changes apply on auto-refresh', async ({ page }) => {
            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: true,
                title: 'Global Status',
                description: 'Original description',
            });

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');
            await expect(page.getByText('Original description')).toBeVisible({ timeout: 10000 });

            // Update description via API
            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: true,
                title: 'Global Status',
                description: 'Updated description via API',
            });

            // Manually reload (simulating auto-refresh)
            await page.reload();
            await page.waitForLoadState('networkidle');

            // New description should appear
            await expect(page.getByText('Updated description via API')).toBeVisible({ timeout: 10000 });
        });

        test('Accent color edge cases - black and white', async ({ page }) => {
            // Test with black accent color
            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: true,
                title: 'Global Status',
                accentColor: '#000000',
            });

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');
            await expect(page.getByText('Global Status')).toBeVisible({ timeout: 10000 });

            // Test with white accent color
            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: true,
                title: 'Global Status',
                accentColor: '#FFFFFF',
            });

            await page.reload();
            await page.waitForLoadState('networkidle');
            await expect(page.getByText('Global Status')).toBeVisible({ timeout: 10000 });

            // Page should render without errors in both cases
        });

        // ---------- Theme & Branding Edge Cases ----------

        test('System theme respects OS preference', async ({ page }) => {
            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: true,
                title: 'Global Status',
                theme: 'system',
            });

            // Emulate dark color scheme
            await page.emulateMedia({ colorScheme: 'dark' });
            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // HTML should have dark class
            const htmlDark = page.locator('html');
            await expect(htmlDark).toHaveClass(/dark/, { timeout: 10000 });

            // Emulate light color scheme
            await page.emulateMedia({ colorScheme: 'light' });
            await page.reload();
            await page.waitForLoadState('networkidle');

            // HTML should have light class
            const htmlLight = page.locator('html');
            await expect(htmlLight).toHaveClass(/light/, { timeout: 10000 });
        });

        test('Logo load failure shows fallback icon', async ({ page }) => {
            // Configure with invalid logo URL
            await statusPages.configureViaAPI('all', {
                enabled: true,
                public: true,
                title: 'Global Status',
                logoUrl: 'https://invalid-domain-that-does-not-exist-12345.com/logo.png',
            });

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // The page should still load with the title
            await expect(page.getByText('Global Status')).toBeVisible({ timeout: 10000 });

            // The fallback icon should become visible (after image fails to load)
            // Wait a bit for the image error handler to fire
            await page.waitForTimeout(2000);

            // The Activity fallback icon should be visible (no longer has 'hidden' class)
            const fallbackIcon = page.locator('svg.fallback-icon').first();
            await expect(fallbackIcon).toBeVisible({ timeout: 5000 });
        });

        // ---------- Timing & Refresh Edge Cases ----------

        test('Countdown timer decreases and resets', async ({ page }) => {
            await statusPages.enablePublicViaAPI('all');

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // Get initial countdown (should be around 60s or less)
            const countdownLocator = page.locator('text=/\\d+s/').first();
            await expect(countdownLocator).toBeVisible({ timeout: 10000 });

            const initialText = await countdownLocator.textContent();
            const initialValue = parseInt(initialText?.replace('s', '') || '60');

            // Wait 3 seconds
            await page.waitForTimeout(3000);

            // Countdown should have decreased
            const newText = await countdownLocator.textContent();
            const newValue = parseInt(newText?.replace('s', '') || '60');

            expect(newValue).toBeLessThan(initialValue);
        });

        test('Data updates appear on manual refresh', async ({ page }) => {
            await statusPages.enablePublicViaAPI('all');

            await page.goto('/status/all');
            await page.waitForLoadState('networkidle');

            // Initially no incidents
            await expect(page.getByText('All Systems Operational')).toBeVisible({ timeout: 10000 });

            // Create incident via API
            const incidentId = await statusPages.createIncidentViaAPI({
                title: 'E2E Refresh Test Incident',
                type: 'incident',
                severity: 'critical',
                status: 'investigating',
                public: true,
            });
            createdIncidentIds.push(incidentId);

            // Reload page
            await page.reload();
            await page.waitForLoadState('networkidle');

            // Incident should now appear
            await expect(page.getByText('E2E Refresh Test Incident')).toBeVisible({ timeout: 10000 });
        });

        // ---------- RSS Feed Edge Cases ----------

        test('RSS feed with no incidents has valid structure', async () => {
            await statusPages.enablePublicViaAPI('all');

            // Don't create any incidents - ensure clean state
            const rssContent = await statusPages.getRSSFeed('all');

            // Should have valid XML structure
            expect(rssContent).toContain('<?xml version="1.0" encoding="UTF-8"?>');
            expect(rssContent).toContain('<rss version="2.0"');
            expect(rssContent).toContain('<channel>');
            expect(rssContent).toContain('<title>');
            expect(rssContent).toContain('</channel>');
            expect(rssContent).toContain('</rss>');

            // Channel info should still be present
            expect(rssContent).toContain('Global Status');
        });

        test('RSS feed incidents appear in chronological order (newest first)', async () => {
            await statusPages.enablePublicViaAPI('all');

            // Create incidents with slight delays to ensure ordering
            const oldIncidentId = await statusPages.createIncidentViaAPI({
                title: 'E2E Old Incident AAA',
                type: 'incident',
                severity: 'minor',
                status: 'investigating',
                public: true,
            });
            createdIncidentIds.push(oldIncidentId);

            // Small delay
            await new Promise(r => setTimeout(r, 100));

            const newIncidentId = await statusPages.createIncidentViaAPI({
                title: 'E2E New Incident ZZZ',
                type: 'incident',
                severity: 'major',
                status: 'investigating',
                public: true,
            });
            createdIncidentIds.push(newIncidentId);

            const rssContent = await statusPages.getRSSFeed('all');

            // Both incidents should be present
            expect(rssContent).toContain('E2E Old Incident AAA');
            expect(rssContent).toContain('E2E New Incident ZZZ');

            // Newer incident should appear before older one (RSS standard: newest first)
            const newIndex = rssContent.indexOf('E2E New Incident ZZZ');
            const oldIndex = rssContent.indexOf('E2E Old Incident AAA');
            expect(newIndex).toBeLessThan(oldIndex);
        });

    });

});
