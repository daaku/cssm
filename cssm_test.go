package cssm

import (
	"bytes"
	"testing"
)

type matchableCSS struct {
	canMatch bool
	value    []byte
}

func newMatchableCSS(canMatch bool, value []byte) matchableCSS {
	return matchableCSS{
		canMatch: canMatch,
		value:    value,
	}
}

var testCasesCSSModules = []struct {
	name                  string
	payload               string
	expectedCSSModules    matchableCSS
	expectedScopedClasses []string
	expectedError         string
}{
	{
		name:                  "ValidCSSModules",
		expectedCSSModules:    newMatchableCSS(false, nil),
		expectedScopedClasses: []string{"test-class"},
		expectedError:         "",

		payload: `.test-class {
color: red;
font-size: large;
}`,
	},
	{
		name: "ValidCSSModules_GlobalKeyword",
		expectedCSSModules: newMatchableCSS(true,
			[]byte(` .test-class { color: red; font-size: large; }`),
		),
		expectedScopedClasses: nil,
		expectedError:         "",

		payload: `:global {.test-class { color: red; font-size: large; }}`,
	},
	{
		name:                  "ValidCSSModules_MediaQueryScoping",
		expectedCSSModules:    newMatchableCSS(false, nil),
		expectedScopedClasses: []string{"test-class"},
		expectedError:         "",

		payload: `@media screen and (min-width: 70ch) and (max-width: 100ch) {
	.test-class {
		color: green;
		font-size: large;
	}
}`,
	},
	{
		name: "ValidCSSModules_Comments",
		expectedCSSModules: newMatchableCSS(true,
			[]byte(`/* Test Comments */ /* Not Closing Comment`),
		),
		expectedScopedClasses: nil,
		expectedError:         "",

		payload: `/* Test Comments */ /* Not Closing Comment`,
	},
	{
		name: "ValidCSSModules_ID#SymbolWillNotBeScoped_AnywaysWillBeWritten",
		expectedCSSModules: newMatchableCSS(true,
			[]byte(`#test-class {color: red; font-size: medium}`),
		),
		expectedScopedClasses: nil,
		expectedError:         "",

		payload: `#test-class {color: red; font-size: medium}`,
	},
	{
		name: "ValidCSSModules_Another@declarationsSupport",
		expectedCSSModules: newMatchableCSS(true, []byte(`@import url("path/to/styles.css");
@keyframes myAnimation {
	from {
		background-color: red;
	}
	to {
		background-color: blue;
	}
}
@keyframes anotherAnimation {
	0% {
		background-color: green;
	}
	10% {
		background-color: red;
	}
	90% {
		background-color: black;
	}
	100% {
		background-color: purple;
	}
}`)),
		expectedScopedClasses: nil,
		expectedError:         "",

		payload: `@import url("path/to/styles.css");
@keyframes myAnimation {
	from {
		background-color: red;
	}
	to {
		background-color: blue;
	}
}
@keyframes anotherAnimation {
	0% {
		background-color: green;
	}
	10% {
		background-color: red;
	}
	90% {
		background-color: black;
	}
	100% {
		background-color: purple;
	}
}`,
	},
	// Invalid css syntax will not return an error
	{
		// According to the css syntax standard, a syntax error should not be considered as a
		// Fatal error, the only problem would be unexpected behaviour if you do not delimit
		// well your global block with {}
		name: "InvalidCSSModules_GlobalBlockMalformed",
		expectedCSSModules: newMatchableCSS(true,
			[]byte(` .test-class { color: red; font-size: large; }`),
		),
		expectedScopedClasses: nil,
		expectedError:         "",

		payload: `:global {.test-class { color: red; font-size: large; }`,
	},
	{
		name:                  "InvalidCSSModules_NilPayload",
		expectedCSSModules:    newMatchableCSS(true, nil),
		expectedScopedClasses: nil,
		expectedError:         "",

		payload: "",
	},
	{
		name: "InvalidCSSModules_ClassNameStartsWithSpace",
		expectedCSSModules: newMatchableCSS(true,
			[]byte(`. test-class { color:red; font-size: large;}`),
		),
		expectedScopedClasses: nil,
		expectedError:         "",

		payload: `. test-class { color:red; font-size: large;}`,
	},
	{
		name:                  "InvalidCSSModules_ClassNameStartsWithSpace_HasPseudoAndCombinator",
		expectedCSSModules:    newMatchableCSS(true, []byte(`. test-class :hover { color:green; font-size: medium; }`)),
		expectedScopedClasses: nil,
		expectedError:         "",

		payload: `. test-class :hover { color:green; font-size: medium; }`,
	},
}

func TestProcess(t *testing.T) {
	for i := range testCasesCSSModules {
		tc := testCasesCSSModules[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			css, scopedClasses, err := Process(tc.payload)
			if err != nil {
				if err.Error() != tc.expectedError {
					t.Errorf("unexpected error value: expected %q got %q", tc.expectedError, err.Error())
					return
				}
			} else {
				if tc.expectedError != "" {
					t.Errorf("unexpected error value: expected %q got <nil>", tc.expectedError)
					return
				}
			}
			if tc.expectedCSSModules.canMatch {
				if !bytes.Equal(tc.expectedCSSModules.value, css) {
					t.Errorf("unexpected css slice of bytes value: expected\n%q\ngot\n%q", tc.expectedCSSModules.value, css)
					return
				}
			}
			for i := range tc.expectedScopedClasses {
				esc := tc.expectedScopedClasses[i]
				if _, ok := scopedClasses[esc]; !ok {
					t.Errorf("unexpected scopedClasses value absence: expected to have %q inside of it, got %q map", esc, scopedClasses)
					return
				}
			}
		})
	}
}
