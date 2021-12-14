package queries

import (
	"testing"
)

func matches(t *testing.T, ls Set, want string) {
	if ls.String() != want {
		t.Errorf("Expected '%s', but got '%s'", want, ls.String())
	}
}

func TestSetString(t *testing.T) {
	matches(t, Set{"x": "y"}, "x=y")
	matches(t, Set{"foo": "bar"}, "foo=bar")
	matches(t, Set{"foo": "bar", "baz": "qup"}, "baz=qup,foo=bar")

	// TODO: Make our query representation robust enough to handle queries
	// with ",=!" characters in their names.
}

func TestQueryHas(t *testing.T) {
	queryHasTests := []struct {
		Ls  Queries
		Key string
		Has bool
	}{
		{Set{"x": "y"}, "x", true},
		{Set{"x": ""}, "x", true},
		{Set{"x": "y"}, "foo", false},
	}
	for _, lh := range queryHasTests {
		if has := lh.Ls.Has(lh.Key); has != lh.Has {
			t.Errorf("%#v.Has(%#v) => %v, expected %v", lh.Ls, lh.Key, has, lh.Has)
		}
	}
}

func TestQueryGet(t *testing.T) {
	ls := Set{"x": "y"}
	if ls.Get("x") != "y" {
		t.Errorf("Set.Get is broken")
	}
}

func TestQueryConflict(t *testing.T) {
	tests := []struct {
		queries1 map[string]string
		queries2 map[string]string
		conflict bool
	}{
		{
			queries1: map[string]string{},
			queries2: map[string]string{},
			conflict: false,
		},
		{
			queries1: map[string]string{"env": "test"},
			queries2: map[string]string{"infra": "true"},
			conflict: false,
		},
		{
			queries1: map[string]string{"env": "test"},
			queries2: map[string]string{"infra": "true", "env": "test"},
			conflict: false,
		},
		{
			queries1: map[string]string{"env": "test"},
			queries2: map[string]string{"env": "dev"},
			conflict: true,
		},
		{
			queries1: map[string]string{"env": "test", "infra": "false"},
			queries2: map[string]string{"infra": "true", "color": "blue"},
			conflict: true,
		},
	}
	for _, test := range tests {
		conflict := Conflicts(Set(test.queries1), Set(test.queries2))
		if conflict != test.conflict {
			t.Errorf("expected: %v but got: %v", test.conflict, conflict)
		}
	}
}

func TestQueryMerge(t *testing.T) {
	tests := []struct {
		queries1     map[string]string
		queries2     map[string]string
		mergedQuerys map[string]string
	}{
		{
			queries1:     map[string]string{},
			queries2:     map[string]string{},
			mergedQuerys: map[string]string{},
		},
		{
			queries1:     map[string]string{"infra": "true"},
			queries2:     map[string]string{},
			mergedQuerys: map[string]string{"infra": "true"},
		},
		{
			queries1:     map[string]string{"infra": "true"},
			queries2:     map[string]string{"env": "test", "color": "blue"},
			mergedQuerys: map[string]string{"infra": "true", "env": "test", "color": "blue"},
		},
	}
	for _, test := range tests {
		mergedQuerys := Merge(Set(test.queries1), Set(test.queries2))
		if !Equals(mergedQuerys, test.mergedQuerys) {
			t.Errorf("expected: %v but got: %v", test.mergedQuerys, mergedQuerys)
		}
	}
}

func TestQuerySelectorParse(t *testing.T) {
	tests := []struct {
		selector string
		queries  map[string]string
		valid    bool
	}{
		{
			selector: "",
			queries:  map[string]string{},
			valid:    true,
		},
		{
			selector: "x=a",
			queries:  map[string]string{"x": "a"},
			valid:    true,
		},
		{
			selector: "x=a,y=b,z=c",
			queries:  map[string]string{"x": "a", "y": "b", "z": "c"},
			valid:    true,
		},
		{
			selector: " x = a , y = b , z = c ",
			queries:  map[string]string{"x": "a", "y": "b", "z": "c"},
			valid:    true,
		},
		{
			selector: "color=green,env=test,service=front",
			queries:  map[string]string{"color": "green", "env": "test", "service": "front"},
			valid:    true,
		},
		{
			selector: "color=green, env=test, service=front",
			queries:  map[string]string{"color": "green", "env": "test", "service": "front"},
			valid:    true,
		},
		{
			selector: ",",
			queries:  map[string]string{},
			valid:    false,
		},
		{
			selector: "x",
			queries:  map[string]string{},
			valid:    false,
		},
		{
			selector: "x,y",
			queries:  map[string]string{},
			valid:    false,
		},
		{
			selector: "x=$y",
			queries:  map[string]string{},
			valid:    false,
		},
		{
			selector: "x!=y",
			queries:  map[string]string{},
			valid:    false,
		},
		{
			selector: "x==y",
			queries:  map[string]string{},
			valid:    false,
		},
		{
			selector: "x=a||y=b",
			queries:  map[string]string{},
			valid:    false,
		},
		{
			selector: "x in (y)",
			queries:  map[string]string{},
			valid:    false,
		},
		{
			selector: "x notin (y)",
			queries:  map[string]string{},
			valid:    false,
		},
		{
			selector: "x y",
			queries:  map[string]string{},
			valid:    false,
		},
	}
	for _, test := range tests {
		queries, err := ConvertSelectorToQueriesMap(test.selector)
		if test.valid && err != nil {
			t.Errorf("selector: %s, expected no error but got: %s", test.selector, err)
		} else if !test.valid && err == nil {
			t.Errorf("selector: %s, expected an error", test.selector)
		}

		if !Equals(Set(queries), test.queries) {
			t.Errorf("expected: %s but got: %s", test.queries, queries)
		}
	}
}
