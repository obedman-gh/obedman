export const NAME_LIMIT = 80;
export const MESSAGE_LIMIT = 2000;
export const characterCount = (value) => Array.from(value.trim()).length;

async function request(url, options = {}) {
  const timeout = AbortSignal.timeout(10000);
  let response;
  try {
    response = await fetch(url, { ...options, signal: options.signal ? AbortSignal.any([options.signal, timeout]) : timeout });
  } catch {
    throw new Error('Layanan belum dapat dihubungi. Periksa koneksi lalu coba lagi.');
  }
  if (!response.ok) {
    if (response.status === 429) throw new Error('Terlalu banyak permintaan. Tunggu satu menit lalu coba lagi.');
    throw new Error('Layanan sementara tidak tersedia. Coba lagi nanti.');
  }
  try { return await response.json(); }
  catch { throw new Error('Respons layanan tidak valid. Coba lagi nanti.'); }
}

export async function loadMessages(cursor = '', signal) {
  const data = await request(`/api/guestbook?limit=20${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`, { signal });
  if (!data || !Array.isArray(data.items) || typeof data.next_cursor !== 'string' || data.items.some(
    (item) => !item || !Number.isSafeInteger(item.id) || typeof item.name !== 'string' || typeof item.message !== 'string' || item.status !== 'approved'
  )) throw new Error('Respons layanan tidak valid. Coba lagi nanti.');
  return data;
}

export async function submitMessage(name, message) {
  const n = characterCount(name), m = characterCount(message);
  if (n < 1 || n > NAME_LIMIT || m < 1 || m > MESSAGE_LIMIT) {
    throw new Error('Nama harus 1–80 karakter dan pesan 1–2000 karakter.');
  }
  const data = await request('/api/guestbook', {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name: name.trim(), message: message.trim() }),
  });
  if (!data || !Number.isSafeInteger(data.id) || data.status !== 'pending') throw new Error('Respons layanan tidak valid.');
  return data;
}
