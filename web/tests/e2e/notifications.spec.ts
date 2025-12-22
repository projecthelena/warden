import { test } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { NotificationsPage } from '../pages/NotificationsPage';

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

});
