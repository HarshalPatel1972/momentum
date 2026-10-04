import { useState } from 'react';
import { BookOpen, Bug, Check, FolderOpen, Github, Heart, Power, Scale, ScrollText, Share2, Trash2 } from 'lucide-react';
import { Activity, api, AppState, Logo, REPO_URL, Toggle } from '../lib';
import ActivityList from './ActivityList';
import IdeList from './IdeList';
import TelegramSetup from './TelegramSetup';
import WhatsAppSetup from './WhatsAppSetup';
import SlackSetup from './SlackSetup';
import ChannelPicker, { ChannelId } from './ChannelPicker';
import { TRUST } from './Home';

export function ActivityPage({ activity, toast }: { activity: Activity[]; toast: (m: string) => void }) {
    const answered = activity.filter(a => a.state === 'answered').length;
    return (
        <div className="page">
            <div className="page-head">
                <div className="grow">
                    <h1>Activity</h1>
                    <p>{activity.length ? `${activity.length} questions · ${answered} answered from your phone` : 'Every question your agents send you, in one place.'}</p>
                </div>
                {activity.length > 0 && (
                    <button className="btn btn-ghost btn-sm" onClick={async () => { await api.clearActivity(); toast('History cleared'); }}>
                        <Trash2 size={14} /> Clear history
                    </button>
                )}
            </div>
            <div className="card card-pad"><ActivityList items={activity} /></div>
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
            <div className="page-head">
                <div className="grow">
                    <h1>IDEs</h1>
                    <p>Every MCP-capable IDE and agent can use Momentum. Several can run at once.</p>
                </div>
            </div>
            <IdeList onChange={refresh} />
            <div className="section-label">Optional: project instructions</div>
            <div className="card">
                <div className="set-row">
                    <BookOpen size={18} className="muted" />
                    <div className="grow">
                        <div className="t">Add Momentum rules to a project</div>
                        <div className="d">Writes a short section into the project's AGENTS.md (and CLAUDE.md / GEMINI.md if present), for agents that weigh project rules more heavily.</div>
                    </div>
                    <button className="btn btn-secondary btn-sm" onClick={addRules}>Choose folder</button>
                </div>
            </div>
        </div>
    );
}

export function SettingsPage({ state, refresh, toast }: { state: AppState; refresh: () => void; toast: (m: string) => void }) {
    const [channel, setChannel] = useState<ChannelId>((['telegram', 'slack', 'whatsapp'].includes(state.channel) ? state.channel : 'telegram') as ChannelId);
    const [logs, setLogs] = useState<string[] | null>(null);

    const showLogs = async () => setLogs(logs ? null : await api.logs());

    return (
        <div className="page">
            <div className="page-head"><div className="grow"><h1>Settings</h1><p>Where questions go, and how Momentum behaves.</p></div></div>

            <div className="section-label" style={{ marginTop: 0 }}>Your phone</div>
            <div className="card card-pad">
                <ChannelPicker value={channel} onChange={setChannel} active={state.channel} />
                <div style={{ marginTop: 20 }}>
                    {channel === 'telegram' && <TelegramSetup showSteps={false} onLinked={() => { toast('Telegram linked. Questions now go there'); refresh(); }} />}
                    {channel === 'slack' && <SlackSetup onLinked={() => { toast('Slack linked. Questions now go there'); refresh(); }} />}
                    {channel === 'whatsapp' && <WhatsAppSetup onSaved={refresh} />}
                </div>
            </div>

            <div className="section-label">Behaviour</div>
            <div className="card">
                <div className="set-row">
                    <div className="grow">
                        <div className="t">Away mode</div>
                        <div className="d">On: agent questions go to your phone. Off: agents ask in the IDE chat.</div>
                    </div>
                    <Toggle on={!state.atDesk} onChange={async v => { await api.setAtDesk(!v); refresh(); }} />
                </div>
            </div>

            <div className="section-label">Data & privacy</div>
            <div className="card">
                <div className="set-row">
                    <FolderOpen size={18} className="muted" />
                    <div className="grow">
                        <div className="t">Your Momentum folder</div>
                        <div className="d">Settings, history and logs live here, and nowhere else: <code style={{ fontSize: 12 }}>{state.dataDir}</code></div>
                    </div>
                    <button className="btn btn-secondary btn-sm" onClick={() => api.openData()}>Open</button>
                </div>
                <div className="set-row">
                    <Trash2 size={18} className="muted" />
                    <div className="grow"><div className="t">Question history</div><div className="d">Stored only on this PC, last 100 questions.</div></div>
                    <button className="btn btn-danger btn-sm" onClick={async () => { await api.clearActivity(); toast('History cleared'); }}>Clear</button>
                </div>
                <div className="set-row">
                    <ScrollText size={18} className="muted" />
                    <div className="grow"><div className="t">Diagnostics log</div><div className="d">Useful when something doesn't arrive. Tokens are never written to it.</div></div>
                    <button className="btn btn-secondary btn-sm" onClick={showLogs}>{logs ? 'Hide' : 'Show'}</button>
                </div>
                {logs && <div className="logs">{logs.length ? logs.join('\n') : 'Nothing logged yet.'}</div>}
            </div>

            <div className="section-label">Quit</div>
            <div className="card">
                <div className="set-row">
                    <Power size={18} className="muted" />
                    <div className="grow"><div className="t">Quit Momentum</div><div className="d">Closing the window keeps Momentum in the tray. This stops it completely, until an IDE needs it again.</div></div>
                    <button className="btn btn-danger btn-sm" onClick={() => api.quit()}>Quit</button>
                </div>
            </div>
        </div>
    );
}

export function AboutPage({ state, toast }: { state: AppState; toast: (m: string) => void }) {
    const [copied, setCopied] = useState(false);
    const share = () => {
        navigator.clipboard.writeText(`Momentum: your AI coding agent asks for approval on your phone, so it keeps working while you're away. Free and open source: ${REPO_URL}`);
        setCopied(true);
        toast('Copied. Paste it anywhere to share Momentum');
        setTimeout(() => setCopied(false), 1600);
    };
    return (
        <div className="page">
            <div className="card about-hero">
                <Logo size={64} />
                <div className="grow">
                    <h2>Momentum</h2>
                    <div className="muted">Permission shouldn't require presence. · Version {state.version}</div>
                </div>
                <button className="btn btn-primary" onClick={share}>{copied ? <Check size={16} /> : <Share2 size={16} />} Share Momentum</button>
            </div>

            <div className="grid-2" style={{ marginTop: 16 }}>
                <div className="card card-pad story">
                    <div className="card-title" style={{ marginBottom: 12 }}>Why Momentum exists</div>
                    <p>AI agents can now work for hours on their own: refactoring, fixing tests, shipping features. But the moment they need a human decision ("delete these files?", "which approach?"), they stop and wait. If you've stepped away, they wait for hours.</p>
                    <p>Momentum keeps that work moving. It gives every agent a way to reach you on your phone, wherever you are. You make the decision in one tap, and the agent picks up exactly where it left off.</p>
                    <p>It's deliberately small: one app on your PC, your own Telegram bot or Slack app, and nothing in between.</p>
                </div>
                <div className="card card-pad">
                    <div className="card-title">Our promises</div>
                    <div className="trust-list">
                        {TRUST.map(t => (
                            <div key={t.title} className="trust-item">
                                <div className="trust-ico"><t.icon size={15} /></div>
                                <div><b>{t.title}</b><span>{t.text}</span></div>
                            </div>
                        ))}
                    </div>
                </div>
            </div>

            <div className="section-label">Open source</div>
            <div className="card card-pad">
                <p className="muted" style={{ marginBottom: 14 }}>Momentum is free and open source under the MIT license. You can read every line of code that runs on your PC.</p>
                <div className="link-row">
                    <button className="btn btn-secondary btn-sm" onClick={() => api.openURL(REPO_URL)}><Github size={14} /> Source code</button>
                    <button className="btn btn-secondary btn-sm" onClick={() => api.openURL(REPO_URL + '/issues/new')}><Bug size={14} /> Report a problem</button>
                    <button className="btn btn-secondary btn-sm" onClick={() => api.openURL(REPO_URL + '/blob/main/LICENSE')}><Scale size={14} /> MIT license</button>
                    <button className="btn btn-ghost btn-sm" onClick={() => api.openURL(REPO_URL)}><Heart size={14} /> Star on GitHub</button>
                </div>
            </div>
        </div>
    );
}

