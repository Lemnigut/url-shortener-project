package snowflake

import (
	"fmt"

	"github.com/bwmarrin/snowflake"
)

// Generator — обёртка над snowflake для генерации уникальных ID.
type Generator struct {
	node *snowflake.Node
}

// New создаёт генератор с указанным nodeID (0–1023).
func New(nodeID int64) (*Generator, error) {
	node, err := snowflake.NewNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("snowflake: не удалось создать ноду %d: %w", nodeID, err)
	}
	return &Generator{node: node}, nil
}

// Generate возвращает новый уникальный ID.
func (g *Generator) Generate() int64 {
	return g.node.Generate().Int64()
}
