import { BrandLogo } from '../lib';

export type ChannelId = 'telegram' | 'slack' | 'whatsapp';

export const CHANNELS: { id: ChannelId; name: string; note: string }[] = [
    { id: 'telegram', name: 'Telegram', note: 'Recommended · 1 minute' },
    { id: 'slack', name: 'Slack', note: 'For work · 3 min' },
    { id: 'whatsapp', name: 'WhatsApp', note: 'Needs an ngrok token' },
];

/** Segmented choice of where questions go, with each app's real logo. */
export default function ChannelPicker({ value, onChange, active }: { value: ChannelId; onChange: (c: ChannelId) => void; active?: string }) {
    return (
        <div className="channel-picker">
            {CHANNELS.map(c => (
                <button key={c.id} className={value === c.id ? 'on' : ''} onClick={() => onChange(c.id)}>
                    <BrandLogo name={c.id} size={22} />
                    <span className="grow" style={{ textAlign: 'left' }}>
                        <b>{c.name}</b>
                        <span>{active === c.id ? 'Active now' : c.note}</span>
                    </span>
                    {active === c.id && <span className="dot ok" />}
                </button>
            ))}
        </div>
    );
}
