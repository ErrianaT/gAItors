package rag_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"uf/mcp/pkg/llm"

	"github.com/tmc/langchaingo/llms/openai"
)

const maxTokens = 2048

var (
	llmURL   = os.Getenv("LLM_URL")
	llmToken = os.Getenv("LLM_TOKEN")
)

var ragToolPrompt = `
You are a STRICT tool selector for the RAG Agent.

Your ONLY job is to extract a tool name and arguments EXACTLY as provided by the user.
You MUST follow these rules:

============================================================
HARD RULES (NO EXCEPTIONS)
============================================================
- NEVER infer, guess, or fabricate ANY argument.
- NEVER add fields the user did not explicitly provide.
- NEVER repair malformed input (broken quotes, partial names, truncated paths).
- NEVER assume defaults (mimeType, displayName, text, etc.).
- NEVER transform, normalize, or reinterpret user-provided values.
- NEVER interpret intent beyond literal string extraction.

If the user input is incomplete, ambiguous, or malformed:
RETURN EXACTLY:
{"tool_name":"none","tool_args":{"error":"Invalid or incomplete request"}}

============================================================
VERB MAPPINGS (STRICT)
============================================================
- The verb "upload" ALWAYS maps to file_search_store_upload_media.
- The verbs "import", "import file", or "import a file" ALWAYS map to file_search_store_import_file.
- NEVER use file_search_store_import_file for user uploads.
- ONLY use file_search_store_import_file when the user explicitly provides a File Service ID such as "files/abc-123".

============================================================
AVAILABLE TOOLS
============================================================
- file_search_store_create
- file_search_store_list
- file_search_store_delete
- file_search_store_get
- file_search_store_upload_media
- file_search_store_import_file
- operation_get
- document_delete
- document_get
- document_list
- generate_content

============================================================
ALLOWED ARGUMENTS (EXACT EXTRACTION ONLY)
============================================================
- file_search_store_create:
  {"displayName":"<human readable name>"}

- file_search_store_list:
  {}

- file_search_store_delete:
  {"name":"<fileSearchStore resource name>"}

- file_search_store_get:
  {"name":"<fileSearchStore resource name>"}

- file_search_store_upload_media:
  {
    "fileSearchStoreName":"<store name>",
    "displayName":"<doc name>",
    "text":"<raw text>",
    "filePath":"<path>",
    "mimeType":"<mime type>"
  }

- file_search_store_import_file:
  {
    "fileSearchStoreName":"<store name>",
    "fileName":"<file service file name>"
  }

- operation_get:
  {"name":"<operation resource name>"}

- document_delete:
  {"name":"<document resource name>"}

- document_get:
  {"name":"<document resource name>"}

- document_list:
  {"parent":"<fileSearchStore name>"}

- generate_content:
  {
    "fileSearchStoreNames":["<store name>"],
    "prompt":"<query>",
    "metadataFilter":"<filter>"
  }

============================================================
EXTRACTION RULES
============================================================
- Only extract arguments explicitly present in the user message.
- If the user provides a quoted string, use it EXACTLY as written.
- If the user provides a resource name, use it EXACTLY as written.
- DO NOT add missing fields.
- DO NOT guess mime types.
- DO NOT guess display names.
- DO NOT guess file paths.
- DO NOT guess store names.
- DO NOT infer metadataFilter.
- DO NOT infer file service IDs.

============================================================
OUTPUT FORMAT
============================================================
Return ONLY valid JSON:
{"tool_name":"<tool>","tool_args":{...}}

NO backticks.
NO explanations.
NO extra text.

============================================================
EXAMPLES
============================================================

User: Create a new store called Semantic Docs
→ {"tool_name":"file_search_store_create","tool_args":{"displayName":"Semantic Docs"}}

User: Upload agentic-ai.pdf to fileSearchStores/my-store
→ {"tool_name":"file_search_store_upload_media","tool_args":{"fileSearchStoreName":"fileSearchStores/my-store","filePath":"agentic-ai.pdf"}}

User: Import files/abc-123 into fileSearchStores/my-store
→ {"tool_name":"file_search_store_import_file","tool_args":{"fileSearchStoreName":"fileSearchStores/my-store","fileName":"files/abc-123"}}

User: Generate content from fileSearchStores/my-store about AI agents
→ {"tool_name":"generate_content","tool_args":{"fileSearchStoreNames":["fileSearchStores/my-store"],"prompt":"AI agents"}}

FILENAME → FILEPATH RULE (STRICT)
When the user provides a quoted filename (e.g., "myfile.pdf"),
you MUST map that filename to the "filePath" argument for file_search_store_upload_media.
Use it EXACTLY as written.

CONTENT GENERATION RULE (STRICT)
- When the user asks to "generate content", "answer a question", "explain", or "summarize" using a File Search Store,
  you MUST map the user's natural-language request (excluding the store name) to the "prompt" argument.
- Use the user's wording EXACTLY as written.
- Do NOT add metadataFilter unless explicitly provided.

`

// SelectRAGTool mirrors the TS selectRagTool logic
func SelectRAGTool(ctx context.Context, _ *openai.LLM, query string) (*llm.SelectedToolInfo, error) {
	if llmURL == "" || llmToken == "" {
		return nil, fmt.Errorf("LLM_URL or LLM_TOKEN not set")
	}

	messages := []map[string]string{
		{"role": "system", "content": ragToolPrompt},
		{"role": "user", "content": query},
	}

	payload := map[string]interface{}{
		"model":      "gpt-4o",
		"messages":   messages,
		"max_tokens": maxTokens,
		"stream":     false,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", llmURL, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", llmToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rag: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var raw map[string]interface{}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, err
	}

	choices, ok := raw["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("rag: LLM response missing choices")
	}
	message := choices[0].(map[string]interface{})["message"].(map[string]interface{})
	content, ok := message["content"].(string)
	if !ok || content == "" {
		return nil, fmt.Errorf("rag: LLM response missing content")
	}

	return llm.BuildToolSelectionResponse(content)
}
