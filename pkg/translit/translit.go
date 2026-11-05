// Package translit writes Cyrillic letters in Latin: the one table the
// slugs and the file names use.
package translit

import "unicode"

// latin - Cyrillic letters in Latin, lower case.
var latin = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "yo", 'ж': "zh",
	'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o",
	'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "h", 'ц': "ts",
	'ч': "ch", 'ш': "sh", 'щ': "sch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
	'і': "i", 'ї': "yi", 'є': "e", 'ґ': "g",
}

// Latin - r in Latin letters, lower case whatever r's case ("Щ" - "sch",
// "ь" - ""); false - r isn't a letter of the table.
func Latin(r rune) (string, bool) {
	s, ok := latin[unicode.ToLower(r)]
	return s, ok
}
