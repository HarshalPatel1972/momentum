import { useEffect, useState } from 'react';
import { motion } from 'framer-motion';
import { Check, ExternalLink, Eye, EyeOff, LoaderCircle, TriangleAlert } from 'lucide-react';
import { api, BrandLogo, DiscordLink } from '../lib';

/** Create a bot → paste its token → add it to a server → DM it → Detect. */
export default function DiscordSetup({ onLinked }: { onLinked?: () => void }) {
    const [token, setToken] = useState('');
    const [reveal, setReveal] = useState(false);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const [linked, setLinked] = useState<DiscordLink | null>(null);

    useEffect(() => {
        api.discord().then(d => {
            if (d?.bot_token) setToken(d.bot_token);
            if (d?.user_id) setLinked({ userId: d.user_id, userName: d.user_name || d.user_id, channelId: d.channel_id || '', bot: '' });
        });
    }, []);

    const invite = async () => {
        setError('');
        const err = await api.discordInvite(token.trim());
        if (err) setError(err);
    };

    const detect = async () => {
        setBusy(true);
        setError('');
        const link = await api.detectDiscord(token.trim());
        if (link.error) {
            setError(link.error);
            setBusy(false);
            return;
        }
        const err = await api.saveDiscord(token.trim(), link);
        setBusy(false);
        if (err) return setError(err);
        setLinked(link);
        onLinked?.();
    };

    const hasToken = token.trim().length > 50;

    return (
        <div className="stack">
            <div className="steps-list">
                <div className="step-item"><span className="step-n">1</span>
                    <span>
                        In the Discord Developer Portal, create an application (name it <b>Momentum</b>), open <b>Bot</b> → <b>Reset Token</b>, and copy it.
                        <span className="row" style={{ marginTop: 8 }}>
                            <button className="key sm" onClick={() => api.openURL('https://discord.com/developers/applications')}><BrandLogo name="discord" size={14} /> Open Developer Portal <ExternalLink size={12} /></button>
                        </span>
                    </span>
                </div>
                <div className="step-item"><span className="step-n">2</span><span>Paste the token below, then add the bot to any server you're in (Discord only lets you DM bots you share a server with). It asks for no permissions.</span></div>
                <div className="step-item"><span className="step-n">3</span><span>Click <b>Detect</b>, then in Discord click the bot's name and send it any DM.</span></div>
            </div>

            <div className="field">
                <label>Bot token</label>
                <div className="input-row">
                    <input className="input" type={reveal ? 'text' : 'password'} value={token} onChange={e => { setToken(e.target.value); setError(''); }} placeholder="MTEy…" spellCheck={false} />
                    <button className="key" onClick={() => setReveal(!reveal)} title={reveal ? 'Hide' : 'Show'}>{reveal ? <EyeOff size={16} /> : <Eye size={16} />}</button>
                </div>
                <div className="row" style={{ marginTop: 4 }}>
                    <button className="key sm" onClick={invite} disabled={!hasToken}>Add bot to a server <ExternalLink size={12} /></button>
                    <button className="key go sm" onClick={detect} disabled={busy || !hasToken}>
                        {busy && <LoaderCircle size={14} className="spin" />}
                        {busy ? 'Waiting for your DM…' : linked ? 'Re-detect' : 'Detect'}
                    </button>
                </div>
                <span className="hint">The token stays on this PC and is only ever sent to Discord. Momentum connects out to Discord, so nothing on your PC is exposed.</span>
            </div>

            {error && <div className="msg bad"><TriangleAlert size={16} style={{ flex: 'none', marginTop: 1 }} />{error}</div>}

            {linked && !error && (
                <motion.div className="linked" initial={{ opacity: 0, y: 6 }} animate={{ opacity: 1, y: 0 }}>
                    <div className="linked-ico"><BrandLogo name="discord" size={30} /></div>
                    <div className="grow">
                        <div style={{ fontWeight: 600 }}>Linked to {linked.userName}</div>
                        <div className="muted" style={{ fontSize: 12.5 }}>Questions arrive as DMs from your bot · only you can answer them</div>
                    </div>
                    <Check size={20} color="var(--ok)" />
                </motion.div>
            )}
        </div>
    );
}
