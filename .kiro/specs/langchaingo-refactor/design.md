# Design Document: LangChain Go AI Layer Refactor

## Overview

本设计文档描述了将 GitSage 的 AI 层从当前的多 provider 独立实现重构为基于 LangChain Go (`github.com/tmc/langchaingo`) 统一抽象的技术方案。

### 设计目标

1. **统一抽象**: 使用 LangChain Go 的 `llms.Model` 接口统一所有 LLM 调用
2. **保持兼容**: 现有的 `Provider` 接口、请求/响应结构保持不变
3. **简化代码**: 移除重复的 HTTP 客户端代码和重试逻辑
4. **易于扩展**: 添加新 provider 只需少量配置代码

### 核心变更

- 引入 `langchaingo` 作为 LLM 调用的统一层
- 使用 `langchaingo/prompts` 管理 prompt 模板
- 保留现有的错误处理和重试机制（包装 LangChain 调用）
- 保留现有的 commit message 解析逻辑

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Application Layer                         │
│                    (internal/app/service.go)                     │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                      AI Provider Interface                       │
│                   Provider.GenerateCommitMessage()               │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                    LangChain Adapter Layer                       │
│                      (internal/pkg/ai/)                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │   OpenAI    │  │   Ollama    │  │  DeepSeek (OpenAI API)  │  │
│  │  Provider   │  │  Provider   │  │       Provider          │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
│         │                │                      │                │
│         └────────────────┼──────────────────────┘                │
│                          ▼                                       │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │              LangChain LLM Wrapper                          ││
│  │         (统一的 llms.Model 调用 + 重试逻辑)                  ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                    LangChain Go Library                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐  │
│  │ llms/openai │  │ llms/ollama │  │   prompts package       │  │
│  └─────────────┘  └─────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## Components and Interfaces

### 1. 保留的接口 (不变)

```go
// Provider 接口保持不变
type Provider interface {
    GenerateCommitMessage(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
    Name() string
    ValidateConfig(config ProviderConfig) error
}

// GenerateRequest 保持不变
type GenerateRequest struct {
    DiffChunks      []git.DiffChunk
    DiffStats       *git.DiffStats
    CustomPrompt    string
    PreviousAttempt string
}

// GenerateResponse 保持不变
type GenerateResponse struct {
    Subject string
    Body    string
    Footer  string
    RawText string
}

// ProviderConfig 保持不变
type ProviderConfig struct {
    APIKey      string
    Model       string
    Endpoint    string
    Temperature float32
    MaxTokens   int
}
```

### 2. 新增的 LangChain 包装器

```go
// LangChainWrapper 封装 LangChain LLM 调用和重试逻辑
type LangChainWrapper struct {
    llm            llms.Model
    promptTemplate *LangChainPromptTemplate
    config         ProviderConfig
    providerName   string
}

// NewLangChainWrapper 创建新的包装器
func NewLangChainWrapper(llm llms.Model, config ProviderConfig, providerName string) *LangChainWrapper

// GenerateWithRetry 带重试的生成方法
func (w *LangChainWrapper) GenerateWithRetry(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
```

### 3. LangChain Prompt 模板

```go
// LangChainPromptTemplate 使用 LangChain prompts 包
type LangChainPromptTemplate struct {
    systemPrompt *prompts.PromptTemplate
    userPrompt   *prompts.PromptTemplate
}

// NewLangChainPromptTemplate 创建默认模板
func NewLangChainPromptTemplate() *LangChainPromptTemplate

// NewLangChainPromptTemplateWithCustom 创建自定义模板
func NewLangChainPromptTemplateWithCustom(systemPrompt, userPrompt string) *LangChainPromptTemplate

// RenderMessages 渲染为 LangChain 消息格式
func (pt *LangChainPromptTemplate) RenderMessages(data *PromptData) ([]llms.MessageContent, error)
```

### 4. Provider 实现简化

```go
// OpenAIProvider 使用 LangChain openai 包
type OpenAIProvider struct {
    wrapper *LangChainWrapper
}

func NewOpenAIProvider(config ProviderConfig) (*OpenAIProvider, error) {
    // 使用 langchaingo/llms/openai
    llm, err := openai.New(
        openai.WithToken(config.APIKey),
        openai.WithModel(config.Model),
        openai.WithBaseURL(config.Endpoint), // 如果有自定义 endpoint
    )
    if err != nil {
        return nil, err
    }
    
    wrapper := NewLangChainWrapper(llm, config, "openai")
    return &OpenAIProvider{wrapper: wrapper}, nil
}

// OllamaProvider 使用 LangChain ollama 包
type OllamaProvider struct {
    wrapper *LangChainWrapper
}

func NewOllamaProvider(config ProviderConfig) (*OllamaProvider, error) {
    // 使用 langchaingo/llms/ollama
    llm, err := ollama.New(
        ollama.WithModel(config.Model),
        ollama.WithServerURL(config.Endpoint),
    )
    if err != nil {
        return nil, err
    }
    
    wrapper := NewLangChainWrapper(llm, config, "ollama")
    return &OllamaProvider{wrapper: wrapper}, nil
}

// DeepSeekProvider 使用 OpenAI 兼容 API
type DeepSeekProvider struct {
    wrapper *LangChainWrapper
}

func NewDeepSeekProvider(config ProviderConfig) (*DeepSeekProvider, error) {
    endpoint := config.Endpoint
    if endpoint == "" {
        endpoint = "https://api.deepseek.com/v1"
    }
    
    // DeepSeek 使用 OpenAI 兼容 API
    llm, err := openai.New(
        openai.WithToken(config.APIKey),
        openai.WithModel(config.Model),
        openai.WithBaseURL(endpoint),
    )
    if err != nil {
        return nil, err
    }
    
    wrapper := NewLangChainWrapper(llm, config, "deepseek")
    return &DeepSeekProvider{wrapper: wrapper}, nil
}
```

## Data Models

### 消息格式转换

```go
// PromptData 保持不变，用于模板渲染
type PromptData struct {
    DiffStats        *git.DiffStats
    Chunks           []git.DiffChunk
    RequiresChunking bool
    PreviousAttempt  string
    CustomPrompt     string
}

// 转换为 LangChain 消息格式
func (pt *LangChainPromptTemplate) RenderMessages(data *PromptData) ([]llms.MessageContent, error) {
    // 渲染 system prompt
    systemContent, err := pt.systemPrompt.Format(nil)
    if err != nil {
        return nil, err
    }
    
    // 渲染 user prompt
    userContent, err := pt.userPrompt.Format(map[string]any{
        "DiffStats":        data.DiffStats,
        "Chunks":           data.Chunks,
        "RequiresChunking": data.RequiresChunking,
        "PreviousAttempt":  data.PreviousAttempt,
        "CustomPrompt":     data.CustomPrompt,
    })
    if err != nil {
        return nil, err
    }
    
    return []llms.MessageContent{
        llms.TextParts(llms.ChatMessageTypeSystem, systemContent),
        llms.TextParts(llms.ChatMessageTypeHuman, userContent),
    }, nil
}
```

### 依赖更新

```go
// go.mod 新增依赖
require (
    github.com/tmc/langchaingo v0.1.x
)

// 移除的依赖
// github.com/sashabaranov/go-openai (被 langchaingo 替代)
```

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system-essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Output Format Consistency

*For any* valid `GenerateRequest` input, the `GenerateResponse` returned by the refactored system SHALL have the same structure (Subject, Body, Footer, RawText fields) and the same parsing behavior as the original implementation.

**Validates: Requirements 1.4, 6.3, 6.4**

### Property 2: Prompt Rendering Equivalence

*For any* `PromptData` input (including custom prompts), the rendered prompt content from the LangChain template system SHALL be semantically equivalent to the original Go template rendering.

**Validates: Requirements 3.4, 3.5**

### Property 3: Provider Factory Correctness

*For any* valid provider name ("openai", "deepseek", "ollama") and configuration, the `NewProvider` factory SHALL create the correct provider type with the specified configuration (endpoint, API key, model).

**Validates: Requirements 5.2, 5.4, 5.5**

### Property 4: Error Wrapping Consistency

*For any* error returned by the LangChain LLM call, the error wrapping logic SHALL produce user-friendly error messages consistent with the original implementation's error handling.

**Validates: Requirements 4.3**

## Error Handling

### 重试机制

```go
func (w *LangChainWrapper) GenerateWithRetry(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
    var lastErr error
    
    for attempt := 0; attempt < MaxRetries; attempt++ {
        resp, err := w.generate(ctx, req)
        if err == nil {
            return resp, nil
        }
        
        lastErr = err
        
        // 检查是否可重试
        if !w.isRetryableError(err) {
            return nil, w.wrapError(err)
        }
        
        // 指数退避
        delay := calculateBackoff(attempt)
        apperrors.LogRetry(attempt+1, MaxRetries, err, delay)
        
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-time.After(delay):
            continue
        }
    }
    
    return nil, w.wrapError(lastErr)
}
```

### 错误类型映射

```go
func (w *LangChainWrapper) wrapError(err error) error {
    if err == nil {
        return nil
    }
    
    errStr := err.Error()
    
    // 认证错误
    if strings.Contains(errStr, "401") || strings.Contains(errStr, "unauthorized") {
        return apperrors.NewAuthenticationError(w.providerName)
    }
    
    // 速率限制
    if strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit") {
        return apperrors.NewRateLimitError(60 * time.Second)
    }
    
    // 超时
    if errors.Is(err, context.DeadlineExceeded) {
        return apperrors.NewTimeoutError(err)
    }
    
    // 连接错误 (Ollama 特有)
    if strings.Contains(errStr, "connection refused") {
        appErr := apperrors.NewNetworkError(err)
        appErr.Message = fmt.Sprintf("cannot connect to %s", w.providerName)
        if w.providerName == "ollama" {
            appErr.WithSuggestion("Please ensure Ollama is running using 'ollama serve'")
        }
        return appErr
    }
    
    return apperrors.NewAIProviderError(w.providerName, err)
}
```

## Testing Strategy

### 单元测试

1. **接口兼容性测试**: 验证 `Provider` 接口、`GenerateRequest`、`GenerateResponse` 结构不变
2. **工厂函数测试**: 验证 `NewProvider` 对各 provider 的创建逻辑
3. **Prompt 模板测试**: 验证 LangChain 模板渲染结果与原实现一致
4. **错误处理测试**: 验证各类错误的包装和用户友好消息

### Property-Based Tests

使用 `github.com/leanovate/gopter` 进行属性测试：

1. **Property 1 测试**: 生成随机 `GenerateRequest`，验证输出格式一致性
2. **Property 2 测试**: 生成随机 `PromptData`，验证模板渲染等价性
3. **Property 3 测试**: 生成随机配置，验证工厂创建正确性
4. **Property 4 测试**: 生成随机错误类型，验证错误包装一致性

### 集成测试

1. **Mock LLM 测试**: 使用 LangChain 的 fake LLM 进行端到端测试
2. **真实 API 测试**: 可选的真实 API 调用测试（需要 API key）

### 测试配置

```go
// 每个属性测试至少运行 100 次迭代
const PropertyTestIterations = 100

// 测试标签格式
// Feature: langchaingo-refactor, Property 1: Output Format Consistency
```
