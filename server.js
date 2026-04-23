const express = require('express');

const app = express();
app.use(express.json());

const PORT = process.env.PORT || 3000;
const OPENAI_API_KEY = process.env.OPENAI_API_KEY;
const ANTHROPIC_API_KEY = process.env.ANTHROPIC_API_KEY;

function detectProvider(model) {
  if (!model) return null;
  const m = model.toLowerCase();
  if (m.startsWith('claude')) return 'anthropic';
  if (m.startsWith('gpt') || m.startsWith('o1') || m.startsWith('o3') || m.startsWith('text-')) return 'openai';
  return null;
}

async function callOpenAI({ model, messages, temperature, max_tokens, stream }) {
  const body = {
    model,
    messages,
    stream: !!stream,
  };
  if (temperature !== undefined) body.temperature = temperature;
  if (max_tokens !== undefined) body.max_tokens = max_tokens;

  const res = await fetch('https://api.openai.com/v1/chat/completions', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${OPENAI_API_KEY}`,
    },
    body: JSON.stringify(body),
  });

  if (!res.ok) {
    const err = await res.text();
    throw new Error(`OpenAI error ${res.status}: ${err}`);
  }

  if (stream) {
    return { stream: res.body, provider: 'openai' };
  }

  return res.json();
}

function openAIMessagesToAnthropic(messages) {
  const systemMsgs = [];
  const otherMsgs = [];
  for (const msg of messages) {
    if (msg.role === 'system') {
      systemMsgs.push({ type: 'text', text: msg.content });
    } else {
      otherMsgs.push(msg);
    }
  }
  return {
    system: systemMsgs.length ? systemMsgs : undefined,
    messages: otherMsgs,
  };
}

async function callAnthropic({ model, messages, temperature, max_tokens, stream }) {
  const { system, messages: anthropicMessages } = openAIMessagesToAnthropic(messages);
  const body = {
    model,
    messages: anthropicMessages,
    max_tokens: max_tokens ?? 4096,
    stream: !!stream,
  };
  if (system) body.system = system;
  if (temperature !== undefined) body.temperature = temperature;

  const res = await fetch('https://api.anthropic.com/v1/messages', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'x-api-key': ANTHROPIC_API_KEY,
      'anthropic-version': '2023-06-01',
    },
    body: JSON.stringify(body),
  });

  if (!res.ok) {
    const err = await res.text();
    throw new Error(`Anthropic error ${res.status}: ${err}`);
  }

  if (stream) {
    return { stream: res.body, provider: 'anthropic' };
  }

  return res.json();
}

function anthropicToOpenAI(anthropicRes, model) {
  const text = anthropicRes.content
    ?.filter(c => c.type === 'text')
    ?.map(c => c.text)
    ?.join('') || '';

  return {
    id: anthropicRes.id,
    object: 'chat.completion',
    created: Math.floor(Date.now() / 1000),
    model,
    choices: [
      {
        index: 0,
        message: {
          role: 'assistant',
          content: text,
        },
        finish_reason: anthropicRes.stop_reason || 'stop',
      },
    ],
    usage: anthropicRes.usage || {},
  };
}

app.post('/v1/chat/completions', async (req, res) => {
  try {
    const { model, messages, temperature, max_tokens, stream } = req.body;

    if (!model) {
      return res.status(400).json({ error: 'Missing "model" parameter' });
    }
    if (!messages || !Array.isArray(messages)) {
      return res.status(400).json({ error: 'Missing "messages" array' });
    }

    const provider = detectProvider(model);
    if (!provider) {
      return res.status(400).json({ error: `Unable to detect provider for model: ${model}` });
    }

    if (provider === 'openai' && !OPENAI_API_KEY) {
      return res.status(500).json({ error: 'OPENAI_API_KEY not set' });
    }
    if (provider === 'anthropic' && !ANTHROPIC_API_KEY) {
      return res.status(500).json({ error: 'ANTHROPIC_API_KEY not set' });
    }

    const params = { model, messages, temperature, max_tokens, stream };

    if (provider === 'openai') {
      if (stream) {
        const { stream: openaiStream } = await callOpenAI(params);
        res.setHeader('Content-Type', 'text/event-stream');
        res.setHeader('Cache-Control', 'no-cache');
        res.setHeader('Connection', 'keep-alive');
        openaiStream.pipe(res);
        return;
      }
      const data = await callOpenAI(params);
      return res.json(data);
    }

    if (provider === 'anthropic') {
      if (stream) {
        const { stream: anthropicStream } = await callAnthropic(params);
        res.setHeader('Content-Type', 'text/event-stream');
        res.setHeader('Cache-Control', 'no-cache');
        res.setHeader('Connection', 'keep-alive');

        const reader = anthropicStream.getReader();
        const encoder = new TextEncoder();

        // Anthropic SSE → OpenAI SSE format
        const read = async () => {
          try {
            while (true) {
              const { done, value } = await reader.read();
              if (done) break;
              res.write(value);
            }
          } catch (e) {
            console.error('Stream error:', e);
          } finally {
            res.end();
          }
        };
        read();
        return;
      }

      const data = await callAnthropic(params);
      return res.json(anthropicToOpenAI(data, model));
    }
  } catch (err) {
    console.error(err);
    return res.status(502).json({ error: err.message });
  }
});

app.get('/v1/models', (_req, res) => {
  res.json({
    object: 'list',
    data: [
      { id: 'gpt-4o', object: 'model', owned_by: 'openai' },
      { id: 'gpt-4o-mini', object: 'model', owned_by: 'openai' },
      { id: 'claude-3-5-sonnet-20241022', object: 'model', owned_by: 'anthropic' },
      { id: 'claude-3-opus-20240229', object: 'model', owned_by: 'anthropic' },
    ],
  });
});

app.get('/health', (_req, res) => {
  res.json({ status: 'ok' });
});

app.listen(PORT, () => {
  console.log(`LLM Gateway listening on http://localhost:${PORT}`);
});
