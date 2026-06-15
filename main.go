package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

func runChecks(path, content string, maxLineLen int) []Issue {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	skip := buildLineSkip(content)

	var issues []Issue
	issues = append(issues, checkFencedCodeBlocks(path, content)...)
	issues = append(issues, checkHeaders(path, content, skip)...)
	issues = append(issues, checkHeadingLevels(path, content, skip)...)
	issues = append(issues, checkDeepHeaders(path, content, skip)...)
	issues = append(issues, checkLinks(path, content, skip)...)
	issues = append(issues, checkTables(path, content, skip)...)
	issues = append(issues, checkLists(path, content, skip)...)
	issues = append(issues, checkCodeBlockLanguage(path, content)...)
	issues = append(issues, checkBlankLines(path, content, skip)...)
	issues = append(issues, checkListMarkers(path, content, skip)...)
	issues = append(issues, checkImageAlt(path, content, skip)...)
	issues = append(issues, checkRefImageAlt(path, content, skip)...)
	issues = append(issues, checkTabs(path, content, skip)...)
	issues = append(issues, checkLongLines(path, content, skip, maxLineLen)...)
	issues = append(issues, checkTrailingWhitespace(path, content, skip)...)
	issues = append(issues, checkTrailingNewline(path, content)...)
	issues = append(issues, checkReferenceLinks(path, content, skip)...)
	issues = append(issues, checkBareURLs(path, content, skip)...)
	return issues
}

func checkFile(path string, maxLineLen int) ([]Issue, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取文件 %s 失败：%w", path, err)
	}

	return runChecks(path, string(content), maxLineLen), nil
}

func checkStdin(maxLineLen int) ([]Issue, error) {
	content, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("读取标准输入失败：%w", err)
	}

	return runChecks("stdin", string(content), maxLineLen), nil
}

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
)

func isTerminal() bool {
	stat, _ := os.Stdout.Stat()
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func printReport(issues []Issue, jsonOutput bool) {
	color := isTerminal()

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if len(issues) == 0 {
			enc.Encode([]Issue{})
		} else {
			enc.Encode(issues)
		}
		return
	}

	if len(issues) == 0 {
		if color {
			fmt.Printf("%s✓%s 所有文件通过检查\n", green, reset)
		} else {
			fmt.Println("✓ 所有文件通过检查")
		}
		return
	}

	if color {
		fmt.Printf("%s发现 %d 个问题%s\n", red, len(issues), reset)
	} else {
		fmt.Printf("发现 %d 个问题\n", len(issues))
	}

	fileIssues := make(map[string][]Issue)
	for _, issue := range issues {
		fileIssues[issue.File] = append(fileIssues[issue.File], issue)
	}

	for file, list := range fileIssues {
		if color {
			fmt.Printf("\n%s%s%s:\n", bold, file, reset)
		} else {
			fmt.Printf("\n%s:\n", file)
		}
		for _, issue := range list {
			if color {
				fmt.Printf("  %s%d:%d%s %s\n", yellow, issue.Line, issue.Column, reset, issue.Message)
				if issue.Suggestion != "" {
					fmt.Printf("    %s→%s %s\n", bold, reset, issue.Suggestion)
				}
			} else {
				fmt.Printf("  %d:%d %s\n", issue.Line, issue.Column, issue.Message)
				if issue.Suggestion != "" {
					fmt.Printf("    → %s\n", issue.Suggestion)
				}
			}
		}
	}
}

var version = "0.3.0"

func main() {
	filePath := flag.String("f", "", "指定单个文件")
	jsonOutput := flag.Bool("json", false, "以 JSON 格式输出结果")
	fix := flag.Bool("fix", false, "自动修复行尾空格、连续空行和缺失换行符")
	maxLineLength := flag.Int("max-line-length", 120, "超长行阈值")
	quiet := flag.Bool("quiet", false, "静默模式，仅用退出码表示结果（不输出到 stdout）")
	showVersion := flag.Bool("version", false, "显示版本号")
	flag.Parse()

	if *showVersion {
		fmt.Printf("mdcheck %s\n", version)
		return
	}

	var files []string

	if *filePath != "" {
		files = append(files, *filePath)
	} else {
		files = flag.Args()
	}

	if len(files) == 0 {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			issues, err := checkStdin(*maxLineLength)
			if err != nil {
				fmt.Fprintf(os.Stderr, "错误：%v\n", err)
				os.Exit(1)
			}
			if !*quiet {
				printReport(issues, *jsonOutput)
			}
			if len(issues) > 0 {
				os.Exit(1)
			}
			return
		}

		fmt.Println("用法：mdcheck [选项] [文件...]")
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

	var mu sync.Mutex
	var allIssues []Issue
	var wg sync.WaitGroup
	for _, file := range files {
		wg.Add(1)
		go func(file string) {
			defer wg.Done()
			content, err := os.ReadFile(file)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "文件不存在：%s\n", file)
				} else {
					fmt.Fprintf(os.Stderr, "读取文件 %s 失败：%v\n", file, err)
				}
				os.Exit(1)
			}

			s := string(content)
			if *fix {
				fixed := fixTrailingWhitespace(s)
				fixed = fixBlankLines(fixed)
				fixed = fixTrailingNewline(fixed)
				if fixed != s {
					if err := os.WriteFile(file, []byte(fixed), 0644); err != nil {
						fmt.Fprintf(os.Stderr, "写入文件 %s 失败：%v\n", file, err)
						os.Exit(1)
					}
					fmt.Printf("已修复：%s\n", file)
				}
				s = fixed
			}

			issues := runChecks(file, s, *maxLineLength)
			mu.Lock()
			allIssues = append(allIssues, issues...)
			mu.Unlock()
		}(file)
	}
	wg.Wait()

	if !*quiet {
		printReport(allIssues, *jsonOutput)
	}

	if len(allIssues) > 0 && !*fix {
		os.Exit(1)
	}

	if *fix && len(allIssues) > 0 {
		fmt.Fprintf(os.Stderr, "修复后仍有 %d 个问题需要手动处理\n", len(allIssues))
		os.Exit(1)
	}
}
