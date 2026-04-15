package programcache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"damai-go/services/program-rpc/pb"

	"github.com/zeromicro/go-zero/core/collection"
	"google.golang.org/protobuf/proto"
)

var ErrProgramNotFound = errors.New("program detail view not found")

type ProgramDetailViewLoader interface {
	Load(ctx context.Context, programID int64) (*pb.ProgramDetailViewInfo, error)
}

type ProgramDetailViewCache struct {
	cache           *collection.Cache
	loader          ProgramDetailViewLoader
	detailTTL       time.Duration
	notFoundTTL     time.Duration
	notFoundPayload []byte
	loadGroup       *loadGroup
}

func NewProgramDetailViewCache(loader ProgramDetailViewLoader, detailTTL, notFoundTTL time.Duration, limit int) (*ProgramDetailViewCache, error) {
	if loader == nil {
		return nil, errors.New("program detail view loader is required")
	}

	localCache, err := collection.NewCache(detailTTL,
		collection.WithLimit(limit),
		collection.WithName("program-detail-view-cache"),
	)
	if err != nil {
		return nil, err
	}

	return &ProgramDetailViewCache{
		cache:           localCache,
		loader:          loader,
		detailTTL:       detailTTL,
		notFoundTTL:     notFoundTTL,
		notFoundPayload: []byte{1},
		loadGroup:       newLoadGroup(),
	}, nil
}

func (c *ProgramDetailViewCache) Get(ctx context.Context, programID int64) (*pb.ProgramDetailViewInfo, error) {
	if _, ok := c.cache.Get(detailNotFoundCacheKey(programID)); ok {
		return nil, ErrProgramNotFound
	}

	if payload, ok := c.cache.Get(detailCacheKey(programID)); ok {
		resp, err := decodeProgramDetailPayload(payload)
		if err == nil {
			return resp, nil
		}
		c.cache.Del(detailCacheKey(programID))
	}

	loaded, err := c.loadGroup.DoWithContext(ctx, detailCacheKey(programID), func(sharedCtx context.Context) (any, error) {
		if _, ok := c.cache.Get(detailNotFoundCacheKey(programID)); ok {
			return nil, ErrProgramNotFound
		}

		if payload, ok := c.cache.Get(detailCacheKey(programID)); ok {
			resp, err := decodeProgramDetailPayload(payload)
			if err == nil {
				return resp, nil
			}
			c.cache.Del(detailCacheKey(programID))
		}

		resp, err := c.loader.Load(sharedCtx, programID)
		if err != nil {
			if errors.Is(err, ErrProgramNotFound) {
				c.cache.SetWithExpire(detailNotFoundCacheKey(programID), c.notFoundPayload, c.notFoundTTL)
				return nil, ErrProgramNotFound
			}
			return nil, err
		}
		if resp == nil {
			return nil, errors.New("program detail view loader returned nil detail")
		}

		payload, err := proto.Marshal(resp)
		if err != nil {
			return nil, err
		}
		c.cache.SetWithExpire(detailCacheKey(programID), payload, c.detailTTL)

		return decodeProgramDetailPayload(payload)
	})
	if err != nil {
		return nil, err
	}

	resp, ok := loaded.(*pb.ProgramDetailViewInfo)
	if !ok || resp == nil {
		return nil, errors.New("program detail view loader returned invalid payload")
	}
	return resp, nil
}

func (c *ProgramDetailViewCache) Invalidate(programID int64) {
	c.cache.Del(detailCacheKey(programID))
	c.cache.Del(detailNotFoundCacheKey(programID))
}

func detailCacheKey(programID int64) string {
	return fmt.Sprintf("program:detail:view:%d", programID)
}

func detailNotFoundCacheKey(programID int64) string {
	return fmt.Sprintf("program:detail:view:notfound:%d", programID)
}

func decodeProgramDetailPayload(payload any) (*pb.ProgramDetailViewInfo, error) {
	bytes, ok := payload.([]byte)
	if !ok {
		return nil, errors.New("program detail view cache payload is not bytes")
	}

	resp := &pb.ProgramDetailViewInfo{}
	if err := proto.Unmarshal(bytes, resp); err != nil {
		return nil, err
	}

	return resp, nil
}
