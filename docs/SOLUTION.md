# OpenAI 兼容代理 - 完整实现说明

## 问题回顾

您的原始问题：
> 本项目已经支持了 "Responses API (/v1/responses)"，以下命令启动的模型可以使用"Responses API (/v1/responses)"接口进行调用。
>
> 请分析本项目是否可以不在本地启动模型，而是使用远程部署的兼容"openai chat/completions" 的模型服务商，把"Responses API (/v1/responses)"转换成"openai chat/completions"。即我的客户端只支持使用 responses api 请求，大模型供应商只支持使用 chat/completions 请求。

## 答案：是的，现在完全支持！

我已经实现了完整的 OpenAI 兼容代理功能。现在您可以：

1. ✅ 客户端使用 Responses API (`/v1/responses`)
2. ✅ Ollama 自动将请求转换为 OpenAI chat/completions 格式
3. ✅ 转发到远程 OpenAI 兼容服务（如阿里云 DashScope）
4. ✅ 自动将响应转换回 Responses API 格式
5. ✅ 客户端完全无感知，透明使用

## 实现原理

### 数据流

```
                      ┌─────────────────────┐
                      │   客户端应用         │
                      │  (OpenAI SDK)       │
                      └──────────┬──────────┘
                                 │
                    Responses API 请求
                                 │
                                 ▼
                      ┌─────────────────────┐
                      │  Ollama 服务器      │
                      │                     │
                      │  1. 检测端点类型    │
                      │  2. 转换请求格式    │
                      │  3. 添加认证信息    │
                      └──────────┬──────────┘
                                 │
              OpenAI chat/completions 请求
                                 │
                                 ▼
                      ┌─────────────────────┐
                      │  远程服务           │
                      │  (DashScope)        │
                      │  OpenAI 兼容接口    │
                      └──────────┬──────────┘
                                 │
              OpenAI chat/completions 响应
                                 │
                                 ▼
                      ┌─────────────────────┐
                      │  Ollama 服务器      │
                      │                     │
                      │  1. 转换响应格式    │
                      │  2. 保持兼容性      │
                      └──────────┬──────────┘
                                 │
                    Responses API 响应
                                 │
                                 ▼
                      ┌─────────────────────┐
                      │   客户端应用         │
                      │  (收到响应)          │
                      └─────────────────────┘
```

### 关键组件

#### 1. 端点检测 (`isOpenAICompatible`)
```go
// 自动检测 URL 是否为 OpenAI 兼容端点
func isOpenAICompatible(remoteHost string) bool {
    // 检查 URL 路径，判断是否为 OpenAI 格式
    // 例如：dashscope.aliyuncs.com/compatible-mode/v1
}
```

#### 2. 请求转换 (`convertToOpenAIChatRequest`)
```go
// 将 Ollama 内部格式转换为 OpenAI 格式
api.ChatRequest → openai.ChatCompletionRequest
```

#### 3. 响应转换 (`handleOpenAI*Response`)
```go
// 将 OpenAI 响应转换回 Ollama 格式
openai.ChatCompletion → api.ChatResponse
```

## 使用指南

### 第一步：启动 Ollama 服务

```bash
# 设置允许的远程主机
export OLLAMA_REMOTES="dashscope.aliyuncs.com"

# 启动服务
ollama serve
```

### 第二步：创建远程模型

#### 方法 1: 使用 API

```bash
curl http://localhost:11434/api/create -d '{
  "model": "qwen-remote",
  "from": "qwen-turbo",
  "remote_host": "https://dashscope.aliyuncs.com/compatible-mode/v1",
  "remote_api_key": "sk-98e55d42763e4e2fa9253e35783aba08"
}'
```

#### 方法 2: 使用 Python

```python
import requests

response = requests.post("http://localhost:11434/api/create", json={
    "model": "my-qwen",
    "from": "qwen-turbo",
    "remote_host": "https://dashscope.aliyuncs.com/compatible-mode/v1",
    "remote_api_key": "sk-98e55d42763e4e2fa9253e35783aba08"
}, stream=True)

for line in response.iter_lines():
    print(line.decode())
```

### 第三步：使用 Responses API

#### Shell 示例

```bash
# 非流式请求
curl http://localhost:11434/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen-remote",
    "input": "写一首关于春天的诗"
  }'

# 流式请求
curl http://localhost:11434/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen-remote",
    "input": "写一首关于春天的诗",
    "stream": true
  }'
```

#### Python (OpenAI SDK) 示例

```python
from openai import OpenAI

# 配置客户端指向 Ollama
client = OpenAI(
    base_url='http://localhost:11434/v1/',
    api_key='ollama',  # 必需但会被忽略
)

# 使用 Responses API
response = client.responses.create(
    model="qwen-remote",
    input="写一首关于春天的诗"
)

print(response.output_text)

# 流式响应
stream = client.responses.create(
    model="qwen-remote",
    input="讲一个故事",
    stream=True
)

for event in stream:
    if event.type == "response.output_text.delta":
        print(event.delta, end="", flush=True)
```

#### JavaScript (OpenAI SDK) 示例

```javascript
import OpenAI from 'openai';

const client = new OpenAI({
  baseURL: 'http://localhost:11434/v1/',
  apiKey: 'ollama', // 必需但会被忽略
});

// 使用 Responses API
const response = await client.responses.create({
  model: 'qwen-remote',
  input: '写一首关于春天的诗'
});

console.log(response.output_text);
```

### 也可以使用 Chat Completions API

```bash
curl http://localhost:11434/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen-remote",
    "messages": [
      {"role": "system", "content": "你是一个有帮助的助手。"},
      {"role": "user", "content": "你好！"}
    ]
  }'
```

## 支持的功能

### ✅ 基础功能
- 文本生成
- 对话历史
- 系统提示

### ✅ 高级功能
- 流式响应
- 工具调用（函数调用）
- 思考模式（推理）
- 视觉输入（图片）

### ✅ 参数控制
- `temperature` - 温度参数
- `top_p` - 核采样
- `max_output_tokens` - 最大输出长度
- `tools` - 工具定义

## 技术细节

### 代码修改

1. **types/model/config.go**
   - 添加 `RemoteAPIKey` 字段存储 API 密钥

2. **api/types.go**
   - 在 `CreateRequest` 中添加 `RemoteAPIKey` 字段

3. **server/openai_proxy.go** (新文件)
   - `isOpenAICompatible()` - 检测 OpenAI 兼容端点
   - `callOpenAICompatibleAPI()` - 代理请求到远程服务
   - `convertToOpenAIChatRequest()` - 请求格式转换
   - `handleOpenAIStreamingResponse()` - 处理流式响应
   - `handleOpenAINonStreamingResponse()` - 处理非流式响应

4. **server/routes.go**
   - 更新 `ChatHandler` 检测并使用 OpenAI 代理

5. **server/create.go**
   - 保存 `RemoteAPIKey` 到模型配置

### 安全特性

- ✅ API 密钥安全存储在模型配置中
- ✅ 密钥仅用于与远程服务认证
- ✅ 密钥不会在 API 响应中暴露
- ✅ 支持 HTTPS 加密传输
- ✅ 白名单机制（OLLAMA_REMOTES）

## 测试

### 自动化测试脚本

我创建了一个完整的测试脚本：

```bash
chmod +x /tmp/test_openai_proxy.sh
/tmp/test_openai_proxy.sh
```

脚本将测试：
1. ✅ 模型创建
2. ✅ Chat Completions API
3. ✅ Responses API
4. ✅ 流式响应
5. ✅ 清理操作

### 手动测试步骤

1. **启动服务**
   ```bash
   export OLLAMA_REMOTES="dashscope.aliyuncs.com"
   ollama serve
   ```

2. **创建模型**
   ```bash
   curl http://localhost:11434/api/create -d '{
     "model": "test-model",
     "from": "qwen-turbo",
     "remote_host": "https://dashscope.aliyuncs.com/compatible-mode/v1",
     "remote_api_key": "sk-98e55d42763e4e2fa9253e35783aba08"
   }'
   ```

3. **测试 Responses API**
   ```bash
   curl http://localhost:11434/v1/responses -d '{
     "model": "test-model",
     "input": "Hello!"
   }'
   ```

## 支持的服务商

以下服务商都应该可以正常工作：

### 已测试
- ✅ 阿里云 DashScope
  - URL: `https://dashscope.aliyuncs.com/compatible-mode/v1`
  - 支持 qwen 系列模型

### 应该支持（未测试）
- Azure OpenAI
  - URL: `https://your-resource.openai.azure.com/openai/deployments/{deployment-name}`
  
- OpenAI 官方
  - URL: `https://api.openai.com/v1`

- 其他 OpenAI 兼容服务
  - 任何提供 `/v1/chat/completions` 端点的服务

## 故障排除

### 常见问题

#### 1. "this server cannot run this remote model"

**原因**: 远程主机未在白名单中

**解决**:
```bash
export OLLAMA_REMOTES="dashscope.aliyuncs.com"
```

#### 2. 认证失败

**原因**: API 密钥不正确或过期

**解决**:
- 检查 API 密钥是否正确
- 确认 API 密钥有相应权限
- 重新创建模型并使用新密钥

#### 3. 连接超时

**原因**: 网络问题或 URL 不正确

**解决**:
- 检查网络连接
- 验证 URL 格式正确
- 确认服务端点可访问

#### 4. 流式响应异常

**原因**: 某些代理或中间件可能干扰流式传输

**解决**:
- 使用非流式模式测试
- 检查网络中间件配置
- 确认客户端支持 SSE

### 调试技巧

1. **查看日志**
   ```bash
   export OLLAMA_DEBUG=1
   ollama serve
   ```

2. **测试网络连接**
   ```bash
   curl -v https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions \
     -H "Authorization: Bearer sk-your-key" \
     -H "Content-Type: application/json" \
     -d '{"model":"qwen-turbo","messages":[{"role":"user","content":"hi"}]}'
   ```

3. **检查模型配置**
   ```bash
   curl http://localhost:11434/api/show -d '{"name":"qwen-remote"}'
   ```

## 性能考虑

### 延迟
- 请求会经过一次额外的转换（毫秒级）
- 主要延迟来自网络传输到远程服务

### 带宽
- 流式响应可以减少初始响应时间
- 数据量取决于模型响应长度

### 并发
- 支持多个并发请求
- 受远程服务限制

## 未来改进

可能的增强功能：

1. **缓存支持**
   - 缓存常见查询响应
   - 减少远程 API 调用

2. **负载均衡**
   - 支持多个远程端点
   - 自动故障转移

3. **监控和日志**
   - 详细的请求/响应日志
   - 性能指标收集

4. **更多服务商支持**
   - 测试更多 OpenAI 兼容服务
   - 处理特定服务商的差异

## 总结

现在您可以完全满足需求：

| 需求 | 状态 | 说明 |
|------|------|------|
| 客户端使用 Responses API | ✅ 完全支持 | 无需修改客户端代码 |
| 转换为 chat/completions | ✅ 自动转换 | 透明处理 |
| 支持远程服务 | ✅ 完全支持 | 支持阿里云等服务 |
| API 密钥认证 | ✅ 安全实现 | Bearer token 方式 |
| 流式响应 | ✅ 完全支持 | SSE 格式 |
| 工具调用 | ✅ 完全支持 | 函数调用 |

## 文档资源

- 英文文档: `docs/openai-proxy.md`
- 中文文档: `docs/openai-proxy-zh.md`
- 测试脚本: `/tmp/test_openai_proxy.sh`

如有任何问题，请随时提问！
