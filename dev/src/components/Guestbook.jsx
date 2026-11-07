import { useState, useEffect, useRef } from 'react';
import { loadMessages, submitMessage, characterCount, NAME_LIMIT, MESSAGE_LIMIT } from './guestbook-api.js';

export default function Guestbook() {
  const [messages, setMessages] = useState([]);
  const [name, setName] = useState('');
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(false);
  const [hydrated, setHydrated] = useState(false);
  const [listLoading, setListLoading] = useState(true);
  const [nextCursor, setNextCursor] = useState('');
  const [listError, setListError] = useState('');
  const [submitError, setSubmitError] = useState('');
  const [notice, setNotice] = useState('');
  const submitting = useRef(false);
  const fetching = useRef(false);

  useEffect(() => {
    setHydrated(true);
    const controller = new AbortController();
    async function initialLoad() {
      try {
        const page = await loadMessages('', controller.signal);
        if (!controller.signal.aborted) { setMessages(page.items); setNextCursor(page.next_cursor); }
      } catch {
        if (!controller.signal.aborted) setListError('Pesan belum dapat dimuat. Silakan coba lagi.');
      } finally {
        if (!controller.signal.aborted) setListLoading(false);
      }
    }
    initialLoad();
    return () => controller.abort();
  }, []);

  async function loadMore() {
    if (fetching.current || listLoading) return;
    fetching.current = true;
    setListLoading(true); setListError('');
    try {
      const page = await loadMessages(nextCursor);
      setMessages((existing) => {
        const ids = new Set(existing.map((item) => item.id));
        return [...existing, ...page.items.filter((item) => !ids.has(item.id))];
      });
      setNextCursor(page.next_cursor);
    } catch { setListError('Pesan belum dapat dimuat. Silakan coba lagi.'); }
    finally { fetching.current = false; setListLoading(false); }
  }

  async function handleSubmit(e) {
    e.preventDefault();
    if (submitting.current) return;
    submitting.current = true;
    setLoading(true); setSubmitError(''); setNotice('');
    try {
      await submitMessage(name, message);
      setName(''); setMessage('');
      setNotice('Pesan diterima dan menunggu moderasi sebelum ditampilkan.');
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : 'Pesan belum dapat dikirim. Silakan coba lagi.');
    } finally { submitting.current = false; setLoading(false); }
  }

  return (
    <div className="mt-12 rounded-lg bg-slate-900/50 p-6 border border-slate-800">
      <h3 className="text-lg font-semibold text-slate-200 mb-4">Guestbook</h3>
      <p id="guestbook-privacy" className="mb-4 text-sm text-slate-400">Nama dan pesan yang disetujui akan dipublikasikan. Hindari informasi pribadi. Semua pesan menunggu moderasi.</p>
      <form onSubmit={handleSubmit} aria-describedby="guestbook-privacy" aria-busy={loading} className="mb-8 flex flex-col gap-4">
        <label htmlFor="guestbook-name" className="text-sm text-slate-300">Nama</label>
        <input id="guestbook-name" type="text" placeholder="Nama Lu (atau Anonim)" required value={name}
          onChange={(e) => setName(e.target.value)} disabled={loading || !hydrated} aria-describedby="guestbook-name-count"
          className="w-full rounded bg-slate-800/50 px-4 py-2 text-sm text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-teal-400" />
        <span id="guestbook-name-count" className="text-xs text-slate-400">{characterCount(name)}/{NAME_LIMIT} karakter</span>
        <label htmlFor="guestbook-message" className="text-sm text-slate-300">Pesan</label>
        <textarea id="guestbook-message" placeholder="Tinggalkan pesan buat gua..." required rows="3" value={message}
          onChange={(e) => setMessage(e.target.value)} disabled={loading || !hydrated} aria-describedby="guestbook-message-count"
          className="w-full rounded bg-slate-800/50 px-4 py-2 text-sm text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-teal-400 resize-none" />
        <span id="guestbook-message-count" className="text-xs text-slate-400">{characterCount(message)}/{MESSAGE_LIMIT} karakter</span>
        <p role="alert" className="text-sm text-red-300">{submitError}</p>
        <p role="status" className="text-sm text-teal-300">{notice}</p>
        <button type="submit" disabled={!hydrated || loading || characterCount(name) > NAME_LIMIT || characterCount(message) > MESSAGE_LIMIT}
          className="w-full sm:w-auto self-end rounded bg-teal-400/10 px-6 py-2 text-sm font-semibold text-teal-300 hover:bg-teal-400/20 transition-colors disabled:opacity-50">
          {loading ? 'Mengirim...' : 'Kirim Pesan'}
        </button>
      </form>
      <div aria-busy={listLoading} className="space-y-4 max-h-64 overflow-y-auto pr-2">
        {messages.map((msg) => (
          <div key={msg.id} className="border-b border-slate-800 pb-4">
            <strong className="block break-words text-slate-300 text-sm">{msg.name}</strong>
            <p className="mt-1 text-slate-400 text-sm leading-relaxed whitespace-pre-wrap break-words">{msg.message}</p>
          </div>
        ))}
      </div>
      <p role="status" className="text-sm text-slate-400">{listLoading ? 'Memuat pesan...' : !listError && messages.length === 0 ? 'Belum ada pesan yang disetujui.' : ''}</p>
      <p role="alert" className="text-sm text-red-300">{listError}</p>
      {(nextCursor || listError) && <button type="button" onClick={loadMore} disabled={listLoading} className="mt-4 text-sm text-teal-300 disabled:opacity-50">{listError ? 'Coba muat lagi' : 'Muat pesan berikutnya'}</button>}
    </div>
  );
}
