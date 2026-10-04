import { Activity, clock, duration } from '../lib';

const label: Record<Activity['state'], string> = {
    answered: '', waiting: 'PAGING', expired: 'EXPIRED', stopped: 'STOPPED', failed: 'NOT SENT', canceled: 'CANCELLED',
};

/** The printed log: every page and its answer, like a receipt. */
export default function Tape({ items, limit, title = 'PRINTED LOG' }: { items: Activity[]; limit?: number; title?: string }) {
    const shown = limit ? items.slice(0, limit) : items;
    return (
        <div className="tape">
            <div className="tape-head"><span>{title}</span><span>{items.length ? `${items.length} PAGE${items.length > 1 ? 'S' : ''}` : ''}</span></div>
            {shown.length === 0 && <div className="tape-empty">No pages yet.<br />When an agent needs you, it prints here.</div>}
            {shown.map(a => (
                <div key={a.id} className="tape-row">
                    <div className="l1">
                        <span>{clock(a.created)} {(a.client || 'AGENT').toUpperCase()}</span>
                        <span className={`st-${a.state}`}>{a.state === 'answered' ? (a.answer || '').toUpperCase().slice(0, 18) : label[a.state]}</span>
                    </div>
                    <div className="q">{a.question}</div>
                    {a.state === 'answered' && duration(a.created, a.closed) && <div className="a">answered in {duration(a.created, a.closed)}{a.project ? ` · ${a.project}` : ''}</div>}
                    {a.state === 'expired' && <div className="a">no answer · agent told not to proceed</div>}
                    {a.state === 'failed' && <div className="a">couldn't reach your phone</div>}
                    {a.state === 'canceled' && <div className="a">the agent stopped waiting</div>}
                </div>
            ))}
            <div style={{ textAlign: 'center', marginTop: 10, letterSpacing: '.3em', color: '#b3ad9e' }}>* * *</div>
        </div>
    );
}
