// Package testkit - the part of the module SDK a module's tests use, as a
// module from its own repository can:
//   - rendering: the engine's templates with the module's (UseEngineRoot,
//     RenderPageContent, RenderPlacement…), the modules switched on and off
//     (WithModules), a theme or the embedded assets (UseTheme,
//     UseModuleAssets);
//   - handler tests: fakes of the core's services (Auth, Users, Nodes),
//     a session with a CSRF token (Session, PostForm), the module's mux
//     (ModuleMux), deleting content (NodeDeleter), the actions recorded
//     for the action log (CaptureActions);
//   - the database (built with the dbintegration tag only, run on the
//     servers KOSATKA_TEST_MYSQL_DSN and KOSATKA_TEST_POSTGRES_DSN name -
//     the engine's scripts/test_db.sh starts both; without them a test is
//     skipped): every set's schema installed (SchemaDB) and the core's
//     repositories and services on it (Deps).
package testkit
