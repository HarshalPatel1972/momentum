import { useState } from 'react';
import { BookOpen, Bug, Check, FolderOpen, Github, Heart, Play, Power, Scale, ScrollText, Share2, Trash2 } from 'lucide-react';
import { Activity, api, AppState, Logo, REPO_URL, Slide } from '../lib';
import Tape from './Tape';
import IdeList from './IdeList';
import TelegramSetup from './TelegramSetup';
import WhatsAppSetup from './WhatsAppSetup';
import SlackSetup from './SlackSetup';
import DiscordSetup from './DiscordSetup';
import NtfySetup from './NtfySetup';
import ChannelPicker, { ChannelId } from './ChannelPicker';
import { TRUST } from './Home';

function Head({ title, sub, children }: { title: string; sub: string; children?: React.ReactNode }) {
    return (
        <div className="page-head">
            <div className="grow"><h1>{title}</h1><p>{sub}</p></div>
            {children}
        </div>
    );
}

export function LogPage({ activity, toast }: { activity: Activity[]; toast: (m: string) => void }) {
    const answered = activity.filter(a => a.state === 'answered').length;
    return (
        <div className="page">
            <Head title="Log" sub={activity.length ? `${activity.length} pages · ${answered} answered` : 'Every page your agents send you, printed here.'}>
                {activity.length > 0 && <button className="key sm" onClick={async () => { await api.clearActivity(); toast('Log cleared'); }}><Trash2 size={13} /> Clear</button>}
            </Head>
            <div style={{ maxWidth: 560 }}><Tape items={activity} title="PRINTED LOG · ALL PAGES" /></div>
        </div>
    );
}

export function IdesPage({ refresh, toast }: { refresh: () => void; toast: (m: string) => void }) {
    const addRules = async () => {
        const res = await api.addRules();
        if (res) toast(res.replace(/^Error:\s*/, ''));
    };
    return (
        <div className="page">
            <Head title="IDEs" sub="Plug the pager into every IDE and agent you use. Several can page you at once." />
            <IdeList onChange={refresh} />
            <div className="cap section-label">Optional</div>
            <div className="plate">
                <div className="set-row">
                    <BookOpen size={18} className="faint" />
                    <div className="grow">
                        <div className="t">Add Momentum rules to a project</div>
                        <div className="d">Writes a short section into AGENTS.md (and CLAUDE.md / GEMINI.md if present) for agents that weigh project rules more heavily.</div>
                    </div>
                    <button className="key sm" onClick={addRules}>Choose folder</button>
                </div>
            </div>
        </div>
    );
}

export function LinkPage({ state, refresh, toast }: { state: AppState; refresh: () => void; toast: (m: string) => void }) {
    const known: ChannelId[] = ['telegram', 'slack', 'discord', 'ntfy', 'whatsapp'];
    const [channel, setChannel] = useState<ChannelId>(known.includes(state.channel as ChannelId) ? state.channel as ChannelId : 'telegram');
    const linked = (name: string) => () => { toast(`${name} linked. Pages now go there`); refresh(); };
    return (
        <div className="page">
            <Head title="Link" sub="Where the pager reaches you. One channel is active at a time." />
            <ChannelPicker value={channel} onChange={setChannel} active={state.channel} />
            <div className="plate plate-pad" style={{ marginTop: 14, maxWidth: 720 }}>
                {channel === 'telegram' && <TelegramSetup showSteps={state.channel !== 'telegram'} onLinked={linked('Telegram')} />}
                {channel === 'slack' && <SlackSetup onLinked={linked('Slack')} />}
                {channel === 'discord' && <DiscordSetup onLinked={linked('Discord')} />}
                {channel === 'ntfy' && <NtfySetup onLinked={linked('ntfy')} />}
                {channel === 'whatsapp' && <WhatsAppSetup onSaved={refresh} />}
            </div>
        </div>
    );
}

export function SettingsPage({ state, refresh, toast }: { state: AppState; refresh: () => void; toast: (m: string) => void }) {
    const [logs, setLogs] = useState<string[] | null>(null);
    return (
        <div className="page" style={{ maxWidth: 760 }}>
            <Head title="Settings" sub="How the pager behaves on this PC." />
            <div className="plate">
                <div className="set-row">
                    <div className="grow"><div className="t">Mode</div><div className="d">AWAY: pages go to your phone. DESK: agents ask in the IDE chat.</div></div>
                    <Slide small on={!state.atDesk} onChange={async v => { await api.setAtDesk(!v); refresh(); }} left="AWAY" right="DESK" />
                </div>
                <div className="set-row">
                    <div className="grow"><div className="t">Pop up the pager</div><div className="d">When a page arrives and this window is closed, show the mini pager in the corner of your screen. You can answer right there.</div></div>
                    <Slide small on={state.popUp} onChange={async v => { await api.setPopUp(v); refresh(); }} />
                </div>
            </div>

            <div className="cap section-label">Data &amp; privacy</div>
            <div className="plate">
                <div className="set-row">
                    <FolderOpen size={18} className="faint" />
                    <div className="grow"><div className="t">Your Momentum folder</div><div className="d">Settings, log and diagnostics live here and nowhere else: <span className="chip">{state.dataDir}</span></div></div>
                    <button className="key sm" onClick={() => api.openData()}>Open</button>
                </div>
                <div className="set-row">
                    <Trash2 size={18} className="faint" />
                    <div className="grow"><div className="t">Page log</div><div className="d">Stored only on this PC, last 100 pages.</div></div>
                    <button className="key sm" onClick={async () => { await api.clearActivity(); toast('Log cleared'); }}>Clear</button>
                </div>
                <div className="set-row">
                    <ScrollText size={18} className="faint" />
                    <div className="grow"><div className="t">Diagnostics</div><div className="d">Useful when a page doesn't arrive. Tokens are never written here.</div></div>
                    <button className="key sm" onClick={async () => setLogs(logs ? null : await api.logs())}>{logs ? 'Hide' : 'Show'}</button>
                </div>
                {logs && <div className="logs">{logs.length ? logs.join('\n') : 'Nothing logged yet.'}</div>}
            </div>

            <div className="cap section-label">Power</div>
            <div className="plate">
                <div className="set-row">
                    <Power size={18} className="faint" />
                    <div className="grow"><div className="t">Switch off</div><div className="d">Closing the window keeps the pager on in the tray. This switches it off until an IDE needs it again.</div></div>
                    <button className="key sm go" onClick={() => api.quit()}>Switch off</button>
                </div>
            </div>
        </div>
    );
}

export function AboutPage({ state, toast, onReplay }: { state: AppState; toast: (m: string) => void; onReplay: () => void }) {
    const [copied, setCopied] = useState(false);
    const share = () => {
        navigator.clipboard.writeText(`Momentum: a pager for your AI coding agents. They page your phone when they need a decision, so they keep working while you're away. Free and open source: ${REPO_URL}`);
        setCopied(true);
        toast('Copied. Paste it anywhere to share Momentum');
        setTimeout(() => setCopied(false), 1600);
    };
    return (
        <div className="page" style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) 300px', gap: 18, alignItems: 'start' }}>
            <div className="stack">
                <div className="row" style={{ gap: 14 }}>
                    <Logo size={58} />
                    <div className="grow">
                        <h1 style={{ fontSize: 24, letterSpacing: '-.02em' }}>Momentum</h1>
                        <div className="muted">The pager for your AI coding agents.</div>
                    </div>
                    <button className="key go" onClick={share}>{copied ? <Check size={15} /> : <Share2 size={15} />} Share</button>
                </div>
                <div className="plate plate-pad" style={{ lineHeight: 1.65 }}>
                    <div className="cap" style={{ marginBottom: 8 }}>Why it exists</div>
                    <p className="muted">AI agents can work for hours on their own. But the moment they need a human decision ("delete these files?", "which approach?") they stop and wait. If you've stepped away, they wait for hours.</p>
                    <p className="muted" style={{ marginTop: 10 }}>Momentum gives every agent a way to page you. The question reaches your phone, you answer with one tap, and the agent picks up where it left off. It's deliberately small: one app on your PC, your own bot in the messaging app you already use, and nothing in between.</p>
                </div>
                <div className="plate plate-pad">
                    <div className="cap" style={{ marginBottom: 12 }}>Our promises</div>
                    <div className="trust-list">
                        {TRUST.map(t => (
                            <div key={t.title} className="trust-item"><div className="trust-ico"><t.icon size={15} /></div><div><b>{t.title}</b><span>{t.text}</span></div></div>
                        ))}
                    </div>
                </div>
            </div>
            <div className="stack">
                <div className="label-plate">
                    <span className="screw" style={{ top: 8, left: 8 }} /><span className="screw" style={{ top: 8, right: 8 }} />
                    <span className="screw" style={{ bottom: 8, left: 8 }} /><span className="screw" style={{ bottom: 8, right: 8 }} />
                    <div className="cap" style={{ textAlign: 'center', marginBottom: 10 }}>Momentum · MP-1</div>
                    <table><tbody>
                        <tr><td>Model</td><td>Agent pager</td></tr>
                        <tr><td>Version</td><td>{state.version}</td></tr>
                        <tr><td>Licence</td><td>MIT · open source</td></tr>
                        <tr><td>Ports opened</td><td>0</td></tr>
                        <tr><td>Servers</td><td>none</td></tr>
                        <tr><td>Made for</td><td>VS Code, Cursor, Claude…</td></tr>
                    </tbody></table>
                </div>
                <div className="plate plate-pad stack" style={{ gap: 8 }}>
                    <button className="key sm go" onClick={onReplay}><Play size={13} /> Replay intro</button>
                    <button className="key sm" onClick={() => api.openURL(REPO_URL)}><Github size={13} /> Source code</button>
                    <button className="key sm" onClick={() => api.openURL(REPO_URL + '/issues/new')}><Bug size={13} /> Report a problem</button>
                    <button className="key sm" onClick={() => api.openURL(REPO_URL + '/blob/main/LICENSE')}><Scale size={13} /> MIT licence</button>
                    <button className="key sm flat" onClick={() => api.openURL(REPO_URL)}><Heart size={13} /> Star on GitHub</button>
                </div>
            </div>
        </div>
    );
}
