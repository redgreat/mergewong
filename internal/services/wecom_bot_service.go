package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const wecomWebhookBase = "https://qyapi.weixin.qq.com/cgi-bin/webhook"

type WecomBotService struct {
	client *http.Client
}

type wecomResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	MediaID string `json:"media_id"`
}

func NewWecomBotService() *WecomBotService {
	return &WecomBotService{client: &http.Client{Timeout: 20 * time.Second}}
}

func normalizeWecomWebhook(target string) string {
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target
	}
	return wecomWebhookBase + "/send?key=" + url.QueryEscape(target)
}

func getWecomKey(target string) (string, error) {
	target = strings.TrimSpace(target)
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		parsed, err := url.Parse(target)
		if err != nil {
			return "", fmt.Errorf("解析企业微信 webhook 失败: %w", err)
		}
		target = parsed.Query().Get("key")
	}
	if target == "" {
		return "", fmt.Errorf("企业微信机器人 ID 不能为空")
	}
	return target, nil
}

// NormalizeWecomRobotID accepts a robot key or a standard webhook URL and returns only the key.
func NormalizeWecomRobotID(target string) (string, error) {
	key, err := getWecomKey(target)
	if err != nil {
		return "", err
	}
	if strings.ContainsAny(key, " \t\r\n&?#") {
		return "", fmt.Errorf("企业微信机器人 ID 格式不正确")
	}
	return key, nil
}

func (s *WecomBotService) SendText(ctx context.Context, target, content string) error {
	payload := map[string]interface{}{"msgtype": "text", "text": map[string]string{"content": content}}
	return s.postJSON(ctx, normalizeWecomWebhook(target), payload)
}

func (s *WecomBotService) UploadFile(ctx context.Context, target, filePath string) (string, error) {
	key, err := getWecomKey(target)
	if err != nil {
		return "", err
	}
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开预警文件失败: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", filepath.Base(filePath))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	uploadURL := wecomWebhookBase + "/upload_media?key=" + url.QueryEscape(key) + "&type=file"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	uploadClient := &http.Client{Timeout: 60 * time.Second}
	resp, err := uploadClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("上传企业微信文件失败: %w", err)
	}
	defer resp.Body.Close()
	result, err := decodeWecomResponse(resp)
	if err != nil {
		return "", err
	}
	if result.MediaID == "" {
		return "", fmt.Errorf("企业微信文件上传返回缺少 media_id")
	}
	return result.MediaID, nil
}

func (s *WecomBotService) SendFile(ctx context.Context, target, filePath string) error {
	mediaID, err := s.UploadFile(ctx, target, filePath)
	if err != nil {
		return err
	}
	payload := map[string]interface{}{"msgtype": "file", "file": map[string]string{"media_id": mediaID}}
	return s.postJSON(ctx, normalizeWecomWebhook(target), payload)
}

func (s *WecomBotService) postJSON(ctx context.Context, endpoint string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送企业微信消息失败: %w", err)
	}
	defer resp.Body.Close()
	_, err = decodeWecomResponse(resp)
	return err
}

func decodeWecomResponse(resp *http.Response) (*wecomResponse, error) {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("企业微信接口返回 HTTP %d", resp.StatusCode)
	}
	var result wecomResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析企业微信响应失败: %w", err)
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("企业微信接口错误: %d %s", result.ErrCode, result.ErrMsg)
	}
	return &result, nil
}
