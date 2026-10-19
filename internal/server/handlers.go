package server

import (
	"encoding/json"
	"net/http"

	"github.com/yixinzhangagent/aide/internal/config"
	"github.com/yixinzhangagent/aide/internal/llm"
)

// handlers 持有处理请求所需的依赖。
type handlers struct {
	client llm.Client
	cfg    *config.Config
}

// health 用于存活探测。
func (h *handlers) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// chat 接收 OpenAI 风格的 ChatRequest，调用后端并返回结果。
// 若调用方未指定 model，则使用配置中的默认模型。
func (h *handlers) chat(w http.ResponseWriter, r *http.Request) {
	var req llm.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请求体解析失败: " + err.Error()})
		return
	}
	if len(req.Messages) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "messages 不能为空"})
		return
	}
	if req.Model == "" {
		req.Model = h.cfg.Model
	}
	if req.Temperature == 0 {
		req.Temperature = h.cfg.Temperature
	}

	resp, err := h.client.Chat(r.Context(), &req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "调用 LLM 失败: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// writeJSON 统一以 JSON 写出响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
