package main

import (
	"testing"
)

func TestCheckFencedCodeBlocks(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "闭合的代码块", content: "```go\npackage main\n```", want: 0},
		{name: "未闭合的代码块", content: "```go\npackage main\nfunc main() {}", want: 1},
		{name: "无代码块", content: "# Hello\nNormal text.", want: 0},
		{name: "~~~闭合的代码块", content: "~~~go\npackage main\n~~~", want: 0},
		{name: "~~~未闭合的代码块", content: "~~~go\npackage main\nfunc main() {}", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkFencedCodeBlocks("test.md", tt.content)
			if len(issues) != tt.want {
				t.Errorf("期望 %d 个问题，得到 %d 个", tt.want, len(issues))
			}
		})
	}
}

func TestCheckHeaders(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "空标题", content: "#\n\n##", want: 2},
		{name: "正常标题", content: "# Title\n## Subtitle", want: 0},
		{name: "重复标题", content: "# Hello\n## World\n## World", want: 1},
		{name: "空标题和重复标题混合", content: "#\n## Test\n## Test", want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkHeaders("test.md", tt.content, nil)
			if len(issues) != tt.want {
				t.Errorf("期望 %d 个问题，得到 %d 个", tt.want, len(issues))
			}
		})
	}
}

func TestCheckLinks(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "有效链接", content: "[Google](https://google.com)", want: 0},
		{name: "无效http链接", content: "[Bad](http://)", want: 1},
		{name: "无效https链接", content: "[Bad](https://)", want: 1},
		{name: "相对链接", content: "[Doc](./doc.md)", want: 0},
		{name: "锚点链接", content: "[Section](#section)", want: 0},
		{name: "空锚点", content: "[Empty](#)", want: 1},
		{name: "未知格式", content: "[Test](badscheme:)", want: 1},
		{name: "图片语法不误报", content: "![alt](image.png)", want: 0},
		{name: "图片空alt不误报", content: "![](image.png)", want: 0},
		{name: "mailto链接", content: "[Email](mailto:user@example.com)", want: 0},
		{name: "tel链接", content: "[Call](tel:+1234567890)", want: 0},
		{name: "空mailto", content: "[Email](mailto:)", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkLinks("test.md", tt.content, nil)
			if len(issues) != tt.want {
				t.Errorf("期望 %d 个问题，得到 %d 个", tt.want, len(issues))
			}
		})
	}
}

func TestCheckTables(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{
			name: "对齐的表格",
			content: `| A | B |
|---|---|
| 1 | 2 |
| 3 | 4 |`,
			want: 0,
		},
		{
			name: "未对齐的表格",
			content: `| A | B |
|---|---|
| 1 | 2 |
| 3 |`,
			want: 1,
		},
		{
			name: "多行表格仅第三行未对齐",
			content: `| A | B |
|---|---|
| 1 | 2 |
| 3 | 4 |
| 5 |`,
			want: 1,
		},
		{
			name: "多行表格全部未对齐",
			content: `| A |
|---|
| 1 | 2 |
| 3 | 4 |`,
			want: 2,
		},
		{
			name: "对齐标记:---",
			content: `| A | B |
|:---|---|
| 1 | 2 |
| 3 | 4 |`,
			want: 0,
		},
		{
			name: "对齐标记:---:和---:",
			content: `| A | B | C |
|:---:|:---:|---:|
| 1 | 2 | 3 |`,
			want: 0,
		},
		{
			name: "单元格内含---",
			content: `| A | B |
|---|---|
| X---Y | 2 |
| 3 | 4 |`,
			want: 0,
		},
		{
			name: "带对齐标记的未对齐",
			content: `| A | B |
|:---:|---|
| 1 | 2 |
| 3 |`,
			want: 1,
		},
		{
			name: "无表格",
			content: "# Hello\nNormal text.",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skip := buildLineSkip(tt.content)
			issues := checkTables("test.md", tt.content, skip)
			if len(issues) != tt.want {
				t.Errorf("期望 %d 个问题，得到 %d 个", tt.want, len(issues))
			}
		})
	}

	// code block in table test
	t.Run("代码块内表格不误报", func(t *testing.T) {
		content := "Not a table\n\n```\n| A | B |\n|---|---|\n| 1 | 2 |\n```"
		skip := buildLineSkip(content)
		issues := checkTables("test.md", content, skip)
		if len(issues) != 0 {
			t.Errorf("代码块内表格不期望问题，得到 %d 个", len(issues))
		}
	})
}

func TestCheckLists(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "缩进一致的无序列表", content: "- Item 1\n- Item 2\n- Item 3", want: 0},
		{name: "缩进一致的有序列表", content: "1. First\n2. Second\n3. Third", want: 0},
		{name: "空内容", content: "", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkLists("test.md", tt.content, nil)
			if len(issues) != tt.want {
				t.Errorf("期望 %d 个问题，得到 %d 个", tt.want, len(issues))
			}
		})
	}
}

func TestCheckCodeBlockLanguage(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "已知语言", content: "```go\npackage main\n```", want: 0},
		{name: "未知语言", content: "```xyz\ncode\n```", want: 1},
		{name: "无语言标识", content: "```\ncode\n```", want: 0},
		{name: "多个未知语言", content: "```xyz\ncode\n```\n\n```abc\nmore\n```", want: 2},
		{name: "~~~已知语言", content: "~~~go\ncode\n~~~", want: 0},
		{name: "~~~未知语言", content: "~~~xyz\ncode\n~~~", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkCodeBlockLanguage("test.md", tt.content)
			if len(issues) != tt.want {
				t.Errorf("期望 %d 个问题，得到 %d 个", tt.want, len(issues))
			}
		})
	}
}

func TestCheckHeadingLevels(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "渐进标题", content: "# H1\n## H2\n### H3", want: 0},
		{name: "H1到H3跳跃", content: "# H1\n### H3", want: 1},
		{name: "H1到H2再到H4", content: "# H1\n## H2\n#### H4", want: 1},
		{name: "单个标题", content: "# H1", want: 0},
		{name: "代码块内跳过", content: "# H1\n```\n## Code H2\n```", want: 0},
		{name: "不连续段落", content: "# H1\n\nsome text\n\n### H3", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skip := buildLineSkip(tt.content)
			issues := checkHeadingLevels("test.md", tt.content, skip)
			if len(issues) != tt.want {
				t.Errorf("期望 %d 个问题，得到 %d 个", tt.want, len(issues))
			}
		})
	}
}

func TestCheckDeepHeaders(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "H7溢出", content: "####### H7", want: 1},
		{name: "H6正常", content: "###### H6", want: 0},
		{name: "代码块内不报", content: "```\n####### H7\n```", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skip := buildLineSkip(tt.content)
			issues := checkDeepHeaders("test.md", tt.content, skip)
			if len(issues) != tt.want {
				t.Errorf("期望 %d 个问题，得到 %d 个", tt.want, len(issues))
			}
		})
	}
}

func TestCheckBlankLines(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "无多余空行", content: "# Hello\n\nNormal text.", want: 0},
		{name: "一个多余空行", content: "# Hello\n\n\nNormal text.", want: 1},
		{name: "多个多余空行", content: "# Hello\n\n\n\nNormal text.", want: 2},
		{name: "连续空行末尾", content: "# Hello\n\nNormal.\n\n\n", want: 1},
		{name: "只有一行", content: "Hello", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkBlankLines("test.md", tt.content, nil)
			if len(issues) != tt.want {
				t.Errorf("期望 %d 个问题，得到 %d 个", tt.want, len(issues))
			}
		})
	}
}

func TestCheckListMarkers(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "统一-列表", content: "- A\n- B\n- C", want: 0},
		{name: "统一*列表", content: "* A\n* B\n* C", want: 0},
		{name: "混用列表", content: "- A\n- B\n* C", want: 1},
		{name: "不同列表间隔开", content: "- A\n- B\n\n* C", want: 0},
		{name: "有序列表不影响", content: "- A\n1. B\n- C", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkListMarkers("test.md", tt.content, nil)
			if len(issues) != tt.want {
				t.Errorf("期望 %d 个问题，得到 %d 个", tt.want, len(issues))
			}
		})
	}
}

func TestCheckImageAlt(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "有alt文本", content: "![logo](logo.png)", want: 0},
		{name: "空alt文本", content: "![](logo.png)", want: 1},
		{name: "普通链接不干扰", content: "[text](url)", want: 0},
		{name: "多个空alt", content: "![](a.png) and ![](b.png)", want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkImageAlt("test.md", tt.content, nil)
			if len(issues) != tt.want {
				t.Errorf("期望 %d 个问题，得到 %d 个", tt.want, len(issues))
			}
		})
	}
}

func TestCheckLongLines(t *testing.T) {
	tests := []struct {
		name    string
		content string
		maxLen  int
		want    int
	}{
		{name: "短行", content: "short", maxLen: 10, want: 0},
		{name: "超长行", content: "this is a very long line that exceeds the limit", maxLen: 10, want: 1},
		{name: "恰好", content: "1234567890", maxLen: 10, want: 0},
		{name: "多行超长", content: "short\nthis is a very long line\nanother very long line here", maxLen: 15, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkLongLines("test.md", tt.content, nil, tt.maxLen)
			if len(issues) != tt.want {
				t.Errorf("期望 %d 个问题，得到 %d 个", tt.want, len(issues))
			}
		})
	}
}

func TestCheckRefImageAlt(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "有空alt", content: "![alt][ref]", want: 0},
		{name: "空alt", content: "![][ref]", want: 1},
		{name: "多个空alt", content: "![][a] and ![][b]", want: 2},
		{name: "代码块跳过", content: "```\n![][ref]\n```", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skip := buildLineSkip(tt.content)
			issues := checkRefImageAlt("test.md", tt.content, skip)
			if len(issues) != tt.want {
				t.Errorf("期望 %d 个问题，得到 %d 个", tt.want, len(issues))
			}
		})
	}
}

func TestCheckTrailingWhitespace(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "无行尾空格", content: "# Hello\nNormal text.", want: 0},
		{name: "单行行尾空格", content: "# Hello   \nNormal text.", want: 1},
		{name: "多行行尾空格", content: "# Hello   \nNormal text. \n", want: 2},
		{name: "空行不计", content: "# Hello\n\n\nNormal text.", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkTrailingWhitespace("test.md", tt.content, nil)
			if len(issues) != tt.want {
				t.Errorf("期望 %d 个问题，得到 %d 个", tt.want, len(issues))
			}
		})
	}
}

func TestCheckFile(t *testing.T) {
	issues, err := checkFile("test_data/passing.md", 120)
	if err != nil {
		t.Fatalf("checkFile 返回错误：%v", err)
	}
	if len(issues) != 0 {
		t.Errorf("passing.md 期望 0 个问题，得到 %d 个", len(issues))
	}

	issues, err = checkFile("test_data/fail_empty_header.md", 120)
	if err != nil {
		t.Fatalf("checkFile 返回错误：%v", err)
	}
	if len(issues) == 0 {
		t.Error("fail_empty_header.md 期望发现问题，但没有找到")
	}
}

func TestFixBlankLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "无多余空行", input: "A\n\nB", want: "A\n\nB"},
		{name: "多余空行", input: "A\n\n\nB", want: "A\n\nB"},
		{name: "多个多余", input: "A\n\n\n\nB", want: "A\n\nB"},
		{name: "代码块内保留", input: "```\n\n\ncode\n\n\n```", want: "```\n\n\ncode\n\n\n```"},
		{name: "首尾空行", input: "\n\n\nA\n\n\n", want: "\nA\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixBlankLines(tt.input)
			if got != tt.want {
				t.Errorf("fixBlankLines() = %q，期望 %q", got, tt.want)
			}
		})
	}
}

func TestCheckTrailingNewline(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "有尾随换行", content: "# Hello\n", want: 0},
		{name: "无尾随换行", content: "# Hello", want: 1},
		{name: "多个尾随换行", content: "# Hello\n\n\n", want: 0},
		{name: "空内容", content: "", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkTrailingNewline("test.md", tt.content)
			if len(issues) != tt.want {
				t.Errorf("期望 %d 个问题，得到 %d 个", tt.want, len(issues))
			}
		})
	}
}

func TestCheckReferenceLinks(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "已定义引用", content: "[text][ref]\n\n[ref]: http://example.com", want: 0},
		{name: "未定义引用", content: "[text][ref]", want: 1},
		{name: "简写引用已定义", content: "[ref][]\n\n[ref]: http://example.com", want: 0},
		{name: "简写引用未定义", content: "[ref][]", want: 1},
		{name: "图片语法不误报", content: "![alt][ref]", want: 0},
		{name: "内联链接不干扰", content: "[text](url)", want: 0},
		{name: "大小写不敏感", content: "[text][Ref]\n\n[ref]: url", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := checkReferenceLinks("test.md", tt.content, nil)
			if len(issues) != tt.want {
				t.Errorf("期望 %d 个问题，得到 %d 个", tt.want, len(issues))
			}
		})
	}
}

func TestCheckBareURLs(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "内联链接不报", content: "[text](https://example.com)", want: 0},
		{name: "自动链接不报", content: "<https://example.com>", want: 0},
		{name: "引用定义不报", content: "[ref]: https://example.com", want: 0},
		{name: "裸URL报错", content: "Visit https://example.com now", want: 1},
		{name: "多个裸URL", content: "https://a.com and https://b.com", want: 2},
		{name: "代码块内不报", content: "```\nhttps://example.com\n```", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skip := buildLineSkip(tt.content)
			issues := checkBareURLs("test.md", tt.content, skip)
			if len(issues) != tt.want {
				t.Errorf("期望 %d 个问题，得到 %d 个", tt.want, len(issues))
			}
		})
	}
}

func TestCheckTabs(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    int
	}{
		{name: "无制表符", content: "no tabs here", want: 0},
		{name: "有制表符", content: "\tindented", want: 1},
		{name: "多行制表符", content: "no\tway\n\tindented", want: 2},
		{name: "代码块内跳过", content: "normal\n```\n\tcode\n```", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skip := buildLineSkip(tt.content)
			issues := checkTabs("test.md", tt.content, skip)
			if len(issues) != tt.want {
				t.Errorf("期望 %d 个问题，得到 %d 个", tt.want, len(issues))
			}
		})
	}
}

func TestFixTrailingNewline(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "已有换行", input: "hello\n", want: "hello\n"},
		{name: "无换行", input: "hello", want: "hello\n"},
		{name: "空内容", input: "", want: ""},
		{name: "多行已有换行", input: "hello\nworld\n", want: "hello\nworld\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixTrailingNewline(tt.input)
			if got != tt.want {
				t.Errorf("fixTrailingNewline() = %q，期望 %q", got, tt.want)
			}
		})
	}
}

func TestFixTrailingWhitespace(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
	}{
		{name: "无空格", input: "Hello\nWorld", want: "Hello\nWorld"},
		{name: "有空格", input: "Hello   \nWorld ", want: "Hello\nWorld"},
		{name: "空行保留", input: "\n\n", want: "\n\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fixTrailingWhitespace(tt.input)
			if got != tt.want {
				t.Errorf("fixTrailingWhitespace() = %q，期望 %q", got, tt.want)
			}
		})
	}
}
