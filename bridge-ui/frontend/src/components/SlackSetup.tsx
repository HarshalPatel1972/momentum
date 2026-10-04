import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { Check, Copy, ExternalLink, Eye, EyeOff, LoaderCircle, TriangleAlert } from 'lucide-react';
import { api, BrandLogo, SlackLink } from '../lib';

/** Create the Slack app from Momentum's manifest → paste two tokens → DM it → Detect. */
export default function SlackSetup({ onLinked }: { onLinked?: () => void }) {
    const [bot, setBot] = useState('');
    const [app, setApp] = useState('');
    const [reveal, setReveal] = useState(false);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const [linked, setLinked] = useState<SlackLink | null>(null);
    const [copied, setCopied] = useState(false);

    useEffect(() => {
        api.slack().then(s => {
            if (s?.bot_token) setBot(s.bot_token);
            if (s?.app_token) setApp(s.app_token);
            if (s?.user_id) setLinked({ userId: s.user_id, userName: s.user_name || s.user_id, channelId: s.channel_id || '', team: s.team || '' });
        });
    }, []);

    const copyManifest = async () => {
        navigator.clipboard.writeText(await api.slackManifest());
        setCopied(true);
        setTimeout(() => setCopied(false), 1500);
    };

    const detect = async () => {
        setBusy(true);
        setError('');
        const link = await api.detectSlack(bot.trim(), app.trim());
        if (link.error) {
            setError(link.error);
            setBusy(false);
            return;
        }
        const err = await api.saveSlack(bot.trim(), app.trim(), link);
        setBusy(false);
        if (err) return setError(err);
        setLinked(link);
        onLinked?.();
    };

    const type = reveal ? 'text' : 'password';
    const ready = bot.trim().startsWith('xoxb-') && app.trim().startsWith('xapp-');

    return (
        <div className="stack">
            <div className="steps-list">
                <div className="step-item"><span className="step-n">1</span>
                    <span>
                        Create the Momentum app in your Slack workspace. It's pre-filled, so just pick the workspace, then <b>Create</b> and <b>Install to Workspace</b>.
                        <span className="row" style={{ marginTop: 8, gap: 8 }}>
                            <button className="key sm" onClick={() => api.openSlackSetup()}><BrandLogo name="slack" size={14} /> Create Slack app <ExternalLink size={12} /></button>
                            <button className="key flat sm" onClick={copyManifest}>{copied ? <Check size={13} /> : <Copy size={13} />}{copied ? 'Copied' : 'Copy manifest instead'}</button>
                        </span>
                    </span>
                </div>
                <div className="step-item"><span className="step-n">2</span><span>Under <b>OAuth &amp; Permissions</b>, copy the <b>Bot User OAuth Token</b> (starts with <span className="chip">xoxb-</span>).</span></div>
                <div className="step-item"><span className="step-n">3</span><span>Under <b>Basic Information → App-Level Tokens</b>, generate a token with the <span className="chip">connections:write</span> scope (starts with <span className="chip">xapp-</span>).</span></div>
                <div className="step-item"><span className="step-n">4</span><span>In Slack, open <b>Momentum</b> under Apps and send it any message. Then click <b>Detect</b>.</span></div>
            </div>

            <div className="field">
                <label>Bot token</label>
                <div className="input-row">
                    <input className="input" type={type} value={bot} onChange={e => { setBot(e.target.value); setError(''); }} placeholder="xoxb-…" spellCheck={false} />
                    <button className="key" onClick={() => setReveal(!reveal)} title={reveal ? 'Hide' : 'Show'}>{reveal ? <EyeOff size={16} /> : <Eye size={16} />}</button>
                </div>
            </div>
            <div className="field">
                <label>App-level token</label>
                <div className="input-row">
                    <input className="input" type={type} value={app} onChange={e => { setApp(e.target.value); setError(''); }} placeholder="xapp-…" spellCheck={false} />
                    <button className="key go" onClick={detect} disabled={busy || !ready}>
                        {busy && <LoaderCircle size={16} className="spin" />}
                        {busy ? 'Waiting for your message…' : linked ? 'Re-detect' : 'Detect'}
                    </button>
                </div>
                <span className="hint">Both tokens stay on this PC and are only ever sent to Slack. Momentum connects out to Slack, so nothing on your PC is exposed.</span>
            </div>

            {error && <div className="msg bad"><TriangleAlert size={16} style={{ flex: 'none', marginTop: 1 }} />{error}</div>}

            {linked && !error && (
                <motion.div className="linked" initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }}>
                    <div className="linked-ico"><BrandLogo name="slack" size={30} /></div>
                    <div className="grow">
                        <div style={{ fontWeight: 600 }}>Linked to {linked.userName}{linked.team ? ` in ${linked.team}` : ''}</div>
                        <div className="muted" style={{ fontSize: 12.5 }}>Questions arrive as DMs from the Momentum app · only you can answer them</div>
                    </div>
                    <Check size={20} color="var(--ok)" />
                </motion.div>
            )}
        </div>
    );
}
