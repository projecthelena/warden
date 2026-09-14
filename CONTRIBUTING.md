# Contributing

Thank you for helping improve Warden. Keep changes focused, include tests for behavior changes, and update the relevant guide when a user-facing workflow changes.

## Checks

Run the complete local check before opening a pull request:

```bash
make check
```

Install the repository's pre-push hook with:

```bash
make hooks
```

## Markdown

Markdown prose stays unwrapped: use one source line per paragraph and let the renderer control its displayed width.

```bash
npm install
npm run format:markdown
npm run check:markdown
```

The pre-push hook and CI run the same Markdown check.
