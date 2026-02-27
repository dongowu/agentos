import OpenAI from 'openai';

const apiKey = process.env.OPENAI_API_KEY;
if (!apiKey) {
  throw new Error('OPENAI_API_KEY is required');
}

const client = new OpenAI({
  apiKey,
  baseURL: 'https://new.xychatai.com',
});
try {
  const r = await client.chat.completions.create({
    model: 'gpt-5.3-codex-high',
    messages: [{ role: 'user', content: '你好，请用一句话介绍你自己' }],
    max_tokens: 200,
  });
  console.log('Response:', r.choices[0]?.message?.content);
  console.log('Model:', r.model);
  console.log('Tokens:', r.usage?.prompt_tokens, 'in /', r.usage?.completion_tokens, 'out');
} catch (e: any) {
  console.log('Error:', e.status, e.message?.slice(0, 200));
}

process.exit(0);
