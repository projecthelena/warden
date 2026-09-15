# Single sign-on

Warden supports Google OAuth and generic OpenID Connect (OIDC). Generic OIDC works with Keycloak, Authentik, Authelia, Dex, Okta, Entra ID, and other standards-compliant providers.

Configure the provider with this redirect URI:

```text
https://warden.example.com/api/auth/sso/oidc/callback
```

Warden discovers the authorization, token, user identity, and signing-key endpoints from the issuer URL. It requests the `openid`, `profile`, and `email` scopes and requires a verified email claim. New identities receive the viewer role when auto-provisioning is enabled.

## LDAP

Warden does not bind directly to LDAP. Put an OIDC identity provider such as Keycloak or Authentik in front of LDAP or Active Directory, then connect Warden to that provider. This keeps directory credentials out of Warden and gives the login flow standard token verification, MFA, and centralized access policies.
