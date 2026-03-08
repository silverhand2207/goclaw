package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/providers"
)

// textReadableMIMEs are MIME types whose content can be returned directly without LLM analysis.
var textReadableMIMEs = map[string]bool{
	"application/json":       true,
	"text/csv":               true,
	"text/plain":             true,
	"text/html":              true,
	"text/xml":               true,
	"application/xml":        true,
	"text/markdown":          true,
	"application/javascript": true,
	"text/css":               true,
	"application/yaml":       true,
	"text/yaml":              true,
}

// documentMaxTextBytes is the max size for direct text return (500KB).
const documentMaxTextBytes = 500 * 1024

// --- Context helpers for media documents ---

const ctxMediaDocRefs toolContextKey = "tool_media_doc_refs"

// WithMediaDocRefs stores document MediaRefs in context for read_document tool access.
func WithMediaDocRefs(ctx context.Context, refs []providers.MediaRef) context.Context {
	return context.WithValue(ctx, ctxMediaDocRefs, refs)
}

// MediaDocRefsFromCtx retrieves stored document MediaRefs from context.
func MediaDocRefsFromCtx(ctx context.Context) []providers.MediaRef {
	v, _ := ctx.Value(ctxMediaDocRefs).([]providers.MediaRef)
	return v
}

// --- ReadDocumentTool ---

// documentMaxBytes is the max file size for document analysis (20MB).
const documentMaxBytes = 20 * 1024 * 1024

// ReadDocumentTool uses a document-capable provider to analyze files
// attached to the current conversation. Follows same pattern as ReadImageTool.
type ReadDocumentTool struct {
	registry    *providers.Registry
	mediaLoader MediaPathLoader
}

func NewReadDocumentTool(registry *providers.Registry, mediaLoader MediaPathLoader) *ReadDocumentTool {
	return &ReadDocumentTool{registry: registry, mediaLoader: mediaLoader}
}

func (t *ReadDocumentTool) Name() string { return "read_document" }

func (t *ReadDocumentTool) Description() string {
	return "Analyze documents (PDF, DOCX, images of documents, etc.) attached to the conversation. " +
		"Use when you see <media:document> tags and need to extract or analyze document content. " +
		"Specify what you want to extract or analyze."
}

func (t *ReadDocumentTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"prompt": map[string]interface{}{
				"type":        "string",
				"description": "What to analyze. E.g. 'Extract all tables', 'Summarize key findings', 'What does page 3 say?'",
			},
			"media_id": map[string]interface{}{
				"type":        "string",
				"description": "Optional: specific media_id from <media:document> tag. If omitted, uses most recent document.",
			},
		},
		"required": []string{"prompt"},
	}
}

func (t *ReadDocumentTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	prompt, _ := args["prompt"].(string)
	if prompt == "" {
		prompt = "Analyze this document and describe its contents."
	}
	mediaID, _ := args["media_id"].(string)

	// Resolve document file path from MediaRefs in context.
	docPath, docMime, err := t.resolveDocumentFile(ctx, mediaID)
	if err != nil {
		return ErrorResult(err.Error())
	}

	slog.Info("read_document: resolved file", "path", docPath, "mime", docMime, "media_id", mediaID)

	// Read document file.
	data, err := os.ReadFile(docPath)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to read document file: %v", err))
	}
	slog.Info("read_document: file loaded", "size_bytes", len(data))
	if len(data) > documentMaxBytes {
		return ErrorResult(fmt.Sprintf("Document too large: %d bytes (max %d)", len(data), documentMaxBytes))
	}

	// Fast path: text-readable files — return content directly without LLM.
	if textReadableMIMEs[docMime] || strings.HasPrefix(docMime, "text/") {
		content := string(data)
		if len(data) > documentMaxTextBytes {
			content = content[:documentMaxTextBytes] + "\n\n[... truncated at 500KB ...]"
		}
		slog.Info("read_document: returning text content directly", "mime", docMime, "size", len(data))
		return NewResult(content)
	}

	// Find a document-capable provider.
	provider, model, err := t.resolveDocumentProviderWithConfig(ctx)
	if err != nil {
		return ErrorResult(err.Error())
	}

	// Try primary provider, fallback to next available on error.
	resp, usedProvider, usedModel := t.callDocumentProvider(ctx, provider, model, prompt, data, docMime)
	if resp == nil {
		// Primary failed — try fallback providers from priority list.
		slog.Warn("read_document: primary provider failed, trying fallback", "primary", provider.Name())
		for _, fbName := range documentProviderPriority {
			if fbName == provider.Name() {
				continue // skip the one that already failed
			}
			fbProvider, fbModel, fbErr := t.resolveDocumentProviderByName(fbName)
			if fbErr != nil {
				continue
			}
			resp, usedProvider, usedModel = t.callDocumentProvider(ctx, fbProvider, fbModel, prompt, data, docMime)
			if resp != nil {
				slog.Info("read_document: fallback succeeded", "provider", usedProvider)
				break
			}
		}
	}
	if resp == nil {
		return ErrorResult("Document analysis failed: all providers returned errors")
	}

	result := NewResult(resp.Content)
	result.Usage = resp.Usage
	result.Provider = usedProvider
	result.Model = usedModel
	return result
}

