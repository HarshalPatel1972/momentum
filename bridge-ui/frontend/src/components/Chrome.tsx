import { Minus, Square, X } from 'lucide-react';
import { WindowMinimise, WindowToggleMaximise } from '../../wailsjs/runtime';
import { api, AppState, Logo } from '../lib';

/** The pager's own title bar: drag to move, double-click to maximise. Close hides to the tray. */
export function TitleBar({ live, children }: { live?: boolean; children?: React.ReactNode }) {
    return (
        <div className="titlebar" onDoubleClick={() => WindowToggleMaximise()}>
            <span className={`led ${live ? 'on blink' : ''}`} />
            <Logo size={20} />
            <span className="name">Momentum</span>
            <span className="model">MP-1 · AGENT PAGER</span>
            {children}
            <div className="ctl" onDoubleClick={e => e.stopPropagation()}>
                <button className="hwbtn" title="Minimise" onClick={() => WindowMinimise()}><Minus size={12} /></button>
                <button className="hwbtn" title="Maximise" onClick={() => WindowToggleMaximise()}><Square size={10} /></button>
                <button className="hwbtn x" title="Close to tray (Momentum keeps running)" onClick={() => api.hide()}><X size={12} /></button>
            </div>
        </div>
    );
}

export type Page = 'home' | 'log' | 'ides' | 'link' | 'settings' | 'about';

export const PAGES: { id: Page; label: string; key: string }[] = [
    { id: 'home', label: 'HOME', key: '1' },
    { id: 'log', label: 'LOG', key: '2' },
    { id: 'ides', label: 'IDEs', key: '3' },
    { id: 'link', label: 'LINK', key: '4' },
    { id: 'settings', label: 'SET', key: ',' },
];

/** Navigation as a column of physical keys. */
export function Rail({ page, go, waiting }: { page: Page; go: (p: Page) => void; waiting: number }) {
    return (
        <nav className="rail">
            {PAGES.map(p => (
                <button key={p.id} className={`railkey ${page === p.id ? 'on' : ''}`} onClick={() => go(p.id)} title={`${p.label} (Ctrl+${p.key})`}>
                    {p.label}
                    {p.id === 'log' && waiting > 0 && <span className="badge-dot" />}
                </button>
            ))}
            <button className={`railkey bottom ${page === 'about' ? 'on' : ''}`} onClick={() => go('about')} title="About">INFO</button>
        </nav>
    );
}

const channelName: Record<string, string> = { telegram: 'TELEGRAM', slack: 'SLACK', discord: 'DISCORD', ntfy: 'NTFY', whatsapp: 'WHATSAPP' };

export function StatusBar({ state, waiting }: { state: AppState; waiting: number }) {
    const online = state.running && state.configured;
    return (
        <div className="statusbar">
            <span className={online ? 'g' : 'o'}>● {online ? 'ONLINE' : state.configured ? 'STOPPED' : 'NOT SET UP'}</span>
            <span>{state.atDesk ? 'AT DESK' : 'AWAY'}</span>
            {state.channel && <span>{channelName[state.channel] || state.channel.toUpperCase()}</span>}
            <span>{state.idesLinked}/{state.idesFound || state.idesLinked} IDEs</span>
            <span>0 PORTS OPEN</span>
            {waiting > 0 && <span className="o">{waiting} WAITING</span>}
            <span className="spacer" />
            <span><kbd>Ctrl</kbd> <kbd>M</kbd> mini pager</span>
            <span>v{state.version}</span>
        </div>
    );
}
