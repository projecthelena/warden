# Password Recovery

If you are locked out of Warden, there are three ways back in, from easiest to most manual.

## An admin resets another user

If any admin can still sign in, they can reset anyone else's password from
Settings > Users: open the row menu next to the user and choose Reset Password. The user's
existing sessions are signed out, so the old password stops working immediately.

## The `reset-password` command (total lockout)

When no admin can sign in, or nobody remembers any password, use the CLI. It talks to the
same database the server uses and never starts the HTTP server, so it works while the app
is stopped.

Run it wherever the binary and the database are reachable, with the same `DB_TYPE`,
`DB_PATH` or `DB_URL` the server uses. In the published image the binary is `/app/warden`:

```
/app/warden reset-password <username>
```

It prompts for the new password twice (without echoing it) and then updates that user. You
can also pipe the password in, which is handy inside a container:

```
echo 'NewPassword123!' | /app/warden reset-password admin
```

In Kubernetes with Postgres this works in the running pod, no downtime, since the login
reads the hash on each attempt:

```
kubectl exec -it deploy/warden -- /app/warden reset-password admin
```

The password must meet the login policy: at least 8 characters, one number and one special
character. Existing sessions for that user are revoked.

## Editing the database directly

The last resort, if you cannot run the binary at all, is to set the password hash yourself.
Warden stores a bcrypt hash in `users.password_hash`. Generate one and update the row:

```
# generate a bcrypt hash (Go accepts $2a$, $2b$ and $2y$ prefixes)
htpasswd -bnBC 12 '' 'NewPassword123!' | tr -d ':\n'

# postgres
psql "$DB_URL" -c "UPDATE users SET password_hash='<hash>' WHERE username='admin';"
```

For SQLite the database is a file, not a network service, so stop the server first to avoid
corrupting it, edit `warden.db` with `sqlite3`, then start it again. The published image is
minimal and does not ship `sqlite3`, so copy the file out, edit it, and copy it back, or
mount the volume in a container that has `sqlite3`.
