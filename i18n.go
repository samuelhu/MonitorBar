package main

import (
	"os"
	"strings"
)

// lang is the interface language: "zh" or "en". It follows the system
// locale (mygo.App.Locale) at launch; tests keep the Chinese default.
var lang = "zh"

// useSystemLanguage picks Chinese or English from a locale such as "zh-Hans",
// and renames the scene presets to match.
func useSystemLanguage(locale string) {
	switch {
	case strings.HasPrefix(strings.ToLower(locale), "zh"):
		lang = "zh"
	case strings.TrimSpace(locale) != "":
		lang = "en"
	}
	nameScenes()
}

// tr returns the Chinese or English text.
func tr(zh, en string) string {
	if lang == "en" {
		return en
	}
	return zh
}

// systemLocale is the OS locale string, for --probe which has no app loop.
func systemLocale() string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}
