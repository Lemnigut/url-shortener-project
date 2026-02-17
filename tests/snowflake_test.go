package tests

import (
	"testing"

	"tinyurl/pkg/snowflake"
)

func TestSnowflakeNew(t *testing.T) {
	g, err := snowflake.New(1)
	if err != nil {
		t.Fatalf("New(1) ошибка: %v", err)
	}
	if g == nil {
		t.Fatal("New(1) вернул nil")
	}
}

func TestSnowflakeNewInvalidNode(t *testing.T) {
	_, err := snowflake.New(1024)
	if err == nil {
		t.Error("New(1024) ожидалась ошибка, получен nil")
	}
}

func TestSnowflakeGenerateUnique(t *testing.T) {
	g, _ := snowflake.New(1)

	seen := make(map[int64]bool)
	for i := 0; i < 10000; i++ {
		id := g.Generate()
		if id <= 0 {
			t.Fatalf("Generate() вернул неположительный ID: %d", id)
		}
		if seen[id] {
			t.Fatalf("Generate() вернул дублирующийся ID: %d на итерации %d", id, i)
		}
		seen[id] = true
	}
}

func TestSnowflakeGenerateMonotonic(t *testing.T) {
	g, _ := snowflake.New(1)

	prev := g.Generate()
	for i := 0; i < 1000; i++ {
		curr := g.Generate()
		if curr <= prev {
			t.Fatalf("ID не монотонны: prev=%d, curr=%d на итерации %d", prev, curr, i)
		}
		prev = curr
	}
}
