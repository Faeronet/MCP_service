'use client';

import { useCallback, useMemo, useState } from 'react';
import { defaultAngelRows } from '../../data/angelTranslations';

function groupItemsForScheduler(rows) {
  const active = rows.filter((r) => r.enabled);
  const m = new Map();
  for (const r of active) {
    const name = (r.nameRu || '').trim();
    const key = name || r.validation;
    if (!m.has(key)) {
      m.set(key, {
        validation: r.validation,
        name,
        time: r.time,
        message: (r.message || '').trim(),
      });
    } else {
      const g = m.get(key);
      if (!g.message && (r.message || '').trim()) {
        g.message = (r.message || '').trim();
      }
    }
  }
  return [...m.values()];
}

export default function NotePage() {
  const [rows, setRows] = useState(() =>
    defaultAngelRows.map((a) => ({
      validation: a.validation,
      nameRu: a.nameRu,
      time: '09:00',
      message: '',
      enabled: true,
    })),
  );
  const [modalOpen, setModalOpen] = useState(false);
  const [tgUser, setTgUser] = useState('');
  const [status, setStatus] = useState('');
  const [busy, setBusy] = useState(false);

  const exportItems = useMemo(
    () =>
      rows
        .filter((r) => r.enabled)
        .map((r) => ({
          validation: r.validation,
          name: r.nameRu,
          time: r.time,
          message: r.message || undefined,
        })),
    [rows],
  );

  const downloadJson = useCallback(() => {
    const blob = new Blob([JSON.stringify({ items: exportItems }, null, 2)], {
      type: 'application/json',
    });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = 'angels-note.json';
    a.click();
    URL.revokeObjectURL(a.href);
  }, [exportItems]);

  const submitSchedule = useCallback(async () => {
    setBusy(true);
    setStatus('');
    const items = groupItemsForScheduler(rows);
    if (items.length === 0) {
      setStatus('Включите хотя бы одну строку с временем.');
      setBusy(false);
      return;
    }
    const u = tgUser.trim().replace(/^@/, '');
    if (!u) {
      setStatus('Укажите Telegram username.');
      setBusy(false);
      return;
    }
    try {
      const r = await fetch('/api/schedule', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ telegram_username: u, items }),
      });
      const data = await r.json().catch(() => ({}));
      if (!r.ok || data.accepted === false) {
        const err = (data.errors && data.errors.join(' ')) || JSON.stringify(data);
        setStatus(err || 'Ошибка отправки');
      } else {
        setStatus(
          `Принято. Запланировано отправлений: ${data.scheduled_count ?? 0} (уникальных ангелов: ${data.grouped_angels_count ?? 0}).`,
        );
        setModalOpen(false);
      }
    } catch (e) {
      setStatus(`Ошибка сети: ${e.message}`);
    }
    setBusy(false);
  }, [rows, tgUser]);

  return (
    <div className="page">
      <h1>Страница заметки</h1>
      <p className="sub">
        Имена в JSON — на русском. Уведомления уходят в сервис scheduler; нужен аккаунт в{' '}
        <a href="https://t.me/tet_mcp_bot" target="_blank" rel="noreferrer">
          Telegram-боте
        </a>{' '}
        и нажатие Start.
      </p>

      <div className="panel">
        {rows.map((r, i) => (
          <div className="row" key={r.validation}>
            <div className="row-label">
              <span className="ru">{r.nameRu}</span>
              <span className="en">{r.validation}</span>
            </div>
            <input
              type="time"
              value={r.time}
              onChange={(e) => {
                const t = e.target.value;
                setRows((prev) => prev.map((x, j) => (j === i ? { ...x, time: t } : x)));
              }}
            />
            <input
              type="text"
              placeholder="Сообщение (необязательно)"
              value={r.message}
              onChange={(e) => {
                const t = e.target.value;
                setRows((prev) => prev.map((x, j) => (j === i ? { ...x, message: t } : x)));
              }}
            />
            <label className="toggle">
              <input
                type="checkbox"
                checked={r.enabled}
                onChange={(e) => {
                  const c = e.target.checked;
                  setRows((prev) => prev.map((x, j) => (j === i ? { ...x, enabled: c } : x)));
                }}
              />
              вкл.
            </label>
          </div>
        ))}
        <div className="actions">
          <button type="button" className="btn" onClick={downloadJson}>
            Скачать JSON
          </button>
          <button type="button" className="btn btn-primary" onClick={() => setModalOpen(true)}>
            Уведомлять
          </button>
        </div>
        {status ? <p className={`msg ${status.includes('Ошибка') || status.includes('user not') ? 'err' : ''}`}>{status}</p> : null}
      </div>

      {modalOpen ? (
        <div className="modal-backdrop" role="presentation" onClick={() => !busy && setModalOpen(false)}>
          <div className="modal" role="dialog" onClick={(e) => e.stopPropagation()}>
            <h2>Уведомления в Telegram</h2>
            <p>
              Введите ваш username (как в Telegram, без @). Если бот вас не знает, откройте{' '}
              <a href="https://t.me/tet_mcp_bot" target="_blank" rel="noreferrer">
                @tet_mcp_bot
              </a>
              , нажмите Start и повторите.
            </p>
            <div className="field">
              <label htmlFor="tg-user">Telegram username</label>
              <input
                id="tg-user"
                type="text"
                autoComplete="username"
                placeholder="username"
                value={tgUser}
                onChange={(e) => setTgUser(e.target.value)}
              />
            </div>
            <div className="modal-actions">
              <button type="button" className="btn" disabled={busy} onClick={() => setModalOpen(false)}>
                Отмена
              </button>
              <button type="button" className="btn btn-primary" disabled={busy} onClick={submitSchedule}>
                {busy ? '…' : 'Отправить'}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
