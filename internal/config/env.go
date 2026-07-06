// Package config 负责加载本地配置与密钥。
//
// 安全红线（设计文档 §6.4）：API key 只存在于本地 .env，
// 绝不写进代码、config.yaml 或任何会被 push 到公网站的文件。.env 已在 .gitignore。
package config

import (
	"bufio"
	"os"
	"strings"
)

// LoadDotEnv 读取 path 指向的 .env 文件，把 KEY=VALUE 注入进程环境变量。
//
// 规则：
//   - 忽略空行与以 # 开头的注释行。
//   - 去掉 value 两端的引号与空白。
//   - 已存在的环境变量不覆盖（真实环境变量优先于 .env）。
//   - 文件不存在不算错误（允许用纯环境变量运行）。
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
	return sc.Err()
}
