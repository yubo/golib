package orm

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithSqlOut(t *testing.T) {
	runTests(t, func(db DB, ctx context.Context) {
		s := "CREATE TABLE test (value int)"
		record := bytes.NewBuffer(nil)
		ctx = WithSqlWriter(ctx, record)
		db.Exec(ctx, "CREATE TABLE test (value int)")
		require.Equal(t, s, record.String())
	})
}
