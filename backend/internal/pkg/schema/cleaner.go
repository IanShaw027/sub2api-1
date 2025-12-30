package schema

import "strings"

// CleanJSONSchema 清理 JSON Schema，移除不支持的字段
// 参考 proxycast 的实现，确保 schema 符合 JSON Schema draft 2020-12
func CleanJSONSchema(schema map[string]any) map[string]any {
	return cleanJSONSchema(schema)
}

// cleanJSONSchema 清理 JSON Schema，移除 Antigravity/Gemini 不支持的字段
// 参考 proxycast 的实现，确保 schema 符合 JSON Schema draft 2020-12
func cleanJSONSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}
	cleaned := cleanSchemaValue(schema)
	result, ok := cleaned.(map[string]any)
	if !ok {
		return nil
	}

	// 确保有 type 字段（默认 OBJECT）
	if _, hasType := result["type"]; !hasType {
		result["type"] = "OBJECT"
	}

	// 确保有 properties 字段（默认空对象）
	if _, hasProps := result["properties"]; !hasProps {
		result["properties"] = make(map[string]any)
	}

	// 验证 required 中的字段都存在于 properties 中
	if required, ok := result["required"].([]any); ok {
		if props, ok := result["properties"].(map[string]any); ok {
			validRequired := make([]any, 0, len(required))
			for _, r := range required {
				if reqName, ok := r.(string); ok {
					if _, exists := props[reqName]; exists {
						validRequired = append(validRequired, r)
					}
				}
			}
			if len(validRequired) > 0 {
				result["required"] = validRequired
			} else {
				delete(result, "required")
			}
		}
	}

	return result
}

// excludedSchemaKeys 不支持的 schema 字段
var excludedSchemaKeys = map[string]bool{
	"$schema":              true,
	"$id":                  true,
	"$ref":                 true,
	"additionalProperties": true,
	"minLength":            true,
	"maxLength":            true,
	"minItems":             true,
	"maxItems":             true,
	"uniqueItems":          true,
	"minimum":              true,
	"maximum":              true,
	"exclusiveMinimum":     true,
	"exclusiveMaximum":     true,
	"pattern":              true,
	"format":               true,
	"default":              true,
	"strict":               true,
	"const":                true,
	"examples":             true,
	"deprecated":           true,
	"readOnly":             true,
	"writeOnly":            true,
	"contentMediaType":     true,
	"contentEncoding":      true,
}

// cleanSchemaValue 递归清理 schema 值
func cleanSchemaValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		result := make(map[string]any)
		for k, val := range v {
			// 跳过不支持的字段
			if excludedSchemaKeys[k] {
				continue
			}

			// 特殊处理 type 字段
			if k == "type" {
				result[k] = cleanTypeValue(val)
				continue
			}

			// 递归清理所有值
			result[k] = cleanSchemaValue(val)
		}
		return result

	case []any:
		// 递归处理数组中的每个元素
		cleaned := make([]any, 0, len(v))
		for _, item := range v {
			cleaned = append(cleaned, cleanSchemaValue(item))
		}
		return cleaned

	default:
		return value
	}
}

// cleanTypeValue 处理 type 字段，转换为大写
func cleanTypeValue(value any) any {
	switch v := value.(type) {
	case string:
		return strings.ToUpper(v)
	case []any:
		// 联合类型 ["string", "null"] -> 取第一个非 null 类型
		for _, t := range v {
			if ts, ok := t.(string); ok && ts != "null" {
				return strings.ToUpper(ts)
			}
		}
		// 如果只有 null，返回 STRING
		return "STRING"
	default:
		return value
	}
}
