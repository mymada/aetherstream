package smartplaylists

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPackageCompiles(t *testing.T) {
	assert.True(t, true, "smartplaylists package should compile")
}

func TestBuildStringClause(t *testing.T) {
	clause, vals := buildStringClause("name", "eq", "foo")
	assert.Equal(t, "name = ?", clause)
	assert.Equal(t, []interface{}{"foo"}, vals)

	clause, vals = buildStringClause("name", "contains", "bar")
	assert.Equal(t, "name LIKE ?", clause)
	assert.Equal(t, []interface{}{"%bar%"}, vals)
}

func TestBuildNumericClause(t *testing.T) {
	clause, vals := buildNumericClause("duration_seconds", "gt", "120")
	assert.Equal(t, "duration_seconds GT ?", clause)
	assert.Equal(t, []interface{}{"120"}, vals)
}

func TestJSONHelpers(t *testing.T) {
	rules := []Rule{{Field: "name", Operator: "eq", Value: "test"}}
	_, err := jsonMarshal(rules)
	assert.NoError(t, err)
	parsed := jsonUnmarshalRules("[]")
	assert.Empty(t, parsed)
}

func TestRuleSerialization(t *testing.T) {
	rules := []Rule{{Field: "genre", Operator: "eq", Value: "Sci-Fi"}}
	b, err := json.Marshal(rules)
	assert.NoError(t, err)
	var out []Rule
	assert.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, "Sci-Fi", out[0].Value)
}
