package coach

import "fmt"

func normalizeLanguage(lang string) string {
	if lang == "fr" {
		return "fr"
	}
	return "en"
}

func languageInstruction(lang string) string {
	switch normalizeLanguage(lang) {
	case "fr":
		return "Respond entirely in French."
	default:
		return "Respond entirely in English."
	}
}

func withLanguage(systemPrompt, lang string) string {
	return fmt.Sprintf("%s\n%s", systemPrompt, languageInstruction(lang))
}
