# GitSage

基于 AI 的 Git 提交信息生成器，分析你的暂存更改并生成符合 Conventional Commits 规范的语义化提交信息。

[English](README.md) | 简体中文

## 功能特性

- **AI 驱动**: 基于实际代码变更生成有意义的提交信息
- **多 AI 供应商**: 支持 OpenAI、DeepSeek 和本地 Ollama 模型
- **规范化提交**: 遵循 Conventional Commits 行业标准格式
- **交互式审查**: 提交前可审查、编辑或重新生成信息
- **智能 Diff 处理**: 自动分块处理大型 diff，排除 lock 文件
- **历史记录**: 保存生成的提交信息历史供参考
- **预览模式**: 预览信息而不实际提交
- **响应缓存**: 缓存 AI 响应，避免重复 API 调用
- **自动 PATH 配置**: 首次运行时自动检测并配置 PATH 环境变量

## 安装

### 使用 Go Install

```bash
go install github.com/gitsage/gitsage@latest
```

### 从源码编译

```bash
git clone https://github.com/gitsage/gitsage.git
cd gitsage
make build
```

编译后的二进制文件位于 `bin/gitsage`。

### 预编译二进制

从 [Releases](https://github.com/gitsage/gitsage/releases) 页面下载预编译的二进制文件。

支持的平台：
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

## 快速开始

1. **初始化配置**：
   ```bash
   gitsage config init
   ```

2. **设置 API 密钥**：
   ```bash
   gitsage config set provider.api_key sk-your-openai-key
   ```

3. **暂存更改并生成提交**：
   ```bash
   git add .
   gitsage
   ```

### 首次运行 PATH 检测

首次运行时，GitSage 会检查自身是否在系统 PATH 中。如果未找到，会提供自动添加选项：

- **Windows**: 通过 `setx` 命令添加到用户 PATH
- **macOS/Linux**: 在 shell 配置文件中添加 export 语句（`.bashrc`、`.zshrc`、`.config/fish/config.fish`）

你可以使用 `--skip-path-check` 参数跳过此检查，或稍后重置：
```bash
gitsage config set security.path_check_done false
```

## 使用方法

### 基本命令

```bash
# 生成并提交（默认操作）
gitsage

# 同上，显式命令
gitsage commit

# 仅生成不提交（预览模式）
gitsage generate

# 自动接受生成的信息
gitsage --yes

# 保存信息到文件
gitsage generate -o commit-msg.txt
```

### 配置命令

```bash
# 初始化配置文件
gitsage config init

# 设置配置值
gitsage config set <key> <value>

# 列出所有配置值
gitsage config list
```

### 历史命令

```bash
# 查看最近历史（默认 20 条）
gitsage history

# 查看指定数量的条目
gitsage history --limit 5

# 清除所有历史
gitsage history clear
```

## 命令参考

### `gitsage` / `gitsage commit`

生成提交信息并可选择提交。

| 参数 | 简写 | 说明 |
|------|------|------|
| `--dry-run` | | 仅生成信息不提交 |
| `--yes` | `-y` | 跳过交互确认 |
| `--output` | `-o` | 将信息写入文件（隐含 --dry-run） |
| `--no-cache` | | 绕过响应缓存 |

### `gitsage generate`

生成提交信息但不提交（等同于 `commit --dry-run`）。

| 参数 | 简写 | 说明 |
|------|------|------|
| `--yes` | `-y` | 跳过交互确认 |
| `--output` | `-o` | 将信息写入文件 |

### `gitsage config`

管理配置设置。

#### `gitsage config init`

在 `~/.gitsage/config.yaml` 创建新的配置文件，使用默认值。

#### `gitsage config set <key> <value>`

设置配置值。支持使用点号表示嵌套键。

示例：
```bash
gitsage config set provider.name openai
gitsage config set provider.api_key sk-xxx
gitsage config set provider.model gpt-4o-mini
gitsage config set git.diff_size_threshold 20480
```

#### `gitsage config list`

显示所有当前配置值（API 密钥会被遮蔽）。

### `gitsage history`

查看提交信息历史。

| 参数 | 简写 | 说明 |
|------|------|------|
| `--limit` | `-l` | 显示的条目数量（默认：20） |

#### `gitsage history clear`

删除所有历史条目。

### 全局参数

这些参数适用于所有命令：

| 参数 | 简写 | 说明 |
|------|------|------|
| `--verbose` | `-v` | 启用详细日志 |
| `--config` | | 自定义配置文件路径 |
| `--provider` | | 临时覆盖 AI 供应商 |
| `--model` | | 临时覆盖 AI 模型 |
| `--skip-path-check` | | 跳过 PATH 检测 |
| `--version` | | 显示版本信息 |
| `--help` | `-h` | 显示帮助 |

## 配置

配置存储在 `~/.gitsage/config.yaml`。使用 `gitsage config init` 创建。

### 配置文件结构

```yaml
provider:
  name: openai          # AI 供应商：openai, deepseek, ollama
  api_key: ""           # API 密钥（ollama 不需要）
  model: gpt-4o-mini    # 使用的模型
  endpoint: ""          # 自定义端点（可选）
  temperature: 0.2      # 响应创造性（0.0-1.0）
  max_tokens: 500       # 最大响应 token 数

git:
  diff_size_threshold: 10240  # 超过此大小的 diff 会分块（字节）
  exclude_patterns:           # 从 diff 中排除的文件
    - "*.lock"
    - "go.sum"
    - "package-lock.json"
    - "yarn.lock"
    - "pnpm-lock.yaml"
    - "Cargo.lock"

ui:
  editor: ""            # 编辑信息的编辑器（使用 $EDITOR）
  color_enabled: true   # 启用彩色输出
  spinner_style: dots   # 加载动画样式

history:
  enabled: true         # 启用历史记录
  max_entries: 1000     # 最大历史条目数
  file_path: ~/.gitsage/history.json

cache:
  enabled: true         # 启用响应缓存
  max_entries: 100      # 最大缓存条目数
  ttl_minutes: 60       # 缓存 TTL（分钟）

security:
  warning_acknowledged: false  # 首次使用安全警告标志
  path_check_done: false       # PATH 检测完成标志
```

### 配置优先级

值按以下顺序加载（优先级从高到低）：

1. 命令行参数（`--provider`、`--model`）
2. 环境变量（`GITSAGE_API_KEY` 等）
3. 配置文件（`~/.gitsage/config.yaml`）
4. 默认值

### 环境变量

| 变量 | 说明 |
|------|------|
| `GITSAGE_API_KEY` | AI 供应商的 API 密钥 |
| `GITSAGE_PROVIDER` | AI 供应商名称 |
| `GITSAGE_MODEL` | AI 模型名称 |
| `GITSAGE_SECURITY_PATH_CHECK_DONE` | 设置为 `true` 时跳过 PATH 检测 |

## AI 供应商

### OpenAI（默认）

```bash
gitsage config set provider.name openai
gitsage config set provider.api_key sk-your-key
gitsage config set provider.model gpt-4o-mini
```

支持的模型：`gpt-4o`、`gpt-4o-mini`、`gpt-4-turbo`、`gpt-3.5-turbo`

### DeepSeek

```bash
gitsage config set provider.name deepseek
gitsage config set provider.api_key your-deepseek-key
gitsage config set provider.model deepseek-chat
```

### Ollama（本地）

```bash
gitsage config set provider.name ollama
gitsage config set provider.model codellama
gitsage config set provider.endpoint http://localhost:11434
```

不需要 API 密钥。确保 Ollama 在本地运行。

## 故障排除

### "No staged changes found"（未找到暂存更改）

确保你已经用 `git add` 暂存了更改：
```bash
git add .
# 或
git add <specific-files>
```

### "Configuration not initialized"（配置未初始化）

先运行初始化命令：
```bash
gitsage config init
```

### "API key validation failed"（API 密钥验证失败）

检查你的 API 密钥是否正确设置：
```bash
gitsage config set provider.api_key your-key
gitsage config list  # 验证（密钥会被遮蔽）
```

### "AI provider API call failed"（AI 供应商 API 调用失败）

1. 检查网络连接
2. 验证 API 密钥是否有效
3. 检查供应商服务是否可用
4. 使用 `--verbose` 参数获取更多详情：
   ```bash
   gitsage --verbose
   ```

### "Request timeout"（请求超时）

默认超时时间为 30 秒。对于大型 diff 或慢速连接：
- 尝试使用较小的 diff
- 检查网络连接
- 考虑使用本地 Ollama 实例

### 大型 Diff

GitSage 自动处理大型 diff：
1. 排除 lock 文件（package-lock.json、go.sum 等）
2. 对超过 10KB 的 diff 进行分块
3. 对非常大的文件（>100KB）进行摘要

你可以调整阈值：
```bash
gitsage config set git.diff_size_threshold 20480  # 20KB
```

### 缓存问题

如果获取到过时的响应，绕过缓存：
```bash
gitsage --no-cache
```

或完全禁用缓存：
```bash
gitsage config set cache.enabled false
```

### 安装后找不到 PATH

如果安装后在 PATH 中找不到 GitSage：

1. **检查 PATH 检测是否运行**: 首次运行时，GitSage 应自动检测并提供添加到 PATH 的选项
2. **手动触发 PATH 检查**: 重置标志并重新运行：
   ```bash
   gitsage config set security.path_check_done false
   gitsage
   ```
3. **跳过自动检测**: 如果你更喜欢手动设置，使用 `--skip-path-check` 参数
4. **手动配置说明**:
   
   **Windows**:
   - 打开"系统属性" → "高级" → "环境变量"
   - 在"用户变量"中找到 PATH
   - 添加包含 `gitsage.exe` 的目录
   - 重启终端
   
   **macOS/Linux (Bash/Zsh)**:
   ```bash
   echo 'export PATH="$PATH:/path/to/gitsage"' >> ~/.bashrc  # 或 ~/.zshrc
   source ~/.bashrc
   ```
   
   **Fish**:
   ```bash
   echo 'set -gx PATH $PATH /path/to/gitsage' >> ~/.config/fish/config.fish
   source ~/.config/fish/config.fish
   ```

## 贡献

欢迎贡献！请遵循以下指南：

### 开发环境设置

1. 克隆仓库：
   ```bash
   git clone https://github.com/gitsage/gitsage.git
   cd gitsage
   ```

2. 安装依赖：
   ```bash
   make deps
   ```

3. 编译：
   ```bash
   make build
   ```

4. 运行测试：
   ```bash
   make test
   ```

### 代码风格

- 遵循标准 Go 规范
- 提交前运行 `make fmt`
- 运行 `make lint` 检查问题
- 运行 `make vet` 进行静态分析

### 测试

- 为新功能编写测试
- 确保所有测试通过：`make test`
- 检查覆盖率：`make test-coverage`

### Pull Request

1. Fork 仓库
2. 创建功能分支：`git checkout -b feature/my-feature`
3. 进行更改
4. 运行测试和 lint
5. 提交有意义的信息（使用 GitSage！）
6. 推送并创建 Pull Request

### 提交信息格式

我们遵循 Conventional Commits：

```
<type>(<scope>): <subject>

[可选正文]

[可选页脚]
```

类型：`feat`、`fix`、`docs`、`style`、`refactor`、`test`、`chore`、`perf`、`ci`、`build`、`revert`

## 许可证

MIT 许可证 - 详见 [LICENSE](LICENSE)。

## 致谢

- [Cobra](https://github.com/spf13/cobra) - CLI 框架
- [Viper](https://github.com/spf13/viper) - 配置管理
- [go-openai](https://github.com/sashabaranov/go-openai) - OpenAI 客户端
- [Charm](https://charm.sh/) - 终端 UI 库
