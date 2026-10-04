import { useEffect, useState } from 'react';
import { Check, TriangleAlert } from 'lucide-react';
import { api } from '../lib';

/** WhatsApp via CallMeBot. It can only send a link, so it also needs ngrok. */
export default function WhatsAppSetup({ onSaved }: { onSaved?: () => void }) {
    const [key, setKey] = useState('');
    const [phone, setPhone] = useState('');
    const [ngrok, setNgrok] = useState('');
    const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null);

    useEffect(() => {
        api.whatsapp().then(w => { setKey(w.apiKey || ''); setPhone(w.phone || ''); setNgrok(w.ngrokToken || ''); });
    }, []);

    const save = async () => {
        const err = await api.saveWhatsApp(key, phone, ngrok);
        setMsg(err ? { ok: false, text: err } : { ok: true, text: 'Saved. WhatsApp is now your channel.' });
        if (!err) onSaved?.();
    };

    return (
        <div className="stack">
            <div className="hint">
                WhatsApp messages can't carry buttons, so you answer on a small web page. That page needs a free <b>ngrok</b> token
                (dashboard.ngrok.com). Telegram is simpler and recommended.
            </div>
            <div className="field"><label>CallMeBot API key</label><input className="input" value={key} onChange={e => setKey(e.target.value)} placeholder="123456" /></div>
            <div className="field"><label>Your WhatsApp number</label><input className="input" value={phone} onChange={e => setPhone(e.target.value)} placeholder="+91 98765 43210" /></div>
            <div className="field"><label>ngrok auth token</label><input className="input" type="password" value={ngrok} onChange={e => setNgrok(e.target.value)} placeholder="2abc…" /></div>
            <div className="row">
                <button className="btn btn-primary" onClick={save} disabled={!key || !phone || !ngrok}>Save WhatsApp</button>
                {msg && <span className={`msg ${msg.ok ? 'ok' : 'bad'}`}>{msg.ok ? <Check size={15} /> : <TriangleAlert size={15} />}{msg.text}</span>}
            </div>
        </div>
    );
}
