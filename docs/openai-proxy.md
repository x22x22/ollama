# Example: Using OpenAI-Compatible Remote Endpoints with Ollama

This guide demonstrates how to configure Ollama to proxy requests to OpenAI-compatible remote endpoints such as Alibaba DashScope, Azure OpenAI, or other compatible services.

## Overview

Ollama can now forward API requests (including `/v1/responses` and `/v1/chat/completions`) to remote OpenAI-compatible endpoints. This allows you to:
- Use OpenAI-compatible services without changing your client code
- Leverage remote model services that only support OpenAI's API format
- Switch between local and remote models transparently

## Configuration

### Method 1: Using a Modelfile

Create a Modelfile that references a remote OpenAI-compatible endpoint:

```modelfile
# Example Modelfile for Alibaba DashScope
FROM qwen-turbo
```

Then use the `ollama create` command with remote host and API key:

```bash
# Add the remote host to allowed remotes
export OLLAMA_REMOTES="dashscope.aliyuncs.com"

# Create a model pointing to DashScope
curl http://localhost:11434/api/create -d '{
  "model": "dashscope-qwen",
  "from": "qwen-turbo",
  "remote_host": "https://dashscope.aliyuncs.com/compatible-mode/v1",
  "remote_api_key": "sk-98e55d42763e4e2fa9253e35783aba08"
}'
```

### Method 2: Using the API Directly

You can also create models programmatically:

```python
import requests

url = "http://localhost:11434/api/create"
data = {
    "model": "my-remote-model",
    "from": "gpt-4",
    "remote_host": "https://dashscope.aliyuncs.com/compatible-mode/v1",
    "remote_api_key": "your-api-key-here"
}

response = requests.post(url, json=data, stream=True)
for line in response.iter_lines():
    if line:
        print(line.decode())
```

## Using the Remote Model

Once configured, you can use the model with any Ollama-compatible client:

### Using the Responses API

```bash
curl http://localhost:11434/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "dashscope-qwen",
    "input": "Write a short poem about the color blue"
  }'
```

### Using the Chat Completions API

```bash
curl http://localhost:11434/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "dashscope-qwen",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

### Using the OpenAI Python Client

```python
from openai import OpenAI

client = OpenAI(
    base_url='http://localhost:11434/v1/',
    api_key='ollama',  # required but ignored for remote models
)

# Responses API
response = client.responses.create(
    model="dashscope-qwen",
    input="Write a short poem about the color blue"
)
print(response.output_text)

# Chat Completions API
chat_response = client.chat.completions.create(
    model="dashscope-qwen",
    messages=[
        {"role": "user", "content": "Hello!"}
    ]
)
print(chat_response.choices[0].message.content)
```

## How It Works

1. **Endpoint Detection**: Ollama automatically detects OpenAI-compatible endpoints by checking if the URL doesn't contain `/api/` path segments
2. **Request Conversion**: Internal API requests are automatically converted to OpenAI's format
3. **API Authentication**: The `remote_api_key` is sent as a Bearer token in the Authorization header
4. **Response Conversion**: OpenAI responses are converted back to Ollama's internal format
5. **Streaming Support**: Both streaming and non-streaming modes are fully supported

## Supported Features

- ✅ Text generation
- ✅ Streaming responses
- ✅ Tool calling (function calling)
- ✅ Thinking/reasoning (for compatible models)
- ✅ Vision (image inputs)
- ✅ Temperature, top_p, max_tokens control
- ✅ System prompts and conversation history

## Supported Providers

Any OpenAI-compatible service should work. Tested with:
- Alibaba DashScope
- Azure OpenAI
- Other providers with `/v1/chat/completions` endpoints

## Environment Variables

```bash
# Allow remote hosts (comma-separated list of hostnames)
export OLLAMA_REMOTES="dashscope.aliyuncs.com,api.openai.com,your-provider.com"
```

## Security Notes

- API keys are stored in the model configuration and used only for authentication with the remote service
- API keys are never exposed through the Ollama API responses
- Use HTTPS URLs for remote endpoints to ensure API keys are transmitted securely
- Only hostnames listed in `OLLAMA_REMOTES` are allowed for security

## Troubleshooting

### "this server cannot run this remote model"
Make sure the hostname is added to `OLLAMA_REMOTES`:
```bash
export OLLAMA_REMOTES="dashscope.aliyuncs.com"
```

### Authentication Errors
Verify your API key is correct and has the necessary permissions for the remote service.

### Connection Timeouts
Check network connectivity to the remote endpoint and ensure the URL is correct.

## Examples

### Alibaba DashScope

```bash
export OLLAMA_REMOTES="dashscope.aliyuncs.com"

curl http://localhost:11434/api/create -d '{
  "model": "qwen-turbo",
  "from": "qwen-turbo",
  "remote_host": "https://dashscope.aliyuncs.com/compatible-mode/v1",
  "remote_api_key": "sk-your-api-key"
}'
```

### Azure OpenAI

```bash
export OLLAMA_REMOTES="your-resource.openai.azure.com"

curl http://localhost:11434/api/create -d '{
  "model": "gpt-4",
  "from": "gpt-4",
  "remote_host": "https://your-resource.openai.azure.com/openai/deployments/gpt-4",
  "remote_api_key": "your-azure-api-key"
}'
```

## Limitations

- Stateful conversations (using `conversation` field) are not supported
- Some provider-specific features may not be available
- Model capabilities depend on the remote provider
