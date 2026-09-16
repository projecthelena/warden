import { expect, test } from '@playwright/test';
import { LoginPage } from '../pages/LoginPage';

test.describe('OIDC configuration', () => {
    test('publishes and removes the configured provider on the login page', async ({ page }) => {
        const login = new LoginPage(page);
        await page.goto('/dashboard');
        if (await login.isVisible()) await login.login();

        await page.goto('/settings?tab=security');
        await expect(page.getByText('Generic OpenID Connect')).toBeVisible();
        await page.getByTestId('oidc-provider-name').fill('Example Identity');
        await page.getByTestId('oidc-issuer-url').fill('https://id.example.com');
        await page.getByTestId('oidc-client-id').fill('warden-e2e');
        await page.getByTestId('oidc-client-secret').fill('e2e-secret');
        await page.getByRole('switch', { name: 'Enable OIDC' }).click();
        await page.getByTestId('oidc-save').click();
        await expect(page.getByTestId('toast-title').filter({ hasText: 'OIDC settings saved' }).last()).toBeVisible();

        const status = await page.request.get('/api/auth/sso/status');
        expect(status.ok()).toBeTruthy();
        await expect(status.json()).resolves.toMatchObject({ oidc: true, oidcProviderName: 'Example Identity' });

        await page.getByTestId('oidc-issuer-url').fill('javascript:alert(1)');
        await page.getByTestId('oidc-save').click();
        await expect(page.getByTestId('toast-title').filter({ hasText: 'Could not save OIDC settings' }).last()).toBeVisible();
        const stored = await page.request.get('/api/settings');
        expect(stored.ok()).toBeTruthy();
        await expect(stored.json()).resolves.toMatchObject({ 'sso.oidc.issuer_url': 'https://id.example.com' });

        await login.logout();
        await expect(page.getByTestId('oidc-sso-btn')).toHaveText('Sign in with Example Identity');

        await login.login();
        await page.goto('/settings?tab=security');
        await page.getByRole('switch', { name: 'Enable OIDC' }).click();
        await page.getByTestId('oidc-save').click();
        await expect(page.getByTestId('toast-title').filter({ hasText: 'OIDC settings saved' }).last()).toBeVisible();
        await login.logout();
        await expect(page.getByTestId('oidc-sso-btn')).toHaveCount(0);
    });
});
