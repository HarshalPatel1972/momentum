import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { Check, Copy, ExternalLink, LoaderCircle, RefreshCw, TriangleAlert } from 'lucide-react';
import { api, BrandLogo } from '../lib';

/** Install the ntfy app → subscribe to a private random topic → Save (sends a test). */
export default function NtfySetup({ onLinked }: { onLinked?: () => void }) {
    const [server, setServer] = useState('https://ntfy.sh');
    const [topic, setTopic] = useState('');
    const [token, setToken] = useState('');
    const [advanced, setAdvanced] = useState(false);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const [saved, setSaved] = useState(false);
    const [copied, setCopied] = useState(false);

    useEffect(() => {
        api.ntfy().then(n => {
            setServer(n.server || 'https://ntfy.sh');
            setTopic(n.topic);
            setToken(n.token || '');
            if (n.server && n.server !== 'https://ntfy.sh') setAdvanced(true);
        });
    }, []);

    const copy = () => {
        navigator.clipboard.writeText(topic);
        setCopied(true);
        setTimeout(() => setCopied(false), 1400);
    };

    const save = async () => {
        setBusy(true);
        setError('');
        const err = await api.saveNtfy(server, topic, token);
        setBusy(false);
        if (err) return setError(err);
        setSaved(true);
        onLinked?.();
    };

    return (
        <div className="stack">
            <div className="steps-list">
                <div className="step-item"><span className="step-n">1</span>
                    <span>
                        Install the free <b>ntfy</b> app on your phone (Android or iPhone). No account needed.
                        <span className="row" style={{ marginTop: 8 }}>
                            <button className="key sm" onClick={() => api.openURL('https://ntfy.sh')}><BrandLogo name="ntfy" size={14} /> Get ntfy <ExternalLink size={12} /></button>
                        </span>
                    </span>
                </div>
                <div className="step-item"><span className="step-n">2</span><span>In the app, tap <b>+</b> and subscribe to this private topic:</span></div>
            </div>

            <div className="topic-box">
                <code>{topic}</code>
                <button className="key flat sm" onClick={copy}>{copied ? <Check size={13} /> : <Copy size={13} />}{copied ? 'Copied' : 'Copy'}</button>
                <button className="key flat sm" onClick={() => api.newNtfyTopic().then(setTopic)} title="Make a new random topic"><RefreshCw size={13} /></button>
            </div>
            <span className="hint">The topic is the key: anyone who knows it could read your questions and answer them, so keep it private. It's long and random, so it can't be guessed.</span>

            <div className="steps-list"><div className="step-item"><span className="step-n">3</span><span>Click <b>Save &amp; send test</b>. A "Momentum is linked" notification should appear on your phone.</span></div></div>

            {advanced ? (
                <>
                    <div className="field"><label>Server</label><input className="input" value={server} onChange={e => setServer(e.target.value)} placeholder="https://ntfy.sh" /></div>
                    <div className="field"><label>Access token (optional)</label><input className="input" type="password" value={token} onChange={e => setToken(e.target.value)} placeholder="tk_…" /><span className="hint">For a protected or self-hosted server.</span></div>
                </>
            ) : (
                <button className="key flat sm" style={{ alignSelf: 'flex-start' }} onClick={() => setAdvanced(true)}>Use my own ntfy server</button>
            )}

            <div className="row">
                <button className="key go" onClick={save} disabled={busy || topic.length < 12}>
                    {busy && <LoaderCircle size={16} className="spin" />} Save &amp; send test
                </button>
            </div>
            <span className="hint">ntfy shows up to 3 answer buttons. To type an answer instead, use the message box in the ntfy app.</span>

            {error && <div className="msg bad"><TriangleAlert size={16} style={{ flex: 'none', marginTop: 1 }} />{error}</div>}
            {saved && !error && (
                <motion.div className="linked" initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }}>
                    <div className="linked-ico"><BrandLogo name="ntfy" size={30} /></div>
                    <div className="grow">
                        <div style={{ fontWeight: 600 }}>ntfy is set up</div>
                        <div className="muted" style={{ fontSize: 12.5 }}>Didn't get the test notification? Check you subscribed to exactly this topic.</div>
                    </div>
                    <Check size={20} color="var(--ok)" />
                </motion.div>
            )}
        </div>
    );
}
