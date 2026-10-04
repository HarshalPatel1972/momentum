// Shared types, API wrappers and small UI pieces.
import { useId } from 'react';
import * as Go from '../wailsjs/go/main/App';

export interface AppState {
    version: string;
    configured: boolean;
    problem: string;
    channel: string;
    chatName: string;
    botUsername: string;
    running: boolean;
    atDesk: boolean;
    idesLinked: number;
    idesFound: number;
    dataDir: string;
}

export interface Activity {
    id: string;
    question: string;
    options: string[];
    client: string;
    project: string;
    state: 'waiting' | 'answered' | 'expired' | 'stopped' | 'failed' | 'canceled';
    answer?: string;
    created: string;
    closed?: string;
}

export interface IDEStatus {
    id: string;
    name: string;
    installed: boolean;
    connected: boolean;
    stale: boolean;
    configPath: string;
    manual: boolean;
    note: string;
    error?: string;
}

export interface TelegramChat { id: string; name: string; bot: string; error?: string; }

// Typed views of the generated Wails bindings.
export const api = {
    state: () => Go.GetState() as Promise<AppState>,
    activity: () => Go.GetActivity() as Promise<Activity[]>,
    clearActivity: () => Go.ClearActivity(),
    ides: () => Go.ListIDEs() as Promise<IDEStatus[]>,
    connectIDE: (id: string) => Go.ConnectIDE(id),
    connectDetected: () => Go.ConnectDetectedIDEs() as Promise<Record<string, string>>,
    disconnectIDE: (id: string) => Go.DisconnectIDE(id),
    snippet: (id: string) => Go.GetIDESnippet(id),
    addRules: () => Go.AddRulesToProject(),
    detectChat: (token: string) => Go.DetectTelegramChat(token) as Promise<TelegramChat>,
    saveTelegram: (token: string, chat: TelegramChat) => Go.SaveTelegram(token, chat.id, chat.name, chat.bot),
    telegram: () => Go.GetTelegramSettings() as Promise<{ bot_token: string; chat_id: string; chat_name?: string; bot_username?: string }>,
    whatsapp: () => Go.GetWhatsAppSettings() as Promise<Record<string, string>>,
    saveWhatsApp: (key: string, phone: string, ngrok: string) => Go.SaveWhatsApp(key, phone, ngrok),
    setAtDesk: (v: boolean) => Go.SetAtDesk(v),
    sendTest: () => Go.SendTestQuestion(),
    logs: () => Go.ReadLogs() as Promise<string[]>,
    openURL: (u: string) => Go.OpenURL(u),
    openData: () => Go.OpenDataFolder(),
    quit: () => Go.QuitApp(),
};

export const REPO_URL = 'https://github.com/HarshalPatel1972/momentum';

export function timeAgo(iso?: string): string {
    if (!iso) return '';
    const s = Math.max(0, Math.floor((Date.now() - new Date(iso).getTime()) / 1000));
    if (s < 45) return 'just now';
    if (s < 3600) return `${Math.round(s / 60)}m ago`;
    if (s < 86400) return `${Math.round(s / 3600)}h ago`;
    return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

export function duration(from?: string, to?: string): string {
    if (!from || !to) return '';
    const s = Math.round((new Date(to).getTime() - new Date(from).getTime()) / 1000);
    if (s < 1) return '';
    return s < 60 ? `${s}s` : `${Math.round(s / 60)}m`;
}

/** The Momentum mark: a forward double-chevron. Unique gradient id per instance. */
export function Logo({ size = 32 }: { size?: number }) {
    const id = useId().replace(/:/g, '');
    return (
        <svg width={size} height={size} viewBox="0 0 32 32" aria-hidden="true">
            <defs>
                <linearGradient id={`g${id}`} x1="0" y1="0" x2="1" y2="1">
                    <stop offset="0" stopColor="#8b6cff" />
                    <stop offset="1" stopColor="#5b8cff" />
                </linearGradient>
            </defs>
            <rect width="32" height="32" rx="9" fill={`url(#g${id})`} />
            <path d="M8.5 21.5 14 16 8.5 10.5" stroke="#fff" strokeOpacity=".5" strokeWidth="2.7" fill="none" strokeLinecap="round" strokeLinejoin="round" />
            <path d="M15.5 21.5 21 16 15.5 10.5" stroke="#fff" strokeWidth="2.7" fill="none" strokeLinecap="round" strokeLinejoin="round" />
            <circle cx="24.5" cy="16" r="1.6" fill="#fff" />
        </svg>
    );
}

export function Brand({ size = 30 }: { size?: number }) {
    return (
        <div className="brand">
            <Logo size={size} />
            <span className="brand-name">Momentum</span>
        </div>
    );
}

export function Toggle({ on, onChange, disabled }: { on: boolean; onChange: (v: boolean) => void; disabled?: boolean }) {
    return <button className={`toggle ${on ? 'on' : ''}`} role="switch" aria-checked={on} disabled={disabled} onClick={() => onChange(!on)} />;
}

// Monogram tiles for IDEs (no third-party logos bundled).
const ideStyle: Record<string, [string, string]> = {
    'vscode': ['VS', '#1f7ad1'], 'vscode-insiders': ['VS', '#169c74'], 'cursor': ['Cu', '#2b2b33'],
    'windsurf': ['Ws', '#0c9f9a'], 'antigravity': ['Ag', '#3a6df0'], 'claude-code': ['CC', '#c96442'],
    'claude-desktop': ['Cl', '#b4593b'], 'gemini-cli': ['Ge', '#5b6cf0'], 'codex': ['Cx', '#10a37f'],
    'cline': ['Cn', '#5a5a66'], 'roo': ['Ro', '#7a4fd6'], 'kiro': ['Ki', '#8a3ffc'], 'zed': ['Ze', '#4a8cff'],
    'jetbrains': ['JB', '#e2367a'], 'other': ['{ }', '#3a3a44'],
};

export function IdeTile({ id }: { id: string }) {
    const [label, bg] = ideStyle[id] || ['•', '#3a3a44'];
    return <div className="ide-logo" style={{ background: bg }}>{label}</div>;
}

export function TelegramIcon({ size = 18 }: { size?: number }) {
    return (
        <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
            <path d="M21.4 4.1 2.9 11.3c-1.3.5-1.2 1.2-.2 1.5l4.7 1.5 1.8 5.6c.2.6.4.8.8.8.4 0 .6-.2.9-.5l2.3-2.2 4.7 3.5c.9.5 1.5.2 1.7-.8l3.1-14.5c.3-1.3-.5-1.9-1.3-1.6Zm-3.6 3.3-8.7 7.9-.3 3.5-1.5-4.9 10-6.3c.5-.3.9-.1.5.2Z" />
        </svg>
    );
}
