package entity

// UserIDs - a user field's value: its users' IDs in the editor's order (a
// field with one value holds at most one); none for anything else.
func UserIDs(v any) []int {
	ids, _ := v.([]int)
	return ids
}

// UserRoles - FieldTypeUser: the roles (IDs) whose users may be chosen
// (storage config "user_roles"); none - any active user.
func (f FieldInstanceDef) UserRoles() []int {
	raw, _ := f.Config["user_roles"].([]any)
	out := make([]int, 0, len(raw))
	for _, item := range raw {
		switch n := item.(type) {
		case float64: // JSON numbers decode as float64
			if n > 0 {
				out = append(out, int(n))
			}
		case int:
			if n > 0 {
				out = append(out, n)
			}
		}
	}
	return out
}
