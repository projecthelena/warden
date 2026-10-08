package db

import (
	"fmt"
	"testing"
)

func TestSQLiteRollupBatchSizeConfig(t *testing.T) {
	for _, size := range []int{0, 1, 10, 50, -1, 51} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			cfg := NewTestConfig()
			cfg.SQLiteRollupBatchSize = size
			s, err := NewStore(cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = s.Close() }()
			want := size
			if want < 1 || want > 50 {
				want = 50
			}
			if s.sqliteRollupBatchSize != want {
				t.Fatalf("batch size = %d, want %d", s.sqliteRollupBatchSize, want)
			}
		})
	}
}
