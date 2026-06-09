package orchestrator

import (
	"context"
	"crypto/sha256"
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
	FromCache  bool   `json:"from_cache,omitempty"`
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

// computeSHA256 calcula el hash SHA-256 del contenido del archivo
func computeSHA256(content []byte) string {
	hash := sha256.Sum256(content)
	return fmt.Sprintf("%x", hash[:])
}

// ProcessDocument orquesta el flujo completo de procesamiento
// Flujo: Hash Check → Extract → Summary → Persist (via Controller → Persistence)
//
// Si el documento ya fue procesado (mismo hash SHA-256), devuelve el resumen
// existente sin volver a procesar. Esto evita saturar la IA con documentos
// duplicados, incluso si el usuario cambia el nombre del archivo.
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

	// ──────────────────────────────────────────────────────────────
	// Paso 1: Calcular SHA-256 del archivo
	// ──────────────────────────────────────────────────────────────
	contentHash := computeSHA256(fileContent)

	// ──────────────────────────────────────────────────────────────
	// Paso 2: Verificar cache primero (por hash)
	// ──────────────────────────────────────────────────────────────
	cacheKey := fmt.Sprintf("document:hash:%s", contentHash)
	var cached DocumentProcessResult
	if o.cache != nil {
		if err := o.cache.Get(ctx, cacheKey, &cached); err == nil {
			cached.FromCache = true
			return &cached, nil
		}
	}

	// ──────────────────────────────────────────────────────────────
	// Paso 3: Verificar si el hash ya existe en la BD (deduplicación)
	// Si ya existe un documento con el mismo contenido, devolver
	// el resumen existente sin procesar de nuevo.
	// ──────────────────────────────────────────────────────────────
	existingDoc, err := o.persistenceService.GetDocumentByHash(ctx, contentHash, headers)
	if err != nil {
		// Log pero no falla — seguimos con el procesamiento normal
		fmt.Printf("warning: failed to check document hash: %v\n", err)
	}

	if existingDoc != nil && existingDoc.ID != "" {
		// Documento ya existe — buscar su resumen
		existingSummary, err := o.persistenceService.GetSummaryByDocumentID(ctx, existingDoc.ID, headers)
		if err == nil && existingSummary != "" {
			result := &DocumentProcessResult{
				DocumentID: existingDoc.ID,
				Summary:    existingSummary,
				Text:       existingDoc.Text,
				Status:     "ready",
				FromCache:  true,
			}

			// Cachear para futuras consultas
			if o.cache != nil {
				ttl := 24 * time.Hour
				if cacheErr := o.cache.Set(ctx, cacheKey, result, ttl); cacheErr != nil {
					fmt.Printf("warning: failed to cache result: %v\n", cacheErr)
				}
			}

			return result, nil
		}
	}

	// ──────────────────────────────────────────────────────────────
	// Paso 4: Extraer texto del PDF
	// ──────────────────────────────────────────────────────────────
	extractResp, err := o.extractService.Extract(ctx, filename, fileContent, headers)
	if err != nil {
		return nil, fmt.Errorf("extract failed: %w", err)
	}

	if extractResp == nil || extractResp.Text == "" {
		return nil, errors.New("no text extracted from document")
	}

	// ──────────────────────────────────────────────────────────────
	// Paso 5: Guardar documento con texto extraído en persistencia
	// (Controller → Persistence, no en Summary Service)
	// ──────────────────────────────────────────────────────────────
	docToSave := &services.Document{
		ID:          documentID,
		UserID:      userID,
		Text:        extractResp.Text,
		Filename:    filename,
		Status:      "extracted",
		ContentHash: contentHash,
	}

	if err := o.persistenceService.SaveDocument(ctx, docToSave, headers); err != nil {
		return nil, fmt.Errorf("persistence save failed: %w", err)
	}

	// ──────────────────────────────────────────────────────────────
	// Paso 6: Generar resumen (Summary Service SOLO resume)
	// ──────────────────────────────────────────────────────────────
	summaryResp, err := o.summaryService.Generate(ctx, extractResp.Text, docToSave.ID, filename, headers)
	if err != nil {
		return nil, fmt.Errorf("summary generation failed: %w", err)
	}

	if summaryResp == nil || summaryResp.Summary == "" {
		return nil, errors.New("no summary generated")
	}

	// ──────────────────────────────────────────────────────────────
	// Paso 7: Guardar resumen en persistencia
	// (Controller → Persistence, no Summary Service)
	// ──────────────────────────────────────────────────────────────
	if err := o.persistenceService.SaveSummary(ctx, docToSave.ID, summaryResp.Summary, headers); err != nil {
		return nil, fmt.Errorf("persistence summary save failed: %w", err)
	}

	// ──────────────────────────────────────────────────────────────
	// Paso 8: Cachear resultado (por hash, no por nombre)
	// ──────────────────────────────────────────────────────────────
	result := &DocumentProcessResult{
		DocumentID: docToSave.ID,
		Summary:    summaryResp.Summary,
		Text:       extractResp.Text,
		Status:     "ready",
	}

	// TTL de 24 horas
	if o.cache != nil {
		ttl := 24 * time.Hour
		if err := o.cache.Set(ctx, cacheKey, result, ttl); err != nil {
			// Log pero no falla el request
			fmt.Printf("warning: failed to cache result: %v\n", err)
		}
		// También cachear por user+document para GetDocument
		userCacheKey := fmt.Sprintf("document:%s:%s", userID, docToSave.ID)
		if err := o.cache.Set(ctx, userCacheKey, result, ttl); err != nil {
			fmt.Printf("warning: failed to cache user result: %v\n", err)
		}
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
	if o.cache != nil {
		if err := o.cache.Get(ctx, cacheKey, &cached); err == nil {
			return &cached, nil
		}
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
	if o.cache != nil {
		if err := o.cache.Set(ctx, cacheKey, result, 24*time.Hour); err != nil {
			fmt.Printf("warning: failed to cache document: %v\n", err)
		}
	}

	return result, nil
}

// InvalidateDocumentCache invalida el cache de un documento
func (o *Orchestrator) InvalidateDocumentCache(ctx context.Context, documentID string, userID string) error {
	cacheKey := fmt.Sprintf("document:%s:%s", userID, documentID)
	if o.cache != nil {
		return o.cache.Delete(ctx, cacheKey)
	}
	return nil
}
