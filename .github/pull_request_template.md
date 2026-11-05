## What and why

<!-- The change, the problem it solves and the issue it closes (Closes #…). -->

## How it was checked

<!-- The tests you added or ran, and what you checked by hand. -->

- [ ] `./scripts/check.sh` passes
- [ ] `./scripts/check.sh -db` passes (the change touches the database)

## Checklist

- [ ] Nothing site-specific in code, templates or JavaScript: no content types, fields, paths, texts or IDs of one site
- [ ] Schema changes: ent schemas regenerated and migrations written by `./scripts/migrations_gen.sh`
- [ ] New interface strings go through the translation catalogs
- [ ] Documentation in `docs/` updated for a change in behaviour or a new mechanism
