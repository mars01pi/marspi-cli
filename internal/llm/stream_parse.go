package llm

import (
	"encoding/json"
)

// ParseStreamData 解析单条 SSE data payload（JSON 或 [DONE]）。
func ParseStreamData(data string) (StreamChunk, error) {
	if data == "[DONE]" {
		return StreamChunk{Done: true}, nil
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return StreamChunk{}, err
	}
	return parseOpenAIStreamObject(raw), nil
}

func parseOpenAIStreamObject(raw map[string]any) StreamChunk {
	var chunk StreamChunk
	if model, _ := raw["model"].(string); model != "" {
		chunk.Model = model
	}
	if u, ok := raw["usage"].(map[string]any); ok {
		chunk.Usage = parseUsageMap(u)
		chunk.HasUsage = true
	}
	choices, _ := raw["choices"].([]any)
	if len(choices) == 0 {
		return chunk
	}
	c0, _ := choices[0].(map[string]any)
	if fr, _ := c0["finish_reason"].(string); fr != "" {
		chunk.FinishReason = fr
	}
	delta, _ := c0["delta"].(map[string]any)
	if delta == nil {
		return chunk
	}
	if v, _ := delta["content"].(string); v != "" {
		chunk.ContentDelta = v
	}
	if v, _ := delta["reasoning_content"].(string); v != "" {
		chunk.ReasoningDelta = v
	}
	if v, _ := delta["reasoning"].(string); v != "" && chunk.ReasoningDelta == "" {
		chunk.ReasoningDelta = v
	}
	chunk.ToolCallDeltas = parseToolCallDeltas(delta)
	return chunk
}

func parseToolCallDeltas(delta map[string]any) []ToolCallDelta {
	raw, _ := delta["tool_calls"].([]any)
	if len(raw) == 0 {
		return nil
	}
	out := make([]ToolCallDelta, 0, len(raw))
	for _, item := range raw {
		tc, ok := item.(map[string]any)
		if !ok {
			continue
		}
		d := ToolCallDelta{Index: asInt(tc["index"])}
		if id, _ := tc["id"].(string); id != "" {
			d.ID = id
		}
		if typ, _ := tc["type"].(string); typ != "" {
			d.Type = typ
		}
		if fn, ok := tc["function"].(map[string]any); ok {
			if name, _ := fn["name"].(string); name != "" {
				d.Name = name
			}
			if args, _ := fn["arguments"].(string); args != "" {
				d.Arguments = args
			}
		}
		out = append(out, d)
	}
	return out
}

func parseUsageMap(u map[string]any) Usage {
	return Usage{
		PromptTokens:     asInt(u["prompt_tokens"]),
		CompletionTokens: asInt(u["completion_tokens"]),
		TotalTokens:      asInt(u["total_tokens"]),
	}
}
