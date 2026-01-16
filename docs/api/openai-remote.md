# Using OpenAI-Compatible Remote APIs with Ollama

Ollama can now act as a proxy to OpenAI-compatible remote model providers, allowing you to use Ollama's API (including the Responses API) with remote services that only support OpenAI's Chat Completions API.

## Overview

This feature enables the following workflow:
- Your client uses Ollama's Responses API (`/v1/responses`) 
- Ollama converts the request to OpenAI's Chat Completions format
- Ollama forwards the request to a remote OpenAI-compatible API
- The remote API processes the request
- Ollama converts the response back to Responses API format

## Creating a Remote Model

To create a model that points to a remote OpenAI-compatible API, use the `/api/create` endpoint:

### Example: DashScope (Alibaba Cloud)

```bash
curl http://localhost:11434/api/create -d '{
  "name": "dashscope-model",
  "from": "qwen-plus",
  "remote_host": "https://dashscope.aliyuncs.com/compatible-mode/v1",
  "remote_type": "openai",
  "remote_api_key": "sk-your-api-key-here"
}'
```

### Parameters

- `name`: The local name for your model
- `from`: The remote model identifier (e.g., "qwen-plus", "gpt-4", etc.)
- `remote_host`: The base URL of the OpenAI-compatible API
- `remote_type`: Set to `"openai"` for OpenAI-compatible APIs (defaults to `"ollama"`)
- `remote_api_key`: Your API key for the remote service

## Using the Remote Model

Once created, you can use the model with any Ollama API endpoint:

### Responses API

```bash
curl http://localhost:11434/v1/responses -d '{
  "model": "dashscope-model",
  "input": "Write a short poem about the color blue"
}'
```

### Chat Completions API

```bash
curl http://localhost:11434/v1/chat/completions -d '{
  "model": "dashscope-model",
  "messages": [
    {"role": "user", "content": "Hello, how are you?"}
  ]
}'
```

### Python Example

```python
from openai import OpenAI

# Point to your local Ollama instance
client = OpenAI(
    base_url='http://localhost:11434/v1/',
    api_key='ollama'  # required but ignored
)

# Use the Responses API
response = client.responses.create(
    model='dashscope-model',
    input='Explain quantum computing in simple terms'
)
print(response.output_text)

# Or use the Chat Completions API
chat_response = client.chat.completions.create(
    model='dashscope-model',
    messages=[
        {'role': 'user', 'content': 'What is the capital of France?'}
    ]
)
print(chat_response.choices[0].message.content)
```

## Supported Remote Providers

Any OpenAI-compatible API should work, including:

- **DashScope** (Alibaba Cloud): `https://dashscope.aliyuncs.com/compatible-mode/v1`
- **Azure OpenAI**: `https://your-resource.openai.azure.com/openai/deployments/your-deployment`
- **OpenAI**: `https://api.openai.com/v1`
- **Any other OpenAI-compatible endpoint**

## Features

### Supported
- ✅ Chat completions (streaming and non-streaming)
- ✅ Responses API (streaming and non-streaming)
- ✅ Tool calling / Function calling
- ✅ Thinking/Reasoning models
- ✅ Temperature, top_p, and other parameters
- ✅ JSON schema output formatting
- ✅ Token usage tracking

### Not Supported
- ❌ `/api/generate` endpoint (OpenAI doesn't have an equivalent completions API in the same format)
- ❌ Image inputs (depends on remote provider support)

## Security Notes

1. **API Keys**: API keys are stored in the model configuration. Ensure your Ollama instance is properly secured.
2. **HTTPS**: Always use HTTPS URLs for remote hosts to protect your API keys in transit.
3. **Firewall**: The Ollama server will make outbound HTTPS requests to the remote API.

## Limitations

- Remote models don't support local model operations like `ollama pull`, `ollama push`, or `ollama rm` on the remote model itself
- The `/api/generate` endpoint is not supported for OpenAI-compatible remotes (use `/api/chat` or `/v1/chat/completions` instead)
- Some model-specific features may not be available depending on the remote provider

## Troubleshooting

### Error: "this server cannot run this remote model"

This error occurs when using the default Ollama remote mode. Make sure you set `"remote_type": "openai"` when creating the model.

### Error: "API error (401): ..."

Check that your `remote_api_key` is correct and has the necessary permissions for the remote API.

### Error: "API error (404): ..."

Verify that:
1. The `remote_host` URL is correct
2. The model name in the `from` field is valid for the remote provider
3. The remote provider's API is accessible from your network
