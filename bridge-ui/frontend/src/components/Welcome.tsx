import { useEffect, useState } from 'react';
import { ArrowRight, Github, ShieldCheck } from 'lucide-react';
import { api, IdeTile, REPO_URL } from '../lib';

// The story, told on the pager's own screen. Each beat: [top line, main text, sub, which key is pressed]
const BEATS: { top: string; main: string; sub: string; press?: 'go'; ms: number; led?: boolean }[] = [
    { top: 'AWAY · ON CALL', main: 'ALL QUIET', sub: 'your agent is refactoring auth…', ms: 2200 },
    { top: 'CURSOR·MY-APP', main: 'PAGE!', sub: 'your agent needs a decision', ms: 1100, led: true },
    { top: 'CURSOR·MY-APP', main: 'DELETE 14 LEGACY FILES?', sub: '[APPROVE]  DENY', ms: 2300, led: true },
    { top: 'CURSOR·MY-APP', main: 'DELETE 14 LEGACY FILES?', sub: '[APPROVE]  DENY', press: 'go', ms: 700, led: true },
    { top: 'CURSOR·MY-APP', main: 'SENT ✓', sub: 'agent carries on · 128 tests passed', ms: 2600 },
];

function StoryPager() {
    const [i, setI] = useState(0);
    useEffect(() => {
        const t = setTimeout(() => setI(n => (n + 1) % BEATS.length), BEATS[i].ms);
        return () => clearTimeout(t);
    }, [i]);
    const b = BEATS[i];
    const page = i >= 1 && i <= 3;
    return (
        <div style={{ transform: i === 1 ? 'rotate(-1.2deg)' : 'none', transition: 'transform .08s' }}>
            <div className="device" style={{ padding: 18 }}>
                <div className="device-label"><span>Momentum · MP-1</span><span className="live"><span className={`led ${b.led ? 'on blink' : 'green'}`} />{page ? '1 page' : 'on call'}</span></div>
                <div className="lcd" style={{ minHeight: 210 }}>
                    <div className="lcd-top"><span>{b.top}</span><span>21:42</span></div>
                    <div className={`lcd-main ${b.main.length > 14 ? 'small' : ''}`} style={{ fontSize: b.main.length > 14 ? 26 : 38 }}>{b.main}{!page && i !== 4 && <span className="cursor" />}</div>
                    <div className="lcd-sub">{b.sub}</div>
                </div>
                <div className="device-keys">
                    <div className="key go" style={b.press ? { transform: 'translateY(3px)', boxShadow: 'inset 0 1px 3px rgba(0,0,0,.3)' } : undefined}>APPROVE</div>
                    <div className="key dark">DENY</div>
                    <div className="key dark">REPLY</div>
                </div>
            </div>
            <div className="row" style={{ justifyContent: 'center', gap: 6, marginTop: 14 }}>
                {BEATS.map((_, n) => <span key={n} style={{ width: n === i ? 22 : 8, height: 4, borderRadius: 3, background: n === i ? 'var(--orange)' : 'var(--line)', transition: 'all .25s' }} />)}
            </div>
        </div>
    );
}

const WORKS_WITH: [string, string][] = [
    ['vscode', 'VS Code'], ['cursor', 'Cursor'], ['windsurf', 'Windsurf'], ['antigravity', 'Antigravity'],
    ['claude-code', 'Claude Code'], ['codex', 'Codex'], ['gemini-cli', 'Gemini CLI'], ['kiro', 'Kiro'],
    ['zed', 'Zed'], ['other', 'Any MCP client'],
];

/**
 * The welcome story. New users see it before setup; set-up users see it once
 * after upgrading to a new major version, and any time from About → Replay intro.
 */
export default function Welcome({ onStart, returning, version }: { onStart: () => void; returning?: boolean; version?: string }) {
    return (
        <div className="welcome">
            <div>
                <div className="cap">{returning ? `Welcome to Momentum ${(version || '').split('.')[0]}.0` : 'The pager for AI coding agents'}</div>
                <h1>When your agent needs you,<br /><em>it pages your phone.</em></h1>
                <p className="lede">
                    {returning
                        ? 'New: works with every IDE, pages you on Telegram, Slack, Discord or ntfy, and lives on your desktop as this pager. Answer from your phone or right here, and the work keeps going.'
                        : 'Agents in VS Code, Cursor, Claude and others stop and wait whenever they need your OK. Momentum pages you on Telegram, Slack, Discord or ntfy. You answer with one tap, and the work keeps going.'}
                </p>
                <div className="cta">
                    <button className="key go lg" onClick={onStart}>{returning ? 'Continue to my pager' : 'Set up in 2 minutes'} <ArrowRight size={16} /></button>
                    <button className="key flat lg" onClick={() => api.openURL(REPO_URL)}><Github size={16} /> Source</button>
                </div>
                <div className="promises">
                    <span className="stamp ok"><ShieldCheck size={12} /> Free &amp; open source</span>
                    <span className="stamp ok"><ShieldCheck size={12} /> No account · no servers</span>
                    <span className="stamp ok"><ShieldCheck size={12} /> 0 ports opened</span>
                </div>
                <div className="works">
                    <div className="cap">Works with</div>
                    <div className="logos">{WORKS_WITH.map(([id, name]) => <div key={id} title={name}><IdeTile id={id} /></div>)}</div>
                </div>
            </div>
            <StoryPager />
        </div>
    );
}
