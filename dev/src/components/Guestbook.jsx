import { useState, useEffect } from 'react';

export default function Guestbook() {
  const [messages, setMessages] = useState([]);
  const [name, setName] = useState('');
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(false);

  // Ambil data saat komponen di-load
  useEffect(() => {
    fetch('/api/guestbook')
      .then((res) => res.json())
      .then((data) => setMessages(data))
      .catch((err) => console.error(err));
  }, []);

  // Fungsi kirim pesan
  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);

    const res = await fetch('/api/guestbook', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, message }),
    });

    if (res.ok) {
      const newEntry = await res.json();
      setMessages([newEntry, ...messages]); // Update list langsung
      setName('');
      setMessage('');
    }
    setLoading(false);
  };

  return (
    <div className="mt-12 rounded-lg bg-slate-900/50 p-6 border border-slate-800">
      <h3 className="text-lg font-semibold text-slate-200 mb-4">Guestbook</h3>
      
      <form onSubmit={handleSubmit} className="mb-8 flex flex-col gap-4">
        <input 
          type="text" 
          placeholder="Nama Lu (atau Anonim)" 
          required
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="w-full rounded bg-slate-800/50 px-4 py-2 text-sm text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-teal-400"
        />
        <textarea 
          placeholder="Tinggalkan pesan buat gua..." 
          required
          rows="3"
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          className="w-full rounded bg-slate-800/50 px-4 py-2 text-sm text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-teal-400 resize-none"
        />
        <button 
          type="submit" 
          disabled={loading}
          className="w-full sm:w-auto self-end rounded bg-teal-400/10 px-6 py-2 text-sm font-semibold text-teal-300 hover:bg-teal-400/20 transition-colors disabled:opacity-50"
        >
          {loading ? 'Mengirim...' : 'Kirim Pesan'}
        </button>
      </form>

      <div className="space-y-4 max-h-64 overflow-y-auto pr-2">
        {messages.map((msg) => (
          <div key={msg.id} className="border-b border-slate-800 pb-4">
            <div className="flex items-baseline gap-2">
              <strong className="text-slate-300 text-sm">{msg.name}</strong>
            </div>
            <p className="mt-1 text-slate-400 text-sm leading-relaxed">{msg.message}</p>
          </div>
        ))}
      </div>
    </div>
  );
}