import { BrandLogo } from '../lib';

export type ChannelId = 'telegram' | 'slack' | 'discord' | 'ntfy' | 'whatsapp';

export const CHANNELS: { id: ChannelId; name: string; note: string }[] = [
    { id: 'telegram', name: 'Telegram', note: 'recommended · 1 min' },
    { id: 'slack', name: 'Slack', note: 'for work · 3 min' },
    { id: 'discord', name: 'Discord', note: 'devs · 3 min' },
    { id: 'ntfy', name: 'ntfy', note: 'no account' },
    { id: 'whatsapp', name: 'WhatsApp', note: 'basic · ngrok' },
];

/** Channels as cartridges you slot into the pager; the active one has a lit LED. */
export default function ChannelPicker({ value, onChange, active }: { value: ChannelId; onChange: (c: ChannelId) => void; active?: string }) {
    return (
        <div className="carts">
            {CHANNELS.map(c => (
                <button key={c.id} className={`cart ${value === c.id ? 'on' : ''}`} onClick={() => onChange(c.id)}>
                    <span className="logo"><BrandLogo name={c.id} size={20} /></span>
                    <span className="grow">
                        <b>{c.name}</b>
                        <span className="note">{active === c.id ? 'active now' : c.note}</span>
                    </span>
                    {active === c.id && <span className="led on" />}
                </button>
            ))}
        </div>
    );
}
