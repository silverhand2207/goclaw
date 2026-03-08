package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/providers"
)

// documentProviderPriority is the order in which providers are tried for document analysis.
// Gemini has best native PDF support (50MB, 258 tokens/page).
var documentProviderPriority = []string{"gemini", "anthropic", "openrouter", "dashscope"}

// documentModelOverrides maps provider names to preferred document-capable models.
var documentModelOverrides = map[string]string{
	"gemini":     "gemini-2.5-flash",
	"openrouter": "google/gemini-2.5-flash",
	"dashscope":  "qwen-vl-max",
}

// resolveDocumentFile finds the document file path from context MediaRefs.
func (t *ReadDocumentTool) resolveDocumentFile(ctx context.Context, mediaID string) (path, mime string, err error) {
	if t.mediaLoader == nil {
		return "", "", fmt.Errorf("no media storage configured — cannot access document files")
	}

	refs := MediaDocRefsFromCtx(ctx)
	if len(refs) == 0 {
		return "", "", fmt.Errorf("no documents available in this conversation. The user may not have sent a document.")
	}

	// Find specific media_id or use most recent document.
	var ref *providers.MediaRef
	if mediaID != "" {
		for i := range refs {
			if refs[i].ID == mediaID {
				ref = &refs[i]
				break
			}
		}
		if ref == nil {
			return "", "", fmt.Errorf("document with media_id %q not found in conversation", mediaID)
		}
	} else {
		// Use the last (most recent) document ref.
		ref = &refs[len(refs)-1]
	}

	p, err := t.mediaLoader.LoadPath(ref.ID)
	if err != nil {
		return "", "", fmt.Errorf("document file not found: %v", err)
	}

	// Determine MIME type: prefer ref's stored MIME, fall back to extension.
	mime = ref.MimeType
	if mime == "" || mime == "application/octet-stream" {
		mime = mimeFromDocExt(filepath.Ext(p))
	}

	return p, mime, nil
}

// resolveDocumentProviderWithConfig checks per-agent config, global settings, then hardcoded priority.
func (t *ReadDocumentTool) resolveDocumentProviderWithConfig(ctx context.Context) (providers.Provider, string, error) {
	// 1. Global builtin_tools.settings
	if p, model, ok := t.resolveFromBuiltinSettings(ctx); ok {
		return p, model, nil
	}
	// 2. Hardcoded defaults
	return t.resolveDocumentProvider()
}

// resolveFromBuiltinSettings checks global builtin tool settings for provider/model config.
func (t *ReadDocumentTool) resolveFromBuiltinSettings(ctx context.Context) (providers.Provider, string, bool) {
	settings := BuiltinToolSettingsFromCtx(ctx)
	if settings == nil {
		return nil, "", false
	}
	raw, ok := settings["read_document"]
	if !ok || len(raw) == 0 {
		return nil, "", false
	}
	var cfg struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil || cfg.Provider == "" {
		return nil, "", false
	}
	p, err := t.registry.Get(cfg.Provider)
	if err != nil {
		return nil, "", false
	}
	model := cfg.Model
	if model == "" {
		model = p.DefaultModel()
	}
	return p, model, true
}

// resolveDocumentProvider finds the first available document-capable provider.
func (t *ReadDocumentTool) resolveDocumentProvider() (providers.Provider, string, error) {
	for _, name := range documentProviderPriority {
		p, err := t.registry.Get(name)
		if err != nil {
			continue
		}
		model := p.DefaultModel()
		if override, ok := documentModelOverrides[name]; ok {
			model = override
		}
		return p, model, nil
	}
	return nil, "", fmt.Errorf("no document-capable provider available (need one of: %v)", documentProviderPriority)
}

// callDocumentProvider sends a document to a provider for analysis.
// For Gemini providers, uses native generateContent API (supports PDF natively).
// For others, falls back to OpenAI-compat chat with base64 document.
func (t *ReadDocumentTool) callDocumentProvider(ctx context.Context, provider providers.Provider, model, prompt string, data []byte, mime string) (*providers.ChatResponse, string, string) {
	provName := provider.Name()

	// Gemini: use native API (OpenAI-compat endpoint doesn't support non-image MIME types).
	if strings.HasPrefix(provName, "gemini") {
		oaiProv, ok := provider.(*providers.OpenAIProvider)
		if !ok {
			slog.Warn("read_document: gemini provider is not OpenAIProvider", "provider", provName)
			return nil, "", ""
		}
		apiKey := oaiProv.APIKey()
		slog.Info("read_document: using gemini native API",
			"provider", provName, "model", model,
			"doc_size", len(data), "mime", mime)
		resp, err := geminiNativeDocumentCall(ctx, apiKey, model, prompt, data, mime)
		if err != nil {
			slog.Warn("read_document: gemini native call failed", "error", err)
			return nil, "", ""
		}
		return resp, provName, model
	}

	// Other providers: use standard Chat API with document as base64 image_url.
	slog.Info("read_document: using chat API", "provider", provName, "model", model, "doc_size", len(data))
	resp, err := provider.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{
				Role:    "user",
				Content: prompt,
				Images:  []providers.ImageContent{{MimeType: mime, Data: base64.StdEncoding.EncodeToString(data)}},
			},
		},
		Model: model,
		Options: map[string]interface{}{
			"max_tokens":  16384,
			"temperature": 0.2,
		},
	})
	if err != nil {
		slog.Warn("read_document: chat call failed", "provider", provName, "error", err)
		return nil, "", ""
	}
	return resp, provName, model
}

// resolveDocumentProviderByName gets a specific provider by name and applies model override.
func (t *ReadDocumentTool) resolveDocumentProviderByName(name string) (providers.Provider, string, error) {
	p, err := t.registry.Get(name)
	if err != nil {
		return nil, "", err
	}
	model := p.DefaultModel()
	if override, ok := documentModelOverrides[name]; ok {
		model = override
	}
	return p, model, nil
}

// mimeFromDocExt returns MIME type for document file extensions.
func mimeFromDocExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".pdf":
		return "application/pdf"
	case ".doc", ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls", ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".ppt", ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".csv":
		return "text/csv"
	default:
		return "application/octet-stream"
	}
}
