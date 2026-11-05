// Command hello-site - an example of a site's own binary: the engine
// (kosatka.Main) with a module from outside it, imported by
// kosatka_modules.go ("app module add" writes it).
package main

import "github.com/kosatka-cms/kosatka"

func main() { kosatka.Main() }
