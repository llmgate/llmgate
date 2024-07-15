package handlers

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/llmgate/llmgate/internal/utils"
	"github.com/llmgate/llmgate/openai"
	"github.com/llmgate/llmgate/pinecone"
	"github.com/llmgate/llmgate/superbase"
)

const (
	ingestionFileKey = "txt"
	chunkSize        = 20000 // Adjust the chunk size as needed
)

type IngestionHandler struct {
	openaiClient    openai.OpenAIClient
	pineconeClient  pinecone.PineconeClient
	superbaseClient superbase.SupabaseClient
	embeddingModal  string
}

func NewIngestionHandler(
	openaiClient openai.OpenAIClient,
	pineconeClient pinecone.PineconeClient,
	superbaseClient superbase.SupabaseClient,
	embeddingModal string) *IngestionHandler {
	return &IngestionHandler{
		openaiClient:    openaiClient,
		pineconeClient:  pineconeClient,
		superbaseClient: superbaseClient,
		embeddingModal:  embeddingModal,
	}
}

func (h *IngestionHandler) IngestData(c *gin.Context) {
	endpointId := c.Param("endpointId")
	if endpointId == "" {
		utils.ProcessGenericBadRequest(c)
		return
	}

	filePath, err := h.saveUploadedFile(c)
	if err != nil {
		utils.ProcessGenericBadRequest(c)
		return
	}
	defer os.Remove(filePath)

	_, err = h.storeIngestionRecord(endpointId)
	if err != nil {
		utils.ProcessGenericInternalError(c)
		return
	}

	content, err := h.readFileContent(filePath)
	if err != nil {
		utils.ProcessGenericInternalError(c)
		return
	}

	chunks := splitIntoChunks(content, chunkSize)
	for _, chunk := range chunks {
		if err := h.processChunk(chunk, endpointId); err != nil {
			utils.ProcessGenericInternalError(c)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *IngestionHandler) saveUploadedFile(c *gin.Context) (string, error) {
	file, err := c.FormFile(ingestionFileKey)
	if err != nil {
		return "", err
	}

	filePath := fmt.Sprintf("./%s", file.Filename)
	err = c.SaveUploadedFile(file, filePath)
	return filePath, err
}

func (h *IngestionHandler) storeIngestionRecord(endpointId string) (string, error) {
	ingestionId := uuid.New().String()
	err := h.superbaseClient.StoreIngestion(superbase.Ingestion{
		IngestionId: ingestionId,
		ContentType: "txt",
		DataUrl:     "todo",
		EndpointId:  endpointId,
	})
	return ingestionId, err
}

func (h *IngestionHandler) readFileContent(filePath string) (string, error) {
	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(contentBytes), nil
}

func (h *IngestionHandler) processChunk(chunk, endpointId string) error {
	embedding, err := h.openaiClient.GenerateEmbeddings(openai.EmbeddingsPayload{
		Model: h.embeddingModal,
		Input: []string{chunk},
	})
	if err != nil || len(embedding.Data) == 0 {
		return fmt.Errorf("something went wrong. please try again!")
	}

	return h.pineconeClient.Store(embedding.Data[0].Embedding, endpointId, chunk)
}

func splitIntoChunks(text string, size int) []string {
	var chunks []string
	for len(text) > 0 {
		if len(text) > size {
			chunks = append(chunks, text[:size])
			text = text[size:]
		} else {
			chunks = append(chunks, text)
			break
		}
	}
	return chunks
}
