# Example: Using DashScope with Ollama's Responses API

This example demonstrates how to use Alibaba Cloud's DashScope API through Ollama's Responses API.

## Prerequisites

1. Ollama server running locally (or accessible via network)
2. A DashScope API key from [Alibaba Cloud](https://dashscope.aliyuncs.com/)

## Step 1: Create the Remote Model

```bash
curl http://localhost:11434/api/create -d '{
  "name": "dashscope-qwen",
  "from": "qwen-plus",
  "remote_host": "https://dashscope.aliyuncs.com/compatible-mode/v1",
  "remote_type": "openai",
  "remote_api_key": "sk-your-api-key-here"
}'
```

## Step 2: Use the Responses API

```bash
curl http://localhost:11434/v1/responses -d '{
  "model": "dashscope-qwen",
  "input": "Write a haiku about programming"
}'
```

## Step 3: Python Example

```python
from openai import OpenAI

client = OpenAI(
    base_url='http://localhost:11434/v1/',
    api_key='ollama'
)

response = client.responses.create(
    model='dashscope-qwen',
    input='Explain quantum entanglement in simple terms'
)
print(response.output_text)
```
