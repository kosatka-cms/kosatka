//go:build dbintegration

package viewquery_test

import (
	"context"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/viewquery"
	"github.com/kosatka-cms/kosatka/internal/testdb"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// TestRunViewEmptyFilters - the empty/not_empty filters by a single-value
// field and by a multi-value one (a node with two terms is listed once).
func TestRunViewEmptyFilters(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		now := time.Now()

		storage := func(name string, typ entity.ContentTypeFieldType, config map[string]any) entity.FieldStorageDef {
			st, err := client.FieldStorageDef.Create().SetMachineName(name).SetFieldType(string(typ)).SetConfig(config).
				SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return entity.FieldStorageDef{ID: st.ID, MachineName: name, FieldType: typ, Config: config}
		}
		subtitle := storage("field_subtitle", entity.FieldTypeText, map[string]any{})
		tags := storage("field_tags", entity.FieldTypeTaxonomy, map[string]any{"multiple": true})

		voc, err := client.TaxonomyVocabulary.Create().SetName("Tags").SetSlug("tags").SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var terms []int
		for _, name := range []string{"a", "b"} {
			term, err := client.TaxonomyTermData.Create().SetVid(voc.ID).SetName(name).SetSlug(name).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			terms = append(terms, term.ID)
		}

		// Node 1: a subtitle and two tags; node 2: a subtitle; node 3: nothing.
		for i, slug := range []string{"one", "two", "three"} {
			seedNode(t, db, i+1, "article", slug, "article/"+slug)
		}
		for _, n := range []int{1, 2} {
			if err := client.FieldValueText.Create().SetNodeID(n).SetFieldID(subtitle.ID).SetValue("sub").Exec(ctx); err != nil {
				t.Fatal(err)
			}
		}
		for _, term := range terms {
			if err := client.FieldValueTaxonomyMulti.Create().SetNodeID(1).SetFieldID(tags.ID).SetTermID(term).Exec(ctx); err != nil {
				t.Fatal(err)
			}
		}

		repo := viewquery.New(client)
		run := func(f entity.FieldStorageDef, op entity.ViewFilterOperator) []int {
			t.Helper()
			id := f.ID
			nodes, total, err := repo.RunView(ctx, entity.ViewRunParams{
				Filters:       entity.ViewFilterDefs{{FieldStorageID: &id, Operator: op}},
				FieldStorages: map[int]entity.FieldStorageDef{f.ID: f},
				PageSize:      10,
			})
			if err != nil {
				t.Fatal(err)
			}
			ids := make([]int, 0, len(nodes))
			for _, n := range nodes {
				ids = append(ids, n.ID)
			}
			slices.Sort(ids)
			if total != len(ids) {
				t.Errorf("%s %s: total %d, listed %v", f.MachineName, op, total, ids)
			}
			return ids
		}
		for _, tc := range []struct {
			field entity.FieldStorageDef
			op    entity.ViewFilterOperator
			want  []int
		}{
			{subtitle, entity.ViewFilterOperatorNotEmpty, []int{1, 2}},
			{subtitle, entity.ViewFilterOperatorEmpty, []int{3}},
			{tags, entity.ViewFilterOperatorNotEmpty, []int{1}},
			{tags, entity.ViewFilterOperatorEmpty, []int{2, 3}},
		} {
			if got := run(tc.field, tc.op); !slices.Equal(got, tc.want) {
				t.Errorf("%s %s: %v, want %v", tc.field.MachineName, tc.op, got, tc.want)
			}
		}
	})
}

// TestRunViewSortByViews - a View sorted by views (all time or today) and
// the admin's content list sorted by views order the nodes by their
// counter, a node never viewed counting 0, on both databases (ent's
// counter ordering failed on PostgreSQL).
func TestRunViewSortByViews(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		views := map[int]int{1: 50, 2: 500} // node 3: never viewed
		for i, slug := range []string{"one", "two", "three"} {
			seedNode(t, db, i+1, "article", slug, "article/"+slug)
		}
		for nid, n := range views {
			if err := client.NodeCounter.Create().SetNid(nid).SetTotalcount(n).SetDaycount(n).SetLastViewed(time.Now()).Exec(ctx); err != nil {
				t.Fatal(err)
			}
		}
		ids := func(nodes entity.Nodes) []int {
			out := make([]int, 0, len(nodes))
			for _, n := range nodes {
				out = append(out, n.ID)
			}
			return out
		}
		repo := viewquery.New(client)
		for _, tc := range []struct {
			dir  string
			want []int
		}{{"desc", []int{2, 1, 3}}, {"asc", []int{3, 1, 2}}} {
			nodes, total, err := repo.RunView(ctx, entity.ViewRunParams{
				Sorts:    []entity.ViewRunSort{{Builtin: entity.ViewSortBuiltinViews, Direction: tc.dir}},
				PageSize: 10,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := ids(nodes); total != 3 || !slices.Equal(got, tc.want) {
				t.Errorf("views %s: %v (total %d), want %v", tc.dir, got, total, tc.want)
			}
		}
		today, _, err := repo.RunView(ctx, entity.ViewRunParams{
			Sorts:    []entity.ViewRunSort{{Builtin: entity.ViewSortBuiltinViewsToday, Direction: "desc"}},
			PageSize: 10,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := ids(today); !slices.Equal(got, []int{2, 1}) {
			t.Errorf("views today: %v, want [2 1]", got)
		}
		list, _, err := database.NewNodeRepository(client).ListForAdmin(ctx, entity.AdminNodesFilter{SortBy: entity.AdminNodesSortViews, Desc: true, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if got := ids(*list); !slices.Equal(got, []int{2, 1, 3}) {
			t.Errorf("admin list by views: %v, want [2 1 3]", got)
		}
	})
}

// TestRunViewSortByViewsTodayAndMore - "Page views today" with another
// criterion orders by today's views first, the other criterion breaking
// ties, and leaves out the nodes not viewed today (a stale count from a
// previous day too), as it does alone.
func TestRunViewSortByViewsTodayAndMore(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		for i, title := range []string{"apple", "banana", "cherry", "date", "elder"} {
			seedNode(t, db, i+1, "article", title, "article/"+title)
		}
		now, yesterday := time.Now(), database.TodayStartSite().Add(-time.Hour)
		for _, c := range []struct {
			nid, day int
			at       time.Time
		}{{1, 5, now}, {2, 5, now}, {3, 9, now}, {4, 99, yesterday}} { // node 5: never viewed
			if err := client.NodeCounter.Create().SetNid(c.nid).SetTotalcount(c.day).SetDaycount(c.day).SetLastViewed(c.at).Exec(ctx); err != nil {
				t.Fatal(err)
			}
		}
		nodes, total, err := viewquery.New(client).RunView(ctx, entity.ViewRunParams{
			Sorts: []entity.ViewRunSort{
				{Builtin: entity.ViewSortBuiltinViewsToday, Direction: "desc"},
				{Builtin: entity.ViewSortBuiltinTitle, Direction: "desc"},
			},
			PageSize: 10,
		})
		if err != nil {
			t.Fatal(err)
		}
		got := make([]int, 0, len(nodes))
		for _, n := range nodes {
			got = append(got, n.ID)
		}
		if want := []int{3, 2, 1}; total != 3 || !slices.Equal(got, want) {
			t.Errorf("views today, then title: %v (total %d), want %v", got, total, want)
		}
	})
}

// TestRunViewSortByViewDays - "Page views, 7/30 days" order by the views
// of those days (today included), alone in SQL or with another criterion
// in Go, and leave out the nodes not viewed in them.
func TestRunViewSortByViewDays(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		for i, title := range []string{"apple", "banana", "cherry", "date"} {
			seedNode(t, db, i+1, "article", title, "article/"+title)
		}
		for _, v := range []struct{ nid, ago, views int }{
			{1, 0, 5}, {2, 3, 8}, {2, 6, 1}, {3, 20, 50}, {3, 40, 500}, // node 4: never viewed
		} {
			day := entity.ViewDay(time.Now().AddDate(0, 0, -v.ago))
			if err := client.NodeViewDay.Create().SetNid(v.nid).SetDay(day).SetViews(v.views).Exec(ctx); err != nil {
				t.Fatal(err)
			}
		}
		for _, tc := range []struct {
			name  string
			sorts []entity.ViewRunSort
			want  []int
		}{
			{"7 days", []entity.ViewRunSort{{Builtin: entity.ViewSortBuiltinViewsWeek, Direction: "desc"}}, []int{2, 1}},
			{"7 days asc", []entity.ViewRunSort{{Builtin: entity.ViewSortBuiltinViewsWeek, Direction: "asc"}}, []int{1, 2}},
			{"30 days", []entity.ViewRunSort{{Builtin: entity.ViewSortBuiltinViewsMonth, Direction: "desc"}}, []int{3, 2, 1}},
			{"30 days, then title", []entity.ViewRunSort{
				{Builtin: entity.ViewSortBuiltinViewsMonth, Direction: "desc"},
				{Builtin: entity.ViewSortBuiltinTitle, Direction: "asc"},
			}, []int{3, 2, 1}},
		} {
			nodes, total, err := viewquery.New(client).RunView(ctx, entity.ViewRunParams{Sorts: tc.sorts, PageSize: 10})
			if err != nil {
				t.Fatal(err)
			}
			got := make([]int, 0, len(nodes))
			for _, n := range nodes {
				got = append(got, n.ID)
			}
			if total != len(tc.want) || !slices.Equal(got, tc.want) {
				t.Errorf("%s: %v (total %d), want %v", tc.name, got, total, tc.want)
			}
		}
	})
}

// TestRunViewPromoteSticky - the front page's View: the promoted nodes
// (filter "promote"), the sticky first, each group the newest first, in
// one SQL ORDER BY; a page of it stays in that order.
func TestRunViewPromoteSticky(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
		// id → promote, sticky, days after base.
		for id, n := range map[int]struct {
			promote, sticky bool
			days            int
		}{1: {true, false, 1}, 2: {true, true, 0}, 3: {false, true, 3}, 4: {true, false, 2}, 5: {true, true, 4}} {
			seedNode(t, db, id, "article", "n", "article/n"+string(rune('0'+id)))
			if err := client.Node.UpdateOneID(id).SetPromote(n.promote).SetSticky(n.sticky).SetCreatedAt(base.AddDate(0, 0, n.days)).Exec(ctx); err != nil {
				t.Fatal(err)
			}
		}
		repo := viewquery.New(client)
		run := func(page int) ([]int, int) {
			t.Helper()
			nodes, total, err := repo.RunView(ctx, entity.ViewRunParams{
				Filters: entity.ViewFilterDefs{{BuiltinField: entity.ViewFilterBuiltinPromote, Operator: entity.ViewFilterOperatorEq, Value: "1"}},
				Sorts: []entity.ViewRunSort{
					{Builtin: entity.ViewSortBuiltinSticky, Direction: "desc"},
					{Builtin: entity.ViewSortBuiltinCreatedAt, Direction: "desc"},
				},
				Page:     page,
				PageSize: 3,
			})
			if err != nil {
				t.Fatal(err)
			}
			ids := make([]int, 0, len(nodes))
			for _, n := range nodes {
				ids = append(ids, n.ID)
			}
			return ids, total
		}
		if got, total := run(1); total != 4 || !slices.Equal(got, []int{5, 2, 4}) {
			t.Errorf("page 1: %v of %d, want [5 2 4] of 4 (node 3 isn't promoted)", got, total)
		}
		if got, _ := run(2); !slices.Equal(got, []int{1}) {
			t.Errorf("page 2: %v, want [1]", got)
		}
	})
}

// TestRunViewDateFilterWithTimes - a date filter by a date covers its
// whole day: "equal" and "not later" take a start with a time on it, the
// end of a range doesn't matter.
func TestRunViewDateFilterWithTimes(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		now := time.Now()
		config := map[string]any{"date_time": "optional", "date_end": "optional"}
		st, err := client.FieldStorageDef.Create().SetMachineName("field_when").SetFieldType(string(entity.FieldTypeDate)).SetConfig(config).
			SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		when := entity.FieldStorageDef{ID: st.ID, MachineName: "field_when", FieldType: entity.FieldTypeDate, Config: config}
		day := func(d, h int) time.Time { return time.Date(2026, 11, d, h, 0, 0, 0, time.UTC) }
		// 1: the 12th at 19:00; 2: the 12th, the whole day; 3: the 13th to the 15th.
		for i, v := range []struct {
			start, end time.Time
			withTime   bool
		}{{day(12, 19), time.Time{}, true}, {day(12, 0), time.Time{}, false}, {day(13, 0), day(15, 0), false}} {
			seedNode(t, db, i+1, "event", "e", "event/"+string(rune('a'+i)))
			create := client.FieldValueDate.Create().SetNodeID(i + 1).SetFieldID(st.ID).SetValue(v.start).SetWithTime(v.withTime)
			if !v.end.IsZero() {
				create = create.SetEndValue(v.end)
			}
			if err := create.Exec(ctx); err != nil {
				t.Fatal(err)
			}
		}
		repo := viewquery.New(client)
		for _, tc := range []struct {
			op   entity.ViewFilterOperator
			want []int
		}{
			{entity.ViewFilterOperatorEq, []int{1, 2}},
			{entity.ViewFilterOperatorLte, []int{1, 2}},
			{entity.ViewFilterOperatorGte, []int{1, 2, 3}},
		} {
			id := when.ID
			nodes, _, err := repo.RunView(ctx, entity.ViewRunParams{
				Filters:       entity.ViewFilterDefs{{FieldStorageID: &id, Operator: tc.op, Value: "2026-11-12"}},
				FieldStorages: map[int]entity.FieldStorageDef{when.ID: when},
				PageSize:      10,
			})
			if err != nil {
				t.Fatal(err)
			}
			var ids []int
			for _, n := range nodes {
				ids = append(ids, n.ID)
			}
			slices.Sort(ids)
			if !slices.Equal(ids, tc.want) {
				t.Errorf("%s 2026-11-12: %v, want %v", tc.op, ids, tc.want)
			}
		}
	})
}

// TestRunViewDateFilterSiteDay - a start with a time is on the site's day:
// at UTC+3, 22:30 UTC on the 12th is the 13th; a date alone stays its day.
func TestRunViewDateFilterSiteDay(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		if err := sitetime.Set("Europe/Moscow"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = sitetime.Set("UTC") })
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		now := time.Now()
		config := map[string]any{"date_time": "optional"}
		st, err := client.FieldStorageDef.Create().SetMachineName("field_when").SetFieldType(string(entity.FieldTypeDate)).SetConfig(config).
			SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		when := entity.FieldStorageDef{ID: st.ID, MachineName: "field_when", FieldType: entity.FieldTypeDate, Config: config}
		// 1: 22:30 UTC on the 12th (01:30 on the 13th at the site);
		// 2: 19:00 UTC on the 12th (22:00 at the site); 3: the 13th.
		for i, v := range []struct {
			start    time.Time
			withTime bool
		}{{time.Date(2026, 11, 12, 22, 30, 0, 0, time.UTC), true}, {time.Date(2026, 11, 12, 19, 0, 0, 0, time.UTC), true}, {time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC), false}} {
			seedNode(t, db, i+1, "event", "e", "event/"+string(rune('a'+i)))
			if err := client.FieldValueDate.Create().SetNodeID(i + 1).SetFieldID(st.ID).SetValue(v.start).SetWithTime(v.withTime).Exec(ctx); err != nil {
				t.Fatal(err)
			}
		}
		repo := viewquery.New(client)
		for _, tc := range []struct {
			op    entity.ViewFilterOperator
			value string
			want  []int
		}{
			{entity.ViewFilterOperatorEq, "2026-11-12", []int{2}},
			{entity.ViewFilterOperatorEq, "2026-11-13", []int{1, 3}},
			{entity.ViewFilterOperatorLte, "2026-11-12", []int{2}},
			{entity.ViewFilterOperatorGte, "2026-11-13", []int{1, 3}},
		} {
			id := when.ID
			nodes, _, err := repo.RunView(ctx, entity.ViewRunParams{
				Filters:       entity.ViewFilterDefs{{FieldStorageID: &id, Operator: tc.op, Value: tc.value}},
				FieldStorages: map[int]entity.FieldStorageDef{when.ID: when},
				PageSize:      10,
			})
			if err != nil {
				t.Fatal(err)
			}
			var ids []int
			for _, n := range nodes {
				ids = append(ids, n.ID)
			}
			slices.Sort(ids)
			if !slices.Equal(ids, tc.want) {
				t.Errorf("%s %s: %v, want %v", tc.op, tc.value, ids, tc.want)
			}
		}
	})
}

// TestRunViewDateStates - a date field's "going on", "not ended" and
// "ended" by now (several dates: any going on / not ended, all ended),
// and the sort by the nearest date not ended (all ended - the latest).
func TestRunViewDateStates(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		now := time.Now()
		config := map[string]any{"multiple": true, "date_time": "optional", "date_end": "optional"}
		st, err := client.FieldStorageDef.Create().SetMachineName("field_shows").SetFieldType(string(entity.FieldTypeDate)).SetConfig(config).
			SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		shows := entity.FieldStorageDef{ID: st.ID, MachineName: "field_shows", FieldType: entity.FieldTypeDate, Config: config}
		today := entity.Today(now, time.UTC)
		type date struct {
			start, end time.Time
			withTime   bool
		}
		// 1: ended days; 2: going on (an hour ago to an hour on); 3: ended
		// and one in 10 days; 4: today (a date); 5: in 3 days.
		nodes := map[int][]date{
			1: {{today.AddDate(0, 0, -5), today.AddDate(0, 0, -3), false}},
			2: {{now.Add(-time.Hour), now.Add(time.Hour), true}},
			3: {{today.AddDate(0, 0, -20), time.Time{}, false}, {today.AddDate(0, 0, 10), time.Time{}, false}},
			4: {{today, time.Time{}, false}},
			5: {{now.AddDate(0, 0, 3), time.Time{}, true}},
		}
		for id := 1; id <= 5; id++ {
			seedNode(t, db, id, "event", "e", "event/"+strconv.Itoa(id))
			for delta, d := range nodes[id] {
				create := client.FieldValueDate.Create().SetNodeID(id).SetFieldID(st.ID).SetValue(d.start).SetWithTime(d.withTime).SetDelta(delta)
				if !d.end.IsZero() {
					create = create.SetEndValue(d.end)
				}
				if err := create.Exec(ctx); err != nil {
					t.Fatal(err)
				}
			}
		}
		repo := viewquery.New(client)
		run := func(p entity.ViewRunParams) []int {
			t.Helper()
			p.FieldStorages, p.PageSize = map[int]entity.FieldStorageDef{shows.ID: shows}, 10
			got, _, err := repo.RunView(ctx, p)
			if err != nil {
				t.Fatal(err)
			}
			var ids []int
			for _, n := range got {
				ids = append(ids, n.ID)
			}
			return ids
		}
		id := shows.ID
		for _, tc := range []struct {
			op   entity.ViewFilterOperator
			want []int
		}{
			{entity.ViewFilterOperatorOngoing, []int{2, 4}},
			{entity.ViewFilterOperatorNotEnded, []int{2, 3, 4, 5}},
			{entity.ViewFilterOperatorEnded, []int{1}},
		} {
			got := run(entity.ViewRunParams{Filters: entity.ViewFilterDefs{{FieldStorageID: &id, Operator: tc.op}}})
			slices.Sort(got)
			if !slices.Equal(got, tc.want) {
				t.Errorf("%s: %v, want %v", tc.op, got, tc.want)
			}
		}
		// Not ended, by the nearest start: 4 (today's midnight) and 2 (an
		// hour ago) by their instants, then 5, then 3 (its date to come).
		want := []int{4, 2, 5, 3}
		if now.Add(-time.Hour).Before(today) {
			want = []int{2, 4, 5, 3}
		}
		got := run(entity.ViewRunParams{
			Filters: entity.ViewFilterDefs{{FieldStorageID: &id, Operator: entity.ViewFilterOperatorNotEnded}},
			Sorts:   []entity.ViewRunSort{{FieldStorage: &shows, Direction: "asc"}},
		})
		if !slices.Equal(got, want) {
			t.Errorf("sorted by the nearest: %v, want %v", got, want)
		}
	})
}

// TestRunViewProfileUser - a profile tab's View: the content naming the
// profile's user in a user field; without a user (no profile) - none; a
// deleted user leaves the field.
func TestRunViewProfileUser(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		now := time.Now()
		if _, err := client.User.Create().SetID(2).SetUsername("bob").SetSlug("bob").SetEmail("bob@example.com").SetPasswordHash("x").Save(ctx); err != nil {
			t.Fatal(err)
		}
		st, err := client.FieldStorageDef.Create().SetMachineName("field_members").SetFieldType(string(entity.FieldTypeUser)).
			SetConfig(map[string]any{"multiple": true}).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		members := entity.FieldStorageDef{ID: st.ID, MachineName: "field_members", FieldType: entity.FieldTypeUser, Config: map[string]any{"multiple": true}}
		// 1: alice and bob; 2: bob; 3: nobody.
		for id, users := range map[int][]int{1: {1, 2}, 2: {2}, 3: nil} {
			seedNode(t, db, id, "project", "p", "project/"+strconv.Itoa(id))
			for delta, uid := range users {
				if err := client.FieldValueUser.Create().SetNodeID(id).SetFieldID(st.ID).SetUserID(uid).SetDelta(delta).Exec(ctx); err != nil {
					t.Fatal(err)
				}
			}
		}
		repo := viewquery.New(client)
		run := func(userID int) []int {
			t.Helper()
			id := members.ID
			got, _, err := repo.RunView(ctx, entity.ViewRunParams{
				Filters:            entity.ViewFilterDefs{{FieldStorageID: &id, Operator: entity.ViewFilterOperatorIsProfileUser, Contextual: true}},
				FieldStorages:      map[int]entity.FieldStorageDef{members.ID: members},
				ContextCreatedByID: userID, PageSize: 10,
			})
			if err != nil {
				t.Fatal(err)
			}
			var ids []int
			for _, n := range got {
				ids = append(ids, n.ID)
			}
			slices.Sort(ids)
			return ids
		}
		if got := run(2); !slices.Equal(got, []int{1, 2}) {
			t.Errorf("bob's: %v", got)
		}
		if got := run(1); !slices.Equal(got, []int{1}) {
			t.Errorf("alice's: %v", got)
		}
		if got := run(0); len(got) != 0 {
			t.Errorf("no profile: %v", got)
		}
		if err := client.User.DeleteOneID(2).Exec(ctx); err != nil {
			t.Fatal(err)
		}
		if n, _ := client.FieldValueUser.Query().Count(ctx); n != 1 {
			t.Errorf("rows after bob's deletion: %d, want alice's 1", n)
		}
	})
}

// TestRunCommentViewPublicNodesOnly - a comment View leaves out the
// comments on unpublished content and content on moderation.
func TestRunCommentViewPublicNodesOnly(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		seedNode(t, db, 1, "article", "Public", "public")
		seedNode(t, db, 2, "article", "Draft", "draft")
		seedNode(t, db, 3, "article", "Waiting", "waiting")
		for _, q := range []string{
			`UPDATE nodes SET status = 0 WHERE id = 2`,
			`UPDATE nodes SET on_moderation = 1 WHERE id = 3`,
			`INSERT INTO comments (id, nid, content, created_by, status) VALUES (1, 1, 'a', 1, 1), (2, 2, 'b', 1, 1), (3, 3, 'c', 1, 1)`,
		} {
			if _, err := db.DB.Exec(q); err != nil {
				t.Fatal(err)
			}
		}
		got, total, err := viewquery.New(entClient(db)).RunCommentView(ctx, entity.ViewRunParams{})
		if err != nil || total != 1 || len(got) != 1 || got[0].ID != 1 {
			t.Errorf("comments: %d %+v %v", total, got, err)
		}
	})
}

// TestRunCommentViewStatus - without a status filter a comment View
// lists the published comments only (not those waiting for approval nor
// spam); with one, what it asks.
func TestRunCommentViewStatus(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		seedNode(t, db, 1, "article", "Public", "public")
		if _, err := db.DB.Exec(`INSERT INTO comments (id, nid, content, created_by, status) VALUES (1, 1, 'a', 1, 1), (2, 1, 'b', 1, 0), (3, 1, 'c', 1, 2)`); err != nil {
			t.Fatal(err)
		}
		engine := viewquery.New(entClient(db))
		if got, total, err := engine.RunCommentView(ctx, entity.ViewRunParams{}); err != nil || total != 1 || got[0].ID != 1 {
			t.Errorf("without a filter: %d %+v %v", total, got, err)
		}
		waiting := entity.ViewFilterDefs{{BuiltinField: entity.ViewFilterBuiltinCommentStatus, Operator: entity.ViewFilterOperatorEq, Value: "0"}}
		if got, total, err := engine.RunCommentView(ctx, entity.ViewRunParams{Filters: waiting}); err != nil || total != 1 || got[0].ID != 2 {
			t.Errorf("status 0: %d %+v %v", total, got, err)
		}
	})
}

// TestRunViewListFilter - a list field's filters, one value and several:
// eq one option, in any of several, not_in none of them, empty and
// not_empty.
func TestRunViewListFilter(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		now := time.Now()
		storage := func(name string, multiple bool) entity.FieldStorageDef {
			t.Helper()
			config := map[string]any{"multiple": multiple}
			st, err := client.FieldStorageDef.Create().SetMachineName(name).SetFieldType(string(entity.FieldTypeList)).SetConfig(config).
				SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return entity.FieldStorageDef{ID: st.ID, MachineName: name, FieldType: entity.FieldTypeList, Config: config}
		}
		verdict, moods := storage("field_verdict", false), storage("field_moods", true)
		// 1: great, calm; 2: fine, calm + tense; 3: masterpiece; 4: nothing.
		for i, v := range []struct {
			verdict string
			moods   []string
		}{{"great", []string{"calm"}}, {"fine", []string{"calm", "tense"}}, {"masterpiece", nil}, {"", nil}} {
			seedNode(t, db, i+1, "review", "r", "review/"+string(rune('a'+i)))
			if v.verdict != "" {
				if err := client.FieldValueList.Create().SetNodeID(i + 1).SetFieldID(verdict.ID).SetValue(v.verdict).Exec(ctx); err != nil {
					t.Fatal(err)
				}
			}
			for _, m := range v.moods {
				if err := client.FieldValueListMulti.Create().SetNodeID(i + 1).SetFieldID(moods.ID).SetValue(m).Exec(ctx); err != nil {
					t.Fatal(err)
				}
			}
		}
		repo := viewquery.New(client)
		for _, tc := range []struct {
			fs    entity.FieldStorageDef
			op    entity.ViewFilterOperator
			value string
			want  []int
		}{
			{verdict, entity.ViewFilterOperatorEq, "great", []int{1}},
			{verdict, entity.ViewFilterOperatorIn, "great, masterpiece", []int{1, 3}},
			{verdict, entity.ViewFilterOperatorNotIn, "great", []int{2, 3, 4}},
			{verdict, entity.ViewFilterOperatorEmpty, "", []int{4}},
			{moods, entity.ViewFilterOperatorEq, "calm", []int{1, 2}},
			{moods, entity.ViewFilterOperatorIn, "tense", []int{2}},
			{moods, entity.ViewFilterOperatorNotEmpty, "", []int{1, 2}},
		} {
			id := tc.fs.ID
			nodes, _, err := repo.RunView(ctx, entity.ViewRunParams{
				Filters:       entity.ViewFilterDefs{{FieldStorageID: &id, Operator: tc.op, Value: tc.value}},
				FieldStorages: map[int]entity.FieldStorageDef{tc.fs.ID: tc.fs},
				PageSize:      10,
			})
			if err != nil {
				t.Fatal(err)
			}
			var ids []int
			for _, n := range nodes {
				ids = append(ids, n.ID)
			}
			slices.Sort(ids)
			if !slices.Equal(ids, tc.want) {
				t.Errorf("%s %s %q: %v, want %v", tc.fs.MachineName, tc.op, tc.value, ids, tc.want)
			}
		}
	})
}
