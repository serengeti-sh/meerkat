package vectorstore

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/serengeti-sh/meerkat/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
}

// NewMilvusClient creates a VectorStore backed by Milvus.
// On first use it ensures the collection exists with the correct schema and index.
func NewMilvusClient(cfg *config.Config) (VectorStore, error) {
	mc := cfg.VectorStore.Milvus

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	config := client.Config{
		Address: mc.Address,
		DBName:  mc.Database,
	}

	if mc.Auth.Enabled {
		if mc.Auth.Token != "" {
			config.APIKey = mc.Auth.Token
		} else {
			config.Username = mc.Auth.User
			config.Password = mc.Auth.Password
		}
	}

	if mc.TLS.Enabled {
		config.EnableTLSAuth = true
		if mc.TLS.CAFile != "" {
			cred, err := credentials.NewClientTLSFromFile(mc.TLS.CAFile, "")
			if err != nil {
				return nil, fmt.Errorf("load tls ca file: %w", err)
			}
			config.DialOptions = append(config.DialOptions, grpc.WithTransportCredentials(cred))
		} else if mc.TLS.SkipVerify {
			config.DialOptions = append(config.DialOptions, grpc.WithTransportCredentials(
				credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})),
			)
		}
	}

	c, err := client.NewClient(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect to milvus: %w", err)
	}

	store := &milvusStore{
		client:     c,
		collection: mc.Collection,
		dimension:  mc.Dimension,
	}

	if err := store.ensureCollection(ctx, mc.Retention); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("ensure collection: %w", err)
	}

	return store, nil
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
		[]string{"id", "body", "service", "severity", "timestamp"},
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
			}
		}

		out = append(out, sr)
	}

	return out, nil
}

func (s *milvusStore) Close() error {
	return s.client.Close()
}
