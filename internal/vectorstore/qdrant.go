package vectorstore

import (
	"context"
	"fmt"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/serengeti-sh/meerkat/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	qdrantDefaultTimeout = 10 * time.Second
)

// qdrantStore implements VectorStore using Qdrant.
type qdrantStore struct {
	conn       *grpc.ClientConn
	client     qdrant.PointsClient
	collection string
	dimension  int
}

var _ Store = (*qdrantStore)(nil)

// NewQdrantClient creates a Store backed by Qdrant.
func NewQdrantClient(cfg *config.Config) (Store, error) {
	qc := cfg.VectorStore.Qdrant

	ctx, cancel := context.WithTimeout(context.Background(), qdrantDefaultTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, qc.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to qdrant: %w", err)
	}

	store := &qdrantStore{
		conn:       conn,
		client:     qdrant.NewPointsClient(conn),
		collection: qc.Collection,
		dimension:  qc.Dimension,
	}

	if err := store.ensureCollection(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ensure collection: %w", err)
	}

	return store, nil
}

func (s *qdrantStore) ensureCollection(ctx context.Context) error {
	collectionsClient := qdrant.NewCollectionsClient(s.conn)

	resp, err := collectionsClient.List(ctx, &qdrant.ListCollectionsRequest{})
	if err != nil {
		return fmt.Errorf("list collections: %w", err)
	}

	for _, col := range resp.GetCollections() {
		if col.GetName() == s.collection {
			return nil
		}
	}

	_, err = collectionsClient.Create(ctx, &qdrant.CreateCollection{
		CollectionName: s.collection,
		VectorsConfig: &qdrant.VectorsConfig{
			Config: &qdrant.VectorsConfig_Params{
				Params: &qdrant.VectorParams{
					Size:     uint64(s.dimension),
					Distance: qdrant.Distance_Cosine,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("create collection: %w", err)
	}

	return nil
}

func (s *qdrantStore) Insert(ctx context.Context, records []Record) error {
	if len(records) == 0 {
		return nil
	}

	points := make([]*qdrant.PointStruct, len(records))
	for i, r := range records {
		points[i] = &qdrant.PointStruct{
			Id:      qdrant.NewIDUUID(r.ID),
			Vectors: qdrant.NewVectors(r.Vector...),
			Payload: map[string]*qdrant.Value{
				"timestamp": {Kind: &qdrant.Value_IntegerValue{IntegerValue: r.Timestamp.UnixMilli()}},
				"service":   {Kind: &qdrant.Value_StringValue{StringValue: r.Service}},
				"severity":  {Kind: &qdrant.Value_StringValue{StringValue: r.Severity}},
				"body":      {Kind: &qdrant.Value_StringValue{StringValue: r.Body}},
			},
		}
	}

	_, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: s.collection,
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("upsert points: %w", err)
	}

	return nil
}

func (s *qdrantStore) Search(ctx context.Context, vector []float32, opts SearchOptions) ([]SearchResult, error) {
	searchReq := &qdrant.SearchPoints{
		CollectionName: s.collection,
		Vector:         vector,
		Limit:          uint64(opts.Limit),
		WithPayload:    qdrant.NewWithPayloadInclude("timestamp", "service", "severity", "body"),
	}

	if opts.Limit <= 0 {
		searchReq.Limit = 10
	}

	var mustConditions []*qdrant.Condition

	if opts.TimeRange > 0 {
		cutoff := float64(time.Now().Add(-opts.TimeRange).UnixMilli())
		mustConditions = append(mustConditions, &qdrant.Condition{
			ConditionOneOf: &qdrant.Condition_Field{
				Field: &qdrant.FieldCondition{
					Key: "timestamp",
					Range: &qdrant.Range{
						Gte: &cutoff,
					},
				},
			},
		})
	}

	if opts.Service != "" {
		mustConditions = append(mustConditions, &qdrant.Condition{
			ConditionOneOf: &qdrant.Condition_Field{
				Field: &qdrant.FieldCondition{
					Key: "service",
					Match: &qdrant.Match{
						MatchValue: &qdrant.Match_Keyword{Keyword: opts.Service},
					},
				},
			},
		})
	}

	if opts.Severity != "" {
		mustConditions = append(mustConditions, &qdrant.Condition{
			ConditionOneOf: &qdrant.Condition_Field{
				Field: &qdrant.FieldCondition{
					Key: "severity",
					Match: &qdrant.Match{
						MatchValue: &qdrant.Match_Keyword{Keyword: opts.Severity},
					},
				},
			},
		})
	}

	if len(mustConditions) > 0 {
		searchReq.Filter = &qdrant.Filter{
			Must: mustConditions,
		}
	}

	results, err := s.client.Search(ctx, searchReq)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	return s.parseSearchResults(results.GetResult()), nil
}

func (s *qdrantStore) parseSearchResults(results []*qdrant.ScoredPoint) []SearchResult {
	out := make([]SearchResult, 0, len(results))
	for _, r := range results {
		sr := SearchResult{
			ID:    r.GetId().GetUuid(),
			Score: float32(r.GetScore()),
		}

		payload := r.GetPayload()
		if body, ok := payload["body"]; ok {
			sr.Body = body.GetStringValue()
		}
		if svc, ok := payload["service"]; ok {
			sr.Service = svc.GetStringValue()
		}
		if sev, ok := payload["severity"]; ok {
			sr.Severity = sev.GetStringValue()
		}
		if ts, ok := payload["timestamp"]; ok {
			sr.Timestamp = time.UnixMilli(ts.GetIntegerValue())
		}

		out = append(out, sr)
	}
	return out
}

func (s *qdrantStore) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	points := make([]*qdrant.PointId, len(ids))
	for i, id := range ids {
		points[i] = qdrant.NewIDUUID(id)
	}

	_, err := s.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: s.collection,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Points{
				Points: &qdrant.PointsIdsList{
					Ids: points,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("delete points: %w", err)
	}

	return nil
}

func (s *qdrantStore) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
