import { useEffect, useRef, useState } from 'react';
import { Activity, api, since } from '../lib';

/** Live clock tick for "waiting 0:42" counters. */
export function useNow(ms = 1000) {
    const [now, setNow] = useState(Date.now());
    useEffect(() => {
        const t = setInterval(() => setNow(Date.now()), ms);
        return () => clearInterval(t);
    }, [ms]);
    return now;
}

/** Fit long questions on the LCD. */
function lcdSize(text: string) {
    return text.length > 60 ? 'small' : '';
}

const SRC = (a: Activity) => [a.client, a.project].filter(Boolean).join('·').toUpperCase() || 'AGENT';

/**
 * The pager device. With a waiting page it shows the question and answer keys
 * (Enter = first option, Esc = second, R = type a reply); otherwise an idle screen.
 * Answering here closes the question on the phone too.
 */
export function PagerDevice({ page, idle, onAnswered, compact, keyboard = true, extra }: {
    page?: Activity;
    idle: { top: string; main: string; sub?: string };
    onAnswered?: (answer: string) => void;
    compact?: boolean;
    keyboard?: boolean;
    extra?: React.ReactNode; // controls shown at the right of the device label
}) {
    const now = useNow();
    const [replying, setReplying] = useState(false);
    const [text, setText] = useState('');
    const [sent, setSent] = useState<string | null>(null);
    const input = useRef<HTMLInputElement>(null);

    useEffect(() => { setReplying(false); setText(''); setSent(null); }, [page?.id]);
    useEffect(() => { if (replying) input.current?.focus(); }, [replying]);

    const answer = async (a: string) => {
        if (!page || !a.trim()) return;
        if (await api.answer(page.id, a.trim())) {
            setSent(a.trim());
            onAnswered?.(a.trim());
        }
    };

    useEffect(() => {
        if (!keyboard || !page || sent) return;
        const onKey = (e: KeyboardEvent) => {
            if (e.ctrlKey || e.metaKey || e.altKey) return;
            if (replying) {
                if (e.key === 'Escape') setReplying(false);
                return;
            }
            const opts = page.options || [];
            if (e.key === 'Enter' && opts[0]) { e.preventDefault(); answer(opts[0]); }
            if (e.key === 'Escape' && opts[1]) { e.preventDefault(); answer(opts[1]); }
            if (e.key.toLowerCase() === 'r') { e.preventDefault(); setReplying(true); }
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [page, replying, sent, keyboard]);

    const opts = page?.options || [];

    return (
        <div className="device">
            <div className="device-label">
                <span>{page ? 'Incoming page' : 'Pager'}</span>
                <span className="live">{page && !sent ? <><span className="led on blink" /> waiting {since(page.created, now)}</> : sent ? 'sent ✓' : 'ready'}{extra}</span>
            </div>
            <div className="lcd" style={compact ? { minHeight: 0 } : { minHeight: 196 }}>
                {page ? (
                    <>
                        <div className="lcd-top"><span>{SRC(page)}</span><span>{since(page.created, now)}</span></div>
                        {sent ? (
                            <>
                                <div className="lcd-main">SENT ✓</div>
                                <div className="lcd-sub">“{sent}” · your agent carries on</div>
                            </>
                        ) : replying ? (
                            <>
                                <div className="lcd-main small">{page.question}</div>
                                <input ref={input} className="lcd-input" value={text} placeholder="type your answer…"
                                    onChange={e => setText(e.target.value)}
                                    onKeyDown={e => { if (e.key === 'Enter') answer(text); }} />
                            </>
                        ) : (
                            <>
                                <div className={`lcd-main ${lcdSize(page.question)}`}>{page.question}</div>
                                <div className="lcd-opts">{opts.slice(0, 4).map((o, i) => <span key={o} className={i === 0 ? 'sel' : ''}>{o}</span>)}</div>
                            </>
                        )}
                    </>
                ) : (
                    <>
                        <div className="lcd-top"><span>{idle.top}</span><span>{new Date(now).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false })}</span></div>
                        <div className="lcd-main">{idle.main}<span className="cursor" /></div>
                        {idle.sub && <div className="lcd-sub">{idle.sub}</div>}
                    </>
                )}
            </div>
            {page && !sent && (
                <div className="device-keys">
                    {replying ? (
                        <>
                            <button className="key go" onClick={() => answer(text)} disabled={!text.trim()}>SEND <span className="hint">ENTER</span></button>
                            <button className="key dark" onClick={() => setReplying(false)}>BACK <span className="hint">ESC</span></button>
                        </>
                    ) : (
                        <>
                            {opts[0] && <button className="key go" onClick={() => answer(opts[0])}>{opts[0]} {keyboard && <span className="hint">ENTER</span>}</button>}
                            {opts[1] && <button className="key dark" onClick={() => answer(opts[1])}>{opts[1]} {keyboard && <span className="hint">ESC</span>}</button>}
                            {!compact && opts.slice(2, 4).map(o => <button key={o} className="key dark" onClick={() => answer(o)}>{o}</button>)}
                            <button className="key dark" onClick={() => setReplying(true)}>REPLY {keyboard && <span className="hint">R</span>}</button>
                        </>
                    )}
                </div>
            )}
        </div>
    );
}
