// 新增 utils.go 文件，集中处理公共逻辑
package ast

import (
	"regexp"
	"strings"

)

// parseCommentParams 解析注释参数
func parseCommentParams(comment string) map[string]string {
	params := make(map[string]string)
	// 合并多行注释为单行
	singleLine := strings.ReplaceAll(comment, "\n", " ")
	// 增强正则表达式，支持多种格式
	re := regexp.MustCompile(`(\w+)\s*=\s*(\[[^\]]+\]|"[^"]+"|'[^']+'|\S+)`)
	matches := re.FindAllStringSubmatch(singleLine, -1)

	for _, m := range matches {
		if m[1] != "" { // 处理数组参数
			params[m[1]] = strings.TrimSpace(m[2])
		} else if m[3] != "" { // 处理普通参数
			params[m[3]] = m[4]
		}
	}
	return params
}

// AppendIfNotExists 辅助函数，避免重复添加
func AppendIfNotExists(slice []string, item string) []string {
	for _, v := range slice {
		if v == item {
			return slice
		}
	}
	return append(slice, item)
}
