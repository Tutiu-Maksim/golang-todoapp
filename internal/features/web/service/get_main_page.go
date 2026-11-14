package web_service

import (
	"fmt"
	"os"
	"path"
)

func (s *WebService) GetMainPage() ([]byte, error) {
	var htmlFilePath string
	if root := os.Getenv("PROJECT_ROOT"); root != "" {
		htmlFilePath = path.Join(root, "public", "index.html")
	} else {
		htmlFilePath = path.Join("public", "index.html")
	}

	html, err := s.webRepository.GetFile(htmlFilePath)
	if err != nil {
		// Fallback to relative ./public/index.html
		if fallbackHtml, fallbackErr := s.webRepository.GetFile("./public/index.html"); fallbackErr == nil {
			return fallbackHtml, nil
		}
		return nil, fmt.Errorf("get file from repository: %w", err)
	}

	return html, nil
}
