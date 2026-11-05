package service

// IsDuplicateKeyErr - err is a unique constraint violation (a module's
// "already exists").
func IsDuplicateKeyErr(err error) bool { return isDuplicateKeyErr(err) }
