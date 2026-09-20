import { API_BASE } from '../apiBase';
import { expect, test, type Page } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';

async function createMonitor(page: Page, groupId: string, name: string, url: string) {
    const response = await page.request.post(`${API_BASE}/api/monitors`, {
        data: { name, type: 'http', url, groupId, interval: 60 },
    });
    expect(response.status(), `create ${name}`).toBe(201);
    return await response.json() as { id: string };
}

test('paginates, searches, filters, and restores a group monitor view from the URL', async ({ page }) => {
    const login = new LoginPage(page);
    await page.goto('/');
    if (await login.isVisible()) await login.login();

    const stamp = Date.now();
    const groupName = `Pagination Group ${stamp}`;
    const groupResponse = await page.request.post(`${API_BASE}/api/groups`, {
        data: { name: groupName },
    });
    expect(groupResponse.status()).toBe(201);
    const group = await groupResponse.json() as { id: string };

    try {
        const monitors = await Promise.all(Array.from({ length: 26 }, (_, index) =>
            createMonitor(
                page,
                group.id,
                index === 25 ? `Needle Monitor ${stamp}` : `Paged Monitor ${String(index + 1).padStart(2, '0')} ${stamp}`,
                `${API_BASE}/healthz?monitor=${index + 1}`,
            ),
        ));
        const paused = await createMonitor(page, group.id, `Paused Monitor ${stamp}`, `${API_BASE}/healthz?paused=1`);
        const issue = await createMonitor(page, group.id, `Issue Monitor ${stamp}`, 'http://127.0.0.1:1');

        expect((await page.request.post(`${API_BASE}/api/monitors/${paused.id}/pause`)).ok()).toBeTruthy();

        await page.goto(`/groups/${group.id}`);
        const cards = page.locator('[data-testid^="monitor-card-"]');
        await expect(cards).toHaveCount(25);
        await expect(page.getByText('Page 1 of 2')).toBeVisible();

        await page.getByRole('button', { name: 'Next page' }).click();
        await expect(page).toHaveURL(new RegExp(`/groups/${group.id}\\?page=2$`));
        await expect(cards).toHaveCount(3);
        await page.reload();
        await expect(page.getByText('Page 2 of 2')).toBeVisible();
        await expect(cards).toHaveCount(3);

        await page.getByRole('combobox', { name: 'Monitors per page' }).click();
        await page.getByRole('option', { name: '50' }).click();
        await expect(page).toHaveURL(new RegExp(`/groups/${group.id}\\?pageSize=50$`));
        await expect(cards).toHaveCount(28);

        await page.getByRole('textbox', { name: 'Search monitors' }).fill('Needle');
        await expect(page).toHaveURL(new RegExp(`search=Needle`));
        await expect(cards).toHaveCount(1);
        await expect(page.getByText(`Needle Monitor ${stamp}`)).toBeVisible();
        await page.reload();
        await expect(page.getByRole('textbox', { name: 'Search monitors' })).toHaveValue('Needle');
        await expect(cards).toHaveCount(1);

        await page.getByRole('button', { name: 'Clear monitor search' }).click();
        await expect(cards).toHaveCount(28);

        await page.getByRole('button', { name: /^Paused 1$/ }).click();
        await expect(page).toHaveURL(new RegExp('status=paused'));
        await expect(cards).toHaveCount(1);
        await expect(page.getByText(`Paused Monitor ${stamp}`)).toBeVisible();

        await page.getByRole('button', { name: /^Issues 1$/ }).click();
        await expect(page).toHaveURL(new RegExp('status=issues'));
        await expect(cards).toHaveCount(1);
        await expect(page.getByText(`Issue Monitor ${stamp}`)).toBeVisible();

        await page.getByRole('button', { name: /^Operational 26$/ }).click();
        await expect(page).toHaveURL(new RegExp('status=operational'));
        await expect(cards).toHaveCount(26);

        expect(monitors).toHaveLength(26);
        expect(issue.id).toBeTruthy();
    } finally {
        const cleanup = await page.request.delete(`${API_BASE}/api/groups/${group.id}`);
        expect(cleanup.ok(), 'cleanup pagination group').toBeTruthy();
    }
});
