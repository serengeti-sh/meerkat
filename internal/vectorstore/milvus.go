package vectorstore

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/rs/zerolog"
	"strings"
	"sync"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/serengeti-sh/meerkat/internal/config"
)

const (
	idMaxLength        = "64"
	serviceMaxLength   = "128"
	severityMaxLength  = "32"
	bodyMaxLength      = "4096"
	hnswM              = 18
	hnswEfConstruction = 300
	searchEF           = 30
)

// milvusStore implements VectorStore using Milvus.
type milvusStore struct {
	client     client.Client
	collection string
	dimension  int
	retention  time.Duration
	initOnce   sync.Once
	initErr    error
	log        zerolog.Logger
}

var _ Store = (*milvusStore)(nil)

// NewMilvusClient creates a Store backed by Milvus.
// The connection is established immediately but collection setup is deferred
// to the first operation via lazy initialization.
func NewMilvusClient(cfg *config.Config) (*milvusStore, error) {
	mc := cfg.VectorStore.Milvus

	clientCfg := client.Config{
		Address: mc.Address,
		DBName:  mc.Database,
	}

	if mc.Auth.Enabled {
		if mc.Auth.Token != "" {
			clientCfg.APIKey = mc.Auth.Token
		} else {
			clientCfg.Username = mc.Auth.User
			clientCfg.Password = mc.Auth.Password
		}
	}

	log := zerolog.New(nil).With().Str("component", "milvus").Logger()

	if mc.TLS.Enabled {
		clientCfg.EnableTLSAuth = true
		if mc.TLS.CAFile != "" {
			cred, err := credentials.NewClientTLSFromFile(mc.TLS.CAFile, "")
			if err != nil {
				return nil, fmt.Errorf("load tls ca file: %w", err)
			}
			clientCfg.DialOptions = append(clientCfg.DialOptions, grpc.WithTransportCredentials(cred))
		} else if mc.TLS.SkipVerify {
			log.Warn().Msg("TLS skip_verify is enabled. This is insecure and should only be used for development.")
			clientCfg.DialOptions = append(clientCfg.DialOptions, grpc.WithTransportCredentials(
				credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})),
			)
		}
	}

	// client.NewClient does not actually dial; connection is lazy.
	c, err := client.NewClient(context.Background(), clientCfg)
	if err != nil {
		return nil, fmt.Errorf("connect to milvus: %w", err)
	}

	return &milvusStore{
		client:     c,
		collection: mc.Collection,
		dimension:  mc.Dimension,
		retention:  mc.Retention,
		log:        log,
	}, nil
}

func (s *milvusStore) lazyInit(ctx context.Context) error {
	s.initOnce.Do(func() {
		s.initErr = s.ensureCollection(ctx, s.retention)
	})
	return s.initErr
}

func (s *milvusStore) ensureCollection(ctx context.Context, retention time.Duration) error {
	exists, err := s.client.HasCollection(ctx, s.collection)
	if err != nil {
		return fmt.Errorf("check collection: %w", err)
	}
	if exists {
		return nil
	}

	schema := &entity.Schema{
		CollectionName: s.collection,
		AutoID:         false,
		Fields: []*entity.Field{
			{
				Name:       "id",
				DataType:   entity.FieldTypeVarChar,
				PrimaryKey: true,
				TypeParams: map[string]string{"max_length": idMaxLength},
			},
			{
				Name:       "vector",
				DataType:   entity.FieldTypeFloatVector,
				TypeParams: map[string]string{"dim": strconv.Itoa(s.dimension)},
			},
			{
				Name:     "timestamp",
				DataType: entity.FieldTypeInt64,
			},
			{
				Name:       "service",
				DataType:   entity.FieldTypeVarChar,
				TypeParams: map[string]string{"max_length": serviceMaxLength},
			},
			{
				Name:       "severity",
				DataType:   entity.FieldTypeVarChar,
				TypeParams: map[string]string{"max_length": severityMaxLength},
			},
			{
				Name:       "body",
				DataType:   entity.FieldTypeVarChar,
				TypeParams: map[string]string{"max_length": bodyMaxLength},
			},
			{
				Name:     "attributes",
				DataType: entity.FieldTypeJSON,
			},
		},
	}

	if err := s.client.CreateCollection(ctx, schema, 1); err != nil {
		return fmt.Errorf("create collection: %w", err)
	}

	idx, err := entity.NewIndexHNSW(entity.L2, hnswM, hnswEfConstruction)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}

	if err := s.client.CreateIndex(ctx, s.collection, "vector", idx, false); err != nil {
		return fmt.Errorf("create index: %w", err)
	}

	if err := s.client.LoadCollection(ctx, s.collection, false); err != nil {
		return fmt.Errorf("load collection: %w", err)
	}

	if retention > 0 {
		if err := s.client.AlterCollection(ctx, s.collection, entity.CollectionTTL(int64(retention.Seconds()))); err != nil {
			return fmt.Errorf("set collection ttl: %w", err)
		}
	}

	return nil
}

func (s *milvusStore) Insert(ctx context.Context, records []Record) error {
	if err := s.lazyInit(ctx); err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	ids := make([]string, len(records))
	vectors := make([][]float32, len(records))
	timestamps := make([]int64, len(records))
	services := make([]string, len(records))
	severities := make([]string, len(records))
	bodies := make([]string, len(records))
	attributes := make([][]byte, len(records))

	for i, r := range records {
		ids[i] = r.ID
		vectors[i] = r.Vector
		timestamps[i] = r.Timestamp.UnixMilli()
		services[i] = r.Service
		severities[i] = r.Severity
		bodies[i] = r.Body
		attrs, _ := json.Marshal(r.Attributes)
		attributes[i] = attrs
	}

	_, err := s.client.Insert(ctx, s.collection, "",
		entity.NewColumnVarChar("id", ids),
		entity.NewColumnFloatVector("vector", s.dimension, vectors),
		entity.NewColumnInt64("timestamp", timestamps),
		entity.NewColumnVarChar("service", services),
		entity.NewColumnVarChar("severity", severities),
		entity.NewColumnVarChar("body", bodies),
		entity.NewColumnJSONBytes("attributes", attributes),
	)
	if err != nil {
		return fmt.Errorf("insert records: %w", err)
	}

	return nil
}

func (s *milvusStore) Search(ctx context.Context, vector []float32, opts SearchOptions) ([]SearchResult, error) {
	if err := s.lazyInit(ctx); err != nil {
		return nil, err
	}
	var exprs []string
	if opts.TimeRange > 0 {
		cutoff := time.Now().Add(-opts.TimeRange).UnixMilli()
		exprs = append(exprs, fmt.Sprintf("timestamp >= %d", cutoff))
	}
	if opts.Service != "" {
		exprs = append(exprs, fmt.Sprintf("service == %q", opts.Service))
	}
	if opts.Severity != "" {
		exprs = append(exprs, fmt.Sprintf("severity == %q", opts.Severity))
	}
	var expr string
	if len(exprs) > 0 {
		expr = strings.Join(exprs, " && ")
	}

	sp, err := entity.NewIndexHNSWSearchParam(searchEF)
	if err != nil {
		return nil, fmt.Errorf("create search param: %w", err)
	}

	results, err := s.client.Search(ctx, s.collection, nil, expr,
		[]string{"id", "body", "service", "severity", "timestamp", "attributes"},
		[]entity.Vector{entity.FloatVector(vector)},
		"vector",
		entity.L2, opts.Limit, sp,
	)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	if len(results) == 0 {
		return nil, nil
	}

	return parseSearchResults(results[0])
}

func parseSearchResults(result client.SearchResult) ([]SearchResult, error) {
	ids, ok := result.IDs.(*entity.ColumnVarChar)
	if !ok {
		return nil, fmt.Errorf("unexpected id column type")
	}

	scores := result.Scores
	fields := result.Fields

	var out []SearchResult
	for i := 0; i < ids.Len(); i++ {
		sr := SearchResult{
			ID:    ids.Data()[i],
			Score: scores[i],
		}

		for _, field := range fields {
			switch field.Name() {
			case "body":
				if col, ok := field.(*entity.ColumnVarChar); ok {
					sr.Body = col.Data()[i]
				}
			case "service":
				if col, ok := field.(*entity.ColumnVarChar); ok {
					sr.Service = col.Data()[i]
				}
			case "severity":
				if col, ok := field.(*entity.ColumnVarChar); ok {
					sr.Severity = col.Data()[i]
				}
			case "timestamp":
				if col, ok := field.(*entity.ColumnInt64); ok {
					sr.Timestamp = time.UnixMilli(col.Data()[i])
				}
			case "attributes":
				if col, ok := field.(*entity.ColumnJSONBytes); ok {
					var attrs map[string]string
					_ = json.Unmarshal(col.Data()[i], &attrs)
					sr.Attributes = attrs
				}
			}
		}

		out = append(out, sr)
	}

	return out, nil
}

func (s *milvusStore) Delete(ctx context.Context, ids []string) error {
	if err := s.lazyInit(ctx); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}

	expr := fmt.Sprintf("id in [%s]", joinQuoted(ids))
	err := s.client.Delete(ctx, s.collection, "", expr)
	if err != nil {
		return fmt.Errorf("delete records: %w", err)
	}

	return nil
}

func joinQuoted(ids []string) string {
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = fmt.Sprintf("%q", id)
	}
	return strings.Join(quoted, ", ")
}

func (s *milvusStore) Ping(ctx context.Context) error {
	_, err := s.client.ListCollections(ctx)
	if err != nil {
		return fmt.Errorf("milvus ping failed: %w", err)
	}
	return nil
}

func (s *milvusStore) Close() error {
	return s.client.Close()
}
