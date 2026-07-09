package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
)

var (
	reAnyXMLTag = regexp.MustCompile(`</?[\w:.-]+(?:\s[^>]*)?>`)
)

func ExtractContentModerationText(protocol string, body []byte) string {
	return ExtractContentModerationInput(protocol, body).Text
}

func ExtractContentModerationInput(protocol string, body []byte) ContentModerationInput {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ContentModerationInput{}
	}
	var parts []string
	var images []string
	switch protocol {
	case ContentModerationProtocolAnthropicMessages:
		collectLastAnthropicUserMessage(gjson.GetBytes(body, "messages"), &parts, &images)
	case ContentModerationProtocolOpenAIChat:
		collectLastRoleMessage(gjson.GetBytes(body, "messages"), "user", &parts, &images)
	case ContentModerationProtocolOpenAIResponses:
		collectLastResponsesInput(gjson.GetBytes(body, "input"), &parts, &images)
	case ContentModerationProtocolOpenAIEmbeddings:
		collectContentValue(gjson.GetBytes(body, "input"), &parts, &images)
	case ContentModerationProtocolGemini:
		collectLastGeminiContent(gjson.GetBytes(body, "contents"), &parts, &images)
	case ContentModerationProtocolOpenAIImages:
		addModerationText(&parts, gjson.GetBytes(body, "prompt").String())
		collectContentValue(gjson.GetBytes(body, "images"), &parts, &images)
	default:
		collectLastResponsesInput(gjson.GetBytes(body, "input"), &parts, &images)
		collectLastRoleMessage(gjson.GetBytes(body, "messages"), "user", &parts, &images)
		collectLastGeminiContent(gjson.GetBytes(body, "contents"), &parts, &images)
	}
	out := ContentModerationInput{
		Text:   normalizeContentModerationText(strings.Join(parts, "\n")),
		Images: normalizeModerationImages(images),
	}
	out.Normalize()
	return out
}

func ExtractContentModerationInputsForLocalBlock(protocol string, body []byte) []ContentModerationInput {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return nil
	}
	var out []ContentModerationInput
	switch protocol {
	case ContentModerationProtocolAnthropicMessages:
		collectAllAnthropicUserMessages(gjson.GetBytes(body, "messages"), &out)
	case ContentModerationProtocolOpenAIChat:
		collectAllRoleMessages(gjson.GetBytes(body, "messages"), "user", &out)
	case ContentModerationProtocolOpenAIResponses:
		collectAllResponsesInputs(gjson.GetBytes(body, "input"), &out)
	case ContentModerationProtocolOpenAIEmbeddings:
		collectAllEmbeddingInputs(gjson.GetBytes(body, "input"), &out)
	case ContentModerationProtocolGemini:
		collectAllGeminiUserContents(gjson.GetBytes(body, "contents"), &out)
	case ContentModerationProtocolOpenAIImages:
		var parts []string
		var images []string
		addModerationText(&parts, gjson.GetBytes(body, "prompt").String())
		collectContentValue(gjson.GetBytes(body, "images"), &parts, &images)
		appendLocalModerationInput(&out, parts, images)
	default:
		collectAllResponsesInputs(gjson.GetBytes(body, "input"), &out)
		collectAllRoleMessages(gjson.GetBytes(body, "messages"), "user", &out)
		collectAllGeminiUserContents(gjson.GetBytes(body, "contents"), &out)
	}
	return out
}

func collectAllEmbeddingInputs(input gjson.Result, out *[]ContentModerationInput) {
	switch {
	case !input.Exists():
		return
	case input.Type == gjson.String || input.IsObject():
		var parts []string
		var images []string
		collectContentValue(input, &parts, &images)
		appendLocalModerationInput(out, parts, images)
	case input.IsArray():
		input.ForEach(func(_, item gjson.Result) bool {
			var parts []string
			var images []string
			collectContentValue(item, &parts, &images)
			appendLocalModerationInput(out, parts, images)
			return true
		})
	}
}

func collectAllRoleMessages(messages gjson.Result, role string, out *[]ContentModerationInput) {
	if !messages.IsArray() {
		return
	}
	messages.ForEach(func(_, item gjson.Result) bool {
		if strings.ToLower(strings.TrimSpace(item.Get("role").String())) != role {
			return true
		}
		var parts []string
		var images []string
		collectContentValue(item.Get("content"), &parts, &images)
		appendLocalModerationInput(out, parts, images)
		return true
	})
}

func collectAllAnthropicUserMessages(messages gjson.Result, out *[]ContentModerationInput) {
	if !messages.IsArray() {
		return
	}
	messages.ForEach(func(_, item gjson.Result) bool {
		if strings.ToLower(strings.TrimSpace(item.Get("role").String())) != "user" {
			return true
		}
		var parts []string
		var images []string
		collectAnthropicUserContentValue(item.Get("content"), &parts, &images)
		appendLocalModerationInput(out, parts, images)
		return true
	})
}

func collectAllResponsesInputs(input gjson.Result, out *[]ContentModerationInput) {
	switch {
	case !input.Exists():
		return
	case input.Type == gjson.String:
		appendLocalModerationInput(out, []string{input.String()}, nil)
	case input.IsArray():
		input.ForEach(func(_, item gjson.Result) bool {
			if !isResponsesUserTextItem(item) {
				return true
			}
			var parts []string
			var images []string
			collectContentValue(item.Get("content"), &parts, &images)
			if item.Get("type").String() == "input_text" || item.Get("text").Exists() {
				collectContentValue(item, &parts, &images)
			}
			appendLocalModerationInput(out, parts, images)
			return true
		})
	case input.IsObject():
		if !isResponsesUserTextItem(input) {
			return
		}
		var parts []string
		var images []string
		collectContentValue(input.Get("content"), &parts, &images)
		if input.Get("type").String() == "input_text" || input.Get("text").Exists() {
			collectContentValue(input, &parts, &images)
		}
		appendLocalModerationInput(out, parts, images)
	}
}

func collectAllGeminiUserContents(contents gjson.Result, out *[]ContentModerationInput) {
	if !contents.IsArray() {
		return
	}
	contents.ForEach(func(_, item gjson.Result) bool {
		role := strings.ToLower(strings.TrimSpace(item.Get("role").String()))
		if role != "" && role != "user" {
			return true
		}
		var parts []string
		var images []string
		if arr := item.Get("parts"); arr.IsArray() {
			arr.ForEach(func(_, part gjson.Result) bool {
				addModerationText(&parts, part.Get("text").String())
				addGeminiModerationImage(&images, part)
				return true
			})
		}
		appendLocalModerationInput(out, parts, images)
		return true
	})
}

func appendLocalModerationInput(out *[]ContentModerationInput, parts []string, images []string) {
	input := ContentModerationInput{
		Text:   normalizeContentModerationText(strings.Join(parts, "\n")),
		Images: normalizeModerationImages(images),
	}
	input.Normalize()
	if input.IsEmpty() {
		return
	}
	*out = append(*out, input)
}

// capLocalModerationInputs 给本地拦截扫描的输入集合施加聚合上限，把每请求成本封成固定预算，
// 避免消息条数×每条文本随请求体无界放大(DoS)。保留最新的若干条(当前轮在尾部)，
// 先按条数截断，再按累计 rune 预算从尾部向前保留，两者取更严者。
func capLocalModerationInputs(inputs []ContentModerationInput, maxInputs, maxTotalRunes int) []ContentModerationInput {
	if len(inputs) == 0 {
		return inputs
	}
	if maxInputs > 0 && len(inputs) > maxInputs {
		inputs = inputs[len(inputs)-maxInputs:]
	}
	total := 0
	for i := len(inputs) - 1; i >= 0; i-- {
		total += len([]rune(inputs[i].KeywordScanText()))
		if total > maxTotalRunes {
			return inputs[i+1:]
		}
	}
	return inputs
}

func collectLastRoleMessage(messages gjson.Result, role string, parts *[]string, images *[]string) {
	if !messages.IsArray() {
		return
	}
	array := messages.Array()
	if len(array) == 0 {
		return
	}
	last := array[len(array)-1]
	if strings.ToLower(strings.TrimSpace(last.Get("role").String())) != role {
		return
	}
	var candidate []string
	var candidateImages []string
	collectContentValue(last.Get("content"), &candidate, &candidateImages)
	if normalizeContentModerationText(strings.Join(candidate, "\n")) == "" && len(candidateImages) == 0 {
		return
	}
	*parts = append(*parts, candidate...)
	*images = append(*images, candidateImages...)
}

func collectLastAnthropicUserMessage(messages gjson.Result, parts *[]string, images *[]string) {
	if !messages.IsArray() {
		return
	}
	array := messages.Array()
	if len(array) == 0 {
		return
	}
	last := array[len(array)-1]
	if strings.ToLower(strings.TrimSpace(last.Get("role").String())) != "user" {
		return
	}
	var candidate []string
	var candidateImages []string
	collectAnthropicUserContentValue(last.Get("content"), &candidate, &candidateImages)
	if normalizeContentModerationText(strings.Join(candidate, "\n")) == "" && len(candidateImages) == 0 {
		return
	}
	*parts = append(*parts, candidate...)
	*images = append(*images, candidateImages...)
}

func collectAnthropicUserContentValue(value gjson.Result, parts *[]string, images *[]string) {
	switch {
	case !value.Exists():
		return
	case value.Type == gjson.String:
		addModerationText(parts, value.String())
	case value.IsArray():
		value.ForEach(func(_, item gjson.Result) bool {
			collectAnthropicUserContentValue(item, parts, images)
			return true
		})
	case value.IsObject():
		typ := strings.ToLower(strings.TrimSpace(value.Get("type").String()))
		switch typ {
		case "", "text", "input_text", "message":
			if value.Get("text").Exists() {
				addModerationText(parts, value.Get("text").String())
			}
			if value.Get("content").Exists() {
				collectAnthropicUserContentValue(value.Get("content"), parts, images)
			}
		case "image_url", "input_image", "image":
			collectContentValue(value, parts, images)
		}
	}
}

func collectLastResponsesInput(input gjson.Result, parts *[]string, images *[]string) {
	switch {
	case !input.Exists():
		return
	case input.Type == gjson.String:
		addModerationText(parts, input.String())
	case input.IsArray():
		array := input.Array()
		if len(array) == 0 {
			return
		}
		last := array[len(array)-1]
		if !isResponsesUserTextItem(last) {
			return
		}
		collectContentValue(last.Get("content"), parts, images)
		if last.Get("type").String() == "input_text" || last.Get("text").Exists() {
			collectContentValue(last, parts, images)
		}
	case input.IsObject():
		if isResponsesUserTextItem(input) {
			collectContentValue(input.Get("content"), parts, images)
			if input.Get("type").String() == "input_text" || input.Get("text").Exists() {
				collectContentValue(input, parts, images)
			}
		}
	}
}

func isResponsesUserTextItem(item gjson.Result) bool {
	role := strings.ToLower(strings.TrimSpace(item.Get("role").String()))
	if role == "user" {
		return responseItemHasModerationText(item)
	}
	if role != "" {
		return false
	}
	return responseItemHasModerationText(item)
}

func responseItemHasModerationText(item gjson.Result) bool {
	var parts []string
	var images []string
	collectContentValue(item.Get("content"), &parts, &images)
	if item.Get("type").String() == "input_text" || item.Get("text").Exists() {
		collectContentValue(item, &parts, &images)
	}
	return normalizeContentModerationText(strings.Join(parts, "\n")) != "" || len(images) > 0
}

func collectLastGeminiContent(contents gjson.Result, parts *[]string, images *[]string) {
	if !contents.IsArray() {
		return
	}
	array := contents.Array()
	if len(array) == 0 {
		return
	}
	last := array[len(array)-1]
	role := strings.ToLower(strings.TrimSpace(last.Get("role").String()))
	if role != "" && role != "user" {
		return
	}
	var candidate []string
	var candidateImages []string
	if arr := last.Get("parts"); arr.IsArray() {
		arr.ForEach(func(_, part gjson.Result) bool {
			addModerationText(&candidate, part.Get("text").String())
			addGeminiModerationImage(&candidateImages, part)
			return true
		})
	}
	if normalizeContentModerationText(strings.Join(candidate, "\n")) == "" && len(candidateImages) == 0 {
		return
	}
	*parts = append(*parts, candidate...)
	*images = append(*images, candidateImages...)
}

func collectContentValue(value gjson.Result, parts *[]string, images *[]string) {
	switch {
	case !value.Exists():
		return
	case value.Type == gjson.String:
		addModerationText(parts, value.String())
	case value.IsArray():
		value.ForEach(func(_, item gjson.Result) bool {
			collectContentValue(item, parts, images)
			return true
		})
	case value.IsObject():
		typ := strings.ToLower(strings.TrimSpace(value.Get("type").String()))
		addModerationImage(images, value.Get("image_url.url").String())
		addModerationImage(images, value.Get("image_url").String())
		addModerationImage(images, value.Get("url").String())
		addModerationImageData(images, value.Get("source.media_type").String(), value.Get("source.data").String())
		addModerationImageData(images, value.Get("source.mediaType").String(), value.Get("source.data").String())
		addModerationImageData(images, value.Get("media_type").String(), value.Get("data").String())
		addModerationImageData(images, value.Get("mime_type").String(), value.Get("data").String())
		addModerationImageData(images, value.Get("mimeType").String(), value.Get("data").String())
		addModerationImage(images, value.Get("source.data").String())
		addModerationImage(images, value.Get("data").String())
		addModerationImage(images, value.Get("base64").String())
		switch typ {
		case "", "text", "input_text", "message":
			if value.Get("text").Exists() {
				addModerationText(parts, value.Get("text").String())
			}
			if value.Get("content").Exists() {
				collectContentValue(value.Get("content"), parts, images)
			}
		case "image_url", "input_image", "image":
		}
	}
}

func addGeminiModerationImage(images *[]string, part gjson.Result) {
	if inlineData := part.Get("inline_data"); inlineData.IsObject() {
		mimeType := strings.TrimSpace(inlineData.Get("mime_type").String())
		data := strings.TrimSpace(inlineData.Get("data").String())
		if mimeType != "" && data != "" {
			addModerationImage(images, fmt.Sprintf("data:%s;base64,%s", mimeType, data))
		}
	}
	if inlineData := part.Get("inlineData"); inlineData.IsObject() {
		mimeType := strings.TrimSpace(inlineData.Get("mimeType").String())
		data := strings.TrimSpace(inlineData.Get("data").String())
		if mimeType != "" && data != "" {
			addModerationImage(images, fmt.Sprintf("data:%s;base64,%s", mimeType, data))
		}
	}
	addModerationImage(images, part.Get("file_data.file_uri").String())
	addModerationImage(images, part.Get("fileData.fileUri").String())
}

func addModerationImageData(images *[]string, mimeType string, data string) {
	mimeType = strings.TrimSpace(mimeType)
	data = strings.TrimSpace(data)
	if mimeType == "" || data == "" {
		return
	}
	addModerationImage(images, fmt.Sprintf("data:%s;base64,%s", mimeType, data))
}

func addModerationImage(images *[]string, image string) {
	image = strings.TrimSpace(image)
	if image == "" {
		return
	}
	if strings.HasPrefix(image, "data:") || strings.HasPrefix(image, "http://") || strings.HasPrefix(image, "https://") {
		*images = append(*images, image)
	}
}

func normalizeModerationImages(images []string) []string {
	out := make([]string, 0, len(images))
	seen := make(map[string]struct{}, len(images))
	for _, image := range images {
		image = strings.TrimSpace(image)
		if image == "" {
			continue
		}
		if _, ok := seen[image]; ok {
			continue
		}
		seen[image] = struct{}{}
		out = append(out, image)
	}
	return out
}

func limitContentModerationImages(images []string) []string {
	if len(images) <= maxContentModerationInputImages {
		return images
	}
	return images[:maxContentModerationInputImages]
}

func addModerationText(parts *[]string, text string) {
	text = stripAnthropicSystemReminderText(text)
	if text == "" {
		return
	}
	*parts = append(*parts, text)
}

func stripAnthropicSystemReminderText(text string) string {
	return stripXMLTagsForModeration(text)
}

func stripXMLTagsForModeration(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = reAnyXMLTag.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func normalizeContentModerationText(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

// normalizeContentModerationHashText 统一 hash/pre-check 与本地关键词匹配的 Unicode 基线，
// 避免全角、零宽或同形字变体绕过已记录的输入哈希；不改动送审 API 使用的原始 Text。
func normalizeContentModerationHashText(text string) string {
	return strings.Join(strings.Fields(normalizeForKeywordMatch(text)), " ")
}
