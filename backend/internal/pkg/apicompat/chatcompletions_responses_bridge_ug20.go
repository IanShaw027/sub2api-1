package apicompat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const customToolInputSchema = `{"type":"object","properties":{"input":{"type":"string","description":"The raw input for this tool, passed through verbatim."}},"required":["input"]}`

const toolSearchProxyName = "tool_search"

const toolSearchProxySchema = `{"type":"object","properties":{"query":{"type":"string","description":"Search query for tools or connectors to load."},"limit":{"type":"integer","description":"Maximum number of tool groups to return."}},"required":["query"]}`

const chatToolNameMaxLen = 64

type NamespacedToolName struct {
	Namespace string
	Name      string
}

func CustomToolNames(tools []ResponsesTool) map[string]bool {
	var out map[string]bool
	for _, tool := range tools {
		if tool.Type == "custom" && tool.Name != "" {
			if out == nil {
				out = make(map[string]bool)
			}
			out[tool.Name] = true
		}
	}
	return out
}

func NamespaceToolNames(tools []ResponsesTool) map[string]NamespacedToolName {
	out := make(map[string]NamespacedToolName)
	for _, tool := range tools {
		collectNamespaceToolNames(tool, "", out)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func collectNamespaceToolNames(tool ResponsesTool, parentNamespace string, out map[string]NamespacedToolName) {
	if !strings.EqualFold(strings.TrimSpace(tool.Type), "namespace") {
		return
	}
	namespace := joinNamespace(parentNamespace, tool.Name)
	children := tool.Tools
	if len(children) == 0 {
		children = tool.Children
	}
	for _, child := range children {
		if strings.EqualFold(strings.TrimSpace(child.Type), "namespace") {
			collectNamespaceToolNames(child, namespace, out)
			continue
		}
		if child.Type != "function" || strings.TrimSpace(child.Name) == "" {
			continue
		}
		flat := responseFunctionChatName(child.Name, namespace)
		if flat != "" {
			out[flat] = NamespacedToolName{Namespace: namespace, Name: strings.TrimSpace(child.Name)}
		}
	}
}

func HasToolSearchTool(tools []ResponsesTool) bool {
	for _, tool := range tools {
		if tool.Type == "tool_search" {
			return true
		}
	}
	return false
}

func toolSearchProxyChatTool() ChatTool {
	return ChatTool{Type: "function", Function: &ChatFunction{
		Name:        toolSearchProxyName,
		Description: "Search and load Codex tools, plugins, connectors, and MCP namespaces for the current task.",
		Parameters:  json.RawMessage(toolSearchProxySchema),
	}}
}

func namespaceChildrenToChatTools(tool ResponsesTool, topLevel map[string]bool, flatOwner map[string]NamespacedToolName) ([]ChatTool, error) {
	return collectNamespaceChildrenToChatTools(tool, "", topLevel, flatOwner)
}

func collectNamespaceChildrenToChatTools(tool ResponsesTool, parentNamespace string, topLevel map[string]bool, flatOwner map[string]NamespacedToolName) ([]ChatTool, error) {
	if !strings.EqualFold(strings.TrimSpace(tool.Type), "namespace") {
		return nil, nil
	}
	namespace := joinNamespace(parentNamespace, tool.Name)
	if namespace == "" {
		return nil, nil
	}
	children := tool.Tools
	if len(children) == 0 {
		children = tool.Children
	}
	var out []ChatTool
	for _, child := range children {
		if strings.EqualFold(strings.TrimSpace(child.Type), "namespace") {
			nested, err := collectNamespaceChildrenToChatTools(child, namespace, topLevel, flatOwner)
			if err != nil {
				return nil, err
			}
			out = append(out, nested...)
			continue
		}
		if child.Type != "function" || child.Name == "" {
			continue
		}
		flat := responseFunctionChatName(child.Name, namespace)
		entry := NamespacedToolName{Namespace: namespace, Name: strings.TrimSpace(child.Name)}
		if topLevel[flat] {
			return nil, fmt.Errorf("namespace tool %q/%q conflicts with top-level tool %q", namespace, child.Name, flat)
		}
		if previous, ok := flatOwner[flat]; ok {
			if previous == entry {
				continue
			}
			return nil, fmt.Errorf("namespace tools %q/%q and %q/%q both flatten to %q", previous.Namespace, previous.Name, namespace, child.Name, flat)
		}
		flatOwner[flat] = entry
		out = append(out, ChatTool{Type: "function", Function: &ChatFunction{
			Name: flat, Description: child.Description, Parameters: child.Parameters, Strict: child.Strict,
		}})
	}
	return out, nil
}

func joinNamespace(parent, child string) string {
	parent = strings.TrimSpace(parent)
	child = strings.TrimSpace(child)
	if parent == "" {
		return child
	}
	if child == "" {
		return parent
	}
	return parent + "." + child
}

func flattenNamespaceToolName(namespace, name string) string {
	full := namespace + "__" + name
	if len(full) <= chatToolNameMaxLen {
		return full
	}
	sum := sha256.Sum256([]byte(full))
	suffix := "__" + hex.EncodeToString(sum[:4])
	prefixLen := chatToolNameMaxLen - len(suffix)
	var prefix strings.Builder
	for _, char := range full {
		if prefix.Len()+len(string(char)) > prefixLen {
			break
		}
		_, _ = prefix.WriteRune(char)
	}
	return prefix.String() + suffix
}

func extractCustomToolCallInput(arguments string) string {
	trimmed := strings.TrimSpace(arguments)
	if trimmed == "" {
		return ""
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &object); err != nil {
		return trimmed
	}
	if raw, ok := object["input"]; ok {
		var input string
		if err := json.Unmarshal(raw, &input); err == nil {
			return input
		}
		return trimmed
	}
	if len(object) == 0 {
		return ""
	}
	return trimmed
}

func toolSearchCallArgumentsJSON(arguments string) json.RawMessage {
	trimmed := strings.TrimSpace(arguments)
	if trimmed == "" {
		return json.RawMessage(`{}`)
	}
	if json.Valid([]byte(trimmed)) {
		return json.RawMessage(trimmed)
	}
	fallback, _ := json.Marshal(arguments)
	return fallback
}
