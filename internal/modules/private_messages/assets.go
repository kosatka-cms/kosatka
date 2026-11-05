package privatemessages

import "embed"

// Assets - the module's templates/ and static/, built into the binary
// (modules.Assets, paths.SetModuleAssets): a site runs without the module's
// source directory.
//
//go:embed templates static
var Assets embed.FS
