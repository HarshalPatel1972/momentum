import { useState } from 'react';
import { ArrowRight, Coffee, Lock, Plug, Send, ServerOff, ShieldCheck, Smartphone, Timer, UserCheck } from 'lucide-react';
import { Activity, api, AppState, BrandLogo, Toggle } from '../lib';
import ActivityList from './ActivityList';

interface Props {
    state: AppState;
    activity: Activity[];
    go: (page: 'activity' | 'ides' | 'settings') => void;
    toast: (msg: string) => void;
    refresh: () => void;
}

export const TRUST = [
    { icon: Lock, title: 'Nothing on your PC is exposed', text: 'Momentum only makes outgoing connections to Telegram or Slack. No open ports, tunnels or remote access.' },
    { icon: UserCheck, title: 'Only you can answer', text: 'Taps and messages from any other chat are ignored, and each question can be answered once.' },
    { icon: ServerOff, title: 'No Momentum servers', text: 'No account, no cloud, no tracking. Your settings stay in this PC’s AppData folder.' },
    { icon: Timer, title: 'Safe when you’re busy', text: 'Unanswered questions expire after 15 minutes and count as “not approved”.' },
];

export default function Home({ state, activity, go, toast, refresh }: Props) {
    const away = !state.atDesk;
    const [testing, setTesting] = useState(false);

    const setAway = async (on: boolean) => {
        const err = await api.setAtDesk(!on);
        if (err) toast(err);
        else toast(on ? 'Away mode on: questions go to your phone' : 'Away mode off: agents will ask in chat');
        refresh();
    };

    const test = async () => {
        setTesting(true);
        toast('Test question sent. Check your phone.');
        const res = await api.sendTest();
        setTesting(false);
        toast(res.startsWith('Error') ? res.replace(/^Error:\s*/, '') : `Got your answer: “${res}”`);
    };

    const channelName = state.channel === 'slack'
        ? `${state.slackUser || 'Slack'}${state.slackTeam ? ` · ${state.slackTeam}` : ''}`
        : state.channel === 'whatsapp' ? 'WhatsApp' : (state.chatName || 'Telegram');
    const channel = <><BrandLogo name={state.channel || 'telegram'} size={16} /> {channelName}</>;

    return (
        <div className="page">
            <div className="page-head">
                <div className="grow">
                    <h1>{away ? 'You’re covered.' : 'Welcome back.'}</h1>
                    <p>{away ? 'Your agents can reach you wherever you are.' : 'Your agents will ask in the IDE while you’re here.'}</p>
                </div>
            </div>

            <div className={`card away ${away ? 'on' : ''}`}>
                <div className="away-top">
                    <div className="away-ico">{away ? <Smartphone size={22} /> : <Coffee size={22} />}</div>
                    <div className="grow">
                        <h2>{away ? 'Away mode is on' : 'You’re at your desk'}</h2>
                        <p className="muted" style={{ marginTop: 3 }}>
                            {away
                                ? 'When an agent needs your OK, it goes to your phone. Tap to answer and it keeps working.'
                                : 'Agents ask their questions in the IDE as usual. Turn Away mode on before you step away.'}
                        </p>
                    </div>
                    <Toggle on={away} onChange={setAway} />
                </div>
                <div className="away-facts">
                    <div className="fact">
                        <div className="fact-label">Status</div>
                        <div className="fact-value">
                            <span className={`dot ${state.running ? 'ok live' : 'bad'}`} />
                            {state.running ? 'Running' : 'Stopped'}
                        </div>
                    </div>
                    <div className="fact">
                        <div className="fact-label">Answers from</div>
                        <div className="fact-value">{channel}</div>
                    </div>
                    <div className="fact" style={{ cursor: 'pointer' }} onClick={() => go('ides')}>
                        <div className="fact-label">IDEs connected</div>
                        <div className="fact-value">
                            <Plug size={15} /> {state.idesLinked} of {state.idesFound || state.idesLinked}
                        </div>
                    </div>
                </div>
                <div className="row">
                    <button className="btn btn-secondary btn-sm" onClick={test} disabled={testing || state.atDesk}><Send size={14} /> Send a test question</button>
                    {state.idesLinked < state.idesFound && (
                        <button className="btn btn-ghost btn-sm" onClick={() => go('ides')}><Plug size={14} /> Connect {state.idesFound - state.idesLinked} more IDE{state.idesFound - state.idesLinked > 1 ? 's' : ''}</button>
                    )}
                </div>
            </div>

            <div className="grid-2" style={{ marginTop: 16 }}>
                <div className="card card-pad">
                    <div className="row">
                        <div className="grow">
                            <div className="card-title">Recent questions</div>
                            <div className="card-sub">What your agents asked, and what you said</div>
                        </div>
                        {activity.length > 0 && <button className="btn btn-ghost btn-sm" onClick={() => go('activity')}>View all <ArrowRight size={14} /></button>}
                    </div>
                    <div style={{ marginTop: 6 }}><ActivityList items={activity} limit={4} /></div>
                </div>

                <div className="card card-pad">
                    <div className="row"><ShieldCheck size={18} color="var(--ok)" /><div className="card-title">Built to be trusted</div></div>
                    <div className="trust-list">
                        {TRUST.map(t => (
                            <div key={t.title} className="trust-item">
                                <div className="trust-ico"><t.icon size={15} /></div>
                                <div><b>{t.title}</b><span>{t.text}</span></div>
                            </div>
                        ))}
                    </div>
                </div>
            </div>
        </div>
    );
}
