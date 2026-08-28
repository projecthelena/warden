import { API_BASE } from '../apiBase';
import { test, expect, Page } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { NotificationsPage } from '../pages/NotificationsPage';

// Run tests serially to avoid auth conflicts
test.describe.configure({ mode: 'serial' });

// Lands on the notifications view, logged in. Every test that talks to the API through
// page.request depends on this having run: the session cookie it leaves on the context is
// what makes those calls anything other than a 401.
async function openNotifications(page: Page) {
    const notifications = new NotificationsPage(page);
    const login = new LoginPage(page);

    await notifications.goto();
    if (await login.isVisible()) {
        await login.login();
        await page.getByRole('button', { name: 'Settings' }).click();
        await page.getByRole('link', { name: 'Notifications' }).click();
    }
    return notifications;
}

// Reads a channel back from the API. Admins get the config unmasked, which is the only way
// to check that a field the form never showed came through unchanged.
async function channelFromApi(page: Page, name: string) {
    const resp = await page.request.get(`${API_BASE}/api/notifications/channels`);
    expect(resp.ok(), 'GET /api/notifications/channels should be authorised').toBeTruthy();

    const body = await resp.json();
    const channel = (body.channels ?? []).find((c: { name: string }) => c.name === name);
    if (!channel) {
        const seen = (body.channels ?? []).map((c: { name: string }) => c.name);
        throw new Error(`channel ${name} not found; the list has: ${seen.join(', ') || '(none)'}`);
    }
    return { ...channel, config: JSON.parse(channel.config) };
}

test.describe('Notification Management', () => {

    test('Create and Delete Slack Channel', async ({ page }) => {
        page.on('console', msg => console.log('BROWSER LOG:', msg.text()));
        const notifications = await openNotifications(page);

        const channelName = `Slack Alerter ${Date.now()}`;
        await notifications.createSlackChannel(channelName, 'https://hooks.slack.com/services/T00000/B00000/XXXXXXXX');
        await notifications.deleteChannel(channelName);
    });

    test('Create and Delete Email Channel', async ({ page }) => {
        const notifications = await openNotifications(page);

        const channelName = `On-call Inbox ${Date.now()}`;
        await notifications.createEmailChannel(channelName, 'Warden <alerts@example.com>', 'oncall@example.com');
        await notifications.deleteChannel(channelName);
    });

});

test.describe('Email channel credentials', () => {

    // A channel the server would refuse must be refused on the form, not accepted and then
    // discovered silently on the night something goes down.
    //
    // Authenticated on purpose, and asserting the exact status: an unauthenticated request
    // is rejected with a 401 whatever the config says, so a test that only asks for "not
    // 200" would pass with the validation deleted.
    test('an email channel without a recipient is rejected, with a reason', async ({ page }) => {
        await openNotifications(page);

        const name = `Broken ${Date.now()}`;
        const resp = await page.request.post(`${API_BASE}/api/notifications/channels`, {
            data: {
                type: 'email',
                name,
                config: { host: 'smtp.example.com', from: 'alerts@example.com' },
                enabled: true,
            },
        });

        expect(resp.status()).toBe(400);

        // The dashboard shows data.error. A plain text body would make it fall back to a
        // generic "Failed to add channel" and the operator would not learn which field is
        // wrong, which is the whole point of validating per field.
        const body = await resp.json();
        expect(body.error).toContain('recipient');

        const list = await page.request.get(`${API_BASE}/api/notifications/channels`);
        const names = ((await list.json()).channels ?? []).map((c: { name: string }) => c.name);
        expect(names, 'the rejected channel must not have been stored').not.toContain(name);
    });

    test('the API rejects a recipient that is not an address', async ({ page }) => {
        await openNotifications(page);

        const resp = await page.request.post(`${API_BASE}/api/notifications/channels`, {
            data: {
                type: 'email',
                name: `Malformed ${Date.now()}`,
                config: { host: 'smtp.example.com', from: 'alerts@example.com', to: 'ops@example.com, nope' },
                enabled: true,
            },
        });

        expect(resp.status()).toBe(400);
        expect((await resp.json()).error).toContain('nope');
    });

    // The form preloads the stored password so that saving an unrelated change does not
    // blank it. Getting this wrong is invisible until the next alert: the channel keeps
    // working in the UI and stops authenticating with the mail server.
    test('renaming a channel leaves its SMTP password alone', async ({ page }) => {
        const notifications = await openNotifications(page);

        const original = `Credentialed ${Date.now()}`;
        const renamed = `${original} renamed`;
        const created = await page.request.post(`${API_BASE}/api/notifications/channels`, {
            data: {
                type: 'email',
                name: original,
                config: {
                    host: 'smtp.example.com',
                    port: '587',
                    username: 'alerts@example.com',
                    password: 'correct-horse-battery-staple',
                    from: 'Warden <alerts@example.com>',
                    to: 'oncall@example.com',
                },
                enabled: true,
            },
        });
        expect(created.status()).toBe(201);

        await page.reload();
        await expect(page.getByText('Wait ...')).toBeHidden();

        await notifications.openChannel(original);
        await notifications.detailsNameInput.fill(renamed);
        await notifications.saveOpenChannel();

        const after = await channelFromApi(page, renamed);
        expect(after.config.password, 'the rename blanked the stored password').toBe('correct-horse-battery-staple');
        expect(after.config.username).toBe('alerts@example.com');
        expect(after.config.host).toBe('smtp.example.com');
        expect(after.config.to).toBe('oncall@example.com');
        expect(after.enabled, 'editing the channel disabled it').toBe(true);

        await page.request.delete(`${API_BASE}/api/notifications/channels/${after.id}`);
    });

    // Send Test exists so a channel is proven before an outage relies on it. A failure has
    // to say what the mail server did, not just that something went wrong.
    test('a failed test send reports what the server said', async ({ page }) => {
        const notifications = await openNotifications(page);

        // Port 1 on loopback refuses immediately, so this asserts the error path without
        // waiting out a dial timeout.
        await notifications.openEmailForm(`Unreachable ${Date.now()}`, 'alerts@example.com', 'oncall@example.com');
        await notifications.smtpHostInput.fill('127.0.0.1');
        await notifications.page.getByTestId('channel-smtp-port-input').fill('1');

        const tested = page.waitForResponse(
            resp => resp.url().includes('/api/notifications/channels/test') && resp.request().method() === 'POST',
            { timeout: 15000 }
        );
        await page.getByTestId('test-channel-btn').click();
        await tested;

        // The toast, not just a status code: the reason has to reach the operator. Matched
        // by test id because "Test Failed" is also a substring of the description.
        await expect(page.getByTestId('toast-title').first()).toHaveText('Test Failed', { timeout: 10000 });
        await expect(page.getByText(/connecting to 127\.0\.0\.1:1/)).toBeVisible({ timeout: 10000 });
    });
});
