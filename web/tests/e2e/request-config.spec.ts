import { test, expect } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';
import { DashboardPage } from '../pages/DashboardPage';

test.describe.configure({ mode: 'serial' });

test.describe('Request Configuration', () => {

    test('Create monitor with advanced request config and verify settings', async ({ page }) => {
        const dashboard = new DashboardPage(page);
        const login = new LoginPage(page);

        // 1. Setup - Login
        await dashboard.goto();
        if (await login.isVisible()) {
            await login.login();
        }

        // 2. Create Group
        const groupName = `ReqConfig Group ${Date.now()}`;
        console.log(`Creating Group: ${groupName}`);
        await dashboard.createGroup(groupName);

        // 3. Open New Monitor sheet
        await dashboard.createMonitorTrigger.click();

        // 4. Fill in basic fields
        const monitorName = `ReqConfig Monitor ${Date.now()}`;
        console.log(`Creating Monitor: ${monitorName}`);
        await dashboard.createMonitorName.fill(monitorName);
        await dashboard.createMonitorUrl.fill('http://localhost:9096/healthz');

        // Select the group
        const groupSelect = page.getByTestId('create-monitor-group-select');
        await groupSelect.click();
        await page.getByRole('option', { name: groupName }).click();

        // 5. Expand Advanced Settings
        await page.getByText('+ Advanced Settings').click();
        await expect(page.getByText('Request Configuration')).toBeVisible({ timeout: 5000 });

        // 6a. Change HTTP Method to POST
        const methodSelect = page.getByTestId('request-method-select');
        await methodSelect.click();
        await page.getByRole('option', { name: 'POST', exact: true }).click();
        // Verify it changed
        await expect(methodSelect).toContainText('POST');

        // 6b. Set Request Timeout to 10
        const timeoutInput = page.getByPlaceholder('5');
        await timeoutInput.click();
        await timeoutInput.fill('10');

        // 6c. Set Retry on Failure to 2 retries
        const retrySelect = page.getByTestId('request-retry-select');
        await retrySelect.click();
        await page.getByRole('option', { name: '2 retries' }).click();
        await expect(retrySelect).toContainText('2 retries');

        // 6d. Set Accepted Status Codes
        const acceptedCodesInput = page.getByPlaceholder('200-399');
        await acceptedCodesInput.click();
        await acceptedCodesInput.fill('200-299,301');

        // 6e. Uncheck Follow Redirects
        const followRedirectsRow = page.locator('.flex.items-center.justify-between').filter({ hasText: 'Follow Redirects' });
        const followRedirectsSwitch = followRedirectsRow.locator('button[role="switch"]');
        await expect(followRedirectsSwitch).toHaveAttribute('data-state', 'checked');
        await followRedirectsSwitch.click();
        await expect(followRedirectsSwitch).toHaveAttribute('data-state', 'unchecked');

        // 6f. Add a custom header
        await page.getByRole('button', { name: '+ Add' }).click();
        const headerNameInput = page.getByPlaceholder('Header name');
        await expect(headerNameInput).toBeVisible({ timeout: 3000 });
        await headerNameInput.fill('User-Agent');
        await page.getByPlaceholder('Value').fill('Warden/1.0');

        // 6g. Fill in Request Body (visible because POST is selected)
        const bodyTextarea = page.getByPlaceholder('{"status": "ok"}');
        await expect(bodyTextarea).toBeVisible({ timeout: 3000 });
        await bodyTextarea.fill('{"check": true}');

        // 7. Submit monitor creation
        await dashboard.createMonitorSubmit.click();

        // 8. Verify creation toast
        const toast = page.getByText(`Monitor "${monitorName}" active and checking.`).first();
        await expect(toast).toBeVisible({ timeout: 15000 });
        await expect(page.getByText(monitorName).first()).toBeVisible();

        // 9. Open monitor details (click on the monitor card)
        await page.getByText(monitorName).first().click();
        await expect(page.locator('[data-state="open"].fixed.inset-0')).toBeVisible({ timeout: 5000 });

        // 10. Go to Settings tab
        await page.getByTestId('monitor-settings-tab').click();
        await expect(page.getByText('Request Configuration')).toBeVisible({ timeout: 5000 });

        // 11. Verify request config values in settings

        // 11a. HTTP Method shows POST
        await expect(page.getByTestId('request-method-select')).toContainText('POST');

        // 11b. Timeout shows 10
        await expect(page.locator('input[placeholder="5"]')).toHaveValue('10');

        // 11c. Retry shows 2 retries
        await expect(page.getByTestId('request-retry-select')).toContainText('2 retries');

        // 11d. Follow Redirects is unchecked
        const settingsSwitch = page.locator('.flex.items-center.justify-between').filter({ hasText: 'Follow Redirects' }).locator('button[role="switch"]');
        await expect(settingsSwitch).toHaveAttribute('data-state', 'unchecked');

        // 11e. Accepted Codes shows "200-299,301"
        await expect(page.locator('input[placeholder="200-399"]')).toHaveValue('200-299,301');

        // 11f. Header row shows User-Agent / Warden/1.0
        await expect(page.getByPlaceholder('Header name')).toHaveValue('User-Agent');
        await expect(page.getByPlaceholder('Value')).toHaveValue('Warden/1.0');

        // 11g. Body shows '{"check": true}'
        await expect(page.getByPlaceholder('{"status": "ok"}')).toHaveValue('{"check": true}');

        // Close the sheet
        await page.getByRole('button', { name: 'Close' }).click();
        await expect(page.locator('[data-state="open"].fixed.inset-0')).toHaveCount(0, { timeout: 5000 });

        // 12. Clean up
        console.log(`Deleting Monitor: ${monitorName}`);
        await dashboard.deleteMonitor(monitorName);

        console.log(`Deleting Group: ${groupName}`);
        await dashboard.deleteGroup(groupName);

        console.log('Request Configuration Test Passed.');
    });

});
