package document

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

const ocrSpaceEndpoint = "https://api.ocr.space/parse/image"

// OCRSpaceExtractText sends image bytes to the OCR.space API and returns concatenated plain text.
func OCRSpaceExtractText(ctx context.Context, apiKey string, imageBytes []byte, filename string) (string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return "", errors.New("ocr is not configured: set OCR_SPACE_API_KEY")
	}
	if len(imageBytes) == 0 {
		return "", errors.New("empty image payload")
	}

	body := &bytes.Buffer{}
	mp := multipart.NewWriter(body)

	if err := mp.WriteField("apikey", apiKey); err != nil {
		return "", err
	}
	if err := mp.WriteField("language", "eng"); err != nil {
		return "", err
	}
	if err := mp.WriteField("isOverlayRequired", "false"); err != nil {
		return "", err
	}
	if err := mp.WriteField("OCREngine", "1"); err != nil {
		return "", err
	}

	part, err := mp.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(imageBytes); err != nil {
		return "", err
	}
	contentType := mp.FormDataContentType()
	if err := mp.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ocrSpaceEndpoint, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", contentType)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ocr.space request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ocr.space http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var parsed struct {
		IsErroredOnProcessing bool `json:"IsErroredOnProcessing"`
		ErrorMessage          *string
		ErrorDetails          *string
		ParsedResults         []struct {
			ParsedText   string  `json:"ParsedText"`
			ErrorMessage *string `json:"ErrorMessage"`
		} `json:"ParsedResults"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("ocr.space invalid json: %w", err)
	}

	if parsed.IsErroredOnProcessing || (parsed.ErrorMessage != nil && strings.TrimSpace(*parsed.ErrorMessage) != "") {
		msg := ""
		if parsed.ErrorMessage != nil {
			msg = strings.TrimSpace(*parsed.ErrorMessage)
		}
		if parsed.ErrorDetails != nil && strings.TrimSpace(*parsed.ErrorDetails) != "" {
			if msg != "" {
				msg += ": "
			}
			msg += strings.TrimSpace(*parsed.ErrorDetails)
		}
		if msg == "" {
			msg = "unknown error from ocr.space"
		}
		return "", fmt.Errorf("ocr.space: %s", msg)
	}

	var parts []string
	for _, pr := range parsed.ParsedResults {
		if pr.ErrorMessage != nil && strings.TrimSpace(*pr.ErrorMessage) != "" {
			continue
		}
		t := strings.TrimSpace(pr.ParsedText)
		if t != "" {
			parts = append(parts, t)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n")), nil
}
