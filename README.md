# LLM Gateway

Simple Express server that abstracts over OpenAI and Anthropic APIs. Pass any supported `model` name and it routes to the right provider, returning responses in OpenAI-compatible format.

## Setup

```bash
npm install
```

Set your API keys:

```bash
export OPENAI_API_KEY="sk-..."
export ANTHROPIC_API_KEY="sk-ant-..."
```

## Run

```bash
npm start
```

Server starts on `http://localhost:3000` (or `PORT` env var).

## Usage

### Chat completions

```bash
curl http://localhost:3000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

```bash
curl http://localhost:3000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### Streaming

Add `"stream": true` to get an SSE stream.

### Models list

```bash
curl http://localhost:3000/v1/models
```

## How provider detection works

- Models starting with `claude` → Anthropic
- Models starting with `gpt`, `o1`, `o3`, or `text-` → OpenAI

Anthropic responses are normalized to OpenAI's `chat.completion` shape so the client always gets the same format.
