package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	deepSeekAPIURL  = "https://api.deepseek.com/v1/chat/completions"
	apiKey          = "sk-4449d931b66a4af988f446f7be0c4e7f" // 替换为您的DeepSeek API密钥
	modelName       = "deepseek-coder"                      // DeepSeek代码模型
	maxConcurrency  = 5                                     // 最大并发API请求数
	maxRetries      = 3                                     // 失败重试次数
	cFileExtension  = ".java"                               // 要处理的C文件扩展名
	hFileExtension  = ".h"                                  // 要处理的头文件扩展名
	goFileExtension = ".go"                                 // Go文件扩展名
	ignoreDir       = "vendor"                              // 要忽略的目录名
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RequestPayload struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type ResponsePayload struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func main() {

	sourceDir := "./Concentus"
	fmt.Printf("Converting C files in directory: %s\n", sourceDir)
	fmt.Printf("Maximum concurrent requests: %d\n", maxConcurrency)
	fmt.Printf("Skipping directories named: %s\n", ignoreDir)

	// 用于控制并发的信号量
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var lock sync.Mutex
	failedFiles := []string{}

	// 遍历目录
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
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

		// 只处理C文件
		ext := strings.ToLower(filepath.Ext(path))
		if ext == cFileExtension {
			wg.Add(1)
			go func(filePath string) {
				defer wg.Done()
				sem <- struct{}{} // 获取信号量槽位

				// 处理单个文件
				err := convertFile(filePath)
				lock.Lock()
				if err != nil {
					fmt.Printf("\n❌ Failed: %s - %v\n", filePath, err)
					failedFiles = append(failedFiles, filePath)
				} else {
					fmt.Printf("\n✅ Converted: %s\n", filePath)
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

func convertFile(cFilePath string) error {
	// 读取C源代码文件
	cCode, err := ioutil.ReadFile(cFilePath)
	if err != nil {
		return fmt.Errorf("reading file failed: %w", err)
	}

	// 准备转换指令
	prompt := fmt.Sprintf(`
You are an expert code translator.
Convert the following Java code to idiomatic, efficient and modern Go code.
Include detailed comments explaining key translation decisions.

Java source code:
%s

Go translated code:
`, cCode)

	// 重试机制
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
		}

		// 发送API请求
		response, err := sendDeepSeekRequest(payload)
		if err != nil {
			if attempt == maxRetries-1 {
				return fmt.Errorf("API request failed: %w", err)
			}
			continue
		}

		// 处理API错误
		if response.Error.Message != "" {
			if attempt == maxRetries-1 {
				return fmt.Errorf("API error: %s", response.Error.Message)
			}
			continue
		}

		// 获取转换结果
		if len(response.Choices) > 0 {
			result := response.Choices[0].Message.Content

			// 创建目标Go文件路径
			goFilePath := strings.TrimSuffix(cFilePath, filepath.Ext(cFilePath)) + goFileExtension

			// 保存转换结果
			if err := ioutil.WriteFile(goFilePath, []byte(result), 0644); err != nil {
				return fmt.Errorf("writing output file failed: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("no choices returned after %d attempts", maxRetries)
}

func sendDeepSeekRequest(payload RequestPayload) (*ResponsePayload, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", deepSeekAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status: %d", resp.StatusCode)
	}

	var responseObj ResponsePayload
	if err := json.Unmarshal(body, &responseObj); err != nil {
		return nil, err
	}

	return &responseObj, nil
}
