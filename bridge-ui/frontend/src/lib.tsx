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
    slackUser: string;
    slackTeam: string;
    discordUser: string;
    running: boolean;
    atDesk: boolean;
    popUp: boolean;
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
export interface DiscordLink { userId: string; userName: string; channelId: string; bot: string; error?: string; }
export interface SlackLink { userId: string; userName: string; channelId: string; team: string; error?: string; }

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
    slack: () => Go.GetSlackSettings() as Promise<{ bot_token: string; app_token: string; user_id: string; user_name?: string; channel_id?: string; team?: string }>,
    detectSlack: (bot: string, app: string) => Go.DetectSlackUser(bot, app) as Promise<SlackLink>,
    saveSlack: (bot: string, app: string, link: SlackLink) => Go.SaveSlack(bot, app, link as any),
    openSlackSetup: () => Go.OpenSlackAppSetup(),
    slackManifest: () => Go.GetSlackManifest(),
    discord: () => Go.GetDiscordSettings() as Promise<{ bot_token: string; user_id: string; user_name?: string; channel_id?: string }>,
    discordInvite: (token: string) => Go.OpenDiscordInvite(token),
    detectDiscord: (token: string) => Go.DetectDiscordUser(token) as Promise<DiscordLink>,
    saveDiscord: (token: string, link: DiscordLink) => Go.SaveDiscord(token, link as any),
    ntfy: () => Go.GetNtfySettings() as Promise<{ server: string; topic: string; token?: string }>,
    saveNtfy: (server: string, topic: string, token: string) => Go.SaveNtfy(server, topic, token),
    newNtfyTopic: () => Go.NewNtfyTopic(),
    setAtDesk: (v: boolean) => Go.SetAtDesk(v),
    sendTest: () => Go.SendTestQuestion(),
    logs: () => Go.ReadLogs() as Promise<string[]>,
    openURL: (u: string) => Go.OpenURL(u),
    openData: () => Go.OpenDataFolder(),
    quit: () => Go.QuitApp(),
    answer: (id: string, answer: string) => Go.AnswerQuestion(id, answer),
    setPopUp: (on: boolean) => Go.SetPopUp(on),
    hide: () => Go.HideWindow(),
    show: () => Go.ShowWindow(),
    startsMini: () => Go.StartsMini(),
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

/** The Momentum mark: a pager with a green LCD and an orange LED.
 *  Same artwork as docs/logo.svg and the app/tray icon. Unique ids per instance. */
export function Logo({ size = 32 }: { size?: number }) {
    const id = useId().replace(/:/g, '');
    return (
        <svg width={size} height={size} viewBox="0 0 64 64" aria-hidden="true">
            <defs>
                <linearGradient id={`b${id}`} x1="0" y1="0" x2=".3" y2="1"><stop offset="0" stopColor="#3a3834" /><stop offset="1" stopColor="#161513" /></linearGradient>
                <linearGradient id={`l${id}`} x1="0" y1="0" x2="1" y2="1"><stop offset="0" stopColor="#cfe58f" /><stop offset="1" stopColor="#8fae50" /></linearGradient>
                <radialGradient id={`g${id}`}><stop offset="0" stopColor="#ff5a1f" stopOpacity=".55" /><stop offset="1" stopColor="#ff5a1f" stopOpacity="0" /></radialGradient>
            </defs>
            <rect x="2" y="6" width="60" height="52" rx="14" fill={`url(#b${id})`} />
            <rect x="2.6" y="6.6" width="58.8" height="50.8" rx="13.4" fill="none" stroke="#5a5750" strokeWidth="1.2" strokeOpacity=".7" />
            <rect x="9" y="13" width="46" height="23" rx="4.5" fill={`url(#l${id})`} />
            <rect x="14" y="19" width="22" height="4" rx="1" fill="#1f2a10" />
            <rect x="14" y="26" width="14" height="4" rx="1" fill="#1f2a10" opacity=".75" />
            <rect x="31" y="26" width="5" height="4" rx="1" fill="#1f2a10" opacity=".75" />
            <circle cx="49" cy="47" r="9" fill={`url(#g${id})`} />
            <circle cx="49" cy="47" r="4.6" fill="#ff5a1f" />
            <rect x="11" y="43.5" width="10" height="7" rx="2.5" fill="#4a4842" />
            <rect x="24" y="43.5" width="10" height="7" rx="2.5" fill="#4a4842" />
        </svg>
    );
}

export function Brand({ size = 26 }: { size?: number }) {
    return (
        <div className="row" style={{ gap: 9 }}>
            <Logo size={size} />
            <span style={{ fontWeight: 700, fontSize: 16, letterSpacing: '-.02em' }}>Momentum</span>
        </div>
    );
}

/** The pager's slide switch. Left label = "on" side. */
export function Slide({ on, onChange, left = 'ON', right = 'OFF', small }: { on: boolean; onChange: (v: boolean) => void; left?: string; right?: string; small?: boolean }) {
    return (
        <div className={`slide ${small ? 'sm' : ''}`} role="switch" aria-checked={on}>
            <button className={on ? 'on' : ''} onClick={() => onChange(true)}>{left}</button>
            <button className={!on ? 'on' : ''} onClick={() => onChange(false)}>{right}</button>
            <span className={`knob ${on ? '' : 'right'}`} style={{ left: on ? 4 : '50%' }} />
        </div>
    );
}

/** Short clock time, e.g. "21:42". */
export function clock(iso?: string): string {
    if (!iso) return '';
    return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false });
}

/** "0:42" since a time. */
export function since(iso: string, now: number): string {
    const s = Math.max(0, Math.floor((now - new Date(iso).getTime()) / 1000));
    return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

// Official logos (see assets/logos/README.md for sources and licenses).
const logoUrls = import.meta.glob('./assets/logos/*.svg', { eager: true, as: 'url' }) as Record<string, string>;
export const logoUrl = (name: string) => logoUrls[`./assets/logos/${name}.svg`];

/** A brand logo (IDE, Telegram, Slack…) as a plain image. */
export function BrandLogo({ name, size = 18 }: { name: string; size?: number }) {
    const url = logoUrl(name);
    if (!url) return null;
    return <img src={url} width={size} height={size} alt="" draggable={false} style={{ display: 'block' }} />;
}

/** IDE logo on a neutral tile, so every brand sits on the same footing. */
export function IdeTile({ id }: { id: string }) {
    return <div className="ide-tile"><BrandLogo name={id} size={20} /></div>;
}

export function TelegramIcon({ size = 18 }: { size?: number }) {
    return <BrandLogo name="telegram" size={size} />;
}
