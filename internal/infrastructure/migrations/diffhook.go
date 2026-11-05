package migrations

import (
	"strings"

	atlasschema "ariga.io/atlas/sql/schema"
	entschema "entgo.io/ent/dialect/sql/schema"
)

// sameDefaultHook drops column changes that differ only in how a default
// expression is written: ent writes DEFAULT CURRENT_TIMESTAMP as the
// expression "(CURRENT_TIMESTAMP)", which MySQL reports back as "(now())"
// (or as CURRENT_TIMESTAMP for a column created without the parentheses,
// as on older sites) - the same default, which Atlas would otherwise
// "change" in every migration.
func sameDefaultHook(next entschema.Differ) entschema.Differ {
	return entschema.DiffFunc(func(current, desired *atlasschema.Schema) ([]atlasschema.Change, error) {
		changes, err := next.Diff(current, desired)
		if err != nil {
			return nil, err
		}
		out := changes[:0]
		for _, c := range changes {
			mt, ok := c.(*atlasschema.ModifyTable)
			if !ok {
				out = append(out, c)
				continue
			}
			kept := mt.Changes[:0]
			for _, tc := range mt.Changes {
				if mc, ok := tc.(*atlasschema.ModifyColumn); ok && mc.Change == atlasschema.ChangeDefault && sameDefault(mc.From.Default, mc.To.Default) {
					continue
				}
				kept = append(kept, tc)
			}
			mt.Changes = kept
			if len(mt.Changes) > 0 {
				out = append(out, mt)
			}
		}
		return out, nil
	})
}

func sameDefault(a, b atlasschema.Expr) bool {
	return a != nil && b != nil && normDefault(a) == normDefault(b)
}

func normDefault(e atlasschema.Expr) string {
	var s string
	switch v := e.(type) {
	case *atlasschema.RawExpr:
		s = v.X
	case *atlasschema.Literal:
		s = v.V
	default:
		return ""
	}
	s = strings.TrimSpace(s)
	for strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	s = strings.ToUpper(s)
	if s == "NOW()" || s == "CURRENT_TIMESTAMP()" {
		s = "CURRENT_TIMESTAMP"
	}
	return s
}
