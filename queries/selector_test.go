package queries

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"github.com/yubo/golib/selection"
	"github.com/yubo/golib/util/sets"
	"github.com/yubo/golib/util/validation/field"
)

var (
	ignoreDetail = cmpopts.IgnoreFields(field.Error{}, "Detail")
)

func TestSelectorParse(t *testing.T) {
	testGoodStrings := []string{
		"x=a,y=b,z=c",
		"",
		"x!=a,y=b",
		"x=",
		"x=true",
		"x= ",
		"x=,z= ",
		"x= ,z= ",
		"x>1",
		"x=~2",
		"x!~2",
		"x !~ 2,x =~ 3",
		"x>1,z<5",
	}
	testBadStrings := []string{
		"x=a||y=b",
		"x==a==b",
		"!x",
		"!x=a",
		"x<a",
	}
	for _, test := range testGoodStrings {
		lq, err := Parse(test)
		if err != nil {
			t.Errorf("%v: error %v (%#v)\n", test, err, err)
		}
		if strings.Replace(test, " ", "", -1) != lq.String() {
			t.Errorf("%v restring gave: %v\n", test, lq.String())
		}
	}
	for _, test := range testBadStrings {
		_, err := Parse(test)
		if err == nil {
			t.Errorf("%v: did not get expected error\n", test)
		}
	}
}

func TestDeterministicParse(t *testing.T) {
	s, err := Parse("x=a,a=x")
	require.NoError(t, err)
	require.Equal(t, "x=a,a=x", s.String())

	s, err = Parse("a=x,x=a")
	require.NoError(t, err)
	require.Equal(t, "a=x,x=a", s.String())

}

func expectMatch(t *testing.T, selector string, ls Set) {
	lq, err := Parse(selector)
	if err != nil {
		t.Errorf("Unable to parse %v as a selector\n", selector)
		return
	}
	if !lq.Matches(ls) {
		t.Errorf("Wanted %s to match '%s', but it did not.\n", selector, ls)
	}
}

func expectNoMatch(t *testing.T, selector string, ls Set) {
	lq, err := Parse(selector)
	if err != nil {
		t.Errorf("Unable to parse %v as a selector\n", selector)
		return
	}
	if lq.Matches(ls) {
		t.Errorf("Wanted '%s' to not match '%s', but it did.", selector, ls)
	}
}

func TestEverything(t *testing.T) {
	if !Everything().Matches(Set{"x": "y"}) {
		t.Errorf("Nil selector didn't match")
	}
	if !Everything().Empty() {
		t.Errorf("Everything was not empty")
	}
}

func TestSelectorMatches(t *testing.T) {
	//expectMatch(t, "", Set{"x": "y"})
	expectMatch(t, "x=y", Set{"x": "y"})
	expectMatch(t, "x=y,z=w", Set{"x": "y", "z": "w"})
	expectMatch(t, "x!=y,z!=w", Set{"x": "z", "z": "a"})
	expectMatch(t, "notin=in", Set{"notin": "in"}) // in and notin in exactMatch
	//expectMatch(t, "x", Set{"x": "z"})
	//expectMatch(t, "!x", Set{"y": "z"})
	expectMatch(t, "x>1", Set{"x": "2"})
	expectMatch(t, "x<1", Set{"x": "0"})
	expectNoMatch(t, "x=z", Set{})
	expectNoMatch(t, "x=y", Set{"x": "z"})
	expectNoMatch(t, "x=y,z=w", Set{"x": "w", "z": "w"})
	expectNoMatch(t, "x!=y,z!=w", Set{"x": "z", "z": "w"})
	//expectNoMatch(t, "x", Set{"y": "z"})
	//expectNoMatch(t, "!x", Set{"x": "z"})
	expectNoMatch(t, "x>1", Set{"x": "0"})
	expectNoMatch(t, "x<1", Set{"x": "2"})

	queryset := Set{
		"foo": "bar",
		"baz": "blah",
	}
	expectMatch(t, "foo=bar", queryset)
	expectMatch(t, "baz=blah", queryset)
	expectMatch(t, "foo=bar,baz=blah", queryset)
	expectNoMatch(t, "foo=blah", queryset)
	expectNoMatch(t, "baz=bar", queryset)
	expectNoMatch(t, "foo=bar,foobar=bar,baz=blah", queryset)
}

func expectMatchDirect(t *testing.T, selector, ls Set) {
	if !SelectorFromSet(selector).Matches(ls) {
		t.Errorf("Wanted %s to match '%s', but it did not.\n", selector, ls)
	}
}

//lint:ignore U1000 currently commented out in TODO of TestSetMatches
func expectNoMatchDirect(t *testing.T, selector, ls Set) {
	if SelectorFromSet(selector).Matches(ls) {
		t.Errorf("Wanted '%s' to not match '%s', but it did.", selector, ls)
	}
}

func TestSetMatches(t *testing.T) {
	queryset := Set{
		"foo": "bar",
		"baz": "blah",
	}
	expectMatchDirect(t, Set{}, queryset)
	expectMatchDirect(t, Set{"foo": "bar"}, queryset)
	expectMatchDirect(t, Set{"baz": "blah"}, queryset)
	expectMatchDirect(t, Set{"foo": "bar", "baz": "blah"}, queryset)

	//TODO: bad values not handled for the moment in SelectorFromSet
	//expectNoMatchDirect(t, Set{"foo": "=blah"}, queryset)
	//expectNoMatchDirect(t, Set{"baz": "=bar"}, queryset)
	//expectNoMatchDirect(t, Set{"foo": "=bar", "foobar": "bar", "baz": "blah"}, queryset)
}

func TestNilMapIsValid(t *testing.T) {
	selector := Set(nil).AsSelector()
	if selector == nil {
		t.Errorf("Selector for nil set should be Everything")
	}
	if !selector.Empty() {
		t.Errorf("Selector for nil set should be Empty")
	}
}

func TestSetIsEmpty(t *testing.T) {
	if !(Set{}).AsSelector().Empty() {
		t.Errorf("Empty set should be empty")
	}
	if !(NewSelector()).Empty() {
		t.Errorf("Nil Selector should be empty")
	}
}

func TestLexer(t *testing.T) {
	cases := []struct {
		s string
		t Token
	}{
		{"", EndOfStringToken},
		{",", CommaToken},
		{"notin", NotInToken},
		{"in", InToken},
		{"=", EqualsToken},
		{"==", DoubleEqualsToken},
		{">", GreaterThanToken},
		{"<", LessThanToken},
		//Note that Lex returns the longest valid token found
		//{"!", DoesNotExistToken},
		{"!=", NotEqualsToken},
		{"(", OpenParToken},
		{")", ClosedParToken},
		//Non-"special" characters are considered part of an identifier
		{"=~", ContainsToken},
		{"!~", NotContainsToken},
		{"||", IdentifierToken},
	}
	for _, c := range cases {
		l := &Lexer{s: c.s, pos: 0}
		token, lit := l.Lex()
		require.Equal(t, c.t, token)

		if c.t != ErrorToken && lit != c.s {
			t.Errorf("Got '%s' it should be '%s'", lit, c.s)
		}
	}
}

func min(l, r int) (m int) {
	m = r
	if l < r {
		m = l
	}
	return m
}

func TestLexerSequence(t *testing.T) {
	cases := []struct {
		s string
		t []Token
	}{
		{"key in ( value )", []Token{IdentifierToken, InToken, OpenParToken, IdentifierToken, ClosedParToken}},
		{"key notin ( value )", []Token{IdentifierToken, NotInToken, OpenParToken, IdentifierToken, ClosedParToken}},
		{"key in ( value1, value2 )", []Token{IdentifierToken, InToken, OpenParToken, IdentifierToken, CommaToken, IdentifierToken, ClosedParToken}},
		{"key", []Token{IdentifierToken}},
		//{"!key", []Token{DoesNotExistToken, IdentifierToken}},
		{"()", []Token{OpenParToken, ClosedParToken}},
		{"x in (),y", []Token{IdentifierToken, InToken, OpenParToken, ClosedParToken, CommaToken, IdentifierToken}},
		{"== != (), = notin", []Token{DoubleEqualsToken, NotEqualsToken, OpenParToken, ClosedParToken, CommaToken, EqualsToken, NotInToken}},
		{"key>2", []Token{IdentifierToken, GreaterThanToken, IdentifierToken}},
		{"key<1", []Token{IdentifierToken, LessThanToken, IdentifierToken}},
	}
	for _, c := range cases {
		var tokens []Token
		l := &Lexer{s: c.s, pos: 0}
		for {
			token, _ := l.Lex()
			if token == EndOfStringToken {
				break
			}
			tokens = append(tokens, token)
		}
		if len(tokens) != len(c.t) {
			t.Errorf("Bad number of tokens for '%s %d, %d", c.s, len(tokens), len(c.t))
		}
		for i := 0; i < min(len(tokens), len(c.t)); i++ {
			if tokens[i] != c.t[i] {
				t.Errorf("Test '%s': Mismatching in token type found '%v' it should be '%v'", c.s, tokens[i], c.t[i])
			}
		}
	}
}
func TestParserLookahead(t *testing.T) {
	cases := []struct {
		s string
		t []Token
	}{
		{"key in ( value )", []Token{IdentifierToken, InToken, OpenParToken, IdentifierToken, ClosedParToken, EndOfStringToken}},
		{"key notin ( value )", []Token{IdentifierToken, NotInToken, OpenParToken, IdentifierToken, ClosedParToken, EndOfStringToken}},
		{"key in ( value1, value2 )", []Token{IdentifierToken, InToken, OpenParToken, IdentifierToken, CommaToken, IdentifierToken, ClosedParToken, EndOfStringToken}},
		{"key", []Token{IdentifierToken, EndOfStringToken}},
		//{"!key", []Token{DoesNotExistToken, IdentifierToken, EndOfStringToken}},
		{"()", []Token{OpenParToken, ClosedParToken, EndOfStringToken}},
		{"", []Token{EndOfStringToken}},
		{"x in (),y", []Token{IdentifierToken, InToken, OpenParToken, ClosedParToken, CommaToken, IdentifierToken, EndOfStringToken}},
		{"== != (), = notin", []Token{DoubleEqualsToken, NotEqualsToken, OpenParToken, ClosedParToken, CommaToken, EqualsToken, NotInToken, EndOfStringToken}},
		{"key>2", []Token{IdentifierToken, GreaterThanToken, IdentifierToken, EndOfStringToken}},
		{"key<1", []Token{IdentifierToken, LessThanToken, IdentifierToken, EndOfStringToken}},
	}
	for _, c := range cases {
		p := &Parser{l: &Lexer{s: c.s, pos: 0}, position: 0}
		p.scan()
		if len(p.scannedItems) != len(c.t) {
			t.Errorf("Expected %d items found %d", len(c.t), len(p.scannedItems))
		}
		for {
			token, lit := p.lookahead(KeyAndOperator)

			token2, lit2 := p.consume(KeyAndOperator)
			if token == EndOfStringToken {
				break
			}
			if token != token2 || lit != lit2 {
				t.Errorf("Bad values")
			}
		}
	}
}

func TestRequirementConstructor(t *testing.T) {
	cases := []struct {
		Key     string
		Op      selection.Operator
		Vals    sets.String
		WantErr field.ErrorList
	}{
		{
			Key: "x1",
			Op:  selection.In,
			WantErr: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "values",
					BadValue: []string{},
				},
			},
		},
		{
			Key:  "x2",
			Op:   selection.NotIn,
			Vals: sets.NewString(),
			WantErr: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "values",
					BadValue: []string{},
				},
			},
		},
		{
			Key:  "x3",
			Op:   selection.In,
			Vals: sets.NewString("foo"),
		},
		{
			Key:  "x4",
			Op:   selection.NotIn,
			Vals: sets.NewString("foo"),
		},
		{
			Key:  "x5",
			Op:   selection.Equals,
			Vals: sets.NewString("foo", "bar"),
			WantErr: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "values",
					BadValue: []string{"bar", "foo"},
				},
			},
		},
		//{
		//	Key: "x6",
		//	Op:  selection.Exists,
		//},
		//{
		//	Key: "x7",
		//	Op:  selection.DoesNotExist,
		//},
		//{
		//	Key:  "x8",
		//	Op:   selection.Exists,
		//	Vals: sets.NewString("foo"),
		//	WantErr: field.ErrorList{
		//		&field.Error{
		//			Type:     field.ErrorTypeInvalid,
		//			Field:    "values",
		//			BadValue: []string{"foo"},
		//		},
		//	},
		//},
		{
			Key:  "x9",
			Op:   selection.In,
			Vals: sets.NewString("bar"),
		},
		{
			Key:  "x10",
			Op:   selection.In,
			Vals: sets.NewString("bar"),
		},
		{
			Key:  "x11",
			Op:   selection.GreaterThan,
			Vals: sets.NewString("1"),
		},
		{
			Key:  "x12",
			Op:   selection.LessThan,
			Vals: sets.NewString("6"),
		},
		{
			Key: "x13",
			Op:  selection.GreaterThan,
			WantErr: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "values",
					BadValue: []string{},
				},
			},
		},
		{
			Key:  "x14",
			Op:   selection.GreaterThan,
			Vals: sets.NewString("bar"),
			WantErr: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "values[0]",
					BadValue: "bar",
				},
			},
		},
		{
			Key:  "x15",
			Op:   selection.LessThan,
			Vals: sets.NewString("bar"),
			WantErr: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "values[0]",
					BadValue: "bar",
				},
			},
		},
		//{
		//	Key: strings.Repeat("a", 254), //breaks DNS rule that len(key) <= 253
		//	Op:  selection.Exists,
		//	WantErr: field.ErrorList{
		//		&field.Error{
		//			Type:     field.ErrorTypeInvalid,
		//			Field:    "key",
		//			BadValue: strings.Repeat("a", 254),
		//		},
		//	},
		//},
		{
			Key:  "x16",
			Op:   selection.Equals,
			Vals: sets.NewString(strings.Repeat("a", 254)),
			WantErr: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "values[0][x16]",
					BadValue: strings.Repeat("a", 254),
				},
			},
		},
		{
			Key:  "x17",
			Op:   selection.Equals,
			Vals: sets.NewString("a b"),
			WantErr: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "values[0][x17]",
					BadValue: "a b",
				},
			},
		},
		{
			Key: "x18",
			Op:  "unsupportedOp",
			WantErr: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeNotSupported,
					Field:    "operator",
					BadValue: selection.Operator("unsupportedOp"),
				},
			},
		},
	}
	for _, c := range cases {
		_, err := NewRequirement(c.Key, c.Op, c.Vals.List())
		if diff := cmp.Diff(c.WantErr.ToAggregate(), err, ignoreDetail); diff != "" {
			t.Errorf("NewRequirement test %v returned unexpected error (-want,+got):\n%s", c.Key, diff)
		}
	}
}

func TestToString(t *testing.T) {
	var req Requirement
	cases := []struct {
		In    *internalSelector
		Out   string
		Valid bool
	}{

		{&internalSelector{
			getRequirement("x", selection.In, sets.NewString("abc", "def"), t),
			getRequirement("y", selection.NotIn, sets.NewString("jkl"), t),
			//getRequirement("z", selection.Exists, nil, t)
		},
			"x in (abc,def),y notin (jkl)", true},
		{&internalSelector{
			getRequirement("x", selection.NotIn, sets.NewString("abc", "def"), t),
			getRequirement("y", selection.NotEquals, sets.NewString("jkl"), t),
			//getRequirement("z", selection.DoesNotExist, nil, t),
		},
			"x notin (abc,def),y!=jkl", true},
		{&internalSelector{
			getRequirement("x", selection.In, sets.NewString("abc", "def"), t),
			req}, // adding empty req for the trailing ','
			"x in (abc,def),", false},
		{&internalSelector{
			getRequirement("x", selection.NotIn, sets.NewString("abc"), t),
			getRequirement("y", selection.In, sets.NewString("jkl", "mno"), t),
			getRequirement("z", selection.NotIn, sets.NewString(""), t)},
			"x notin (abc),y in (jkl,mno),z notin ()", true},
		{&internalSelector{
			getRequirement("x", selection.Equals, sets.NewString("abc"), t),
			getRequirement("y", selection.DoubleEquals, sets.NewString("jkl"), t),
			getRequirement("z", selection.NotEquals, sets.NewString("a"), t),
			//getRequirement("z", selection.Exists, nil, t),
		},
			"x=abc,y==jkl,z!=a", true},
		{&internalSelector{
			getRequirement("x", selection.GreaterThan, sets.NewString("2"), t),
			getRequirement("y", selection.LessThan, sets.NewString("8"), t),
			//getRequirement("z", selection.Exists, nil, t),
		},
			"x>2,y<8", true},
	}
	for _, c := range cases {
		if out := c.In.String(); out == "" && c.Valid {
			t.Errorf("%#v.String() => '%v' expected no error", c.In, out)
		} else if out != c.Out {
			t.Errorf("%#v.String() => '%v' want '%v'", c.In, out, c.Out)
		}
	}
}

func TestRequirementSelectorMatching(t *testing.T) {
	var req Requirement
	cases := []struct {
		Set   Set
		Sel   Selector
		Match bool
	}{
		{Set{"x": "foo", "y": "baz"}, &internalSelector{
			req,
		}, false},
		{Set{"x": "foo", "y": "baz"}, &internalSelector{
			getRequirement("x", selection.In, sets.NewString("foo"), t),
			getRequirement("y", selection.NotIn, sets.NewString("alpha"), t),
		}, true},
		{Set{"x": "foo", "y": "baz"}, &internalSelector{
			getRequirement("x", selection.In, sets.NewString("foo"), t),
			getRequirement("y", selection.In, sets.NewString("alpha"), t),
		}, false},
		{Set{"y": ""}, &internalSelector{
			getRequirement("x", selection.NotIn, sets.NewString(""), t),
			//getRequirement("y", selection.Exists, nil, t),
		}, true},
		//{Set{"y": ""}, &internalSelector{
		//	getRequirement("x", selection.DoesNotExist, nil, t),
		//	//getRequirement("y", selection.Exists, nil, t),
		//}, true},
		{Set{"y": ""}, &internalSelector{
			getRequirement("x", selection.NotIn, sets.NewString(""), t),
			//getRequirement("y", selection.DoesNotExist, nil, t),
		}, true},
		{Set{"y": "baz"}, &internalSelector{
			getRequirement("x", selection.In, sets.NewString(""), t),
		}, false},
		{Set{"z": "2"}, &internalSelector{
			getRequirement("z", selection.GreaterThan, sets.NewString("1"), t),
		}, true},
		{Set{"z": "v2"}, &internalSelector{
			getRequirement("z", selection.GreaterThan, sets.NewString("1"), t),
		}, false},
	}
	for _, c := range cases {
		if match := c.Sel.Matches(c.Set); match != c.Match {
			t.Errorf("%+v.Matches(%#v) => %v, want %v", c.Sel, c.Set, match, c.Match)
		}
	}
}

func TestSetSelectorParser(t *testing.T) {
	cases := []struct {
		In    string
		Out   Selector
		Match bool
		Valid bool
	}{
		{"", NewSelector(), true, true},
		//{"\rx", internalSelector{
		//	getRequirement("x", selection.Exists, nil, t),
		//}, true, true},
		//{"this-is-a-dns.domain.com/key-with-dash", internalSelector{
		//	getRequirement("this-is-a-dns.domain.com/key-with-dash", selection.Exists, nil, t),
		//}, true, true},
		{"this-is-another-dns.domain.com/key-with-dash in (so,what)", internalSelector{
			getRequirement("this-is-another-dns.domain.com/key-with-dash", selection.In, sets.NewString("so", "what"), t),
		}, true, true},
		{"0.1.2.domain/99 notin (10.10.100.1, tick.tack.clock)", internalSelector{
			getRequirement("0.1.2.domain/99", selection.NotIn, sets.NewString("10.10.100.1", "tick.tack.clock"), t),
		}, true, true},
		{"foo  in	 (abc)", internalSelector{
			getRequirement("foo", selection.In, sets.NewString("abc"), t),
		}, true, true},
		{"x notin\n (abc)", internalSelector{
			getRequirement("x", selection.NotIn, sets.NewString("abc"), t),
		}, true, true},
		{"x  notin	\t	(abc,def)", internalSelector{
			getRequirement("x", selection.NotIn, sets.NewString("abc", "def"), t),
		}, true, true},
		{"x in (abc,def)", internalSelector{
			getRequirement("x", selection.In, sets.NewString("abc", "def"), t),
		}, true, true},
		{"x in (abc,)", internalSelector{
			getRequirement("x", selection.In, sets.NewString("abc", ""), t),
		}, true, true},
		{"x in ()", internalSelector{
			getRequirement("x", selection.In, sets.NewString(""), t),
		}, true, true},
		//{"x notin (abc,,def),bar,z in (),w", internalSelector{
		//	getRequirement("bar", selection.Exists, nil, t),
		//	getRequirement("w", selection.Exists, nil, t),
		//	getRequirement("x", selection.NotIn, sets.NewString("abc", "", "def"), t),
		//	getRequirement("z", selection.In, sets.NewString(""), t),
		//}, true, true},
		//{"x,y in (a)", internalSelector{
		//	getRequirement("y", selection.In, sets.NewString("a"), t),
		//	getRequirement("x", selection.Exists, nil, t),
		//}, false, true},
		{"x=a", internalSelector{
			getRequirement("x", selection.Equals, sets.NewString("a"), t),
		}, true, true},
		{"x>1", internalSelector{
			getRequirement("x", selection.GreaterThan, sets.NewString("1"), t),
		}, true, true},
		{"x<7", internalSelector{
			getRequirement("x", selection.LessThan, sets.NewString("7"), t),
		}, true, true},
		{"x=a,y!=b", internalSelector{
			getRequirement("x", selection.Equals, sets.NewString("a"), t),
			getRequirement("y", selection.NotEquals, sets.NewString("b"), t),
		}, true, true},
		{"x=a,y!=b,z in (h,i,j)", internalSelector{
			getRequirement("x", selection.Equals, sets.NewString("a"), t),
			getRequirement("y", selection.NotEquals, sets.NewString("b"), t),
			getRequirement("z", selection.In, sets.NewString("h", "i", "j"), t),
		}, true, true},
		{"x=a||y=b", internalSelector{}, false, false},
		{"x,,y", nil, true, false},
		{",x,y", nil, true, false},
		{"x nott in (y)", nil, true, false},
		{"x notin ( )", internalSelector{
			getRequirement("x", selection.NotIn, sets.NewString(""), t),
		}, true, true},
		{"x notin (, a)", internalSelector{
			getRequirement("x", selection.NotIn, sets.NewString("", "a"), t),
		}, true, true},
		{"a in (xyz),", nil, true, false},
		{"a in (xyz)b notin ()", nil, true, false},
		//{"a ", internalSelector{
		//	getRequirement("a", selection.Exists, nil, t),
		//}, true, true},
		{"a in (x,y,notin, z,in)", internalSelector{
			getRequirement("a", selection.In, sets.NewString("in", "notin", "x", "y", "z"), t),
		}, true, true}, // operator 'in' inside list of identifiers
		{"a in (xyz abc)", nil, false, false}, // no comma
		{"a notin(", nil, true, false},        // bad formed
		{"a (", nil, false, false},            // cpar
		{"(", nil, false, false},              // opar
	}

	for _, c := range cases {
		if sel, err := Parse(c.In); err != nil && c.Valid {
			t.Errorf("Parse(%s) => %v expected no error", c.In, err)
		} else if err == nil && !c.Valid {
			t.Errorf("Parse(%s) => %+v expected error", c.In, sel)
		} else if c.Match && !reflect.DeepEqual(sel, c.Out) {
			t.Errorf("Parse(%s) => parse output '%#v' doesn't match '%#v' expected match", c.In, sel, c.Out)
		}
	}
}

func getRequirement(key string, op selection.Operator, vals sets.String, t *testing.T) Requirement {
	req, err := NewRequirement(key, op, vals.List())
	if err != nil {
		t.Errorf("NewRequirement(%v, %v, %v) resulted in error:%v", key, op, vals, err)
		return Requirement{}
	}
	return *req
}

func TestAdd(t *testing.T) {
	cases := []struct {
		name        string
		sel         Selector
		key         string
		operator    selection.Operator
		values      []string
		refSelector Selector
	}{
		{
			"keyInOperator",
			internalSelector{},
			"key",
			selection.In,
			[]string{"value"},
			internalSelector{Requirement{"key", selection.In, []string{"value"}}},
		},
		{
			"keyEqualsOperator",
			internalSelector{Requirement{"key", selection.In, []string{"value"}}},
			"key2",
			selection.Equals,
			[]string{"value2"},
			internalSelector{
				Requirement{"key", selection.In, []string{"value"}},
				Requirement{"key2", selection.Equals, []string{"value2"}},
			},
		},
	}
	for _, c := range cases {
		req, err := NewRequirement(c.key, c.operator, c.values)
		if err != nil {
			t.Errorf("%s - Unable to create queries.Requirement", c.name)
		}
		c.sel = c.sel.Add(*req)
		if !reflect.DeepEqual(c.sel, c.refSelector) {
			t.Errorf("%s - Expected %v found %v", c.name, c.refSelector, c.sel)
		}
	}
}

func TestSafeSort(t *testing.T) {
	cases := []struct {
		name   string
		in     []string
		inCopy []string
		want   []string
	}{
		{
			name:   "nil strings",
			in:     nil,
			inCopy: nil,
			want:   nil,
		},
		{
			name:   "ordered strings",
			in:     []string{"bar", "foo"},
			inCopy: []string{"bar", "foo"},
			want:   []string{"bar", "foo"},
		},
		{
			name:   "unordered strings",
			in:     []string{"foo", "bar"},
			inCopy: []string{"foo", "bar"},
			want:   []string{"bar", "foo"},
		},
		{
			name:   "duplicated strings",
			in:     []string{"foo", "bar", "foo", "bar"},
			inCopy: []string{"foo", "bar", "foo", "bar"},
			want:   []string{"bar", "bar", "foo", "foo"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := safeSort(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("safeSort() = %v, want %v", got, c.want)
			}
			if !reflect.DeepEqual(c.in, c.inCopy) {
				t.Errorf("after safeSort(), input = %v, want %v", c.in, c.inCopy)
			}
		})
	}
}

func BenchmarkSelectorFromValidatedSet(b *testing.B) {
	set := map[string]string{
		"foo": "foo",
		"bar": "bar",
	}

	for i := 0; i < b.N; i++ {
		if SelectorFromValidatedSet(set).Empty() {
			b.Errorf("Unexpected selector")
		}
	}
}

func TestRequiresExactMatch(t *testing.T) {
	cases := []struct {
		name          string
		sel           Selector
		query         string
		expectedFound bool
		expectedValue string
	}{
		{
			name:          "keyInOperatorExactMatch",
			sel:           internalSelector{Requirement{"key", selection.In, []string{"value"}}},
			query:         "key",
			expectedFound: true,
			expectedValue: "value",
		},
		{
			name:          "keyInOperatorNotExactMatch",
			sel:           internalSelector{Requirement{"key", selection.In, []string{"value", "value2"}}},
			query:         "key",
			expectedFound: false,
			expectedValue: "",
		},
		{
			name: "keyInOperatorNotExactMatch",
			sel: internalSelector{
				Requirement{"key", selection.In, []string{"value", "value1"}},
				Requirement{"key2", selection.In, []string{"value2"}},
			},
			query:         "key2",
			expectedFound: true,
			expectedValue: "value2",
		},
		{
			name:          "keyEqualOperatorExactMatch",
			sel:           internalSelector{Requirement{"key", selection.Equals, []string{"value"}}},
			query:         "key",
			expectedFound: true,
			expectedValue: "value",
		},
		{
			name:          "keyDoubleEqualOperatorExactMatch",
			sel:           internalSelector{Requirement{"key", selection.DoubleEquals, []string{"value"}}},
			query:         "key",
			expectedFound: true,
			expectedValue: "value",
		},
		{
			name:          "keyNotEqualOperatorExactMatch",
			sel:           internalSelector{Requirement{"key", selection.NotEquals, []string{"value"}}},
			query:         "key",
			expectedFound: false,
			expectedValue: "",
		},
		{
			name: "keyEqualOperatorExactMatchFirst",
			sel: internalSelector{
				Requirement{"key", selection.In, []string{"value"}},
				Requirement{"key2", selection.In, []string{"value2"}},
			},
			query:         "key",
			expectedFound: true,
			expectedValue: "value",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			value, found := c.sel.RequiresExactMatch(c.query)
			if found != c.expectedFound {
				t.Errorf("Expected match %v, found %v", c.expectedFound, found)
			}
			if found && value != c.expectedValue {
				t.Errorf("Expected value %v, found %v", c.expectedValue, value)
			}

		})
	}
}

func TestValidatedSelectorFromSet(t *testing.T) {
	cases := []struct {
		name             string
		input            Set
		expectedSelector internalSelector
		expectedError    field.ErrorList
	}{
		{
			name:  "Simple Set, no error",
			input: Set{"key": "val"},
			expectedSelector: internalSelector{
				Requirement{
					key:       "key",
					operator:  selection.Equals,
					strValues: []string{"val"},
				},
			},
		},
		{
			name:  "Invalid Set, value too long",
			input: Set{"Key": "axahm2EJ8Phiephe2eixohbee9eGeiyees1thuozi1xoh0GiuH3diewi8iem7Nui"},
			expectedError: field.ErrorList{
				&field.Error{
					Type:     field.ErrorTypeInvalid,
					Field:    "values[0][Key]",
					BadValue: "axahm2EJ8Phiephe2eixohbee9eGeiyees1thuozi1xoh0GiuH3diewi8iem7Nui",
				},
			},
		},
	}

	for _, c := range cases {
		selector, err := ValidatedSelectorFromSet(c.input)
		if diff := cmp.Diff(c.expectedError.ToAggregate(), err, ignoreDetail); diff != "" {
			t.Errorf("ValidatedSelectorFromSet %#v returned unexpected error (-want,+got):\n%s", c.name, diff)
		}
		if err == nil {
			if diff := cmp.Diff(c.expectedSelector, selector); diff != "" {
				t.Errorf("ValidatedSelectorFromSet %#v returned unexpected selector (-want,+got):\n%s", c.name, diff)
			}
		}
	}
}

func BenchmarkRequirementString(b *testing.B) {
	r := Requirement{
		key:      "environment",
		operator: selection.NotIn,
		strValues: []string{
			"dev",
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if r.String() != "environment notin (dev)" {
			b.Errorf("Unexpected Requirement string")
		}
	}
}

func TestRequirementEqual(t *testing.T) {
	cases := []struct {
		name string
		x, y *Requirement
		want bool
	}{
		{
			name: "same requirements should be equal",
			x: &Requirement{
				key:       "key",
				operator:  selection.Equals,
				strValues: []string{"foo", "bar"},
			},
			y: &Requirement{
				key:       "key",
				operator:  selection.Equals,
				strValues: []string{"foo", "bar"},
			},
			want: true,
		},
		{
			name: "requirements with different keys should not be equal",
			x: &Requirement{
				key:       "key1",
				operator:  selection.Equals,
				strValues: []string{"foo", "bar"},
			},
			y: &Requirement{
				key:       "key2",
				operator:  selection.Equals,
				strValues: []string{"foo", "bar"},
			},
			want: false,
		},
		{
			name: "requirements with different operators should not be equal",
			x: &Requirement{
				key:       "key",
				operator:  selection.Equals,
				strValues: []string{"foo", "bar"},
			},
			y: &Requirement{
				key:       "key",
				operator:  selection.In,
				strValues: []string{"foo", "bar"},
			},
			want: false,
		},
		{
			name: "requirements with different values should not be equal",
			x: &Requirement{
				key:       "key",
				operator:  selection.Equals,
				strValues: []string{"foo", "bar"},
			},
			y: &Requirement{
				key:       "key",
				operator:  selection.Equals,
				strValues: []string{"foobar"},
			},
			want: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cmp.Equal(c.x, c.y); got != c.want {
				t.Errorf("cmp.Equal() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestRequirementSql(t *testing.T) {
	cases := []struct {
		name  string
		r     *Requirement
		query string
		args  []interface{}
	}{{
		name: "requirement sql equal",
		r: &Requirement{
			key:       "user_name",
			operator:  selection.Equals,
			strValues: []string{"tom"},
		},
		query: "`user_name` = ?",
		args:  []interface{}{"tom"},
	}, {
		name: "requirement sql in",
		r: &Requirement{
			key:       "user_name",
			operator:  selection.In,
			strValues: []string{"tom", "jerry"},
		},
		query: "`user_name` in (?,?)",
		args:  []interface{}{"tom", "jerry"},
	}, {
		name: "requirement sql not equal",
		r: &Requirement{
			key:       "user_name",
			operator:  selection.NotEquals,
			strValues: []string{"tom"},
		},
		query: "`user_name` != ?",
		args:  []interface{}{"tom"},
	}, {
		name: "requirement sql not in",
		r: &Requirement{
			key:       "user_name",
			operator:  selection.NotIn,
			strValues: []string{"tom", "jerry"},
		},
		query: "`user_name` not in (?,?)",
		args:  []interface{}{"tom", "jerry"},
	}, {

		name: "requirement sql gt",
		r: &Requirement{
			key:       "id",
			operator:  selection.GreaterThan,
			strValues: []string{"2"},
		},
		query: "`id` > ?",
		args:  []interface{}{"2"},
	}, {
		name: "requirement sql lt",
		r: &Requirement{
			key:       "id",
			operator:  selection.LessThan,
			strValues: []string{"2"},
		},
		query: "`id` < ?",
		args:  []interface{}{"2"},
	}, {
		name: "requirement sql contains",
		r: &Requirement{
			key:       "name",
			operator:  selection.Contains,
			strValues: []string{"2"},
		},
		query: "`name` like ?",
		args:  []interface{}{"%2%"},
	}, {
		name: "requirement sql not contains",
		r: &Requirement{
			key:       "name",
			operator:  selection.NotContains,
			strValues: []string{"2"},
		},
		query: "`name` not like ?",
		args:  []interface{}{"%2%"},
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			query, args := c.r.Sql()
			require.Equal(t, c.query, query)
			require.Equal(t, c.args, args)
		})
	}
}

func TestSelectorSql(t *testing.T) {
	cases := []struct {
		name     string
		selector string
		query    string
		args     []interface{}
	}{{
		name:     "empty",
		selector: "",
		query:    "",
		args:     []interface{}(nil),
	}, {
		name:     "empty space",
		selector: "  ",
		query:    "",
		args:     []interface{}(nil),
	}, {
		name:     "equal",
		selector: "user_name = tom",
		query:    "`user_name` = ?",
		args:     []interface{}{"tom"},
	}, {
		name:     "in",
		selector: "user_name in (tom, jerry)",
		query:    "`user_name` in (?,?)",
		args:     []interface{}{"jerry", "tom"},
	}, {
		name:     "not equal",
		selector: "user_name != tom",
		query:    "`user_name` != ?",
		args:     []interface{}{"tom"},
	}, {
		name:     "not in",
		selector: "user_name notin (tom, jerry)",
		query:    "`user_name` not in (?,?)",
		args:     []interface{}{"jerry", "tom"},
	}, {

		name:     "gt",
		selector: "id > 2",
		query:    "`id` > ?",
		args:     []interface{}{"2"},
	}, {
		name:     "lt",
		selector: "id < 2",
		query:    "`id` < ?",
		args:     []interface{}{"2"},
	}, {
		name:     "and",
		selector: "user_name!=tom , id<10",
		query:    "`user_name` != ? and `id` < ?",
		args:     []interface{}{"tom", "10"},
	}, {
		name:     "contains",
		selector: "user_name =~ tom",
		query:    "`user_name` like ?",
		args:     []interface{}{"%tom%"},
	}, {
		name:     "and",
		selector: "user_name!~tom",
		query:    "`user_name` not like ?",
		args:     []interface{}{"%tom%"},
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := Parse(c.selector)
			require.NoError(t, err)

			query, args := s.Sql()
			require.Equal(t, c.query, query, c.selector)
			require.Equal(t, c.args, args, c.selector)

		})
	}
}
