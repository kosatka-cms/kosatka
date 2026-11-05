// Command app - the engine's binary: the site's server and its commands
// (kosatka.Main). A site with modules from outside the engine builds its
// own binary the same way, from a main package importing them
// (docs/modules/using-modules.md).
package main

import "github.com/kosatka-cms/kosatka"

func main() { kosatka.Main() }
