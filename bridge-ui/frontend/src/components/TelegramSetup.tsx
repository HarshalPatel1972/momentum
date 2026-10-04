import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { Check, Eye, EyeOff, LoaderCircle, TriangleAlert } from 'lucide-react';
import { api, TelegramChat, TelegramIcon } from '../lib';

interface Props {
    onLinked?: (chat: TelegramChat) => void;
    showSteps?: boolean;
}

/** Bot token → Detect → linked. Saves the channel as soon as the chat is found. */
export default function TelegramSetup({ onLinked, showSteps = true }: Props) {
    const [token, setToken] = useState('');
    const [reveal, setReveal] = useState(false);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const [linked, setLinked] = useState<TelegramChat | null>(null);

    useEffect(() => {
        api.telegram().then(t => {
            if (t?.bot_token) setToken(t.bot_token);
            if (t?.chat_id) setLinked({ id: t.chat_id, name: t.chat_name || t.chat_id, bot: t.bot_username || '' });
        });
    }, []);

    const detect = async () => {
        setBusy(true);
        setError('');
        const chat = await api.detectChat(token.trim());
        if (chat.error) {
            setError(chat.error);
            setBusy(false);
            return;
        }
        const err = await api.saveTelegram(token.trim(), chat);
        setBusy(false);
        if (err) {
            setError(err);
            return;
        }
        setLinked(chat);
        onLinked?.(chat);
    };

    return (
        <div className="stack">
            {showSteps && (
                <div className="howto">
                    <div className="howto-item"><span className="howto-n">1</span><span>In Telegram, open <b>@BotFather</b> and send <span className="kbd">/newbot</span>. Pick any name.</span></div>
                    <div className="howto-item"><span className="howto-n">2</span><span>Copy the <b>token</b> it gives you and paste it below.</span></div>
                    <div className="howto-item"><span className="howto-n">3</span><span>Open your new bot, press <b>Start</b>, then click <b>Detect</b>.</span></div>
                </div>
            )}

            <div className="field">
                <label>Bot token</label>
                <div className="input-wrap">
                    <input
                        className="input"
                        type={reveal ? 'text' : 'password'}
                        value={token}
                        onChange={e => { setToken(e.target.value); setError(''); }}
                        placeholder="123456789:AAH…"
                        spellCheck={false}
                    />
                    <button className="btn btn-secondary" onClick={() => setReveal(!reveal)} title={reveal ? 'Hide' : 'Show'}>
                        {reveal ? <EyeOff size={16} /> : <Eye size={16} />}
                    </button>
                    <button className="btn btn-primary" onClick={detect} disabled={busy || token.trim().length < 20}>
                        {busy ? <LoaderCircle size={16} className="spin" /> : null}
                        {busy ? 'Detecting…' : linked ? 'Re-detect' : 'Detect'}
                    </button>
                </div>
                <span className="hint">Stays on this PC, in your Momentum settings. It's only ever sent to Telegram.</span>
            </div>

            {error && <div className="msg bad"><TriangleAlert size={16} style={{ flex: 'none', marginTop: 1 }} />{error}</div>}

            {linked && !error && (
                <motion.div className="linked" initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }}>
                    <div className="linked-ico"><TelegramIcon size={38} /></div>
                    <div className="grow">
                        <div style={{ fontWeight: 600 }}>Linked to {linked.name}</div>
                        <div className="muted" style={{ fontSize: 12.5 }}>
                            {linked.bot ? <>via @{linked.bot} · </> : null}only this chat can answer your agents
                        </div>
                    </div>
                    <Check size={20} color="var(--ok)" />
                </motion.div>
            )}
        </div>
    );
}
