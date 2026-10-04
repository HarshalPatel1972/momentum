import { Check, Clock, Inbox, X } from 'lucide-react';
import { Activity, duration, timeAgo } from '../lib';

const icons = {
    answered: <Check size={16} />,
    waiting: <Clock size={16} />,
    expired: <X size={16} />,
    stopped: <X size={16} />,
    failed: <X size={16} />,
    canceled: <X size={16} />,
};

function outcome(a: Activity) {
    switch (a.state) {
        case 'answered': return <>You answered <b>“{a.answer}”</b>{duration(a.created, a.closed) && <span className="faint"> · in {duration(a.created, a.closed)}</span>}</>;
        case 'waiting': return <span style={{ color: 'var(--warn)' }}>Waiting for your answer on your phone</span>;
        case 'expired': return <span className="faint">Expired unanswered: the agent was told not to proceed</span>;
        case 'stopped': return <span className="faint">Momentum stopped before it was answered</span>;
        case 'failed': return <span style={{ color: 'var(--bad)' }}>Couldn't reach your phone</span>;
        case 'canceled': return <span className="faint">The agent stopped waiting (its IDE was closed)</span>;
    }
}

export default function ActivityList({ items, limit }: { items: Activity[]; limit?: number }) {
    const shown = limit ? items.slice(0, limit) : items;
    if (shown.length === 0) {
        return (
            <div className="empty">
                <div className="empty-ico"><Inbox size={24} /></div>
                <div>No questions yet</div>
                <div style={{ fontSize: 12.5, maxWidth: 280 }}>When an agent needs you, its question and your answer appear here.</div>
            </div>
        );
    }
    return (
        <div className="act-list">
            {shown.map(a => (
                <div key={a.id} className="act">
                    <div className={`act-ico ${a.state}`}>{icons[a.state]}</div>
                    <div className="grow">
                        <div className="act-q">{a.question}</div>
                        <div className="act-meta">
                            {a.client && <span className="badge accent" style={{ height: 19 }}>{a.client}</span>}
                            {a.project && <span>{a.project}</span>}
                            <span>· {timeAgo(a.created)}</span>
                        </div>
                        <div className="act-answer">{outcome(a)}</div>
                    </div>
                </div>
            ))}
        </div>
    );
}
