const BOT_URL = 'https://t.me/tet_mcp_bot';

function schedulerBase() {
  return (process.env.SCHEDULER_INTERNAL_URL || 'http://127.0.0.1:8090').replace(/\/$/, '');
}

export async function POST(request) {
  const base = schedulerBase();
  let body;
  try {
    body = await request.text();
  } catch {
    return Response.json({ accepted: false, errors: ['invalid body'] }, { status: 400 });
  }
  try {
    const r = await fetch(`${base}/schedule/from-note`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body,
    });
    const text = await r.text();
    return new Response(text, {
      status: r.status,
      headers: { 'Content-Type': 'application/json' },
    });
  } catch {
    return Response.json(
      {
        accepted: false,
        errors: [`scheduler unreachable (${base}). Настройте SCHEDULER_INTERNAL_URL. Бот: ${BOT_URL}`],
      },
      { status: 502 },
    );
  }
}

export async function GET(request) {
  const base = schedulerBase();
  const url = new URL(request.url);
  const tg = (url.searchParams.get('telegram_username') || '').trim();
  if (!tg) {
    return Response.json({ note_item_ids: [] }, { status: 200 });
  }
  try {
    const r = await fetch(`${base}/schedule/list?telegram_username=${encodeURIComponent(tg)}`, {
      method: 'GET',
    });
    const text = await r.text();
    return new Response(text, {
      status: r.status,
      headers: { 'Content-Type': 'application/json' },
    });
  } catch {
    return Response.json(
      {
        note_item_ids: [],
        errors: [`scheduler unreachable (${base}). Настройте SCHEDULER_INTERNAL_URL. Бот: ${BOT_URL}`],
      },
      { status: 502 },
    );
  }
}
