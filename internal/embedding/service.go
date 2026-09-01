package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	chromem "github.com/philippgille/chromem-go"
	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/logger"
	"github.com/songtianlun/diarum/internal/store"
)

// EmbeddingService handles diary embedding operations
type EmbeddingService struct {
	store         *store.Store
	vectorDB      *VectorDB
	configService *config.ConfigService
}

// BuildResult represents the result of a build operation
type BuildResult struct {
	Success int      `json:"success"`
	Failed  int      `json:"failed"`
	Total   int      `json:"total"`
	Errors  []string `json:"errors,omitempty"`
}

// VectorStats represents statistics about the vector index
type VectorStats struct {
	DiaryCount    int `json:"diary_count"`
	IndexedCount  int `json:"indexed_count"`
	OutdatedCount int `json:"outdated_count"`
	PendingCount  int `json:"pending_count"`
}

// EmbeddingRequest represents a request to the embedding API
type EmbeddingRequest struct {
	Input string `json:"input"`
	Model string `json:"model"`
}

// EmbeddingResponse represents the response from the embedding API
type EmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// NewEmbeddingService creates a new EmbeddingService
func NewEmbeddingService(store *store.Store, vectorDB *VectorDB) *EmbeddingService {
	return &EmbeddingService{
		store:         store,
		vectorDB:      vectorDB,
		configService: config.NewConfigService(store),
	}
}

// createEmbeddingFunc creates an embedding function for the given user's configuration
func (s *EmbeddingService) createEmbeddingFunc(userID string) (chromem.EmbeddingFunc, error) {
	apiKey, err := s.configService.GetString(userID, "ai.api_key")
	if err != nil || apiKey == "" {
		return nil, fmt.Errorf("AI API key not configured")
	}

	baseURL, err := s.configService.GetString(userID, "ai.base_url")
	if err != nil || baseURL == "" {
		return nil, fmt.Errorf("AI base URL not configured")
	}

	embeddingModel, err := s.configService.GetString(userID, "ai.embedding_model")
	if err != nil || embeddingModel == "" {
		return nil, fmt.Errorf("embedding model not configured")
	}

	// Normalize base URL: strip trailing /v1 to avoid double /v1 in request URLs
	baseURL = strings.TrimSuffix(baseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/v1")

	// Debug log configuration (mask API key)
	maskedKey := "***"
	if len(apiKey) > 8 {
		maskedKey = apiKey[:4] + "***" + apiKey[len(apiKey)-4:]
	}
	logger.Debug("[EmbeddingService] config: baseURL=%s, model=%s, apiKey=%s", baseURL, embeddingModel, maskedKey)

	return func(ctx context.Context, text string) ([]float32, error) {
		return s.generateEmbedding(ctx, baseURL, apiKey, embeddingModel, text)
	}, nil
}

// generateEmbedding produces an embedding for text of any length.
//
// Embedding models have a hard input limit (commonly 8192 tokens), so long
// diaries are split into chunks that are embedded separately and then averaged
// into a single vector. If the provider still reports a context-length error
// (our token estimate is heuristic), the budget is halved and retried.
func (s *EmbeddingService) generateEmbedding(ctx context.Context, baseURL, apiKey, model, text string) ([]float32, error) {
	maxTokens := defaultMaxEmbeddingTokens

	for attempt := 0; ; attempt++ {
		vector, err := s.embedChunked(ctx, baseURL, apiKey, model, text, maxTokens)
		if err == nil {
			return vector, nil
		}
		if !isContextLengthError(err) || attempt >= maxEmbeddingSplitRetries {
			return nil, err
		}

		next := maxTokens / 2
		if next < minEmbeddingChunkTokens {
			return nil, err
		}
		logger.Warn("[EmbeddingService] context length exceeded, retrying with chunk budget %d tokens: %v", next, err)
		maxTokens = next
	}
}

// embedChunked splits text into chunks under maxTokens, embeds each chunk and
// merges the results into one vector.
func (s *EmbeddingService) embedChunked(ctx context.Context, baseURL, apiKey, model, text string, maxTokens int) ([]float32, error) {
	chunks := splitTextForEmbedding(text, maxTokens)
	if len(chunks) == 0 {
		// Blank input: keep the previous behaviour and let the provider decide.
		return s.requestEmbedding(ctx, baseURL, apiKey, model, text)
	}
	if len(chunks) == 1 {
		return s.requestEmbedding(ctx, baseURL, apiKey, model, chunks[0])
	}

	logger.Info("[EmbeddingService] text exceeds embedding limit (~%d tokens), splitting into %d chunks",
		estimateTokens(text), len(chunks))

	vectors := make([][]float32, 0, len(chunks))
	weights := make([]float64, 0, len(chunks))
	for i, chunk := range chunks {
		vector, err := s.requestEmbedding(ctx, baseURL, apiKey, model, chunk)
		if err != nil {
			return nil, fmt.Errorf("chunk %d/%d: %w", i+1, len(chunks), err)
		}
		vectors = append(vectors, vector)
		weights = append(weights, float64(estimateTokens(chunk)))
	}

	return averageVectors(vectors, weights), nil
}

// requestEmbedding calls the OpenAI-compatible embedding API for a single input
func (s *EmbeddingService) requestEmbedding(ctx context.Context, baseURL, apiKey, model, text string) ([]float32, error) {
	url := baseURL + "/v1/embeddings"

	reqBody := EmbeddingRequest{
		Input: text,
		Model: model,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Error("[EmbeddingService] embedding API error: status=%d, url=%s, response=%s", resp.StatusCode, url, string(body))
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var embResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data in response")
	}

	return embResp.Data[0].Embedding, nil
}

// BuildAllVectors rebuilds vectors for ALL diaries (full rebuild)
func (s *EmbeddingService) BuildAllVectors(ctx context.Context, userID string) (*BuildResult, error) {
	logger.Info("[EmbeddingService] starting full vector rebuild for user: %s", userID)

	// Check if AI is enabled
	enabled, _ := s.configService.GetBool(userID, "ai.enabled")
	if !enabled {
		return nil, fmt.Errorf("AI features are not enabled")
	}

	// Create embedding function
	embeddingFunc, err := s.createEmbeddingFunc(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding function: %w", err)
	}

	// Delete existing collection and create a new one
	if err := s.vectorDB.DeleteCollection(userID); err != nil {
		logger.Warn("[EmbeddingService] failed to delete existing collection: %v", err)
	}

	collection, err := s.vectorDB.GetOrCreateCollection(ctx, userID, embeddingFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to create collection: %w", err)
	}

	// Get all diaries for the user
	diaries, err := s.store.ListDiaries(userID, "", "", "-date", 0)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch diaries: %w", err)
	}

	result := &BuildResult{
		Total:  len(diaries),
		Errors: make([]string, 0),
	}

	if len(diaries) == 0 {
		logger.Info("[EmbeddingService] no diaries found for user: %s", userID)
		return result, nil
	}

	// Process all diaries
	for _, diary := range diaries {
		if err := s.processDiary(ctx, collection, diary, embeddingFunc, nil); err != nil {
			result.Failed++
			dateStr := store.DateOnly(diary.Date)
			result.Errors = append(result.Errors, dateStr)
			logger.Error("[EmbeddingService] Diary %s: %v", dateStr, err)
		} else {
			result.Success++
		}
	}

	logger.Info("[EmbeddingService] full rebuild completed for user %s: %d success, %d failed",
		userID, result.Success, result.Failed)

	return result, nil
}

// BuildIncrementalVectors builds vectors only for new and outdated diaries,
// optimizing for metadata-only changes (weather/mood/tags/...) by reusing
// existing embeddings instead of re-calling the embedding API.
func (s *EmbeddingService) BuildIncrementalVectors(ctx context.Context, userID string) (*BuildResult, error) {
	logger.Info("[EmbeddingService] starting incremental vector build for user: %s", userID)

	// Check if AI is enabled
	enabled, _ := s.configService.GetBool(userID, "ai.enabled")
	if !enabled {
		return nil, fmt.Errorf("AI features are not enabled")
	}

	// Create embedding function
	embeddingFunc, err := s.createEmbeddingFunc(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding function: %w", err)
	}

	// Get or create collection (keep existing)
	collection, err := s.vectorDB.GetOrCreateCollection(ctx, userID, embeddingFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to create collection: %w", err)
	}

	// Get all diaries for the user
	diaries, err := s.store.ListDiaries(userID, "", "", "-date", 0)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch diaries: %w", err)
	}

	result := &BuildResult{
		Total:  len(diaries),
		Errors: make([]string, 0),
	}

	if len(diaries) == 0 {
		logger.Info("[EmbeddingService] no diaries found for user: %s", userID)
		return result, nil
	}

	// 分类处理：content 变更 → 重建 embedding；纯 metadata 变更 → 复用向量仅同步 metadata
	skipped := 0
	metadataSynced := 0
	for _, diary := range diaries {
		status, existingDoc, found := s.classifyDiaryStatus(ctx, collection, diary)
		switch status {
		case vectorStatusUpToDate:
			skipped++
			continue
		case vectorStatusMetadataSyncNeeded:
			// 复用已有向量，只更新 chromem 里的 metadata（weather/mood/tags 等），不重调 embedding API
			var existingEmbedding []float32
			if found {
				existingEmbedding = existingDoc.Embedding
			}
			if err := s.processDiary(ctx, collection, diary, embeddingFunc, existingEmbedding); err != nil {
				result.Failed++
				dateStr := store.DateOnly(diary.Date)
				result.Errors = append(result.Errors, dateStr)
				logger.Error("[EmbeddingService] Diary %s metadata sync: %v", dateStr, err)
			} else {
				metadataSynced++
				result.Success++
			}
		case vectorStatusMissing, vectorStatusRebuildNeeded:
			// content 变了或 chromem 里没有 → 需要新调用 embedding API
			if err := s.processDiary(ctx, collection, diary, embeddingFunc, nil); err != nil {
				result.Failed++
				dateStr := store.DateOnly(diary.Date)
				result.Errors = append(result.Errors, dateStr)
				logger.Error("[EmbeddingService] Diary %s: %v", dateStr, err)
			} else {
				result.Success++
			}
		}
	}

	logger.Info("[EmbeddingService] incremental build completed for user %s: %d rebuilt, %d metadata-synced, %d skipped, %d failed",
		userID, result.Success-metadataSynced, metadataSynced, skipped, result.Failed)

	return result, nil
}

// processDiary processes a single diary entry.
// existingEmbedding 为 nil 时会新调用 embedding API；非 nil 时直接复用（仅更新 metadata）。
func (s *EmbeddingService) processDiary(ctx context.Context, collection *chromem.Collection, diary *store.Diary, embeddingFunc chromem.EmbeddingFunc, existingEmbedding []float32) error {
	content := diary.Content
	if content == "" {
		return nil // Skip empty diaries
	}

	diaryID := diary.ID
	dateStr := store.DateOnly(diary.Date)
	mood := diary.Mood
	moodStatesJSON, _ := json.Marshal(diary.MoodStates)
	scenariosJSON, _ := json.Marshal(diary.Scenarios)
	weather := diary.Weather
	builtAt := time.Now().UTC().Format(time.RFC3339Nano)

	var embedding []float32
	if existingEmbedding != nil {
		// 复用已有向量（metadata-only sync，不重调 embedding API）
		embedding = existingEmbedding
	} else {
		var err error
		embedding, err = embeddingFunc(ctx, content)
		if err != nil {
			return fmt.Errorf("failed to generate embedding: %w", err)
		}
	}

	// Create document with metadata and pre-generated embedding
	doc := chromem.Document{
		ID:        diaryID,
		Content:   content,
		Embedding: embedding,
		Metadata: map[string]string{
			"date":        dateStr,
			"mood":        fmt.Sprintf("%d", mood),
			"mood_states": string(moodStatesJSON),
			"scenarios":   string(scenariosJSON),
			"weather":     weather,
			"built_at":    builtAt,
		},
	}

	// Add document to collection (upsert by ID)
	if err := collection.AddDocument(ctx, doc); err != nil {
		return fmt.Errorf("failed to add document: %w", err)
	}

	return nil
}

func parseStoreTime(value string) (time.Time, error) {
	if parsed, err := time.Parse("2006-01-02 15:04:05.000Z", value); err == nil {
		return parsed, nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, nil
	}
	return time.Parse(time.RFC3339, value)
}

// diaryVectorStatus classifies what action a diary needs relative to the vector index.
type diaryVectorStatus int

const (
	vectorStatusMissing diaryVectorStatus = iota // chromem 里没有 → 需要完整构建
	vectorStatusRebuildNeeded                    // content 变了 → 需要重新 embedding
	vectorStatusMetadataSyncNeeded               // 只有 metadata 变了 → 只同步 metadata
	vectorStatusUpToDate                         // 都没变 → 跳过
)

// classifyDiaryStatus 检查一条日记相对于 chromem 向量索引的状态。
// 区分 content 变更（需重建 embedding）和纯 metadata 变更（只需同步 metadata）。
// 返回: 状态、chromem 中的现有文档（仅 metadata sync 时需要其 Embedding 复用）、是否找到现有文档
func (s *EmbeddingService) classifyDiaryStatus(ctx context.Context, collection *chromem.Collection, diary *store.Diary) (diaryVectorStatus, chromem.Document, bool) {
	if collection == nil {
		return vectorStatusMissing, chromem.Document{}, false
	}

	doc, err := collection.GetByID(ctx, diary.ID)
	if err != nil {
		return vectorStatusMissing, chromem.Document{}, false
	}

	// content_updated 为空（极旧数据）时降级：视为 content 没变，只比较 updated
	contentUpdatedStr := diary.ContentUpdated
	if contentUpdatedStr == "" {
		contentUpdatedStr = diary.Updated
	}
	contentUpdated, err := parseStoreTime(contentUpdatedStr)
	if err != nil {
		return vectorStatusRebuildNeeded, doc, true
	}

	updated, err := parseStoreTime(diary.Updated)
	if err != nil {
		updated = contentUpdated
	}

	builtAtStr, ok := doc.Metadata["built_at"]
	if !ok || builtAtStr == "" {
		return vectorStatusRebuildNeeded, doc, true
	}
	builtAt, err := time.Parse(time.RFC3339Nano, builtAtStr)
	if err != nil {
		builtAt, err = time.Parse(time.RFC3339, builtAtStr)
		if err != nil {
			return vectorStatusRebuildNeeded, doc, true
		}
	}

	// content 变了 → 需要重建 embedding
	if contentUpdated.After(builtAt) {
		return vectorStatusRebuildNeeded, doc, true
	}

	// 只有 metadata 变了 → 只需同步 metadata
	if updated.After(builtAt) {
		return vectorStatusMetadataSyncNeeded, doc, true
	}

	return vectorStatusUpToDate, doc, true
}

// needsBuildVector 保留向后兼容：仅判断是否需要重建 embedding（content 变了或缺失）
func (s *EmbeddingService) needsBuildVector(ctx context.Context, collection *chromem.Collection, diary *store.Diary) bool {
	status, _, _ := s.classifyDiaryStatus(ctx, collection, diary)
	return status == vectorStatusMissing || status == vectorStatusRebuildNeeded
}

// DiarySearchResult represents a diary found by vector search
type DiarySearchResult struct {
	ID         string   `json:"id"`
	Date       string   `json:"date"`
	Content    string   `json:"content"`
	Mood       int      `json:"mood,omitempty"`
	MoodStates []string `json:"mood_states,omitempty"`
	Scenarios  []string `json:"scenarios,omitempty"`
	Weather    string   `json:"weather,omitempty"`
	Score      float32  `json:"score"`
}

// QuerySimilar finds diaries similar to the given query
func (s *EmbeddingService) QuerySimilar(ctx context.Context, userID, query string, limit int) ([]DiarySearchResult, error) {
	logger.Info("[EmbeddingService] querying similar diaries for user: %s", userID)

	// Check if AI is enabled
	enabled, _ := s.configService.GetBool(userID, "ai.enabled")
	if !enabled {
		return nil, fmt.Errorf("AI features are not enabled")
	}

	// Create embedding function
	embeddingFunc, err := s.createEmbeddingFunc(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding function: %w", err)
	}

	// Get collection
	collection, err := s.vectorDB.GetOrCreateCollection(ctx, userID, embeddingFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection: %w", err)
	}

	// Check collection count and adjust limit if necessary
	// chromem-go requires nResults <= number of documents in collection
	docCount := collection.Count()
	if docCount == 0 {
		logger.Info("[EmbeddingService] collection is empty, no documents to query")
		return []DiarySearchResult{}, nil
	}
	if limit > docCount {
		logger.Debug("[EmbeddingService] adjusting limit from %d to %d (collection size)", limit, docCount)
		limit = docCount
	}

	// Query similar documents
	results, err := collection.Query(ctx, query, limit, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query collection: %w", err)
	}

	// Convert to DiarySearchResult
	searchResults := make([]DiarySearchResult, 0, len(results))
	for _, result := range results {
		moodInt := 0
		fmt.Sscanf(result.Metadata["mood"], "%d", &moodInt)
		var moodStates []string
		_ = json.Unmarshal([]byte(result.Metadata["mood_states"]), &moodStates)
		var scenarios []string
		_ = json.Unmarshal([]byte(result.Metadata["scenarios"]), &scenarios)
		searchResults = append(searchResults, DiarySearchResult{
			ID:         result.ID,
			Date:       result.Metadata["date"],
			Content:    result.Content,
			Mood:       moodInt,
			MoodStates: moodStates,
			Scenarios:  scenarios,
			Weather:    result.Metadata["weather"],
			Score:      result.Similarity,
		})
	}

	logger.Info("[EmbeddingService] found %d similar diaries", len(searchResults))
	return searchResults, nil
}

// GetVectorStats returns statistics about the vector index for a user
func (s *EmbeddingService) GetVectorStats(ctx context.Context, userID string) (*VectorStats, error) {
	stats := &VectorStats{}

	// Get all diaries for the user
	diaries, err := s.store.ListDiaries(userID, "", "", "updated", 0)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch diaries: %w", err)
	}
	stats.DiaryCount = len(diaries)

	// Get collection
	collection := s.vectorDB.GetCollection(userID)

	// Compare each diary with its vector
	for _, diary := range diaries {
		diaryID := diary.ID
		// content_updated 才是向量是否过时的判断依据
		contentUpdatedStr := diary.ContentUpdated
		if contentUpdatedStr == "" {
			contentUpdatedStr = diary.Updated // 旧数据降级
		}
		diaryContentUpdated, err := parseStoreTime(contentUpdatedStr)
		if err != nil {
			stats.OutdatedCount++
			continue
		}

		// Try to get the vector document
		if collection == nil {
			stats.PendingCount++
			continue
		}

		doc, err := collection.GetByID(ctx, diaryID)
		if err != nil {
			// Document not found - pending
			stats.PendingCount++
			continue
		}

		// Check build time from metadata
		builtAtStr, ok := doc.Metadata["built_at"]
		if !ok || builtAtStr == "" {
			// No build time - treat as outdated
			stats.OutdatedCount++
			continue
		}

		builtAt, err := time.Parse(time.RFC3339Nano, builtAtStr)
		if err != nil {
			// Fallback to RFC3339 for backward compatibility
			builtAt, err = time.Parse(time.RFC3339, builtAtStr)
			if err != nil {
				stats.OutdatedCount++
				continue
			}
		}

		// Compare content update time — 只有 content 变更才会让向量语义过时
		if diaryContentUpdated.After(builtAt) {
			stats.OutdatedCount++
		} else {
			stats.IndexedCount++
		}
	}

	return stats, nil
}
