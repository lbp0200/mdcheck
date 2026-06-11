// Package mdcheck 用于检查 Markdown 文件是否规范，防止 AI 生成文档导致渲染错乱
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Issue 表示发现的问题
type Issue struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	Message   string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

var allIssues []Issue

// checkFile 检查单个 Markdown 文件
func checkFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取文件 %s 失败：%w", path, err)
	}

	checkFencedCodeBlocks(string(content))
	checkHeaders(string(content))
	checkLinks(string(content))
	checkTables(string(content))
	checkLists(string(content))
	checkCodeBlockLanguage(string(content))

	return nil
}

// checkFencedCodeBlocks 检查未闭合的代码块
func checkFencedCodeBlocks(content string) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 1
	inFence := false

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		if strings.HasPrefix(line, "```") {
			if !inFence {
				inFence = true
			} else {
				inFence = false
			}
		} else if inFence {
			if strings.Contains(line, "```") {
				issue := Issue{
					File:      "",
					Line:      lineNum,
					Column:    strings.Index(line, "```") + 1,
					Message:   "代码块中可能包含意外的 ``` 标记",
					Suggestion: "检查是否意外在代码内容中包含了 ```",
				}
				allIssues = append(allIssues, issue)
			}
		}
	}

	if inFence {
		issue := Issue{
			File:      "",
			Line:      lineNum - 1,
			Column:    1,
			Message:   "未闭合的代码块，未找到对应的结束标记 ```",
			Suggestion: "添加 ``` 结束标记，或检查是否意外在代码内容中包含了 ```",
		}
		allIssues = append(allIssues, issue)
	}
}

// checkHeaders 检查标题问题
func checkHeaders(content string) {
	headerRegex := regexp.MustCompile(`^#{1,6}\s*$`)
	headerWithTextRegex := regexp.MustCompile(`^#{1,6}\s+(.+)$`)

	lines := strings.Split(content, "\n")
	headerMap := make(map[string]int)

	for i, line := range lines {
		if headerRegex.MatchString(line) {
			issue := Issue{
				File:      "",
				Line:      i + 1,
				Column:    1,
				Message:   "空标题：标题后没有内容",
				Suggestion: "在标题后添加内容，或删除空标题",
			}
			allIssues = append(allIssues, issue)
		}

		matches := headerWithTextRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			text := strings.TrimSpace(matches[1])
			text = strings.TrimPrefix(text, "*")

			if _, exists := headerMap[text]; exists {
				issue := Issue{
					File:      "",
					Line:      i + 1,
					Column:    1,
					Message:   fmt.Sprintf("重复的标题：'%s'", text),
					Suggestion: "使用更具体的标题名称，或修改其中一个标题",
				}
				allIssues = append(allIssues, issue)
			}
			headerMap[text] = i + 1
		}
	}
}

// checkLinks 检查链接格式
func checkLinks(content string) {
	linkRegex := regexp.MustCompile(`\[(.+?)\]\(([^)\s]+)(?:\s+["\'](.+?)["\'])?\)`)

	matches := linkRegex.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		url := match[2]

		if !isValidURL(url) {
			issue := Issue{
				File:      "",
				Line:      0,
				Message:   fmt.Sprintf("可能无效的链接：%s", url),
				Suggestion: "检查链接是否可访问，或使用相对路径",
			}
			allIssues = append(allIssues, issue)
		}
	}
}

// isValidURL 简单的 URL 有效性检查
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

	return false
}

// checkTables 检查表格格式
func checkTables(content string) {
	lines := strings.Split(content, "\n")

	for i := 0; i < len(lines)-1; i++ {
		line := lines[i]

		if strings.Contains(line, "|---|") || strings.Contains(line, "---|") {
			nextLine := lines[i+1]

			if !isTableRowAligned(line, nextLine) {
				issue := Issue{
					File:      "",
					Line:      i + 2,
					Column:    1,
					Message:   "表格行未正确对齐",
					Suggestion: "确保表格列使用 | 分隔，并且列数一致",
				}
				allIssues = append(allIssues, issue)
			}
		}
	}
}

// isTableRowAligned 检查表格行是否对齐
func isTableRowAligned(headerLine, dataLine string) bool {
	headerCols := countColumns(headerLine)
	dataCols := countColumns(dataLine)

	return headerCols == dataCols
}

// countColumns 计算表格列数
func countColumns(line string) int {
	count := strings.Count(line, "|")
	if count == 0 {
		return 1
	}
	return count + 1
}

// checkLists 检查列表格式
func checkLists(content string) {
	lines := strings.Split(content, "\n")

	inList := false
	expectedIndent := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		isListStart := strings.HasPrefix(trimmed, "- ") ||
			strings.HasPrefix(trimmed, "* ") ||
			strings.HasPrefix(trimmed, "1. ")

		if isListStart {
			if inList {
				currentIndent := getIndentation(line)
				if currentIndent != expectedIndent {
					issue := Issue{
						File:      "",
						Line:      i + 1,
						Column:    1,
						Message:   fmt.Sprintf("列表缩进不一致：期望 %d 个空格，实际 %d 个", expectedIndent, currentIndent),
						Suggestion: "保持列表项缩进一致",
					}
					allIssues = append(allIssues, issue)
				}
			}
			inList = true
			expectedIndent = getIndentation(line)
		} else if inList {
			inList = false
		}
	}
}

// getIndentation 获取缩进空格数
func getIndentation(line string) int {
	for i, ch := range line {
		if ch != ' ' && ch != '\t' {
			return i
		}
	}
	return len(line)
}

// checkCodeBlockLanguage 检查代码块语言标识
func checkCodeBlockLanguage(content string) {
	lines := strings.Split(content, "\n")

	for i, line := range lines {
		if strings.HasPrefix(line, "```") {
			l := strings.TrimSpace(line[3:])
			if l != "" && l != "```" {
				language := strings.Split(l, " ")[0]

				if !isKnownLanguage(language) {
					issue := Issue{
						File:      "",
						Line:      i + 1,
						Column:    1,
						Message:   fmt.Sprintf("未知的代码块语言：%s", language),
						Suggestion: "使用已知的语言标识，如 go, python, javascript, html 等",
					}
					allIssues = append(allIssues, issue)
				}
			}
		}
	}
}

// isKnownLanguage 检查是否是已知的语言
func isKnownLanguage(lang string) bool {
	knownLanguages := map[string]bool{
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
	}

	return knownLanguages[strings.ToLower(lang)]
}

func main() {
	filePath := flag.String("f", "", "指定单个文件")
	flag.Parse()

	var files []string

	if *filePath != "" {
		files = append(files, *filePath)
	} else {
		files = flag.Args()
	}

	if len(files) == 0 {
		fmt.Println("用法：mdcheck [选项] [文件...]\n")
		fmt.Println("检查 Markdown 文件是否规范，防止 AI 生成文档导致渲染错乱")
		fmt.Println("\n选项:")
		flag.PrintDefaults()
		fmt.Println("\n示例:")
		fmt.Println("  mdcheck README.md")
		fmt.Println("  mdcheck docs/*.md")
		fmt.Println("  mdcheck -f document.md")
		fmt.Println("\n从 stdin 读取:")
		fmt.Println("  cat file.md | mdcheck")
		os.Exit(1)
	}

	// 检查每个文件
	for _, file := range files {
		if _, err := os.Stat(file); err == nil {
			if err := checkFile(file); err != nil {
				fmt.Fprintf(os.Stderr, "错误：%v\n", err)
				os.Exit(1)
			}
		} else if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "文件不存在：%s\n", file)
			os.Exit(1)
		}
	}

	// 输出报告
	if len(allIssues) == 0 {
		fmt.Println("✓ 所有文件通过检查")
		os.Exit(0)
	}

	fmt.Printf("发现 %d 个问题\n", len(allIssues))

	// 按文件分组输出
	fileIssues := make(map[string][]Issue)
	for _, issue := range allIssues {
		fileIssues[issue.File] = append(fileIssues[issue.File], issue)
	}

	for file, fileIssuesList := range fileIssues {
		fmt.Printf("\n%s:\n", file)
		for _, issue := range fileIssuesList {
			fmt.Printf("  %d:%d %s\n", issue.Line, issue.Column, issue.Message)
			if issue.Suggestion != "" {
				fmt.Printf("    → %s\n", issue.Suggestion)
			}
		}
	}

	os.Exit(1)
}
