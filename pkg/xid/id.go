package xid

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/bwmarrin/snowflake"
)

var global struct {
	mu  sync.RWMutex
	gen *generator
}

const (
	generatorStateReady uint32 = iota
	generatorStateClosed
)

var errGeneratorClosed = errors.New("xid generator closed")

type generator struct {
	node      *snowflake.Node
	closeOnce sync.Once
	state     atomic.Uint32
}

func Init(cfg Config) error {
	nodeID, err := resolveNodeID(cfg)
	if err != nil {
		return err
	}

	node, err := snowflake.NewNode(nodeID)
	if err != nil {
		return err
	}

	return installGenerator(&generator{node: node})
}

func MustInit(cfg Config) {
	if err := Init(cfg); err != nil {
		panic(err)
	}
}

func New() int64 {
	global.mu.RLock()
	gen := global.gen
	global.mu.RUnlock()

	if gen == nil {
		panic("xid not initialized")
	}

	id, err := gen.newID()
	if err != nil {
		panic(err)
	}

	return id
}

func Close() error {
	global.mu.Lock()
	gen := global.gen
	global.gen = nil
	global.mu.Unlock()

	if gen == nil {
		return nil
	}

	return gen.close()
}

func installGenerator(gen *generator) error {
	global.mu.Lock()
	defer global.mu.Unlock()

	if global.gen != nil {
		return errors.New("xid already initialized")
	}

	global.gen = gen
	return nil
}

func (g *generator) newID() (int64, error) {
	if g.state.Load() == generatorStateClosed {
		return 0, errGeneratorClosed
	}

	return g.node.Generate().Int64(), nil
}

func (g *generator) close() error {
	g.closeOnce.Do(func() {
		g.state.Store(generatorStateClosed)
	})

	return nil
}
