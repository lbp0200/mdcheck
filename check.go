package main

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	tableSeparatorRegex   = regexp.MustCompile(`\|\s*:?-{3,}:?\s*\|`)
	linkRegex             = regexp.MustCompile(`\[(.+?)\]\(([^)\s]+)(?:\s+["'](.+?)["'])?\)`)
	emptyHeaderRegex      = regexp.MustCompile(`^#{1,6}\s*$`)
	headerWithTextRegex   = regexp.MustCompile(`^#{1,6}\s+(.+)$`)
	imageAltRegex         = regexp.MustCompile(`!\[(.*?)\]\(([^)]+)\)`)
	referenceLinkRegex    = regexp.MustCompile(`\[([^\]]+)\]\[([^\]]*)\]`)
	referenceDefRegex     = regexp.MustCompile(`(?m)^\[([^\]]+)\]:\s+(\S+)`)
	bareURLRegex          = regexp.MustCompile(`(https?://[^\s\)\]>]+)`)
	deepHeaderRegex       = regexp.MustCompile(`^#{7,}\s+\S+`)
	refImageAltRegex      = regexp.MustCompile(`!\[([^\]]*)\]\[([^\]]*)\]`)
)

func fenceDelimiter(line string) string {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "```") {
		return "```"
	}
	if strings.HasPrefix(trimmed, "~~~") {
		return "~~~"
	}
	return ""
}

var knownLanguages = map[string]bool{
	"": true,
	"go": true, "golang": true, "shell": true, "bash": true, "sh": true,
	"python": true, "py": true, "javascript": true, "js": true, "ts": true, "typescript": true,
	"html": true, "htm": true, "css": true, "scss": true, "sass": true, "less": true,
	"java": true, "c": true, "cpp": true, "c++": true, "cs": true,
	"rust": true, "rs": true, "ruby": true, "rb": true, "php": true,
	"swift": true, "kt": true, "kotlin": true, "scala": true,
	"sql": true, "yaml": true, "yml": true, "json": true, "xml": true,
	"markdown": true, "md": true, "tex": true, "latex": true,
	"dockerfile": true, "docker": true, "makefile": true, "make": true,
	"perl": true, "pl": true, "r": true, "racket": true, "lisp": true,
	"vue": true, "svelte": true, "jsx": true, "tsx": true,
	"zig": true, "dart": true, "lua": true, "haskell": true, "hs": true,
	"elixir": true, "ex": true, "exs": true, "clojure": true, "clj": true,
	"graphql": true, "gql": true, "proto": true, "protobuf": true,
	"diff": true, "patch": true, "toml": true, "ini": true, "cfg": true,
	"nginx": true, "apache": true, "http": true, "env": true,
	"csv": true, "svg": true, "mermaid": true, "text": true, "txt": true,
}

func buildLineSkip(content string) func(int) bool {
	lines := strings.Split(content, "\n")
	inFence := make([]bool, len(lines))
	inside := false
	for i, line := range lines {
		if fenceDelimiter(line) != "" {
			inside = !inside
		}
		inFence[i] = inside
	}
	return func(lineNum int) bool {
		if lineNum < 1 || lineNum > len(inFence) {
			return false
		}
		return inFence[lineNum-1]
	}
}

func byteOffsetToLineCol(content string, offset int) (line, col int) {
	if offset <= 0 {
		return 1, 1
	}
	line = 1 + strings.Count(content[:offset], "\n")
	lastNewline := strings.LastIndex(content[:offset], "\n")
	col = offset - lastNewline
	return
}

func checkFencedCodeBlocks(path, content string) []Issue {
	var issues []Issue
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	inFence := false
	var delim string

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if d := fenceDelimiter(line); d != "" {
			if !inFence {
				inFence = true
				delim = d
			} else if d == delim {
				inFence = false
			}
		} else if inFence && strings.Contains(line, delim) {
			issues = append(issues, Issue{
				File:       path,
				Line:       lineNum,
				Column:     strings.Index(line, delim) + 1,
				Message:    fmt.Sprintf("代码块中可能包含意外的 %s 标记", delim),
				Suggestion: "检查是否意外在代码内容中包含了结束标记",
			})
		}
	}

	if inFence {
		issues = append(issues, Issue{
			File:       path,
			Line:       lineNum,
			Column:     1,
			Message:    fmt.Sprintf("未闭合的代码块，未找到对应的结束标记 %s", delim),
			Suggestion: fmt.Sprintf("添加 %s 结束标记，或检查是否意外在代码内容中包含了结束标记", delim),
		})
	}
	return issues
}

func checkHeaders(path, content string, skip func(int) bool) []Issue {
	var issues []Issue
	lines := strings.Split(content, "\n")
	headerMap := make(map[string]int)

	for i, line := range lines {
		if skip != nil && skip(i+1) {
			continue
		}
		if emptyHeaderRegex.MatchString(line) {
			issues = append(issues, Issue{
				File:       path,
				Line:       i + 1,
				Column:     1,
				Message:    "空标题：标题后没有内容",
				Suggestion: "在标题后添加内容，或删除空标题",
			})
		}

		matches := headerWithTextRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			text := strings.TrimSpace(matches[1])
			text = strings.TrimPrefix(text, "*")

			if _, exists := headerMap[text]; exists {
				issues = append(issues, Issue{
					File:       path,
					Line:       i + 1,
					Column:     1,
					Message:    fmt.Sprintf("重复的标题：'%s'", text),
					Suggestion: "使用更具体的标题名称，或修改其中一个标题",
				})
			}
			headerMap[text] = i + 1
		}
	}
	return issues
}

func checkLinks(path, content string, skip func(int) bool) []Issue {
	var issues []Issue
	matches := linkRegex.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) < 6 || match[4] < 0 {
			continue
		}

		if match[0] > 0 && content[match[0]-1] == '!' {
			continue
		}

		if skip != nil {
			line, _ := byteOffsetToLineCol(content, match[0])
			if skip(line) {
				continue
			}
		}

		url := content[match[4]:match[5]]

		if !isValidURL(url) {
			line, col := byteOffsetToLineCol(content, match[4])
			issues = append(issues, Issue{
				File:       path,
				Line:       line,
				Column:     col,
				Message:    fmt.Sprintf("可能无效的链接：%s", url),
				Suggestion: "检查链接是否可访问，或使用相对路径",
			})
		}
	}
	return issues
}

func isValidURL(url string) bool {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return len(url) > 10
	}

	if strings.HasPrefix(url, "/") || strings.HasPrefix(url, "./") || strings.HasPrefix(url, "../") {
		return true
	}

	if strings.HasPrefix(url, "#") {
		return len(url) > 1
	}

	// mailto:, tel:, ftp:, sftp://, file:// 等合法 URI scheme
	if idx := strings.Index(url, ":"); idx > 0 {
		scheme := url[:idx]
		return len(scheme) > 0 && isAlphaScheme(scheme) && len(url) > idx+1
	}

	return false
}

func isAlphaScheme(s string) bool {
	for _, ch := range s {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')) {
			return false
		}
	}
	return len(s) > 0
}

func checkTables(path, content string, skip func(int) bool) []Issue {
	var issues []Issue
	lines := strings.Split(content, "\n")

	for i := 0; i < len(lines); i++ {
		if skip != nil && skip(i+1) {
			continue
		}
		if !isTableSeparatorLine(lines[i]) {
			continue
		}

		refColumns := countColumns(lines[i])
		row := i + 1
		for row < len(lines) && isTableDataLine(lines[row]) && !(skip != nil && skip(row+1)) {
			if countColumns(lines[row]) != refColumns {
				issues = append(issues, Issue{
					File:       path,
					Line:       row + 1,
					Column:     1,
					Message:    "表格行未正确对齐",
					Suggestion: "确保表格列使用 | 分隔，并且列数一致",
				})
			}
			row++
		}

		i = row
	}
	return issues
}

func isTableSeparatorLine(line string) bool {
	return tableSeparatorRegex.MatchString(line)
}

func isTableDataLine(line string) bool {
	return strings.Contains(line, "|") && !isTableSeparatorLine(line)
}

func countColumns(line string) int {
	trimmed := strings.Trim(line, "|")
	if trimmed == "" {
		return 0
	}
	return strings.Count(trimmed, "|") + 1
}

func checkLists(path, content string, skip func(int) bool) []Issue {
	var issues []Issue
	lines := strings.Split(content, "\n")

	inList := false
	expectedIndent := 0

	for i, line := range lines {
		if skip != nil && skip(i+1) {
			continue
		}
		trimmed := strings.TrimSpace(line)

		isListStart := strings.HasPrefix(trimmed, "- ") ||
			strings.HasPrefix(trimmed, "* ") ||
			strings.HasPrefix(trimmed, "1. ")

		if isListStart {
			if inList {
				currentIndent := getIndentation(line)
				if currentIndent != expectedIndent {
					issues = append(issues, Issue{
						File:       path,
						Line:       i + 1,
						Column:     1,
						Message:    fmt.Sprintf("列表缩进不一致：期望 %d 个空格，实际 %d 个", expectedIndent, currentIndent),
						Suggestion: "保持列表项缩进一致",
					})
				}
			}
			inList = true
			expectedIndent = getIndentation(line)
		} else if inList {
			inList = false
		}
	}
	return issues
}

func getIndentation(line string) int {
	for i, ch := range line {
		if ch != ' ' && ch != '\t' {
			return i
		}
	}
	return len(line)
}

func checkHeadingLevels(path, content string, skip func(int) bool) []Issue {
	var issues []Issue
	lines := strings.Split(content, "\n")
	lastLevel := 0

	for i, line := range lines {
		if skip != nil && skip(i+1) {
			continue
		}
		if !headerWithTextRegex.MatchString(line) {
			continue
		}
		level := 0
		for _, ch := range line {
			if ch == '#' {
				level++
			} else {
				break
			}
		}
		if lastLevel > 0 && level-lastLevel > 1 {
			issues = append(issues, Issue{
				File:       path,
				Line:       i + 1,
				Column:     1,
				Message:    fmt.Sprintf("标题级别跳跃：从 H%d 到 H%d，跳过了 H%d", lastLevel, level, lastLevel+1),
				Suggestion: fmt.Sprintf("考虑添加 H%d 标题，或调整标题级别", lastLevel+1),
			})
		}
		lastLevel = level
	}
	return issues
}

func checkDeepHeaders(path, content string, skip func(int) bool) []Issue {
	var issues []Issue
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if skip != nil && skip(i+1) {
			continue
		}
		if deepHeaderRegex.MatchString(line) {
			issues = append(issues, Issue{
				File:       path,
				Line:       i + 1,
				Column:     1,
				Message:    "标题层级过深（超过 H6）",
				Suggestion: "Markdown 最多支持 6 级标题，将标题层级降低到 H1-H6 范围内",
			})
		}
	}
	return issues
}

func checkCodeBlockLanguage(path, content string) []Issue {
	var issues []Issue
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		d := fenceDelimiter(line)
		if d == "" {
			continue
		}
		trimmed := strings.TrimLeft(line, " \t")
		if len(trimmed) <= len(d) {
			continue
		}
		l := strings.TrimSpace(trimmed[len(d):])
		if l != "" {
			language := strings.Split(l, " ")[0]

			if !isKnownLanguage(language) {
				issues = append(issues, Issue{
					File:       path,
					Line:       i + 1,
					Column:     1,
					Message:    fmt.Sprintf("未知的代码块语言：%s", language),
					Suggestion: "使用已知的语言标识，如 go, python, javascript, html 等",
				})
			}
		}
	}
	return issues
}

func isKnownLanguage(lang string) bool {
	return knownLanguages[strings.ToLower(lang)]
}

func checkLongLines(path, content string, skip func(int) bool, maxLen int) []Issue {
	var issues []Issue
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		if skip != nil && skip(i+1) {
			continue
		}
		if len(line) > maxLen {
			issues = append(issues, Issue{
				File:       path,
				Line:       i + 1,
				Column:     maxLen + 1,
				Message:    fmt.Sprintf("行过长（%d 字符，超过 %d 字符限制）", len(line), maxLen),
				Suggestion: "考虑换行或拆分内容",
			})
		}
	}
	return issues
}

func checkBlankLines(path, content string, skip func(int) bool) []Issue {
	var issues []Issue
	lines := strings.Split(content, "\n")
	// 去掉末尾空行伪影（strings.Split 在文件以 \n 结尾时产生）
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	consecutive := 0

	for i, line := range lines {
		if skip != nil && skip(i+1) {
			consecutive = 0
			continue
		}
		if strings.TrimSpace(line) == "" {
			consecutive++
			if consecutive > 1 {
				issues = append(issues, Issue{
					File:       path,
					Line:       i + 1,
					Column:     1,
					Message:    "连续空行",
					Suggestion: "最多保留一个空行，删除多余的空行",
				})
			}
		} else {
			consecutive = 0
		}
	}
	return issues
}

func checkListMarkers(path, content string, skip func(int) bool) []Issue {
	var issues []Issue
	lines := strings.Split(content, "\n")
	inList := false
	var marker string

	for i, line := range lines {
		if skip != nil && skip(i+1) {
			continue
		}
		trimmed := strings.TrimSpace(line)

		var currentMarker string
		isList := false
		if strings.HasPrefix(trimmed, "- ") {
			currentMarker = "-"
			isList = true
		} else if strings.HasPrefix(trimmed, "* ") {
			currentMarker = "*"
			isList = true
		}

		if isList {
			if inList && currentMarker != marker {
				issues = append(issues, Issue{
					File:       path,
					Line:       i + 1,
					Column:     1,
					Message:    fmt.Sprintf("混用列表标记：使用了 '%s'，但之前用的是 '%s'", currentMarker, marker),
					Suggestion: "统一使用 '-' 或 '*' 作为无序列表标记",
				})
			}
			inList = true
			marker = currentMarker
		} else {
			inList = false
		}
	}
	return issues
}

func checkImageAlt(path, content string, skip func(int) bool) []Issue {
	var issues []Issue
	matches := imageAltRegex.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) < 4 || match[2] != match[3] {
			continue
		}
		if skip != nil {
			line, _ := byteOffsetToLineCol(content, match[0])
			if skip(line) {
				continue
			}
		}
		line, col := byteOffsetToLineCol(content, match[0])
		issues = append(issues, Issue{
			File:       path,
			Line:       line,
			Column:     col,
			Message:    "图片缺少替代文本（alt text）",
			Suggestion: "为图片添加描述性替代文本，如 ![描述](url)",
		})
	}
	return issues
}

func checkRefImageAlt(path, content string, skip func(int) bool) []Issue {
	var issues []Issue
	for _, match := range refImageAltRegex.FindAllStringSubmatchIndex(content, -1) {
		if len(match) < 6 || match[4] < 0 {
			continue
		}
		if skip != nil {
			line, _ := byteOffsetToLineCol(content, match[0])
			if skip(line) {
				continue
			}
		}
		if match[2] == match[3] {
			line, col := byteOffsetToLineCol(content, match[0])
			issues = append(issues, Issue{
				File:       path,
				Line:       line,
				Column:     col,
				Message:    "引用式图片缺少替代文本（alt text）",
				Suggestion: "为图片添加描述性替代文本，如 ![描述][ref]",
			})
		}
	}
	return issues
}

func checkTrailingWhitespace(path, content string, skip func(int) bool) []Issue {
	var issues []Issue
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		if skip != nil && skip(i+1) {
			continue
		}
		trimmed := strings.TrimRightFunc(line, unicode.IsSpace)
		if len(line) > 0 && len(trimmed) < len(line) {
			issues = append(issues, Issue{
				File:       path,
				Line:       i + 1,
				Column:     len(trimmed) + 1,
				Message:    "行尾有多余的空格",
				Suggestion: "删除行尾多余的空格",
			})
		}
	}
	return issues
}

func checkTrailingNewline(path, content string) []Issue {
	if content == "" || strings.HasSuffix(content, "\n") {
		return nil
	}
	line := strings.Count(content, "\n") + 1
	return []Issue{{
		File:       path,
		Line:       line,
		Column:     1,
		Message:    "文件末尾缺少换行符",
		Suggestion: "在文件末尾添加一个换行符",
	}}
}

func checkReferenceLinks(path, content string, skip func(int) bool) []Issue {
	defs := make(map[string]bool)
	for _, m := range referenceDefRegex.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			defs[strings.ToLower(m[1])] = true
		}
	}

	var issues []Issue
	for _, match := range referenceLinkRegex.FindAllStringSubmatchIndex(content, -1) {
		if len(match) < 6 || match[2] < 0 || match[4] < 0 {
			continue
		}

		if match[0] > 0 && content[match[0]-1] == '!' {
			continue
		}

		if skip != nil {
			line, _ := byteOffsetToLineCol(content, match[0])
			if skip(line) {
				continue
			}
		}

		ref := content[match[4]:match[5]]
		if ref == "" {
			ref = content[match[2]:match[3]]
		}

		if !defs[strings.ToLower(ref)] {
			line, col := byteOffsetToLineCol(content, match[0])
			issues = append(issues, Issue{
				File:       path,
				Line:       line,
				Column:     col,
				Message:    fmt.Sprintf("引用式链接 [%s] 未定义", ref),
				Suggestion: fmt.Sprintf("添加 [%s]: <url> 定义", ref),
			})
		}
	}
	return issues
}

func checkBareURLs(path, content string, skip func(int) bool) []Issue {
	var issues []Issue
	for _, m := range bareURLRegex.FindAllStringSubmatchIndex(content, -1) {
		if len(m) < 4 || m[2] < 0 {
			continue
		}
		if skip != nil {
			line, _ := byteOffsetToLineCol(content, m[2])
			if skip(line) {
				continue
			}
		}
		// Skip URLs inside markdown links: [text](url) or autolinks: <url>
		if m[2] > 0 {
			ch := content[m[2]-1]
			if ch == '(' || ch == '<' {
				continue
			}
		}
		// Skip URLs inside reference definitions: [ref]: url
		if m[2] >= 3 && content[m[2]-3:m[2]] == "]: " {
			continue
		}

		url := content[m[2]:m[3]]
		line, col := byteOffsetToLineCol(content, m[2])
		issues = append(issues, Issue{
			File:       path,
			Line:       line,
			Column:     col,
			Message:    fmt.Sprintf("裸 URL：%s", shortText(url, 50)),
			Suggestion: "将 URL 包裹在 [text](url) 或 <url> 中",
		})
	}
	return issues
}

func shortText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func checkTabs(path, content string, skip func(int) bool) []Issue {
	var issues []Issue
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if skip != nil && skip(i+1) {
			continue
		}
		if col := strings.IndexByte(line, '\t'); col >= 0 {
			issues = append(issues, Issue{
				File:       path,
				Line:       i + 1,
				Column:     col + 1,
				Message:    "包含制表符（Tab）",
				Suggestion: "使用空格代替制表符",
			})
		}
	}
	return issues
}

func fixTrailingNewline(content string) string {
	if content == "" || strings.HasSuffix(content, "\n") {
		return content
	}
	return content + "\n"
}

func fixTrailingWhitespace(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRightFunc(line, unicode.IsSpace)
	}
	return strings.Join(lines, "\n")
}

func fixBlankLines(content string) string {
	lines := strings.Split(content, "\n")
	inFence := false
	var result []string

	for i, line := range lines {
		if d := fenceDelimiter(line); d != "" {
			inFence = !inFence
			result = append(result, line)
			continue
		}
		if inFence {
			result = append(result, line)
			continue
		}
		if strings.TrimSpace(line) == "" {
			if i > 0 && strings.TrimSpace(lines[i-1]) == "" && fenceDelimiter(lines[i-1]) == "" {
				continue
			}
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}
