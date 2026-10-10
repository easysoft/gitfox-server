/*
 * @copyright: Copyright 2009-2026 禅道软件（青岛）有限公司(ZenTao Software (Qingdao) Co., Ltd. www.zentao.net)
 * @license: ZPL(http://zpl.pub/page/zplv12.html) or AGPL(https://www.gnu.org/licenses/agpl-3.0.en.html)
 * @author: mengyutong <mengyutong@chandao.com>
 */

package inputs

import (
	"regexp"
	"strings"
)

const (
	// GlobalPrefix 是全局参数在执行参数表中的保留前缀。
	//
	// 全局参数需要在触发侧解析、随 executed params 传到运行期，而运行期只能从
	// build params 拿到数据，所以用保留前缀把两者区分开：带此前缀的键不会进入
	// parameters 命名空间，也不会注入容器环境变量，只会被剥离到 global 命名空间。
	GlobalPrefix = "__global__."

	// GlobalMaskKey 记录需要做日志掩码的全局参数名，多个名字用逗号分隔。
	GlobalMaskKey = "__global_mask__"

	// RedactedValue 密文参数在展示层的占位值。
	RedactedValue = "******"
)

// GlobalKey 返回全局参数在执行参数表中的传输键。
func GlobalKey(name string) string {
	return GlobalPrefix + name
}

// SplitGlobal 把执行参数表拆成三部分：
//
//	plain  —— 普通参数，供 parameters 命名空间与容器环境变量使用
//	global —— 全局参数，供 global 命名空间（${{global.<name>}}）使用
//	masked —— 需要复用日志掩码通道的全局参数：name -> value
//
// 全局参数以 GlobalPrefix 为前缀混在同一个参数表里传输，运行期必须先剥离，否则它们会同时出现在 parameters.* 和容器 env 里。
func SplitGlobal(params map[string]string) (plain map[string]string, global map[string]interface{}, masked map[string]string) {
	plain = make(map[string]string, len(params))
	global = make(map[string]interface{})
	masked = make(map[string]string)

	var maskNames []string
	for k, v := range params {
		switch {
		case k == GlobalMaskKey:
			for _, name := range strings.Split(v, ",") {
				if name = strings.TrimSpace(name); name != "" {
					maskNames = append(maskNames, name)
				}
			}
		case strings.HasPrefix(k, GlobalPrefix):
			global[strings.TrimPrefix(k, GlobalPrefix)] = v
		default:
			plain[k] = v
		}
	}

	for _, name := range maskNames {
		if v, ok := global[name]; ok {
			masked[name] = v.(string)
		}
	}

	return plain, global, masked
}

// globalRefPattern 匹配 ${{global.<name>}} 这类引用，取出其中的参数名。
// 前置字符做了一次排除，避免把 foo.global.bar 这种成员访问误判成引用。
var globalRefPattern = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.])global\.([A-Za-z_][A-Za-z0-9_]*)`)

// GlobalRefs 收集配置文本中引用到的全局参数名，按首次出现顺序去重返回。
func GlobalRefs(config []byte) []string {
	matches := globalRefPattern.FindAllSubmatch(config, -1)
	seen := make(map[string]struct{}, len(matches))
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		name := string(m[1])
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

// RedactGlobalParams 返回用于对外展示的执行参数副本：
//
//   - 内部的全局参数掩码清单键（GlobalMaskKey）会被移除；
//   - 清单中列出的全局参数值替换成 RedactedValue。
//
// 其它参数保持原样。该函数只用于展示层，执行链路必须使用原始参数。
func RedactGlobalParams(params map[string]string) map[string]string {
	if len(params) == 0 {
		return params
	}

	masked := map[string]struct{}{}
	for _, name := range strings.Split(params[GlobalMaskKey], ",") {
		if name = strings.TrimSpace(name); name != "" {
			masked[name] = struct{}{}
		}
	}

	out := make(map[string]string, len(params))
	for k, v := range params {
		if k == GlobalMaskKey {
			continue
		}
		if len(masked) > 0 {
			if name, ok := globalName(k); ok {
				if _, hit := masked[name]; hit {
					v = RedactedValue
				}
			}
		}
		out[k] = v
	}
	return out
}

// globalName 返回全局参数键对应的参数名；非全局参数键返回 false。
func globalName(key string) (string, bool) {
	if !strings.HasPrefix(key, GlobalPrefix) {
		return "", false
	}
	return strings.TrimPrefix(key, GlobalPrefix), true
}
