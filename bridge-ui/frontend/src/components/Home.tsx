import { useState } from 'react';
import { Lock, Send, ServerOff, Timer, UserCheck } from 'lucide-react';
import { Activity, api, AppState, BrandLogo, Slide } from '../lib';
import { PagerDevice } from './Pager';
import Tape from './Tape';

interface Props {
    state: AppState;
    activity: Activity[];
    go: (page: 'log' | 'ides' | 'link') => void;
    toast: (msg: string) => void;
    refresh: () => void;
}

export const TRUST = [
    { icon: Lock, title: 'Nothing on your PC is exposed', text: 'Momentum only connects out to your messaging app. No open ports, tunnels or remote access.' },
    { icon: UserCheck, title: 'Only you can answer', text: 'Taps and messages from anyone else are ignored, and each page can be answered once.' },
    { icon: ServerOff, title: 'No Momentum servers', text: 'No account, no cloud, no tracking. Your settings stay in this PC’s AppData folder.' },
    { icon: Timer, title: 'Safe when you’re busy', text: 'Unanswered pages expire after 15 minutes and count as “not approved”.' },
];

const channelLabel = (s: AppState) => ({
    slack: `Slack · ${s.slackUser || 'you'}`,
    discord: `Discord · ${s.discordUser || 'you'}`,
    ntfy: 'ntfy',
    whatsapp: 'WhatsApp',
}[s.channel] || `Telegram · ${s.chatName || 'you'}`);

function today(items: Activity[]) {
    const start = new Date(); start.setHours(0, 0, 0, 0);
    const t = items.filter(a => new Date(a.created) >= start);
    const answered = t.filter(a => a.state === 'answered' && a.closed);
    const avg = answered.length
        ? Math.round(answered.reduce((s, a) => s + (new Date(a.closed!).getTime() - new Date(a.created).getTime()) / 1000, 0) / answered.length)
        : 0;
    return { count: t.length, answered: answered.length, avg };
}

export default function Home({ state, activity, go, toast, refresh }: Props) {
    const [testing, setTesting] = useState(false);
    const waiting = activity.find(a => a.state === 'waiting');
    const stats = today(activity);

    const setAway = async (away: boolean) => {
        const err = await api.setAtDesk(!away);
        toast(err || (away ? 'AWAY: pages go to your phone' : 'AT DESK: agents ask in the IDE'));
        refresh();
    };

    const test = async () => {
        setTesting(true);
        toast('Test page sent. Answer it here or on your phone');
        const res = await api.sendTest();
        setTesting(false);
        toast(res.startsWith('Error') ? res.replace(/^Error:\s*/, '') : `Got your answer: “${res}”`);
    };

    const idle = !state.configured
        ? { top: 'NOT LINKED', main: 'LINK A PHONE', sub: 'open LINK to choose Telegram, Slack, Discord or ntfy' }
        : state.atDesk
            ? { top: 'AT DESK', main: 'PAGER QUIET', sub: 'agents ask in the IDE while you’re here' }
            : { top: 'AWAY · ON CALL', main: 'NO PAGES', sub: 'all quiet · your agents can reach you anywhere' };

    return (
        <div className="home">
            <div className="home-left">
                <PagerDevice page={waiting} idle={idle} onAnswered={a => toast(`Answered “${a}” from this PC`)} />

                <div className="plate mode-plate">
                    <Slide on={!state.atDesk} onChange={setAway} left="AWAY" right="DESK" />
                    <div className="grow">
                        <div style={{ fontWeight: 700 }}>{state.atDesk ? 'At your desk' : 'Away: pages go to your phone'}</div>
                        <p className="row" style={{ gap: 6 }}>
                            {state.atDesk ? 'Flip to AWAY before you step away.' : <><BrandLogo name={state.channel || 'telegram'} size={14} /> {channelLabel(state)}</>}
                        </p>
                    </div>
                    <button className="key sm" onClick={test} disabled={testing || state.atDesk || !state.configured}><Send size={13} /> Test page</button>
                </div>

                <div className="counters">
                    <div className="plate counter"><div className="cap">Pages today</div><div className="n">{stats.count}<small>{stats.answered} answered</small></div></div>
                    <div className="plate counter"><div className="cap">Avg. answer</div><div className="n">{stats.avg ? (stats.avg < 60 ? `${stats.avg}s` : `${Math.round(stats.avg / 60)}m`) : '—'}</div></div>
                    <button className="plate counter" style={{ textAlign: 'left' }} onClick={() => go('ides')}>
                        <div className="cap">IDEs on the line</div>
                        <div className="n">{state.idesLinked}<small>of {state.idesFound || state.idesLinked}{state.idesLinked < state.idesFound ? ' · connect more' : ''}</small></div>
                    </button>
                </div>
            </div>

            <div>
                <Tape items={activity} limit={5} title="PRINTED LOG · LATEST" />
                {activity.length > 5 && <button className="key flat sm" style={{ marginTop: 8 }} onClick={() => go('log')}>Full log →</button>}
            </div>
        </div>
    );
}
