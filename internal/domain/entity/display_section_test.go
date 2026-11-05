package entity

import "testing"

func TestDisplaySectionsWithout(t *testing.T) {
	cover := FieldInstanceDef{ID: 1, MachineName: "cover_image"}
	body := FieldInstanceDef{ID: 2, MachineName: "content"}
	s := DisplaySections{
		{Image: &cover},
		{Body: &body},
		{Fields: FieldInstanceDefs{{ID: 3, MachineName: "cover_image"}, {ID: 4, MachineName: "year"}}},
		{Fields: FieldInstanceDefs{{ID: 5, MachineName: "cover_image"}}},
	}
	got := s.Without("cover_image")
	if len(got) != 2 || got[0].Body == nil || len(got[1].Fields) != 1 || got[1].Fields[0].MachineName != "year" {
		t.Fatalf("Without = %+v", got)
	}
	if len(s[2].Fields) != 2 {
		t.Fatal("Without must not change the original sections")
	}
}
