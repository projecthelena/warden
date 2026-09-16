# Single sign-on

Warden supports Google OAuth and one generic OpenID Connect (OIDC) provider. Generic OIDC works with Keycloak, Authentik, Authelia, Dex, Okta, Entra ID, and other standards-compliant providers.

## What can be enabled at the same time

Google and the generic OIDC provider are independent. You can enable both at the same time, in which case the login page shows all three available methods:

- Username and password
- **Sign in with Google**
- **Sign in with _provider name_** for the configured OIDC provider

The generic OIDC configuration is a single slot. For example, Warden can show Google and Entra ID together, or Google and Authentik together, but it cannot show Authentik, Entra ID, and Okta as three separate generic OIDC buttons. Supporting an arbitrary list of OIDC providers would require a different settings and identity model; it is not part of the current implementation.

If several upstream directories must share one button, use an identity broker such as Keycloak or Authentik as Warden's OIDC provider and connect the other directories or identity providers to that broker.

Each Warden user currently has one SSO identity. Do not expect the same Warden account to be linked independently to both Google and the generic OIDC provider.

Configure the provider with this redirect URI:

```text
https://warden.example.com/api/auth/sso/oidc/callback
```

Then open **Settings → Security → Generic OpenID Connect** and configure:

1. **Provider name**: the label shown on the login button.
2. **Issuer URL**: the exact issuer identifier published by the provider. For Keycloak this normally includes the realm; for Authentik it includes the application slug.
3. **Client ID** and **Client Secret**: credentials for a confidential web client. Leaving the secret field empty later preserves the stored secret.
4. **Allowed email domains**: an optional comma-separated allowlist such as `example.com, company.org`.
5. **Auto-provision users**: when enabled, a new verified identity is created with the viewer role. When disabled, only an identity that was already linked through a previous successful SSO login can sign in. Merely creating a password user with the same email does not link it, and Warden currently has no manual account-linking screen.

Save the settings, enable OIDC, sign out, and verify the provider button from a private browser window. Test with both an allowed account and, when a domain allowlist is configured, a denied account before depending on SSO for access.

Warden discovers the authorization, token, and signing-key endpoints from the issuer URL. It requests the `openid`, `profile`, and `email` scopes and requires a verified email claim. The authorization-code flow uses state, nonce, and PKCE; the returned ID token must have a valid signature, issuer, audience, expiry, and matching nonce before Warden creates a session.

Keep at least one working administrator login until the OIDC flow has been verified. Disabling OIDC removes its button from the login page but does not delete users or invalidate existing sessions.

## Access policies, groups, and password login

Warden can restrict SSO by exact email domain through **Allowed email domains**. It does not currently read OIDC group claims or map identity-provider groups to Warden roles.

To permit only members of a particular group, configure that authorization policy in the identity provider. For example, assign only the intended Entra ID users or groups to the enterprise application, or put a group policy on the Warden application in Keycloak or Authentik. The provider must refuse authorization for everyone else; Warden then accepts only the verified identities the provider allows through. Keep Warden's own role assignment separate: newly provisioned identities always start as viewers, and an administrator can change their Warden role afterward.

Username and password login is always available in the current release. There is no global **SSO only** switch and hiding the form would not be sufficient, because the password API would also need to reject logins. Keeping password login available provides an administrator recovery path if the identity provider is unavailable or misconfigured. If an SSO-only mode is added later, it should preserve an explicit break-glass recovery mechanism rather than only removing fields from the page.

## Production smoke test

Test SSO in a private browser window before relying on it or changing access policies:

1. Keep an existing administrator session open in a different browser window.
2. Register the callback URI shown in **Settings → Security** at the provider. It must match exactly, including scheme, hostname, path, and port.
3. Enable one provider and save its settings.
4. Open `/login` in the private window and confirm its button is visible.
5. Sign in with an allowed, verified account. Confirm that Warden reaches the dashboard and that a newly auto-provisioned account is a viewer.
6. Sign out and try an account outside the allowed email domains or provider-side group. Confirm that access is denied.
7. Disable **Auto-provision users**, then try a never-before-seen identity. Confirm that Warden rejects it. Confirm that an identity which already signed in successfully can still sign in.
8. If testing Google and generic OIDC together, enable both and confirm that both buttons appear and complete each flow independently.
9. Test username and password login as the administrator to confirm the recovery path still works.

A useful open-source production test is Google plus Authentik or Keycloak. Entra ID is equally valid as the generic provider. Use separate test users and a non-production group while validating provider-side access rules; Warden does not provide a simulated login or a **Send test** equivalent because a real browser redirect and provider-issued token are required to validate the complete OIDC flow.

## LDAP

Warden does not bind directly to LDAP. Put an OIDC identity provider such as Keycloak or Authentik in front of LDAP or Active Directory, then connect Warden to that provider. This keeps directory credentials out of Warden and gives the login flow standard token verification, MFA, and centralized access policies.
