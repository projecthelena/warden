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

## Account ownership and roles

Warden deliberately keeps local and SSO accounts separate:

- **Local accounts** are created by a Warden administrator with a username, password, and role. Their passwords are changed or reset in Warden.
- **SSO accounts** are created on the first successful identity-provider login when auto-provisioning is enabled. They start as viewers, and a Warden administrator can promote them to editor or administrator afterward.
- Warden does not create an SSO identity or set its password. Passwords, MFA, disabling the identity, and other authentication policy belong to Google or the configured OIDC provider.
- Warden therefore hides password-change and password-reset controls for SSO accounts, and its API rejects attempts to add a local password to one.

The **New Local User** action only creates password-based accounts. There is currently no invitation or pre-provisioning flow for SSO identities because Warden links them using the stable subject identifier received in a verified provider token, not an administrator-entered email address. If auto-provisioning is disabled, only SSO identities that completed a previous successful login remain able to sign in.

### Exactly what happens on an SSO login

Authentication and authorization are separate:

1. Google or the generic OIDC provider authenticates the person and returns a signed identity token.
2. Warden verifies the token, issuer, audience, nonce, expiry, and verified email.
3. If this provider identity is already linked to a Warden user, Warden signs in that existing user with the role already stored in Warden.
4. If the identity is new and **Auto-provision users** is enabled, Warden creates it with the `viewer` role. It never becomes an administrator merely because the identity provider accepted it.
5. To grant more access, an existing Warden administrator opens **Settings → Users** and changes the user from Viewer to Editor or Admin. The new role is stored in Warden and applies on subsequent authenticated requests; signing in through SSO does not overwrite it.
6. If the identity is new and auto-provisioning is disabled, Warden rejects the login. The current release has no SSO invitation or administrator-driven pre-provisioning flow.

The identity provider currently does not choose the Warden role. Warden does not consume group or role claims and does not map Google groups, Dex groups, Entra groups, or other provider claims to `admin`, `editor`, `viewer`, or `status_viewer`.

| Warden role | Access |
| --- | --- |
| `admin` | Full access, including security settings, user creation, and role assignment |
| `editor` | Can create and modify operational resources, but cannot administer security or users |
| `viewer` | Read-only access to the dashboard; this is the default for every new SSO identity |
| `status_viewer` | Can only access the status pages explicitly assigned by an administrator |

Promoting an SSO user does not create a Warden password. The user continues signing in through the same identity provider, but Warden loads the locally assigned role after validating the session. At least one existing administrator must therefore complete the first promotion.

### What happens when the email already exists

Warden does not treat a matching email as sufficient proof that two login methods belong to the same person:

| Existing Warden account | Incoming SSO login | Result |
| --- | --- | --- |
| Same provider and same provider subject | Any role | Warden signs in the existing user and preserves its Warden role |
| Local account with a password and the same email | Google or generic OIDC | Login is rejected with `account_linking_required`; Warden does not merge or create a duplicate |
| SSO-only account with the same email but a different provider identity | Another configured SSO provider | The current implementation re-links that SSO-only account to the new provider; only one provider identity is stored at a time |
| No matching provider identity or email | Auto-provision on | Warden creates a new viewer |
| No matching provider identity or email | Auto-provision off | Login is rejected |

Rejecting automatic linkage to a password account prevents an attacker who controls an external identity with a matching email from taking over the local account. Warden does not currently provide a user-confirmed account-linking flow. Administrators should therefore avoid pre-creating a local account for someone expected to use SSO; let the person complete the first SSO login and then assign the desired Warden role.

Because only one SSO provider identity can be stored per user, using Google and generic OIDC interchangeably with the same email is not a supported account-linking strategy. Pick one provider per person until Warden has an explicit multi-provider linking model.

## Configure Google

Create an OAuth 2.0 client in Google Cloud as a **Web application**. If the OAuth consent screen is still in testing mode, add every account that will exercise the flow as a test user. Register this exact redirect URI, replacing the hostname with the public Warden hostname:

```text
https://warden.example.com/api/auth/sso/google/callback
```

In **Settings → Security → Single Sign-On (SSO)**:

1. Enter the Google client ID and client secret.
2. Leave **Redirect URL** empty to use the callback above. Only set an override when the externally visible callback really differs from Warden's origin.
3. Optionally restrict **Allowed email domains** and choose whether to **Auto-provision users**.
4. Save, then enable Google SSO and save again.

The **Test Configuration** button only checks that the stored values have a plausible shape. It does not contact Google or validate a login. The complete test is signing in through a private browser window and returning successfully to Warden.

## Configure a generic OpenID Connect provider

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

## Reproducible Dex test setup

Dex is useful for testing the complete generic OIDC flow without depending on Google or a production identity platform. It is a test identity provider, not part of Warden and not required in a normal deployment.

One simple layout runs both services behind the same public hostname:

- Warden: `https://warden.example.com`
- Dex issuer: `https://warden.example.com/dex`

Configure the reverse proxy to route `/dex` to Dex and the remaining paths to Warden. The `/dex` path is part of the issuer identity and must appear exactly in the Dex configuration and in Warden.

Use values like these, replacing every example with credentials and hostnames generated for the test environment:

| Setting       | Value                                                   |
| ------------- | ------------------------------------------------------- |
| Provider name | `Test Dex`                                              |
| Issuer URL    | `https://warden.example.com/dex`                        |
| Redirect URI  | `https://warden.example.com/api/auth/sso/oidc/callback` |
| Client ID     | `warden-test`                                           |
| Client secret | A newly generated secret shared by Dex and Warden       |
| Test email    | `test-user@example.com`                                 |

Generate a client secret and a test password for each environment. Put the client secret and the bcrypt password hash in the secret manager used by the deployment. Store the plaintext test password in a password manager or local Keychain, never in Git. For example, a macOS Keychain entry can be read without hard-coding its value:

```bash
security find-generic-password -w -s warden-dex-test -a test-user@example.com | pbcopy
```

To exercise the deployment:

1. Keep an administrator session open in one browser window.
2. Open `https://warden.example.com/login` in a private window.
3. Choose **Sign in with Test Dex**.
4. Enter the generated test identity and password on Dex's login page, then grant access if Dex shows its approval screen.
5. Confirm that the browser returns to `/dashboard`. On the first successful login, Warden creates a `viewer` user when auto-provisioning is enabled.
6. In the administrator session, open **Settings → Users** and confirm the user's role and SSO provider.

The flow validates discovery, authorization-code exchange, state, nonce, PKCE, signed ID-token verification, verified email handling, user provisioning, session creation, and the final role-based redirect. Opening Dex's discovery document is a useful connectivity check, but it does not replace the browser flow:

```bash
curl -fsS https://warden.example.com/dex/.well-known/openid-configuration
```

The Dex static client's redirect URI and Warden's callback URI must be byte-for-byte identical. Generate new credentials for every environment and never copy credentials from another installation.

## Test each control deliberately

Google and generic OIDC have separate **Enable** switches, credentials, domain allowlists, and auto-provision controls. Turning one off does not affect the other. Saving an empty secret field preserves the already stored secret; disabling a provider hides its login button but preserves its configuration and existing Warden users.

Use this matrix while learning or validating a deployment:

| Test | Configuration | Expected result |
| --- | --- | --- |
| Generic OIDC happy path | OIDC on, correct Dex values, auto-provision on | Dex returns to Warden and a new identity becomes a viewer |
| OIDC off | Disable OIDC and save | The generic provider button disappears; local and Google login are unaffected |
| Domain rejected | Set an allowlist that does not contain the test email's domain | Warden rejects the callback and creates no session |
| Existing identity | Auto-provision off, use an identity that was linked previously | Login still succeeds |
| Unknown identity | Auto-provision off, use a second Dex identity never seen by Warden | Login is rejected |
| Google only | Google on and OIDC off | Only the Google SSO button appears alongside password login |
| Both providers | Google on and OIDC on | Both buttons appear and each flow completes independently |
| Recovery | Test the existing administrator password | Local login remains available if either identity provider fails |

After a negative test, restore the original domain allowlist and enable state from the administrator session. A single existing Dex user can prove the happy path, domain rejection, disabling, and existing-identity behavior. Testing unknown-identity rejection requires adding a second static Dex user or using another OIDC provider account that has never signed in to Warden.

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
