package bot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

var LocalizedTexts = make(map[string]map[string]string)

func LoadLocales(localesDirPath string) error {
	languages := []string{"uz", "ru", "en"}

	for _, lang := range languages {
		filePath := filepath.Join(localesDirPath, fmt.Sprintf("%s.json", lang))

		fileBytes, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read locale file (%s): %w", lang, err)
		}

		var translations map[string]string
		if err := json.Unmarshal(fileBytes, &translations); err != nil {
			return fmt.Errorf("failed to unmarshal json (%s): %w", lang, err)
		}

		LocalizedTexts[lang] = translations
	}

	return nil
}

func T(lang, key string) string {
	if translations, ok := LocalizedTexts[lang]; ok {
		if text, found := translations[key]; found {
			return text
		}
	}

	if translations, ok := LocalizedTexts["en"]; ok {
		if text, found := translations[key]; found {
			return text
		}
	}

	return key
}
