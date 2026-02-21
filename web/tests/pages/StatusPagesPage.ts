import { Page, Locator, expect } from '@playwright/test';

export interface StatusPageConfig {
    enabled?: boolean;
    public?: boolean;
    title?: string;
    description?: string;
    logoUrl?: string;
    accentColor?: string;
    theme?: 'light' | 'dark' | 'system';
    showUptimeBars?: boolean;
    showUptimePercentage?: boolean;
    showIncidentHistory?: boolean;
}

export class StatusPagesPage {
    readonly page: Page;

    constructor(page: Page) {
        this.page = page;
    }

    async goto() {
        await this.page.goto('/status-pages');
        await this.page.waitForLoadState('networkidle');
    }

    /** Navigate to Status Pages via sidebar */
    async navigateViaSidebar() {
        await this.page.getByRole('link', { name: 'Status Pages' }).click();
        await expect(this.page).toHaveURL(/.*status-pages/);
    }

    /** Get the row locator for a specific status page by slug */
    getRow(slug: string): Locator {
        return this.page.getByTestId(`status-page-row-${slug}`);
    }

    /** Get the badge locator for a specific status page */
    getBadge(slug: string): Locator {
        return this.page.getByTestId(`status-page-badge-${slug}`);
    }

    /** Get the enabled toggle for a specific status page */
    getEnabledToggle(slug: string): Locator {
        return this.page.getByTestId(`status-page-enabled-toggle-${slug}`);
    }

    /** Get the public toggle for a specific status page */
    getPublicToggle(slug: string): Locator {
        return this.page.getByTestId(`status-page-public-toggle-${slug}`);
    }

    /** Get the "Visit Page" link for a specific status page */
    getVisitLink(slug: string): Locator {
        return this.page.getByTestId(`status-page-visit-${slug}`);
    }

    /** Toggle the enabled state of a status page and wait for the toast */
    async toggleEnabled(slug: string) {
        await this.getEnabledToggle(slug).click();
        await this.waitForToast();
    }

    /** Toggle the public state of a status page and wait for the toast */
    async togglePublic(slug: string) {
        await this.getPublicToggle(slug).click();
        await this.waitForToast();
    }

    /** Wait for the "Status Page Updated" toast to appear and then dismiss */
    async waitForToast() {
        const toast = this.page.getByTestId('toast-title').first();
        await expect(toast).toBeVisible({ timeout: 5000 });
        // Wait for toast to disappear so subsequent toasts can be detected
        await expect(toast).toHaveCount(0, { timeout: 10000 });
    }

    /** Assert the badge text for a status page */
    async expectBadge(slug: string, text: 'Disabled' | 'Public' | 'Private') {
        await expect(this.getBadge(slug)).toHaveText(text, { timeout: 5000 });
    }

    /** Assert the "Visit Page" link is visible */
    async expectVisitLinkVisible(slug: string) {
        await expect(this.getVisitLink(slug)).toBeVisible({ timeout: 5000 });
    }

    /** Assert the "Visit Page" link is NOT visible */
    async expectVisitLinkHidden(slug: string) {
        await expect(this.getVisitLink(slug)).toHaveCount(0, { timeout: 5000 });
    }

    /** Assert the public toggle is disabled (not clickable) */
    async expectPublicToggleDisabled(slug: string) {
        await expect(this.getPublicToggle(slug)).toBeDisabled({ timeout: 5000 });
    }

    /** Assert the public toggle is enabled (clickable) */
    async expectPublicToggleEnabled(slug: string) {
        await expect(this.getPublicToggle(slug)).toBeEnabled({ timeout: 5000 });
    }

    /** Wait for the status pages to load */
    async waitForLoad() {
        await expect(this.page.getByRole('heading', { name: 'Status Pages' })).toBeVisible({ timeout: 10000 });
        // Wait for at least one row to appear (Global Status is always present)
        await expect(this.getRow('all')).toBeVisible({ timeout: 10000 });
    }

    /** Reset the "all" status page to disabled+private via API */
    async resetToDefaults() {
        await this.page.request.patch('/api/status-pages/all', {
            data: {
                enabled: false,
                public: false,
                title: 'Global Status',
                description: '',
                logoUrl: '',
                accentColor: '',
                theme: 'system',
                showUptimeBars: true,
                showUptimePercentage: true,
                showIncidentHistory: true,
            }
        });
    }

    /** Configure a status page via API */
    async configureViaAPI(slug: string, config: StatusPageConfig) {
        await this.page.request.patch(`/api/status-pages/${slug}`, {
            data: config
        });
    }

    /** Enable and make public via API (shortcut) */
    async enablePublicViaAPI(slug: string) {
        await this.configureViaAPI(slug, { enabled: true, public: true, title: 'Global Status' });
    }

    /** Create an incident via API */
    async createIncidentViaAPI(options: {
        title: string;
        description?: string;
        type?: 'incident' | 'maintenance';
        severity?: 'minor' | 'major' | 'critical';
        status?: string;
        public?: boolean;
        affectedGroups?: string[];
    }) {
        const isMaintenance = options.type === 'maintenance';

        if (isMaintenance) {
            // Use the maintenance endpoint for maintenance windows
            const response = await this.page.request.post('/api/maintenance', {
                data: {
                    title: options.title,
                    description: options.description || 'Test incident',
                    status: options.status || 'scheduled',
                    affectedGroups: options.affectedGroups || [],
                    startTime: new Date().toISOString(),
                    endTime: new Date(Date.now() + 3600000).toISOString(), // 1 hour from now
                }
            });
            const data = await response.json();
            return data.id;
        }

        // Use the incidents endpoint for regular incidents
        const response = await this.page.request.post('/api/incidents', {
            data: {
                title: options.title,
                description: options.description || 'Test incident',
                severity: options.severity || 'major',
                status: options.status || 'investigating',
                public: options.public ?? true,
                affectedGroups: options.affectedGroups || [],
                startTime: new Date().toISOString(),
            }
        });
        const data = await response.json();
        return data.id;
    }

    /** Update an incident via API */
    async updateIncidentViaAPI(id: string, updates: Record<string, unknown>) {
        await this.page.request.put(`/api/incidents/${id}`, {
            data: updates
        });
    }

    /** Delete an incident via API */
    async deleteIncidentViaAPI(id: string) {
        await this.page.request.delete(`/api/incidents/${id}`);
    }

    /** Delete a maintenance window via API */
    async deleteMaintenanceViaAPI(id: string) {
        await this.page.request.delete(`/api/maintenance/${id}`);
    }

    /** Add an incident update via API */
    async addIncidentUpdateViaAPI(incidentId: string, status: string, message: string) {
        await this.page.request.post(`/api/incidents/${incidentId}/updates`, {
            data: { status, message }
        });
    }

    /** Get RSS feed content */
    async getRSSFeed(slug: string): Promise<string> {
        const response = await this.page.request.get(`/api/s/${slug}/rss`);
        return response.text();
    }

    /** Get public status page data via API */
    async getPublicStatusViaAPI(slug: string): Promise<{ status: number; data?: Record<string, unknown> }> {
        const response = await this.page.request.get(`/api/s/${slug}`);
        const status = response.status();
        if (status === 200) {
            return { status, data: await response.json() };
        }
        return { status };
    }
}
