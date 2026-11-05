# Migration files

Generated, never edited by hand: `scripts/migrations_gen.sh <name>` writes
the next file of every set whose ent tables changed, for each database:

    <dialect>/<set>/<version>_<name>.sql   (+ atlas.sum)

`<dialect>` is `mysql` or `postgres`; `<set>` is `core` or a module
(`entity.Module.Tables`). `app migrate` applies the pending files and
records them in `schema_migrations`.
