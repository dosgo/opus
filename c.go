package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const (
	deepSeekAPIURL  = "https://api.deepseek.com/v1/chat/completions"
	modelName       = "deepseek-coder"
	maxConcurrency  = 5
	maxRetries      = 3
	cFileExtension  = ".java"
	hFileExtension  = ".h"
	goFileExtension = ".go"
	ignoreDir       = "vendor"
	apiKeyFile      = "apikey.txt" // API密钥文件
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RequestPayload struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens,omitempty"` // 添加max_tokens参数
}

type ResponsePayload struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func main() {
	sourceDir := "./test"
	fmt.Printf("Converting C files in directory: %s\n", sourceDir)
	fmt.Printf("Maximum concurrent requests: %d\n", maxConcurrency)
	fmt.Printf("Skipping directories named: %s\n", ignoreDir)
	fmt.Printf("Reading API key from: %s\n", apiKeyFile)

	// 读取API密钥
	apiKey, err := readAPIKey(apiKeyFile)
	if err != nil {
		fmt.Printf("❌ Failed to read API key: %v\n", err)
		os.Exit(1)
	}

	// 用于控制并发的信号量
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var lock sync.Mutex
	failedFiles := []string{}
	successCount := 0

	// 遍历目录
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录处理
		if info.IsDir() {
			if strings.Contains(path, ignoreDir) {
				return filepath.SkipDir
			}
			return nil
		}

		// 只处理Java文件
		ext := strings.ToLower(filepath.Ext(path))
		if ext == cFileExtension {
			wg.Add(1)
			go func(filePath string) {
				defer wg.Done()
				sem <- struct{}{} // 获取信号量槽位

				// 处理单个文件
				err := convertFile(filePath, apiKey)
				lock.Lock()
				if err != nil {
					fmt.Printf("\n❌ Failed: %s - %v\n", filePath, err)
					failedFiles = append(failedFiles, filePath)
				} else {
					fmt.Printf("\n✅ Converted: %s\n", filePath)
					successCount++
				}
				lock.Unlock()

				<-sem // 释放信号量槽位
			}(path)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking directory: %v\n", err)
	}

	wg.Wait() // 等待所有goroutine完成

	fmt.Println("\n================= Conversion Summary =================")
	fmt.Printf("Successfully converted: %d files\n", successCount)
	fmt.Printf("Total failed files: %d\n", len(failedFiles))
	if len(failedFiles) > 0 {
		fmt.Println("Failed files:")
		for i, f := range failedFiles {
			fmt.Printf("%d. %s\n", i+1, f)
		}
	} else {
		fmt.Println("✅ All files converted successfully!")
	}
}

func readAPIKey(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("opening API key file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading API key file: %w", err)
	}

	return "", fmt.Errorf("API key file is empty")
}

func convertFile(cFilePath, apiKey string) error {
	// 读取Java源代码文件
	cCode, err := ioutil.ReadFile(cFilePath)
	if err != nil {
		return fmt.Errorf("reading file failed: %w", err)
	}

	// 准备转换指令 - 要求保持类名、函数名、变量名不变
	prompt := fmt.Sprintf(`
You are an expert code translator. Convert the following Java code to idiomatic, efficient and modern Go code.
IMPORTANT: Preserve all original class names, function names, and variable names exactly as they are.
Do not add any explanations or comments outside the code. Only return the converted Go code.

Java source code:
%s

Go translated code (preserving all identifiers):
`, cCode)

	// 重试机制
	var result string
	for attempt := 0; attempt < maxRetries; attempt++ {
		// 准备API请求
		payload := RequestPayload{
			Model: modelName,
			Messages: []Message{
				{
					Role:    "user",
					Content: prompt,
				},
			},
			MaxTokens: 128000,
		}

		// 发送API请求
		response, err := sendDeepSeekRequest(payload, apiKey)
		if err != nil {
			if attempt == maxRetries-1 {
				return fmt.Errorf("API request failed: %w", err)
			}
			fmt.Printf("⚠️ Retrying %s (attempt %d/%d)\n", cFilePath, attempt+1, maxRetries)
			continue
		}

		// 处理API错误
		if response.Error.Message != "" {
			if attempt == maxRetries-1 {
				return fmt.Errorf("API error: %s", response.Error.Message)
			}
			fmt.Printf("⚠️ API error, retrying %s (attempt %d/%d): %s\n",
				cFilePath, attempt+1, maxRetries, response.Error.Message)
			continue
		}

		// 获取转换结果
		if len(response.Choices) > 0 {
			result = response.Choices[0].Message.Content
			break
		}
	}

	if result == "" {
		return fmt.Errorf("no choices returned after %d attempts", maxRetries)
	}

	// 提取纯代码部分（去除任何解释性文本）
	cleanCode := extractPureGoCode(result)
	if cleanCode == "" {
		return fmt.Errorf("unable to extract pure Go code from response")
	}

	// 创建目标Go文件路径
	goFilePath := strings.TrimSuffix(cFilePath, filepath.Ext(cFilePath)) + goFileExtension

	// 保存转换结果
	if err := ioutil.WriteFile(goFilePath, []byte(cleanCode), 0644); err != nil {
		return fmt.Errorf("writing output file failed: %w", err)
	}

	return nil
}

// 提取纯Go代码（去除任何解释性文本）
func extractPureGoCode(content string) string {
	// 尝试提取代码块（```go ... ```）
	codeBlockRegex := regexp.MustCompile("(?s)```go(.*?)```")
	if matches := codeBlockRegex.FindStringSubmatch(content); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// 如果找不到代码块，尝试提取代码部分（通常以package开头）
	packageIndex := strings.Index(content, "package ")
	if packageIndex != -1 {
		return content[packageIndex:]
	}

	// 如果都没有，返回整个内容
	return content
}

func sendDeepSeekRequest(payload RequestPayload, apiKey string) (*ResponsePayload, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling payload: %w", err)
	}

	req, err := http.NewRequest("POST", deepSeekAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var responseObj ResponsePayload
	if err := json.Unmarshal(body, &responseObj); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	return &responseObj, nil
}
