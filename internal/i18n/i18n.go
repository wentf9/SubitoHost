package i18n

import (
	"os"
	"strings"
)

var defaultLang = "en"

var dicts = map[string]map[string]string{
	"zh": {
		"Lightweight MIDI router and SoundFont host for live performance": "专注于现场演出的轻量级 MIDI 路由器与 SoundFont 宿主",
		"config file path":                               "配置文件路径",
		"engine API base URL":                            "引擎 API 基础 URL",
		"Start the engine daemon":                        "启动引擎后台进程",
		"using default config: %v":                       "使用默认配置: %v",
		"recovering: setlist=%s index=%d":                "正在恢复: 歌单=%s 索引=%d",
		"recovery failed: %v":                            "恢复失败: %v",
		"load setlist: %w":                               "加载歌单失败: %w",
		"API server: %v":                                 "API 服务器错误: %v",
		"shutting down...":                               "正在关闭应用...",
		"setlist file to load on start":                  "启动时加载的歌单文件",
		"Stop the engine":                                "停止引擎",
		"Use Ctrl+C or kill the daemon process to stop.": "请使用 Ctrl+C 或通过杀进程来停止后台服务。",
		"Query engine status":                            "查询引擎运行状态",
		"List MIDI devices":                              "列出系统 MIDI 设备",
		"Connect a MIDI device":                          "连接一个指定的 MIDI 设备",
		"Manage the setlist":                             "管理当前歌单配置",
		"Load a setlist file":                            "加载一个新的歌单文件",
		"Advance to next profile":                        "切换到列表中的下一首/下一个音色配置",
		"Go to previous profile":                         "退回到上一首/上一个音色配置",
		"Jump to profile at index":                       "跳转至指定索引的配置",
		"Show current setlist and position":              "显示当前歌单及索引位置",
		"no setlist loaded":                              "未加载任何歌单",
	},
}

func init() {
	langKeys := []string{"LC_ALL", "LC_MESSAGES", "LANG"}
	for _, k := range langKeys {
		val := os.Getenv(k)
		if val != "" {
			if strings.HasPrefix(strings.ToLower(val), "zh") {
				defaultLang = "zh"
			}
			break
		}
	}
}

// T translates a message based on the detected language.
func T(msg string) string {
	if defaultLang == "en" {
		return msg
	}
	if translated, ok := dicts[defaultLang][msg]; ok {
		return translated
	}
	return msg // fallback to english
}
