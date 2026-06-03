package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"service-controller-notebookum/internal/core/cache"
	"service-controller-notebookum/internal/transport/services"
)

// DocumentProcessResult es el resultado final del procesamiento
type DocumentProcessResult struct {
	DocumentID string `json:"document_id"`
	Summary    string `json:"summary"`
	Text       string `json:"text,omitempty"`
	Status     string `json:"status"`
}

// Orchestrator coordina el flujo completo: extract → summary → persist
type Orchestrator struct {
	userService        *services.UserService
	extractService     *services.ExtractService
	summaryService     *services.SummaryService
	persistenceService *services.PersistenceService
	cache              *cache.RedisCache
}

// New crea una nueva instancia del orquestador
func New(
	userSvc *services.UserService,
	extractSvc *services.ExtractService,
	summarySvc *services.SummaryService,
	persistenceSvc *services.PersistenceService,
	redisCache *cache.RedisCache,
) *Orchestrator {
	return &Orchestrator{
		userService:        userSvc,
		extractService:     extractSvc,
		summaryService:     summarySvc,
		persistenceService: persistenceSvc,
		cache:              redisCache,
	}
}

// ProcessDocument orquesta el flujo completo de procesamiento
// Flujo: Extract → Save Text → Summary → Save Summary → Cache → Return
func (o *Orchestrator) ProcessDocument(
	ctx context.Context,
	documentID string,
	filename string,
	fileContent []byte,
	userID string,
	correlationID string,
) (*DocumentProcessResult, error) {

	// Crear headers con información del request
	headers := http.Header{
		"X-Correlation-ID": {correlationID},
		"X-User-ID":        {userID},
	}

	// Paso 1: Verificar cache primero
	cacheKey := fmt.Sprintf("document:%s:%s", userID, documentID)
	var cached DocumentProcessResult
	if err := o.cache.Get(ctx, cacheKey, &cached); err == nil {
		return &cached, nil
	}

	// Paso 2: Extraer texto del PDF
	extractResp, err := o.extractService.Extract(ctx, filename, fileContent, headers)
	if err != nil {
		return nil, fmt.Errorf("extract failed: %w", err)
	}

	if extractResp == nil || extractResp.Text == "" {
		return nil, errors.New("no text extracted from document")
	}

	// Paso 3: Guardar documento con texto extraído en persistencia
	docToSave := &services.Document{
		ID:       documentID,
		UserID:   userID,
		Text:     extractResp.Text,
		Filename: filename,
		Status:   "extracted",
	}

	if err := o.persistenceService.SaveDocument(ctx, docToSave, headers); err != nil {
		return nil, fmt.Errorf("persistence save failed: %w", err)
	}

	// Paso 4: Generar resumen
	summaryResp, err := o.summaryService.Generate(ctx, extractResp.Text, headers)
	if err != nil {
		return nil, fmt.Errorf("summary generation failed: %w", err)
	}

	if summaryResp == nil || summaryResp.Summary == "" {
		return nil, errors.New("no summary generated")
	}

	// Paso 5: Guardar resumen en persistencia
	if err := o.persistenceService.SaveSummary(ctx, documentID, summaryResp.Summary, headers); err != nil {
		return nil, fmt.Errorf("persistence summary save failed: %w", err)
	}

	// Paso 6: Cachear resultado
	result := &DocumentProcessResult{
		DocumentID: documentID,
		Summary:    summaryResp.Summary,
		Text:       extractResp.Text,
		Status:     "ready",
	}

	// TTL de 24 horas
	ttl := 24 * time.Hour
	if err := o.cache.Set(ctx, cacheKey, result, ttl); err != nil {
		// Log pero no falla el request
		fmt.Printf("warning: failed to cache result: %v\n", err)
	}

	return result, nil
}

// GetDocument obtiene un documento (desde cache o persistencia)
func (o *Orchestrator) GetDocument(
	ctx context.Context,
	documentID string,
	userID string,
	correlationID string,
) (*DocumentProcessResult, error) {

	headers := http.Header{
		"X-Correlation-ID": {correlationID},
		"X-User-ID":        {userID},
	}

	// Intentar obtener del cache
	cacheKey := fmt.Sprintf("document:%s:%s", userID, documentID)
	var cached DocumentProcessResult
	if err := o.cache.Get(ctx, cacheKey, &cached); err == nil {
		return &cached, nil
	}

	// Si no está en cache, obtener de persistencia
	doc, err := o.persistenceService.GetDocument(ctx, documentID, headers)
	if err != nil {
		return nil, err
	}

	result := &DocumentProcessResult{
		DocumentID: doc.ID,
		Summary:    doc.Summary,
		Text:       doc.Text,
		Status:     doc.Status,
	}

	// Cachear para futuras consultas
	if err := o.cache.Set(ctx, cacheKey, result, 24*time.Hour); err != nil {
		fmt.Printf("warning: failed to cache document: %v\n", err)
	}

	return result, nil
}

// InvalidateDocumentCache invalida el cache de un documento
func (o *Orchestrator) InvalidateDocumentCache(ctx context.Context, documentID string, userID string) error {
	cacheKey := fmt.Sprintf("document:%s:%s", userID, documentID)
	return o.cache.Delete(ctx, cacheKey)
}
