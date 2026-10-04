import { useEffect, useState } from 'react';
import { Check, Copy, FileCode2, LoaderCircle, Plug, RefreshCw, TriangleAlert, Unplug } from 'lucide-react';
import { api, IDEStatus, IdeTile } from '../lib';

interface Props {
    compact?: boolean; // setup wizard: only IDEs found on this PC
    onChange?: () => void;
}

export default function IdeList({ compact, onChange }: Props) {
    const [ides, setIdes] = useState<IDEStatus[]>([]);
    const [busy, setBusy] = useState<string | null>(null);
    const [errors, setErrors] = useState<Record<string, string>>({});
    const [open, setOpen] = useState<{ id: string; text: string } | null>(null);
    const [copied, setCopied] = useState(false);

    const refresh = () => api.ides().then(l => { setIdes(l || []); onChange?.(); });
    useEffect(() => { refresh(); }, []);

    const act = async (ide: IDEStatus, connect: boolean) => {
        setBusy(ide.id);
        const err = connect ? await api.connectIDE(ide.id) : await api.disconnectIDE(ide.id);
        setBusy(null);
        setErrors(e => ({ ...e, [ide.id]: err }));
        if (err) setOpen({ id: ide.id, text: await api.snippet(ide.id) });
        refresh();
    };

    const connectAll = async () => {
        setBusy('all');
        const failed = await api.connectDetected();
        const byId: Record<string, string> = {};
        for (const i of ides) if (failed[i.name]) byId[i.id] = failed[i.name];
        setErrors(byId);
        setBusy(null);
        refresh();
    };

    const toggleSnippet = async (id: string) => {
        if (open?.id === id) return setOpen(null);
        setOpen({ id, text: await api.snippet(id) });
    };

    const copy = () => {
        if (!open) return;
        navigator.clipboard.writeText(open.text);
        setCopied(true);
        setTimeout(() => setCopied(false), 1400);
    };

    const found = ides.filter(i => !i.manual && (i.installed || i.connected));
    const others = ides.filter(i => !i.manual && !i.installed && !i.connected);
    const manual = ides.filter(i => i.manual);
    const pending = found.filter(i => !i.connected || i.stale).length;

    const status = (i: IDEStatus) => {
        if (errors[i.id]) return <span className="stamp bad"><TriangleAlert size={12} /> Needs manual setup</span>;
        if (i.stale) return <span className="stamp warn"><TriangleAlert size={12} /> Points to an old Momentum</span>;
        if (i.connected) return <span className="stamp ok"><Check size={12} /> Connected</span>;
        if (i.installed) return <span className="stamp dim">Found on this PC</span>;
        return null;
    };

    const item = (i: IDEStatus) => (
        <div key={i.id}>
            <div className="ide-row">
                <IdeTile id={i.id} />
                <div className="grow">
                    <div className="row" style={{ gap: 8 }}><span className="ide-name">{i.name}</span>{status(i)}</div>
                    <div className="ide-sub">{errors[i.id] || (i.manual ? i.note : i.configPath)}</div>
                </div>
                {!i.manual && (i.connected && !i.stale ? (
                    !compact && <button className="key flat sm" disabled={!!busy} onClick={() => act(i, false)}><Unplug size={14} /> Disconnect</button>
                ) : (
                    <button className="key sm" disabled={!!busy} onClick={() => act(i, true)}>
                        {busy === i.id ? <LoaderCircle size={14} className="spin" /> : i.stale ? <RefreshCw size={14} /> : <Plug size={14} />}
                        {i.stale ? 'Repair' : 'Connect'}
                    </button>
                ))}
                {!compact && (
                    <button className="key flat sm" onClick={() => toggleSnippet(i.id)} title="Show the config to paste yourself">
                        <FileCode2 size={14} /> {i.manual ? 'Get config' : 'Manual'}
                    </button>
                )}
            </div>
            {open?.id === i.id && (
                <div className="snippet">
                    <div className="row" style={{ marginBottom: 8 }}>
                        <span className="faint grow" style={{ fontSize: 12 }}>{i.configPath ? `Add to ${i.configPath}` : i.note}</span>
                        <button className="key flat sm" onClick={copy}>{copied ? <Check size={13} /> : <Copy size={13} />}{copied ? 'Copied' : 'Copy'}</button>
                    </div>
                    <pre>{open.text}</pre>
                </div>
            )}
        </div>
    );

    return (
        <div className="stack">
            <div className="plate">
                <div className="row" style={{ padding: '14px 16px', borderBottom: '1px solid var(--line)' }}>
                    <div className="grow">
                        <div className="plate-title">On this PC</div>
                        <div className="plate-sub">{found.length ? `${found.length - pending} of ${found.length} connected` : 'No supported IDEs found yet'}</div>
                    </div>
                    {pending > 0 && (
                        <button className="key go sm" onClick={connectAll} disabled={!!busy}>
                            {busy === 'all' ? <LoaderCircle size={14} className="spin" /> : <Plug size={14} />} Connect all
                        </button>
                    )}
                </div>
                <div className="ide-list">{found.map(item)}</div>
            </div>

            {!compact && others.length > 0 && (
                <>
                    <div className="cap section-label">Also supported</div>
                    <div className="plate"><div className="ide-list">{others.map(item)}</div></div>
                </>
            )}
            {!compact && (
                <>
                    <div className="cap section-label">Anything else that speaks MCP</div>
                    <div className="plate"><div className="ide-list">{manual.map(item)}</div></div>
                </>
            )}
            <div className="hint">
                Momentum adds one entry to each IDE's MCP settings and keeps everything else as it was (a backup is saved next to the file).
                Restart the IDE afterwards.
            </div>
        </div>
    );
}
