import { API_BASE } from '../apiBase';
import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { AlertingSettingsPage } from '../pages/AlertingSettingsPage';

test.describe.configure({ mode: 'serial' });

// Reads the settings the server actually holds, which is the only way to tell a form that
// saved from a form that merely looked like it did.
//
// Uses page.request rather than the bare `request` fixture: the settings endpoint is behind
// session auth, and only the page's context carries the login cookie.
async function serverSettings(page: import('@playwright/test').Page) {
    const resp = await page.request.get(`${API_BASE}/api/settings`);
    expect(resp.ok(), 'GET /api/settings should be authorised').toBeTruthy();
    return (await resp.json()) as Record<string, string>;
}

test.describe('Sustained alert ladder', () => {
    test.beforeEach(async ({ page }) => {
        const login = new LoginPage(page);
        await page.goto('/dashboard');
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 10000 });
        if (await login.isVisible()) {
            await login.login();
            await expect(page).toHaveURL(/.*dashboard/);
        }
    });

    // A fresh install must show the ladder it is really using. Blanks here would be saved
    // back as zeros the first time anyone touched the form, silently turning off the
    // silent window and the reminders.
    test('shows the real defaults rather than blanks', async ({ page }) => {
        const settings = new AlertingSettingsPage(page);
        await settings.goto();

        const ladder = await settings.readLadder();
        expect(ladder.sustained).toBe('180');
        expect(ladder.reminder).toBe('30');
        expect(ladder.repeat).toBe('60');
    });

    test('persists a changed ladder across a reload', async ({ page }) => {
        const settings = new AlertingSettingsPage(page);
        await settings.goto();

        await settings.setLadder('300', '15', '120');
        expect(await settings.saveAndWait()).toBe(200);

        // The server is the source of truth, not the form still holding what we typed.
        const stored = await serverSettings(page);
        expect(stored['notification.alert.sustained_seconds']).toBe('300');
        expect(stored['notification.alert.reminder_minutes']).toBe('15');
        expect(stored['notification.alert.repeat_reminder_minutes']).toBe('120');

        await page.reload();
        const reloaded = await settings.readLadder();
        expect(reloaded).toEqual({ sustained: '300', reminder: '15', repeat: '120' });

        // Put it back so the rest of the suite sees the defaults.
        await settings.setLadder('180', '30', '60');
        expect(await settings.saveAndWait()).toBe(200);
    });

    // Zero is a real choice on all three — no silent window, no reminders, no repeats — so
    // the form must accept and round-trip it rather than treating it as an empty field.
    test('accepts zero as a deliberate choice', async ({ page }) => {
        const settings = new AlertingSettingsPage(page);
        await settings.goto();

        await settings.setLadder('0', '0', '0');
        expect(await settings.saveAndWait()).toBe(200);

        const stored = await serverSettings(page);
        expect(stored['notification.alert.sustained_seconds']).toBe('0');
        expect(stored['notification.alert.reminder_minutes']).toBe('0');

        await page.reload();
        const reloaded = await settings.readLadder();
        expect(reloaded.sustained).toBe('0');
        expect(reloaded.reminder).toBe('0');

        await settings.setLadder('180', '30', '60');
        expect(await settings.saveAndWait()).toBe(200);
    });

    // The copy is the whole remedy for the footgun that cost weeks of missed alerts. If it
    // ever goes back to saying the digest silences things, the fix has been undone.
    test('the digest section no longer claims to silence alerts', async ({ page }) => {
        const settings = new AlertingSettingsPage(page);
        await settings.goto();
        await settings.openDigest();

        // The scope note only renders once the digest is enabled, so turn it on if this
        // instance has it off.
        const digestToggle = page.getByTestId('digest-enabled');
        await expect(digestToggle).toBeVisible();

        const note = page.getByTestId('digest-scope-note');
        if (!(await note.isVisible())) {
            await digestToggle.click();
        }

        await expect(note).toBeVisible({ timeout: 5000 });
        await expect(note).toContainText('Immediate alerts are controlled separately');
        await expect(page.getByText('Include in the digest')).toBeVisible();
        await expect(page.getByText('stop sending immediate alerts')).toHaveCount(0);
        await expect(page.getByText('Batched Events')).toHaveCount(0);
    });
    // The manager silently falls back to its defaults for a value it cannot parse, so an
    // out-of-range number that the API accepted would leave the operator believing they
    // had configured something they had not.
    test('the API rejects an out-of-range ladder', async ({ page }) => {
        for (const body of [
            { 'notification.alert.sustained_seconds': '-1' },
            { 'notification.alert.sustained_seconds': '999999' },
            { 'notification.alert.reminder_minutes': 'not-a-number' },
        ]) {
            const resp = await page.request.patch(`${API_BASE}/api/settings`, { data: body });
            expect(resp.status(), `expected ${JSON.stringify(body)} to be rejected`).not.toBe(200);
        }

        const stored = await serverSettings(page);
        expect(stored['notification.alert.sustained_seconds']).toBe('180');
        expect(stored['notification.alert.reminder_minutes']).toBe('30');
    });
});

test.describe('Adaptive latency thresholds', () => {
    test.beforeEach(async ({ page }) => {
        const login = new LoginPage(page);
        await page.goto('/dashboard');
        await expect(page.getByText('Wait ...')).toBeHidden({ timeout: 10000 });
        if (await login.isVisible()) {
            await login.login();
            await expect(page).toHaveURL(/.*dashboard/);
        }
    });

    // On by default, and the detail fields only make sense while it is on — showing a
    // multiplier for a feature that is off would invite configuring something inert.
    test('is on by default and reveals its settings', async ({ page }) => {
        const settings = new AlertingSettingsPage(page);
        await settings.goto();
        await settings.openLatency();

        const toggle = page.getByTestId('adaptive-latency-switch');
        await expect(toggle).toHaveAttribute('data-state', 'checked');
        await expect(page.getByLabel(/Slow at/)).toBeVisible();
        await expect(page.getByLabel(/Learn from/)).toBeVisible();

        await toggle.click();
        await expect(page.getByLabel(/Slow at/)).toHaveCount(0);
        // And it says what happens instead, rather than leaving a blank panel.
        await expect(page.getByText(/judged against the fixed latency threshold/i)).toBeVisible();

        await toggle.click();
        await expect(page.getByLabel(/Slow at/)).toBeVisible();
    });

    test('persists the multiplier and the window', async ({ page }) => {
        const settings = new AlertingSettingsPage(page);
        await settings.goto();
        await settings.openLatency();

        await page.getByLabel(/Slow at/).fill('200');
        await page.getByLabel(/Learn from/).fill('14');
        expect(await settings.saveAndWait()).toBe(200);

        const stored = await serverSettings(page);
        expect(stored['notification.latency.factor_percent']).toBe('200');
        expect(stored['notification.latency.baseline_days']).toBe('14');

        await page.reload();
        await settings.openLatency();
        await expect(page.getByLabel(/Slow at/)).toHaveValue('200');
        await expect(page.getByLabel(/Learn from/)).toHaveValue('14');

        // Restore the defaults for the rest of the suite.
        await page.getByLabel(/Slow at/).fill('150');
        await page.getByLabel(/Learn from/).fill('7');
        expect(await settings.saveAndWait()).toBe(200);
    });

    // A multiplier below 1x would mark a service degraded at its own median. The manager
    // rejects it and falls back, so the API must not accept it in the first place.
    test('the API rejects a multiplier below 1x', async ({ page }) => {
        const resp = await page.request.patch(`${API_BASE}/api/settings`, {
            data: { 'notification.latency.factor_percent': '50' },
        });
        expect(resp.status()).not.toBe(200);

        const stored = await serverSettings(page);
        expect(stored['notification.latency.factor_percent']).toBe('150');
    });
});
