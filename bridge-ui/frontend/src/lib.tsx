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

/** The Momentum mark: a play arrow in motion, plus the green "ping" that reaches your phone.
 *  Same artwork as docs/logo.svg and the app/tray icon. Unique gradient id per instance. */
export function Logo({ size = 32, ping = true }: { size?: number; ping?: boolean }) {
    const id = useId().replace(/:/g, '');
    return (
        <svg width={size} height={size} viewBox="0 0 64 64" aria-hidden="true">
            <defs>
                <linearGradient id={`g${id}`} x1="0" y1="0" x2="1" y2="1">
                    <stop offset="0" stopColor="#7c5cff" />
                    <stop offset=".55" stopColor="#5b7cff" />
                    <stop offset="1" stopColor="#2fb6ff" />
                </linearGradient>
            </defs>
            <rect width="64" height="64" rx="15" fill={`url(#g${id})`} />
            <rect x="10" y="25" width="10" height="4.4" rx="2.2" fill="#fff" opacity=".45" />
            <rect x="6.5" y="31" width="13.5" height="4.4" rx="2.2" fill="#fff" opacity=".75" />
            <rect x="10" y="37" width="10" height="4.4" rx="2.2" fill="#fff" opacity=".45" />
            <path d="M25 20.5c0-2.5 2.7-4 4.8-2.7l17.4 10.8c2 1.3 2 4.2 0 5.5L29.8 44.9C27.7 46.2 25 44.7 25 42.2z" fill="#fff" />
            {ping && <circle cx="51" cy="13" r="5.2" fill="#3ddc97" stroke="#6a6dff" strokeWidth="2.6" />}
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
    return <div className="ide-logo"><BrandLogo name={id} size={20} /></div>;
}

export function TelegramIcon({ size = 18 }: { size?: number }) {
    return <BrandLogo name="telegram" size={size} />;
}
