import { API_BASE } from '../apiBase';
import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { NotificationsPage } from '../pages/NotificationsPage';

// Run tests serially to avoid auth conflicts
test.describe.configure({ mode: 'serial' });

test.describe('Notification Management', () => {

    test('Create and Delete Slack Channel', async ({ page }) => {
        page.on('console', msg => console.log('BROWSER LOG:', msg.text()));
        const notifications = new NotificationsPage(page);
        const login = new LoginPage(page);

        // 1. Setup
        await notifications.goto();
        if (await login.isVisible()) {
            await login.login();

            const cookies = await page.context().cookies();
            console.log('Cookies after login:', JSON.stringify(cookies));

            // Open Settings menu if needed (SPA Nav)
            await page.getByRole('button', { name: 'Settings' }).click();
            await page.getByRole('link', { name: 'Notifications' }).click();
        }

        // 2. Create Channel
        const channelName = `Slack Alerter ${Date.now()}`;
        console.log(`Creating Channel: ${channelName}`);
        await notifications.createSlackChannel(channelName, 'https://hooks.slack.com/services/T00000/B00000/XXXXXXXX');

        // 3. Delete Channel
        console.log(`Deleting Channel: ${channelName}`);
        await notifications.deleteChannel(channelName);
    });


    test('Create and Delete Email Channel', async ({ page }) => {
        const notifications = new NotificationsPage(page);
        const login = new LoginPage(page);

        await notifications.goto();
        if (await login.isVisible()) {
            await login.login();
            await page.getByRole('button', { name: 'Settings' }).click();
            await page.getByRole('link', { name: 'Notifications' }).click();
        }

        const channelName = `On-call Inbox ${Date.now()}`;
        await notifications.createEmailChannel(channelName, 'Warden <alerts@example.com>', 'oncall@example.com');
        await notifications.deleteChannel(channelName);
    });

});

test.describe('Email channel credentials', () => {
    // A channel the server would refuse must be refused on the form, not accepted and then
    // discovered silently on the night something goes down.
    test('an email channel without a recipient is rejected', async ({ page }) => {
        const resp = await page.request.post(`${API_BASE}/api/notifications/channels`, {
            data: {
                type: 'email',
                name: `Broken ${Date.now()}`,
                config: { host: 'smtp.example.com', from: 'alerts@example.com' },
                enabled: true,
            },
        });
        expect(resp.status()).not.toBe(200);
        expect(resp.status()).not.toBe(201);
    });
});
