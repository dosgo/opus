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
	modelName       = "deepseek-reasoner"
	maxConcurrency  = 2
	maxRetries      = 3
	cFileExtension  = ".java"
	goFileExtension = ".go"
	ignoreDir       = "vendor"
	apiKeyFile      = "apikey.txt" // API密钥文件
	targetPackage   = "opus"       // 统一的目标包名
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RequestPayload struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens,omitempty"`
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
	fmt.Printf("Converting Java files in directory: %s\n", sourceDir)
	fmt.Printf("Maximum concurrent requests: %d\n", maxConcurrency)
	fmt.Printf("Skipping directories named: %s\n", ignoreDir)
	fmt.Printf("Reading API key from: %s\n", apiKeyFile)
	fmt.Printf("Setting package name to: %s\n", targetPackage)

	apiKey, err := readAPIKey(apiKeyFile)
	if err != nil {
		fmt.Printf("❌❌ Failed to read API key: %v\n", err)
		os.Exit(1)
	}

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var lock sync.Mutex
	failedFiles := []string{}
	successCount := 0

	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if strings.Contains(path, ignoreDir) {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == cFileExtension {
			wg.Add(1)
			go func(filePath string) {
				defer wg.Done()
				sem <- struct{}{}

				err := convertFile(filePath, apiKey)
				lock.Lock()
				if err != nil {
					fmt.Printf("\n❌❌ Failed: %s - %v\n", filePath, err)
					failedFiles = append(failedFiles, filePath)
				} else {
					fmt.Printf("\n✅ Converted: %s\n", filePath)
					successCount++
				}
				lock.Unlock()

				<-sem
			}(path)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking directory: %v\n", err)
	}

	wg.Wait()

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

func convertFile(javaFilePath, apiKey string) error {
	javaCode, err := ioutil.ReadFile(javaFilePath)
	if err != nil {
		return fmt.Errorf("reading file failed: %w", err)
	}

	prompt := fmt.Sprintf(`
You are an expert code translator. Convert the following Java code to idiomatic, efficient and modern Go code.
IMPORTANT: 
1. Preserve all original class names, function names, and variable names exactly as they are.
2. Do not add any explanations or comments outside the code. 
3. Only return the converted Go code.

Java source code:
%s

Go translated code (preserving all identifiers):
`, javaCode)

	var result string
	for attempt := 0; attempt < maxRetries; attempt++ {
		payload := RequestPayload{
			Model: modelName,
			Messages: []Message{
				{Role: "user", Content: prompt},
			},
			MaxTokens: 65536,
		}

		response, err := sendDeepSeekRequest(payload, apiKey)
		if err != nil {
			if attempt == maxRetries-1 {
				return fmt.Errorf("API request failed: %w", err)
			}
			fmt.Printf("⚠️ Retrying %s (attempt %d/%d)\n", javaFilePath, attempt+1, maxRetries)
			continue
		}

		if response.Error.Message != "" {
			if attempt == maxRetries-1 {
				return fmt.Errorf("API error: %s", response.Error.Message)
			}
			fmt.Printf("⚠️ API error, retrying %s (attempt %d/%d): %s\n",
				javaFilePath, attempt+1, maxRetries, response.Error.Message)
			continue
		}

		if len(response.Choices) > 0 {
			result = response.Choices[0].Message.Content
			break
		}
	}

	if result == "" {
		return fmt.Errorf("no choices returned after %d attempts", maxRetries)
	}

	cleanCode := extractPureGoCode(result)
	if cleanCode == "" {
		return fmt.Errorf("unable to extract pure Go code from response")
	}

	// 强制设置包名为opus
	cleanCode = setPackageName(cleanCode, targetPackage)

	goFilePath := strings.TrimSuffix(javaFilePath, filepath.Ext(javaFilePath)) + goFileExtension

	if err := ioutil.WriteFile(goFilePath, []byte(cleanCode), 0644); err != nil {
		return fmt.Errorf("writing output file failed: %w", err)
	}

	return nil
}

// 强制设置包名
func setPackageName(code, packageName string) string {
	// 匹配所有包声明语句
	packageRegex := regexp.MustCompile(`(?m)^\s*package\s+\w+\s*$`)

	// 如果已有包声明，替换为指定包名
	if packageRegex.MatchString(code) {
		return packageRegex.ReplaceAllString(code, "package "+packageName)
	}

	// 如果没有包声明，在文件开头添加
	return "package " + packageName + "\n\n" + code
}

// 提取纯Go代码
func extractPureGoCode(content string) string {
	codeBlockRegex := regexp.MustCompile("(?s)```go(.*?)```")
	if matches := codeBlockRegex.FindStringSubmatch(content); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	packageIndex := strings.Index(content, "package ")
	if packageIndex != -1 {
		return content[packageIndex:]
	}

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
