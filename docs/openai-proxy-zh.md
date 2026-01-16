# OpenAI兼容代理支持 - 解决方案说明

## 问题分析

根据您的需求，您希望：
1. 客户端使用 Responses API (`/v1/responses`) 进行请求
2. 大模型供应商只支持 OpenAI chat/completions 接口
3. Ollama 需要将 Responses API 请求转换为 chat/completions 请求，并转发到远程OpenAI兼容服务

## 解决方案

我已经实现了对 OpenAI 兼容远程端点的完整支持。现在 Ollama 可以：
- 自动检测 OpenAI 兼容端点
- 将内部请求格式转换为 OpenAI 格式
- 将 OpenAI 响应转换回 Ollama 格式
- 支持 API key 认证
- 支持流式和非流式响应

## 实现的功能

### 1. 模型配置增强
- 新增 `RemoteAPIKey` 字段用于存储 API 密钥
- 支持通过 Modelfile 或 API 配置远程模型

### 2. 自动端点检测
- 自动识别 OpenAI 兼容端点（基于 URL 路径分析）
- 传统 Ollama 端点和 OpenAI 兼容端点都支持

### 3. 请求转换
- 将 `api.ChatRequest` 转换为 `openai.ChatCompletionRequest`
- 支持消息、工具调用、图片、思考等所有特性

### 4. 响应转换
- 将 OpenAI 响应转换回 `api.ChatResponse`
- 保持与原始 Ollama API 的完全兼容

## 使用方法

### 步骤 1: 配置环境变量

```bash
# 允许访问阿里云 DashScope
export OLLAMA_REMOTES="dashscope.aliyuncs.com"

# 启动 Ollama 服务
ollama serve
```

### 步骤 2: 创建指向远程服务的模型

```bash
curl http://localhost:11434/api/create -d '{
  "model": "qwen-remote",
  "from": "qwen-turbo",
  "remote_host": "https://dashscope.aliyuncs.com/compatible-mode/v1",
  "remote_api_key": "sk-98e55d42763e4e2fa9253e35783aba08"
}'
```

### 步骤 3: 使用 Responses API

现在您可以使用 Responses API，它会自动转发到 DashScope 的 chat/completions 接口：

```bash
curl http://localhost:11434/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen-remote",
    "input": "写一首关于蓝色的短诗"
  }'
```

或者使用 OpenAI Python SDK：

```python
from openai import OpenAI

client = OpenAI(
    base_url='http://localhost:11434/v1/',
    api_key='ollama',  # 必需但会被忽略
)

response = client.responses.create(
    model="qwen-remote",
    input="写一首关于蓝色的短诗"
)
print(response.output_text)
```

### 步骤 4: 也可以使用 Chat Completions API

```bash
curl http://localhost:11434/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen-remote",
    "messages": [
      {"role": "user", "content": "你好！"}
    ]
  }'
```

## 工作原理

```
客户端 (Responses API)
    ↓
Ollama 服务器
    ├─ 检测 OpenAI 兼容端点
    ├─ 转换请求格式
    ├─ 添加 API Key 认证
    ↓
远程服务 (DashScope chat/completions)
    ↓
Ollama 服务器
    ├─ 转换响应格式
    ↓
客户端 (收到 Responses 格式响应)
```

## 支持的功能

- ✅ 文本生成
- ✅ 流式响应
- ✅ 工具调用（函数调用）
- ✅ 思考/推理（支持的模型）
- ✅ 视觉（图片输入）
- ✅ 温度、top_p、max_tokens 控制
- ✅ 系统提示和对话历史

## 测试您的配置

我已经创建了一个测试脚本，您可以使用它来验证配置：

```bash
chmod +x /tmp/test_openai_proxy.sh
/tmp/test_openai_proxy.sh
```

该脚本将：
1. 创建使用 DashScope 的模型
2. 测试 chat/completions 端点
3. 测试 responses 端点
4. 测试流式响应
5. 清理测试模型

## 代码修改说明

### 文件修改：
1. **types/model/config.go**: 添加 `RemoteAPIKey` 字段
2. **api/types.go**: 在 `CreateRequest` 中添加 `RemoteAPIKey` 字段
3. **server/openai_proxy.go**: 新文件，实现 OpenAI 代理逻辑
4. **server/routes.go**: 更新 `ChatHandler` 以支持 OpenAI 代理
5. **server/create.go**: 更新模型创建逻辑以保存 API key

### 核心函数：
- `isOpenAICompatible()`: 检测 OpenAI 兼容端点
- `callOpenAICompatibleAPI()`: 转发请求到 OpenAI 兼容服务
- `convertToOpenAIChatRequest()`: 请求格式转换
- `handleOpenAIStreamingResponse()`: 处理流式响应
- `handleOpenAINonStreamingResponse()`: 处理非流式响应

## 安全说明

- API 密钥存储在模型配置中
- API 密钥仅用于与远程服务认证
- API 密钥不会在 Ollama API 响应中暴露
- 建议使用 HTTPS URL 确保安全传输
- 只有 `OLLAMA_REMOTES` 中列出的主机名才被允许访问

## 故障排除

### 错误: "this server cannot run this remote model"
确保主机名添加到 `OLLAMA_REMOTES`：
```bash
export OLLAMA_REMOTES="dashscope.aliyuncs.com"
```

### 认证错误
验证 API key 是否正确且具有必要权限。

### 连接超时
检查网络连接和 URL 是否正确。

## 其他支持的服务商

除了阿里云 DashScope，以下服务商也应该可以工作：
- Azure OpenAI
- 其他支持 `/v1/chat/completions` 端点的服务
- 任何 OpenAI 兼容的 API

## 总结

现在您可以：
1. ✅ 使用 Responses API 访问远程 OpenAI 兼容服务
2. ✅ 无需修改客户端代码
3. ✅ 透明地在本地和远程模型之间切换
4. ✅ 支持所有主要功能（流式、工具调用等）

如果您有任何问题或需要进一步的帮助，请随时告诉我！
