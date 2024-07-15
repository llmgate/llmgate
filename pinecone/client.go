package pinecone

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/llmgate/llmgate/internal/config"
	"github.com/pinecone-io/go-pinecone/pinecone"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	contentKey = "content"
	idKey      = "endpointId"
)

type PineconeClient struct {
	pineconeConfig config.PineconeConfig
}

func NewPineconeClient(pineconeConfig config.PineconeConfig) *PineconeClient {
	return &PineconeClient{
		pineconeConfig: pineconeConfig,
	}
}

func (c PineconeClient) Store(embedding []float32,
	endpointId string,
	content string) error {
	ctx := context.Background()

	client, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey: c.pineconeConfig.Key,
	})
	if err != nil {
		return err
	}

	index, err := client.Index(c.pineconeConfig.IndexHost)
	if err != nil {
		return err
	}

	metadata, err := structpb.NewStruct(map[string]interface{}{
		contentKey: content,
		idKey:      endpointId,
	})
	if err != nil {
		return err
	}

	vector := &pinecone.Vector{
		Id:       uuid.New().String(),
		Values:   embedding,
		Metadata: metadata,
	}

	_, err = index.UpsertVectors(ctx, []*pinecone.Vector{vector})
	return err
}

func (c PineconeClient) Query(
	queryEmbedding []float32,
	endpointId string) (string, error) {
	ctx := context.Background()

	client, err := pinecone.NewClient(pinecone.NewClientParams{
		ApiKey: c.pineconeConfig.Key,
	})
	if err != nil {
		return "", err
	}
	index, err := client.Index(c.pineconeConfig.IndexHost)
	if err != nil {
		return "", err
	}

	filter, err := structpb.NewStruct(map[string]interface{}{
		idKey: endpointId,
	})
	if err != nil {
		return "", err
	}

	queryReq := pinecone.QueryByVectorValuesRequest{
		TopK:            5,
		Vector:          queryEmbedding,
		IncludeMetadata: true,
		Filter:          filter,
	}

	queryRes, err := index.QueryByVectorValues(ctx, &queryReq)
	if err != nil {
		return "", err
	}

	return aggregateResults(queryRes.Matches), nil
}

func aggregateResults(results []*pinecone.ScoredVector) string {
	var aggregatedContent strings.Builder
	for _, result := range results {
		if content, ok := result.Vector.Metadata.Fields[contentKey]; ok {
			aggregatedContent.WriteString(content.GetStringValue())
			aggregatedContent.WriteString("\n\n")
		}
	}
	return aggregatedContent.String()
}
